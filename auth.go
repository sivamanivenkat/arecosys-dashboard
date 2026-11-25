package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Roles: viewer can read, editor can also change leads and design posts, admin can
// additionally schedule or publish posts, send email replies and read the audit log.
var roles = []string{"viewer", "editor", "admin"}

type account struct {
	Name string `json:"name"`
	Role string `json:"role"`
	Hash string `json:"hash,omitempty"`
}

const (
	pbkdf2Rounds   = 600000
	failureLimit   = 8
	failureWindow  = 15 * time.Minute
	verifiedMemory = 5 * time.Minute
)

func b64(data []byte) string { return base64.RawStdEncoding.EncodeToString(data) }

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Rounds, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Rounds, b64(salt), b64(key)), nil
}

func verifyPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	rounds, err := strconv.Atoi(parts[1])
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[2])
	want, wantErr := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || rounds < 1000 || rounds > 2000000 || saltErr != nil || wantErr != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, rounds, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

type failure struct {
	count int
	since time.Time
}

type authority struct {
	accounts   map[string]account
	legacy     string
	trustProxy bool
	dummy      string
	mu         sync.Mutex
	failures   map[string]failure
	verified   map[[32]byte]time.Time
}

// newAuthority reads the optional users file (a JSON array of {name, role, hash}) and the
// single shared password, which signs in as the admin "arecosys".
func newAuthority(legacyPassword, usersFile string) (*authority, error) {
	a := &authority{accounts: map[string]account{}, legacy: legacyPassword, trustProxy: os.Getenv("ARECOSYS_TRUST_PROXY") == "1", failures: map[string]failure{}, verified: map[[32]byte]time.Time{}}
	if usersFile != "" {
		raw, err := os.ReadFile(usersFile)
		if err != nil {
			return nil, errors.New("users file could not be read")
		}
		var list []account
		raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")) // editors on Windows often add a byte-order mark
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, errors.New("users file is not valid JSON")
		}
		for _, acc := range list {
			if acc.Name == "" || !slices.Contains(roles, acc.Role) || strings.Count(acc.Hash, "$") != 3 || strings.ContainsAny(acc.Name, ":\r\n") {
				return nil, fmt.Errorf("users file entry %q needs a name, a role (viewer, editor or admin) and a hash", acc.Name)
			}
			a.accounts[acc.Name] = acc
		}
	}
	if a.enabled() {
		a.dummy, _ = hashPassword("unused")
	}
	return a, nil
}

func (a *authority) enabled() bool { return a.legacy != "" || len(a.accounts) > 0 }

func (a *authority) clientIP(r *http.Request) string {
	if a.trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *authority) check(name, password string) (account, bool) {
	if acc, ok := a.accounts[name]; ok {
		key := sha256.Sum256([]byte(name + "\x00" + password + "\x00" + acc.Hash))
		a.mu.Lock()
		expires, cached := a.verified[key]
		a.mu.Unlock()
		if cached && time.Now().Before(expires) {
			return acc, true
		}
		if verifyPassword(acc.Hash, password) {
			a.mu.Lock()
			a.verified[key] = time.Now().Add(verifiedMemory)
			for k, until := range a.verified {
				if time.Now().After(until) {
					delete(a.verified, k)
				}
			}
			a.mu.Unlock()
			return acc, true
		}
		return account{}, false
	}
	if a.legacy != "" && name == "arecosys" && same(password, a.legacy) {
		return account{Name: "arecosys", Role: "admin"}, true
	}
	verifyPassword(a.dummy, password) // keep unknown-user timing close to a real check
	return account{}, false
}

// require signs the request in. It answers 401 or 429 itself and returns false when the
// request must not continue. With no accounts configured everyone is a local admin.
func (a *authority) require(w http.ResponseWriter, r *http.Request, audit *auditLog) (account, bool) {
	if !a.enabled() {
		return account{Name: "local", Role: "admin"}, true
	}
	ip := a.clientIP(r)
	a.mu.Lock()
	state := a.failures[ip]
	if time.Since(state.since) > failureWindow {
		state = failure{}
	}
	a.mu.Unlock()
	if state.count >= failureLimit {
		w.Header().Set("Retry-After", strconv.Itoa(int((failureWindow-time.Since(state.since)).Seconds())+1))
		http.Error(w, "too many failed sign-ins, try again later", http.StatusTooManyRequests)
		return account{}, false
	}
	name, password, supplied := r.BasicAuth()
	if supplied {
		if acc, ok := a.check(name, password); ok {
			a.mu.Lock()
			delete(a.failures, ip)
			a.mu.Unlock()
			return acc, true
		}
		a.mu.Lock()
		if state.count == 0 {
			state.since = time.Now()
		}
		state.count++
		a.failures[ip] = state
		a.mu.Unlock()
		audit.add(auditEntry{User: name, IP: ip, Method: r.Method, Path: "sign-in", Status: http.StatusUnauthorized})
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="AR ECOSYS"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return account{}, false
}

type accountKey struct{}

func withAccount(r *http.Request, acc account) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), accountKey{}, acc))
}

func currentAccount(r *http.Request) account {
	acc, _ := r.Context().Value(accountKey{}).(account)
	return acc
}

// requireAdmin answers 403 unless the signed-in account is an admin.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if currentAccount(r).Role == "admin" {
		return true
	}
	http.Error(w, "this action needs an admin account", http.StatusForbidden)
	return false
}

type auditEntry struct {
	At     string `json:"at"`
	User   string `json:"user"`
	IP     string `json:"ip"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}

// auditLog keeps the most recent entries in memory and appends every entry to a file
// when ARECOSYS_AUDIT_FILE is set. It never records query strings or request bodies.
type auditLog struct {
	mu     sync.Mutex
	path   string
	recent []auditEntry
}

func newAuditLog(path string) *auditLog {
	l := &auditLog{path: path}
	if path == "" {
		return l
	}
	file, err := os.Open(path)
	if err != nil {
		return l
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var entry auditEntry
		if json.Unmarshal(scanner.Bytes(), &entry) == nil {
			l.recent = append(l.recent, entry)
		}
	}
	if len(l.recent) > 500 {
		l.recent = l.recent[len(l.recent)-500:]
	}
	return l
}

func (l *auditLog) add(entry auditEntry) {
	if l == nil {
		return
	}
	entry.At = time.Now().Format(time.RFC3339)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recent = append(l.recent, entry)
	if len(l.recent) > 500 {
		l.recent = l.recent[1:]
	}
	if l.path == "" {
		return
	}
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	line, _ := json.Marshal(entry)
	file.Write(append(line, '\n'))
}

func (l *auditLog) list(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	l.mu.Lock()
	out := make([]auditEntry, len(l.recent))
	for i, entry := range l.recent {
		out[len(out)-1-i] = entry
	}
	l.mu.Unlock()
	jsonResponse(w, out)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func me(w http.ResponseWriter, r *http.Request) {
	acc := currentAccount(r)
	jsonResponse(w, account{Name: acc.Name, Role: acc.Role})
}

// hashPasswordCommand implements `dashboard hash-password`: it reads a password from
// standard input and prints a hash for the users file.
func hashPasswordCommand() error {
	fmt.Fprint(os.Stderr, "Password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return errors.New("no password given")
	}
	password := strings.TrimRight(line, "\r\n")
	if len(password) < 10 {
		return errors.New("use a password of at least 10 characters")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}
