package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func Fetch(ctx context.Context, name, days string) Source {
	key := "ARECOSYS_" + strings.ToUpper(name)
	address := os.Getenv(key + "_URL")
	if address == "" && name == "website" && GA4Configured() {
		return GA4Source(ctx, days)
	}
	if address == "" && name == "instagram" && InsightsConfigured() {
		return InsightsSource(ctx, days)
	}
	if address == "" {
		return Source{State: "demo"}
	}
	endpoint, err := url.Parse(address)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil {
		return Source{State: "error"}
	}
	query := endpoint.Query()
	query.Set("days", days)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Source{State: "error"}
	}
	if token := os.Getenv(key + "_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	var data map[string]json.RawMessage
	if err := RequestJSON(request, &data); err != nil || data == nil {
		return Source{State: "error"}
	}
	return Source{State: "live", Data: data}
}
