package upstream

import (
	"arecosys-dashboard/internal/adapters/analytics"
	"arecosys-dashboard/internal/domain"
	"arecosys-dashboard/internal/platform/httpx"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func Fetch(ctx context.Context, name, days string) domain.Source {
	key := "ARECOSYS_" + strings.ToUpper(name)
	address := os.Getenv(key + "_URL")
	if address == "" && name == "website" && analytics.GA4Configured() {
		return analytics.GA4Source(ctx, days)
	}
	if address == "" && name == "instagram" && analytics.InsightsConfigured() {
		return analytics.InsightsSource(ctx, days)
	}
	if address == "" {
		return domain.Source{State: "demo"}
	}
	endpoint, err := url.Parse(address)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil {
		return domain.Source{State: "error"}
	}
	query := endpoint.Query()
	query.Set("days", days)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return domain.Source{State: "error"}
	}
	if token := os.Getenv(key + "_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	var data map[string]json.RawMessage
	if err := httpx.RequestJSON(request, &data); err != nil || data == nil {
		return domain.Source{State: "error"}
	}
	return domain.Source{State: "live", Data: data}
}
