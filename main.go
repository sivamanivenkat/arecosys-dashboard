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
