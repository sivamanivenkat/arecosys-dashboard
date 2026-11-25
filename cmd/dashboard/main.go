package main

import (
	"arecosys-dashboard/internal/adapters/analytics"
	"arecosys-dashboard/internal/adapters/gmail"
	"arecosys-dashboard/internal/api"
	"arecosys-dashboard/internal/domain"
	"arecosys-dashboard/internal/security"
	"arecosys-dashboard/internal/services/alerts"
	"arecosys-dashboard/internal/services/design"
	"arecosys-dashboard/internal/services/leads"
	"arecosys-dashboard/web"
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
		if err := security.HashPasswordCommand(); err != nil {
			log.Fatal(err)
		}
		return
	}
	store := &leads.Store{Path: os.Getenv("ARECOSYS_LEAD_FILE"), Rows: []domain.Lead{}}
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
	auth, err := security.NewAuthority(password, os.Getenv("ARECOSYS_USERS_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	audit := security.NewAuditLog(os.Getenv("ARECOSYS_AUDIT_FILE"))
	liveConfigured := store.Path != "" || os.Getenv("ARECOSYS_MEDIA_DIR") != "" || gmail.Configured() || os.Getenv("ARECOSYS_WEBSITE_URL") != "" || analytics.GA4Configured() || analytics.InsightsConfigured() || alerts.AlertsConfigured() || leads.WhatsAppConfigured() || os.Getenv("ARECOSYS_FLEET_URL") != "" || os.Getenv("ARECOSYS_INSTAGRAM_URL") != ""
	if liveConfigured && !auth.Enabled() {
		log.Fatal("set ARECOSYS_DASHBOARD_PASSWORD or ARECOSYS_USERS_FILE before enabling a private data source")
	}
	design, err := design.NewStore(os.Getenv("ARECOSYS_MEDIA_DIR"))
	if err != nil {
		log.Fatal("design storage could not be opened: ", err)
	}
	if design.Dir != "" {
		go design.Run()
	}
	if alerts.AlertsConfigured() {
		go alerts.NewAlerter(store, design).Run()
	}
	app := http.NewServeMux()
	app.HandleFunc("GET /api/dashboard", api.Dashboard(store))
	app.HandleFunc("PATCH /api/leads/{id}", store.Update)
	app.HandleFunc("POST /api/inbox/{id}/reply", gmail.Reply)
	app.HandleFunc("GET /api/me", security.Me)
	app.HandleFunc("GET /api/audit", audit.List)
	design.Routes(app)
	for _, file := range []string{"index.html", "styles.css", "design.js", "dashboard.js", "logo.png"} {
		app.HandleFunc("GET /"+file, func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, web.Assets, file) })
	}
	app.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, web.Assets, "index.html") })
	port := os.Getenv("ARECOSYS_PORT")
	if port == "" {
		port = "8765"
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		sw := &security.StatusWriter{ResponseWriter: w, Status: http.StatusOK}
		who := "-"
		defer func() {
			if r.Method != http.MethodGet && r.Method != http.MethodHead || sw.Status >= 400 && sw.Status != http.StatusUnauthorized {
				audit.Add(security.AuditEntry{User: who, IP: auth.ClientIP(r), Method: r.Method, Path: r.URL.Path, Status: sw.Status})
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
		app.ServeHTTP(sw, security.WithAccount(r, acc))
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
