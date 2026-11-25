package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		if err := HashPasswordCommand(); err != nil {
			log.Fatal(err)
		}
		return
	}
	store := &LeadStore{Path: os.Getenv("ARECOSYS_LEAD_FILE"), Rows: []Lead{}}
	if store.Path != "" {
		data, err := os.ReadFile(store.Path)
		if err == nil {
			if err := json.Unmarshal(data, &store.Rows); err != nil {
				log.Fatal("invalid lead storage file")
			}
		} else if !os.IsNotExist(err) {
			log.Fatal("lead storage file could not be read")
		}
	}
	password := os.Getenv("ARECOSYS_DASHBOARD_PASSWORD")
	auth, err := NewAuthority(password, os.Getenv("ARECOSYS_USERS_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	audit := NewAuditLog(os.Getenv("ARECOSYS_AUDIT_FILE"))
	liveConfigured := store.Path != "" || os.Getenv("ARECOSYS_MEDIA_DIR") != "" || Configured() || os.Getenv("ARECOSYS_WEBSITE_URL") != "" || GA4Configured() || InsightsConfigured() || AlertsConfigured() || WhatsAppConfigured() || os.Getenv("ARECOSYS_FLEET_URL") != "" || os.Getenv("ARECOSYS_INSTAGRAM_URL") != ""
	if liveConfigured && !auth.Enabled() {
		log.Fatal("set ARECOSYS_DASHBOARD_PASSWORD or ARECOSYS_USERS_FILE before enabling a private data source")
	}
	design, err := NewDesignStore(os.Getenv("ARECOSYS_MEDIA_DIR"))
	if err != nil {
		log.Fatal("design storage could not be opened: ", err)
	}
	if design.Dir != "" {
		go design.Run()
	}
	if AlertsConfigured() {
		go NewAlerter(store, design).Run()
	}
	app := http.NewServeMux()
	app.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		days := r.URL.Query().Get("days")
		if days != "7" && days != "30" && days != "90" {
			days = "30"
		}
		if r.URL.Query().Get("source") == "fleet" {
			JSONResponse(w, Fetch(r.Context(), "fleet", days))
			return
		}
		type result struct {
			name  string
			value Source
		}
		results := make(chan result, 4)
		for _, name := range []string{"website", "fleet", "instagram"} {
			go func() { results <- result{name, Fetch(r.Context(), name, days)} }()
		}
		go func() { results <- result{"inbox", Inbox(r.Context())} }()
		output := map[string]Source{"leads": {State: "demo"}}
		if store.Path != "" {
			store.Mu.Lock()
			output["leads"] = Source{State: "live", Data: append([]Lead{}, store.Rows...)}
			store.Mu.Unlock()
		}
		for range 4 {
			res := <-results
			output[res.name] = res.value
		}
		JSONResponse(w, output)
	})
	app.HandleFunc("PATCH /api/leads/{id}", store.Update)
	app.HandleFunc("POST /api/inbox/{id}/reply", Reply)
	app.HandleFunc("GET /api/me", Me)
	app.HandleFunc("GET /api/audit", audit.List)
	design.Routes(app)
	for _, file := range []string{"index.html", "styles.css", "design.js", "dashboard.js", "logo.png"} {
		app.HandleFunc("GET /"+file, func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, Assets, file) })
	}
	app.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, Assets, "index.html") })
	port := os.Getenv("ARECOSYS_PORT")
	if port == "" {
		port = "8765"
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		sw := &StatusWriter{ResponseWriter: w, Status: http.StatusOK}
		who := "-"
		defer func() {
			if r.Method != http.MethodGet && r.Method != http.MethodHead || sw.Status >= 400 && sw.Status != http.StatusUnauthorized {
				audit.Add(AuditEntry{User: who, IP: auth.ClientIP(r), Method: r.Method, Path: r.URL.Path, Status: sw.Status})
			}
		}()
		if r.Method == http.MethodPost && r.URL.Path == "/api/leads" {
			who = "intake"
			store.Ingest(sw, r)
			return
		}
		if r.URL.Path == "/webhooks/whatsapp" {
			who = "whatsapp"
			store.WhatsApp(sw, r)
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/public/media/") {
			design.Public(sw, r)
			return
		}
		acc, ok := auth.Require(sw, r, audit)
		if !ok {
			return
		}
		who = acc.Name
		if acc.Role == "viewer" && r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(sw, "this account is read-only", http.StatusForbidden)
			return
		}
		app.ServeHTTP(sw, WithAccount(r, acc))
	})
	host := os.Getenv("ARECOSYS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" && !auth.Enabled() {
		log.Fatal("set ARECOSYS_DASHBOARD_PASSWORD or ARECOSYS_USERS_FILE before listening on a non-loopback address")
	}
	server := &http.Server{Addr: net.JoinHostPort(host, port), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("AR ECOSYS dashboard at http://%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
