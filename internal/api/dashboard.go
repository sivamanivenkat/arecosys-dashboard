// Package api holds the HTTP handlers that compose several services into one response.
package api

import (
	"arecosys-dashboard/internal/adapters/gmail"
	"arecosys-dashboard/internal/adapters/upstream"
	"arecosys-dashboard/internal/domain"
	"arecosys-dashboard/internal/platform/httpx"
	"arecosys-dashboard/internal/services/leads"
	"net/http"
)

// Dashboard returns the combined dashboard payload: leads from the store plus the
// website, fleet, Instagram and inbox sources.
func Dashboard(store *leads.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := r.URL.Query().Get("days")
		if days != "7" && days != "30" && days != "90" {
			days = "30"
		}
		if r.URL.Query().Get("source") == "fleet" {
			httpx.JSONResponse(w, upstream.Fetch(r.Context(), "fleet", days))
			return
		}
		type result struct {
			name  string
			value domain.Source
		}
		results := make(chan result, 4)
		for _, name := range []string{"website", "fleet", "instagram"} {
			go func() { results <- result{name, upstream.Fetch(r.Context(), name, days)} }()
		}
		go func() { results <- result{"inbox", gmail.Inbox(r.Context())} }()
		output := map[string]domain.Source{"leads": {State: "demo"}}
		if store.Path != "" {
			store.Mu.Lock()
			output["leads"] = domain.Source{State: "live", Data: append([]domain.Lead{}, store.Rows...)}
			store.Mu.Unlock()
		}
		for range 4 {
			res := <-results
			output[res.name] = res.value
		}
		httpx.JSONResponse(w, output)
	}
}
