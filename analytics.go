package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Built-in sources. When configured they replace the generic HTTPS endpoint for the
// website (Google Analytics 4) and Instagram (Insights) so no separate adapter is needed.

var (
	ga4TokenURL = "https://oauth2.googleapis.com/token"
	ga4DataURL  = "https://analyticsdata.googleapis.com/v1beta/"
	ga4Property = regexp.MustCompile(`^[0-9]{1,20}$`)
	ga4EventRe  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}$`)
)

func ga4Configured() bool {
	return ga4Property.MatchString(os.Getenv("ARECOSYS_GA4_PROPERTY_ID")) && os.Getenv("ARECOSYS_GA4_CREDENTIALS_FILE") != ""
}

func igInsightsConfigured() bool {
	return os.Getenv("ARECOSYS_INSTAGRAM_INSIGHTS") == "1" && igNumeric.MatchString(os.Getenv("ARECOSYS_INSTAGRAM_USER_ID")) && os.Getenv("ARECOSYS_INSTAGRAM_ACCESS_TOKEN") != ""
}

func rawMap(values map[string]any) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		out[key], _ = json.Marshal(value)
	}
	return out
}

type point struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

var ga4Cache struct {
	mu      sync.Mutex
	token   string
	expires time.Time
}

func b64url(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

// ga4Token exchanges the service account's signed JWT for a short-lived access token.
func ga4Token(ctx context.Context) (string, error) {
	ga4Cache.mu.Lock()
	defer ga4Cache.mu.Unlock()
	if ga4Cache.token != "" && time.Now().Before(ga4Cache.expires) {
		return ga4Cache.token, nil
	}
	raw, err := os.ReadFile(os.Getenv("ARECOSYS_GA4_CREDENTIALS_FILE"))
	if err != nil {
		return "", errors.New("credentials file could not be read")
	}
	var creds struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if json.Unmarshal(raw, &creds) != nil {
		return "", errors.New("credentials file is not valid JSON")
	}
	block, _ := pem.Decode([]byte(creds.PrivateKey))
	if block == nil || creds.ClientEmail == "" {
		return "", errors.New("credentials file has no service account key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, isRSA := parsed.(*rsa.PrivateKey)
	if err != nil || !isRSA {
		return "", errors.New("service account key is not an RSA key")
	}
	now := time.Now()
	claims, _ := json.Marshal(map[string]any{"iss": creds.ClientEmail, "scope": "https://www.googleapis.com/auth/analytics.readonly", "aud": ga4TokenURL, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	signing := b64url([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + b64url(claims)
	digest := sha256.Sum256([]byte(signing))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {signing + "." + b64url(signature)}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ga4TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := requestJSON(request, &result); err != nil || result.AccessToken == "" {
		return "", errors.New("Google rejected the service account")
	}
	ga4Cache.token = result.AccessToken
	ga4Cache.expires = now.Add(time.Duration(max(result.ExpiresIn-60, 60)) * time.Second)
	return ga4Cache.token, nil
}

type ga4Report struct {
	Rows []struct {
		DimensionValues []struct{ Value string } `json:"dimensionValues"`
		MetricValues    []struct{ Value string } `json:"metricValues"`
	} `json:"rows"`
}

func (r ga4Report) number(row, metric int) float64 {
	if row >= len(r.Rows) || metric >= len(r.Rows[row].MetricValues) {
		return 0
	}
	value, _ := strconv.ParseFloat(r.Rows[row].MetricValues[metric].Value, 64)
	return value
}

func (r ga4Report) dimension(row, index int) string {
	if row >= len(r.Rows) || index >= len(r.Rows[row].DimensionValues) {
		return ""
	}
	return r.Rows[row].DimensionValues[index].Value
}

func ga4Source(ctx context.Context, days string) source {
	token, err := ga4Token(ctx)
	if err != nil {
		return source{State: "error"}
	}
	leadEvent := os.Getenv("ARECOSYS_GA4_LEAD_EVENT")
	if leadEvent == "" {
		leadEvent = "generate_lead"
	}
	events := []string{leadEvent}
	for _, name := range strings.Split(os.Getenv("ARECOSYS_GA4_ACTION_EVENTS"), ",") {
		if name = strings.TrimSpace(name); ga4EventRe.MatchString(name) && name != leadEvent {
			events = append(events, name)
		}
	}
	if !ga4EventRe.MatchString(leadEvent) {
		return source{State: "error"}
	}
	rng := []map[string]string{{"startDate": days + "daysAgo", "endDate": "today"}}
	byDate := []map[string]any{{"dimension": map[string]string{"dimensionName": "date"}}}
	requests := []map[string]any{
		{"dateRanges": rng, "metrics": []map[string]string{{"name": "sessions"}, {"name": "screenPageViews"}}},
		{"dateRanges": rng, "dimensions": []map[string]string{{"name": "date"}}, "metrics": []map[string]string{{"name": "sessions"}}, "orderBys": byDate},
		{"dateRanges": rng, "dimensions": []map[string]string{{"name": "sessionDefaultChannelGroup"}}, "metrics": []map[string]string{{"name": "sessions"}}},
		{"dateRanges": rng, "dimensions": []map[string]string{{"name": "pagePath"}, {"name": "pageTitle"}}, "metrics": []map[string]string{{"name": "screenPageViews"}}, "orderBys": []map[string]any{{"metric": map[string]string{"metricName": "screenPageViews"}, "desc": true}}, "limit": 12},
		{"dateRanges": rng, "dimensions": []map[string]string{{"name": "date"}, {"name": "eventName"}}, "metrics": []map[string]string{{"name": "eventCount"}}, "dimensionFilter": map[string]any{"filter": map[string]any{"fieldName": "eventName", "inListFilter": map[string]any{"values": events}}}, "orderBys": byDate},
	}
	body, _ := json.Marshal(map[string]any{"requests": requests})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ga4DataURL+"properties/"+os.Getenv("ARECOSYS_GA4_PROPERTY_ID")+":batchRunReports", bytes.NewReader(body))
	if err != nil {
		return source{State: "error"}
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	var result struct {
		Reports []ga4Report `json:"reports"`
	}
	if err := requestJSON(request, &result); err != nil || len(result.Reports) != 5 {
		return source{State: "error"}
	}
	totals, daily, channelRows, pageRows, eventRows := result.Reports[0], result.Reports[1], result.Reports[2], result.Reports[3], result.Reports[4]
	label := func(date string) string {
		if parsed, err := time.Parse("20060102", date); err == nil {
			return parsed.Format("Jan 2")
		}
		return date
	}
	traffic, labels := []point{}, map[string]int{}
	for i := range daily.Rows {
		labels[daily.dimension(i, 0)] = i
		traffic = append(traffic, point{label(daily.dimension(i, 0)), daily.number(i, 0)})
	}
	inquiries, actionTotals, leads := make([]point, len(traffic)), map[string]float64{}, 0.0
	for i := range traffic {
		inquiries[i] = point{Label: traffic[i].Label}
	}
	for i := range eventRows.Rows {
		count, name := eventRows.number(i, 0), eventRows.dimension(i, 1)
		actionTotals[name] += count
		if name == leadEvent {
			leads += count
			if index, ok := labels[eventRows.dimension(i, 0)]; ok {
				inquiries[index].Value += count
			}
		}
	}
	actions := []map[string]any{}
	for _, name := range events {
		actions = append(actions, map[string]any{"name": name, "count": actionTotals[name]})
	}
	sessions := totals.number(0, 0)
	shares := map[string]float64{}
	for i := range channelRows.Rows {
		bucket := map[string]string{"Organic Search": "organic", "Direct": "direct", "Organic Social": "social", "Referral": "referral"}[channelRows.dimension(i, 0)]
		if bucket != "" && sessions > 0 {
			shares[bucket] += channelRows.number(i, 0) / sessions * 100
		}
	}
	for key, value := range shares {
		shares[key] = float64(int(value*10+.5)) / 10
	}
	pages := []map[string]any{}
	for i := range pageRows.Rows {
		name := strings.TrimSpace(pageRows.dimension(i, 1))
		if name == "" || name == "(not set)" {
			name = pageRows.dimension(i, 0)
		}
		pages = append(pages, map[string]any{"name": name, "path": pageRows.dimension(i, 0), "views": pageRows.number(i, 0)})
	}
	return source{State: "live", Data: rawMap(map[string]any{"sessions": sessions, "pageviews": totals.number(0, 1), "leads": leads, "traffic": traffic, "inquiries": inquiries, "channels": shares, "pages": pages, "actions": actions})}
}

type igSeries struct {
	Data []struct {
		Name   string `json:"name"`
		Values []struct {
			Value   float64 `json:"value"`
			EndTime string  `json:"end_time"`
		} `json:"values"`
		TotalValue struct {
			Value float64 `json:"value"`
		} `json:"total_value"`
	} `json:"data"`
}

// igInsightsSource builds the Instagram source from the Insights API. Unique reach for
// a whole period comes from the API's period total, never from summing daily reach.
func igInsightsSource(ctx context.Context, days string) source {
	count, _ := strconv.Atoi(days)
	user := os.Getenv("ARECOSYS_INSTAGRAM_USER_ID")
	end := time.Now()
	series := []point{}
	for chunkEnd := end; chunkEnd.After(end.AddDate(0, 0, -count)); {
		chunkStart := chunkEnd.AddDate(0, 0, -30)
		if limit := end.AddDate(0, 0, -count); chunkStart.Before(limit) {
			chunkStart = limit
		}
		var chunk igSeries
		values := url.Values{"metric": {"reach"}, "period": {"day"}, "since": {strconv.FormatInt(chunkStart.Unix(), 10)}, "until": {strconv.FormatInt(chunkEnd.Unix(), 10)}}
		if err := igCall(ctx, http.MethodGet, user+"/insights", values, &chunk); err != nil {
			return source{State: "error"}
		}
		part := []point{}
		for _, metric := range chunk.Data {
			for _, value := range metric.Values {
				name := value.EndTime
				if parsed, err := time.Parse("2006-01-02T15:04:05-0700", value.EndTime); err == nil {
					name = parsed.Format("Jan 2")
				}
				part = append(part, point{name, value.Value})
			}
		}
		series = append(part, series...)
		chunkEnd = chunkStart
	}
	out := map[string]any{"reach": nil, "engagements": nil, "profileVisits": nil, "followers": nil, "series": series}
	var totals igSeries
	values := url.Values{"metric": {"reach,total_interactions,profile_views"}, "metric_type": {"total_value"}, "period": {"day"}, "since": {strconv.FormatInt(end.AddDate(0, 0, -count).Unix(), 10)}, "until": {strconv.FormatInt(end.Unix(), 10)}}
	if err := igCall(ctx, http.MethodGet, user+"/insights", values, &totals); err != nil {
		if count <= 30 {
			return source{State: "error"}
		} // Long periods may be refused for period totals. Leave them empty rather than guess.
	} else {
		for _, metric := range totals.Data {
			switch metric.Name {
			case "reach":
				out["reach"] = metric.TotalValue.Value
			case "total_interactions":
				out["engagements"] = metric.TotalValue.Value
			case "profile_views":
				out["profileVisits"] = metric.TotalValue.Value
			}
		}
	}
	var profile struct {
		Followers *float64 `json:"followers_count"`
	}
	if err := igCall(ctx, http.MethodGet, user, url.Values{"fields": {"followers_count"}}, &profile); err == nil && profile.Followers != nil {
		out["followers"] = *profile.Followers
	}
	return source{State: "live", Data: rawMap(out)}
}
