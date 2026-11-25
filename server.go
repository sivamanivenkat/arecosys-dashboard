package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

//go:embed index.html styles.css design.js dashboard.js logo.png
var assets embed.FS

var client = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type source struct {
	State string `json:"state"`
	Data  any    `json:"data,omitempty"`
}

type lead struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Company   string     `json:"company"`
	Interest  string     `json:"interest"`
	Source    string     `json:"source"`
	Status    string     `json:"status"`
	Note      string     `json:"note"`
	Campaign  string     `json:"campaign"`
	Created   string     `json:"created"`
	Assignee  string     `json:"assignee"`
	Responded string     `json:"responded"`
	Phone     string     `json:"phone"`
	Activity  []activity `json:"activity"`
}

// activity is one entry in a lead's timeline: a note or an automatic change record.
type activity struct {
	At   string `json:"at"`
	By   string `json:"by"`
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
}

var phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 ()-]{5,19}$`)

type leadStore struct {
	mu   sync.Mutex
	path string
	rows []lead
}

func same(a, b string) bool {
	x, y := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}

func readJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType != "application/json" {
		http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return false
	}
	return true
}

func jsonResponse(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Print("response encoding failed")
	}
}

func (s *leadStore) save(rows []lead) error {
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".leads-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(data)
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return err
	}
	s.rows = rows
	return nil
}

// addLead stores a validated lead. A repeated ID returns the stored lead unchanged, so
// retries from the website or a webhook never create duplicates.
func (s *leadStore) addLead(item lead) (lead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.rows {
		if item.ID == existing.ID {
			return existing, nil
		}
	}
	item.Status = "New"
	item.Created = time.Now().Format(time.RFC3339)
	item.Activity = []activity{{At: item.Created, By: "system", Text: "Received via " + item.Source}}
	if err := s.save(append([]lead{item}, s.rows...)); err != nil {
		return lead{}, err
	}
	return item, nil
}

func (s *leadStore) ingest(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("ARECOSYS_LEAD_TOKEN")
	if s.path == "" || secret == "" || !same(r.Header.Get("Authorization"), "Bearer "+secret) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var item lead
	if !readJSON(w, r, &item) {
		return
	}
	item.Email = strings.TrimSpace(item.Email)
	item.Phone = strings.TrimSpace(item.Phone)
	if item.Email != "" {
		if address, err := mail.ParseAddress(item.Email); err != nil || address.Address != item.Email || len(item.Email) > 254 {
			http.Error(w, "invalid contact email", http.StatusBadRequest)
			return
		}
	}
	if item.Phone != "" && !phonePattern.MatchString(item.Phone) {
		http.Error(w, "invalid phone number", http.StatusBadRequest)
		return
	}
	if item.Email == "" && item.Phone == "" || len(item.Note) > 8000 {
		http.Error(w, "invalid contact email or message", http.StatusBadRequest)
		return
	}
	for _, field := range []*string{&item.ID, &item.Name, &item.Company, &item.Interest, &item.Source, &item.Campaign} {
		*field = strings.TrimSpace(*field)
		if len(*field) > 160 {
			http.Error(w, "contact field too long", http.StatusBadRequest)
			return
		}
	}
	if item.ID == "" {
		item.ID = rand.Text()
	}
	if item.Name == "" {
		item.Name = cmp.Or(item.Email, item.Phone)
	}
	if item.Source == "" {
		item.Source = "Website"
	}
	// Server-managed fields cannot be set by the submitter.
	item.Assignee, item.Responded, item.Activity = "", "", nil
	saved, err := s.addLead(item)
	if err != nil {
		http.Error(w, "lead could not be saved", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, saved)
}

func (s *leadStore) update(w http.ResponseWriter, r *http.Request) {
	var item struct {
		Status   *string `json:"status"`
		Assignee *string `json:"assignee"`
		Note     *string `json:"note"`
	}
	if !readJSON(w, r, &item) {
		return
	}
	if item.Status == nil && item.Assignee == nil && item.Note == nil {
		http.Error(w, "nothing to update", http.StatusBadRequest)
		return
	}
	if item.Status != nil {
		switch *item.Status {
		case "New", "Contacted", "Qualified", "Closed":
		default:
			http.Error(w, "invalid stage", http.StatusBadRequest)
			return
		}
	}
	if item.Assignee != nil {
		trimmed := strings.TrimSpace(*item.Assignee)
		item.Assignee = &trimmed
		if len(trimmed) > 80 {
			http.Error(w, "assignee too long", http.StatusBadRequest)
			return
		}
	}
	if item.Note != nil {
		trimmed := strings.TrimSpace(*item.Note)
		item.Note = &trimmed
		if trimmed == "" || len(trimmed) > 1000 {
			http.Error(w, "a note must be 1 to 1000 characters", http.StatusBadRequest)
			return
		}
	}
	who := currentAccount(r).Name
	if who == "" {
		who = "system"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, row := range s.rows {
		if row.ID != r.PathValue("id") {
			continue
		}
		rows := append([]lead{}, s.rows...)
		now := time.Now().Format(time.RFC3339)
		entries := slices.Clone(row.Activity)
		if item.Status != nil && *item.Status != row.Status {
			entries = append(entries, activity{At: now, By: who, Text: "Stage: " + row.Status + " to " + *item.Status})
		}
		if item.Status != nil {
			rows[i].Status = *item.Status
			// The first move out of New records when the lead was first answered.
			if *item.Status != "New" && rows[i].Responded == "" {
				rows[i].Responded = now
			}
		}
		if item.Assignee != nil && *item.Assignee != row.Assignee {
			text := "Assigned to " + *item.Assignee
			if *item.Assignee == "" {
				text = "Unassigned"
			}
			entries = append(entries, activity{At: now, By: who, Text: text})
			rows[i].Assignee = *item.Assignee
		}
		if item.Note != nil {
			entries = append(entries, activity{At: now, By: who, Text: *item.Note})
		}
		rows[i].Activity = entries
		if err := s.save(rows); err != nil {
			http.Error(w, "lead could not be saved", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.NotFound(w, r)
}

func upstream(ctx context.Context, name, days string) source {
	key := "ARECOSYS_" + strings.ToUpper(name)
	address := os.Getenv(key + "_URL")
	if address == "" && name == "website" && ga4Configured() {
		return ga4Source(ctx, days)
	}
	if address == "" && name == "instagram" && igInsightsConfigured() {
		return igInsightsSource(ctx, days)
	}
	if address == "" {
		return source{State: "demo"}
	}
	endpoint, err := url.Parse(address)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil {
		return source{State: "error"}
	}
	query := endpoint.Query()
	query.Set("days", days)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return source{State: "error"}
	}
	if token := os.Getenv(key + "_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	var data map[string]json.RawMessage
	if err := requestJSON(request, &data); err != nil || data == nil {
		return source{State: "error"}
	}
	return source{State: "live", Data: data}
}

func requestJSON(request *http.Request, value any) error {
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("upstream status %d", response.StatusCode)
	}
	if value == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(value)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		if err := hashPasswordCommand(); err != nil {
			log.Fatal(err)
		}
		return
	}
	store := &leadStore{path: os.Getenv("ARECOSYS_LEAD_FILE"), rows: []lead{}}
	if store.path != "" {
		data, err := os.ReadFile(store.path)
		if err == nil {
			if err := json.Unmarshal(data, &store.rows); err != nil {
				log.Fatal("invalid lead storage file")
			}
		} else if !os.IsNotExist(err) {
			log.Fatal("lead storage file could not be read")
		}
	}
	password := os.Getenv("ARECOSYS_DASHBOARD_PASSWORD")
	auth, err := newAuthority(password, os.Getenv("ARECOSYS_USERS_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	audit := newAuditLog(os.Getenv("ARECOSYS_AUDIT_FILE"))
	liveConfigured := store.path != "" || os.Getenv("ARECOSYS_MEDIA_DIR") != "" || gmailConfigured() || os.Getenv("ARECOSYS_WEBSITE_URL") != "" || ga4Configured() || igInsightsConfigured() || alertsConfigured() || whatsappConfigured() || os.Getenv("ARECOSYS_FLEET_URL") != "" || os.Getenv("ARECOSYS_INSTAGRAM_URL") != ""
	if liveConfigured && !auth.enabled() {
		log.Fatal("set ARECOSYS_DASHBOARD_PASSWORD or ARECOSYS_USERS_FILE before enabling a private data source")
	}
	design, err := newDesignStore(os.Getenv("ARECOSYS_MEDIA_DIR"))
	if err != nil {
		log.Fatal("design storage could not be opened: ", err)
	}
	if design.dir != "" {
		go design.run()
	}
	if alertsConfigured() {
		go newAlerter(store, design).run()
	}
	app := http.NewServeMux()
	app.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		days := r.URL.Query().Get("days")
		if days != "7" && days != "30" && days != "90" {
			days = "30"
		}
		if r.URL.Query().Get("source") == "fleet" {
			jsonResponse(w, upstream(r.Context(), "fleet", days))
			return
		}
		type result struct {
			name  string
			value source
		}
		results := make(chan result, 4)
		for _, name := range []string{"website", "fleet", "instagram"} {
			go func() { results <- result{name, upstream(r.Context(), name, days)} }()
		}
		go func() { results <- result{"inbox", gmailInbox(r.Context())} }()
		output := map[string]source{"leads": {State: "demo"}}
		if store.path != "" {
			store.mu.Lock()
			output["leads"] = source{State: "live", Data: append([]lead{}, store.rows...)}
			store.mu.Unlock()
		}
		for range 4 {
			res := <-results
			output[res.name] = res.value
		}
		jsonResponse(w, output)
	})
	app.HandleFunc("PATCH /api/leads/{id}", store.update)
	app.HandleFunc("POST /api/inbox/{id}/reply", gmailReply)
	app.HandleFunc("GET /api/me", me)
	app.HandleFunc("GET /api/audit", audit.list)
	design.routes(app)
	for _, file := range []string{"index.html", "styles.css", "design.js", "dashboard.js", "logo.png"} {
		app.HandleFunc("GET /"+file, func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, assets, file) })
	}
	app.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, assets, "index.html") })
	port := os.Getenv("ARECOSYS_PORT")
	if port == "" {
		port = "8765"
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		who := "-"
		defer func() {
			if r.Method != http.MethodGet && r.Method != http.MethodHead || sw.status >= 400 && sw.status != http.StatusUnauthorized {
				audit.add(auditEntry{User: who, IP: auth.clientIP(r), Method: r.Method, Path: r.URL.Path, Status: sw.status})
			}
		}()
		if r.Method == http.MethodPost && r.URL.Path == "/api/leads" {
			who = "intake"
			store.ingest(sw, r)
			return
		}
		if r.URL.Path == "/webhooks/whatsapp" {
			who = "whatsapp"
			store.whatsapp(sw, r)
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/public/media/") {
			design.public(sw, r)
			return
		}
		acc, ok := auth.require(sw, r, audit)
		if !ok {
			return
		}
		who = acc.Name
		if acc.Role == "viewer" && r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(sw, "this account is read-only", http.StatusForbidden)
			return
		}
		app.ServeHTTP(sw, withAccount(r, acc))
	})
	host := os.Getenv("ARECOSYS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" && !auth.enabled() {
		log.Fatal("set ARECOSYS_DASHBOARD_PASSWORD or ARECOSYS_USERS_FILE before listening on a non-loopback address")
	}
	server := &http.Server{Addr: net.JoinHostPort(host, port), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("AR ECOSYS dashboard at http://%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
