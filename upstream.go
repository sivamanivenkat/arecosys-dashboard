package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

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
