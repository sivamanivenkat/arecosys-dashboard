package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "correct horse battery") || verifyPassword(hash, "wrong") {
		t.Fatal("hash does not verify correctly")
	}
	for _, bad := range []string{"", "plain", "pbkdf2-sha256$1$a$b", "pbkdf2-sha256$99999999$AA$AA"} {
		if verifyPassword(bad, "x") {
			t.Errorf("malformed hash %q verified", bad)
		}
	}
}

func usersFile(t *testing.T) string {
	t.Helper()
	list := []account{}
	for _, name := range []string{"viewer", "editor", "admin"} {
		hash, _ := hashPassword(name + "-password")
		list = append(list, account{Name: name, Role: name, Hash: hash})
	}
	raw, _ := json.Marshal(list)
	path := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAuthorityAccounts(t *testing.T) {
	a, err := newAuthority("shared-secret", usersFile(t))
	if err != nil {
		t.Fatal(err)
	}
	if acc, ok := a.check("editor", "editor-password"); !ok || acc.Role != "editor" {
		t.Errorf("editor sign-in: %v %v", acc, ok)
	}
	if _, ok := a.check("editor", "admin-password"); ok {
		t.Error("wrong password accepted")
	}
	if acc, ok := a.check("arecosys", "shared-secret"); !ok || acc.Role != "admin" {
		t.Errorf("shared password sign-in: %v %v", acc, ok)
	}
	if _, ok := a.check("nobody", "x"); ok {
		t.Error("unknown user accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`[{"name":"x","role":"root","hash":"a$b$c$d"}]`), 0o600)
	if _, err := newAuthority("", bad); err == nil {
		t.Error("users file with an unknown role was accepted")
	}
}

func TestSignInRateLimit(t *testing.T) {
	a, _ := newAuthority("shared-secret", "")
	audit := newAuditLog("")
	attempt := func(password string) int {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "203.0.113.9:5555"
		r.SetBasicAuth("arecosys", password)
		w := httptest.NewRecorder()
		a.require(w, r, audit)
		return w.Code
	}
	for i := 0; i < failureLimit; i++ {
		if c := attempt("wrong"); c != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, c)
		}
	}
	if c := attempt("shared-secret"); c != http.StatusTooManyRequests {
		t.Errorf("locked-out client got %d, want 429", c)
	}
	other := httptest.NewRequest("GET", "/", nil)
	other.RemoteAddr = "198.51.100.1:1"
	other.SetBasicAuth("arecosys", "shared-secret")
	if _, ok := a.require(httptest.NewRecorder(), other, audit); !ok {
		t.Error("a different client must not be locked out")
	}
	if len(audit.recent) != failureLimit {
		t.Errorf("failed sign-ins audited: %d", len(audit.recent))
	}
}

func TestForwardedAddressOnlyWhenTrusted(t *testing.T) {
	a := &authority{}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.2:1"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := a.clientIP(r); got != "10.0.0.2" {
		t.Errorf("untrusted proxy header used: %s", got)
	}
	a.trustProxy = true
	if got := a.clientIP(r); got != "5.6.7.8" {
		t.Errorf("trusted proxy address: %s", got)
	}
}

func TestAuditLogPersistsAndOmitsQuery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	log := newAuditLog(path)
	log.add(auditEntry{User: "editor", Method: "PATCH", Path: "/api/leads/x", Status: 204})
	reopened := newAuditLog(path)
	if len(reopened.recent) != 1 || reopened.recent[0].User != "editor" || reopened.recent[0].At == "" {
		t.Fatalf("entries after restart: %+v", reopened.recent)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "?") {
		t.Error("audit entry contains a query string")
	}
}

func TestRolesGateActions(t *testing.T) {
	admin := httptest.NewRequest("GET", "/", nil)
	editor := withAccount(httptest.NewRequest("GET", "/", nil), account{Name: "e", Role: "editor"})
	if requireAdmin(httptest.NewRecorder(), editor) {
		t.Error("editor passed an admin check")
	}
	if !requireAdmin(httptest.NewRecorder(), withAccount(admin, account{Name: "a", Role: "admin"})) {
		t.Error("admin failed an admin check")
	}
}

func TestUsersFileWithByteOrderMark(t *testing.T) {
	hash, _ := hashPassword("some-long-password")
	raw, _ := json.Marshal([]account{{Name: "ann", Role: "editor", Hash: hash}})
	path := filepath.Join(t.TempDir(), "users.json")
	os.WriteFile(path, append([]byte("\xef\xbb\xbf"), raw...), 0o600)
	a, err := newAuthority("", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.check("ann", "some-long-password"); !ok {
		t.Error("account from a BOM-prefixed file cannot sign in")
	}
}
