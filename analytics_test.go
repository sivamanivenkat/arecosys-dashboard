package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGA4Source(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	creds, _ := json.Marshal(map[string]string{"client_email": "svc@example.test", "private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))})
	credsPath := filepath.Join(t.TempDir(), "creds.json")
	os.WriteFile(credsPath, creds, 0o600)
	t.Setenv("ARECOSYS_GA4_PROPERTY_ID", "123456")
	t.Setenv("ARECOSYS_GA4_CREDENTIALS_FILE", credsPath)
	t.Setenv("ARECOSYS_GA4_ACTION_EVENTS", "whatsapp_click, bad event!")

	var reportRequest map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			r.ParseForm()
			parts := strings.Split(r.PostForm.Get("assertion"), ".")
			if len(parts) != 3 {
				http.Error(w, "bad assertion", 400)
				return
			}
			signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
			digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature) != nil {
				http.Error(w, "bad signature", 400)
				return
			}
			w.Write([]byte(`{"access_token":"gat","expires_in":3600}`))
		case "/properties/123456:batchRunReports":
			if r.Header.Get("Authorization") != "Bearer gat" {
				http.Error(w, "no token", 401)
				return
			}
			json.NewDecoder(r.Body).Decode(&reportRequest)
			w.Write([]byte(`{"reports":[
 {"rows":[{"metricValues":[{"value":"1000"},{"value":"2250"}]}]},
 {"rows":[{"dimensionValues":[{"value":"20261123"}],"metricValues":[{"value":"400"}]},{"dimensionValues":[{"value":"20261124"}],"metricValues":[{"value":"600"}]}]},
 {"rows":[{"dimensionValues":[{"value":"Organic Search"}],"metricValues":[{"value":"420"}]},{"dimensionValues":[{"value":"Direct"}],"metricValues":[{"value":"300"}]},{"dimensionValues":[{"value":"Paid Search"}],"metricValues":[{"value":"100"}]}]},
 {"rows":[{"dimensionValues":[{"value":"/"},{"value":"Home"}],"metricValues":[{"value":"580"}]},{"dimensionValues":[{"value":"/erp"},{"value":"(not set)"}],"metricValues":[{"value":"320"}]}]},
 {"rows":[{"dimensionValues":[{"value":"20261123"},{"value":"generate_lead"}],"metricValues":[{"value":"3"}]},{"dimensionValues":[{"value":"20261124"},{"value":"generate_lead"}],"metricValues":[{"value":"5"}]},{"dimensionValues":[{"value":"20261124"},{"value":"whatsapp_click"}],"metricValues":[{"value":"9"}]}]}
]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	oldToken, oldData := GA4TokenURL, GA4DataURL
	GA4TokenURL, GA4DataURL = fake.URL+"/token", fake.URL+"/"
	defer func() { GA4TokenURL, GA4DataURL = oldToken, oldData }()

	result := Fetch(t.Context(), "website", "30")
	if result.State != "live" {
		t.Fatalf("state %s", result.State)
	}
	data := result.Data.(map[string]json.RawMessage)
	get := func(name string, out any) { json.Unmarshal(data[name], out) }
	var sessions, pageviews, leads float64
	get("sessions", &sessions)
	get("pageviews", &pageviews)
	get("leads", &leads)
	if sessions != 1000 || pageviews != 2250 || leads != 8 {
		t.Errorf("totals: %v %v %v", sessions, pageviews, leads)
	}
	var channels map[string]float64
	get("channels", &channels)
	if channels["organic"] != 42 || channels["direct"] != 30 || channels["social"] != 0 {
		t.Errorf("channels: %v", channels)
	}
	var inquiries []Point
	get("inquiries", &inquiries)
	if len(inquiries) != 2 || inquiries[0].Label != "Nov 23" || inquiries[0].Value != 3 || inquiries[1].Value != 5 {
		t.Errorf("inquiries: %+v", inquiries)
	}
	var pages []map[string]any
	get("pages", &pages)
	if len(pages) != 2 || pages[0]["name"] != "Home" || pages[1]["name"] != "/erp" {
		t.Errorf("pages: %v", pages)
	}
	var actions []map[string]any
	get("actions", &actions)
	if len(actions) != 2 || actions[1]["name"] != "whatsapp_click" || actions[1]["count"] != 9.0 {
		t.Errorf("actions (a malformed event name must be dropped): %v", actions)
	}
	if reportRequest == nil || len(reportRequest["requests"].([]any)) != 5 {
		t.Error("expected one batch of five reports")
	}
}

func TestInstagramInsightsSource(t *testing.T) {
	t.Setenv("ARECOSYS_INSTAGRAM_INSIGHTS", "1")
	t.Setenv("ARECOSYS_INSTAGRAM_USER_ID", "123")
	t.Setenv("ARECOSYS_INSTAGRAM_ACCESS_TOKEN", "token")
	totalsFail := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/123/insights" && q.Get("metric_type") == "total_value":
			if totalsFail {
				http.Error(w, `{"error":{"message":"range too long"}}`, 400)
				return
			}
			w.Write([]byte(`{"data":[{"name":"reach","total_value":{"value":8400}},{"name":"total_interactions","total_value":{"value":350}},{"name":"profile_views","total_value":{"value":520}}]}`))
		case r.URL.Path == "/123/insights":
			end := time.Now().Format("2006-01-02T15:04:05-0700")
			w.Write([]byte(`{"data":[{"name":"reach","values":[{"value":1200,"end_time":"` + end + `"}]}]}`))
		case r.URL.Path == "/123":
			w.Write([]byte(`{"followers_count":3264}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	old := Graph
	Graph = fake.URL + "/"
	defer func() { Graph = old }()

	read := func(days string) map[string]any {
		result := Fetch(t.Context(), "instagram", days)
		if result.State != "live" {
			t.Fatalf("days %s: state %s", days, result.State)
		}
		out := map[string]any{}
		for name, raw := range result.Data.(map[string]json.RawMessage) {
			var value any
			json.Unmarshal(raw, &value)
			out[name] = value
		}
		return out
	}
	week := read("7")
	if week["reach"] != 8400.0 || week["engagements"] != 350.0 || week["profileVisits"] != 520.0 || week["followers"] != 3264.0 {
		t.Errorf("7 days: %v", week)
	}
	if len(week["series"].([]any)) != 1 {
		t.Errorf("series: %v", week["series"])
	}
	if long := read("90"); len(long["series"].([]any)) != 3 {
		t.Errorf("90 days should be fetched in three 30-day chunks: %v", long["series"])
	}
	totalsFail = true
	if long := read("90"); long["reach"] != nil || long["engagements"] != nil {
		t.Errorf("refused period totals must stay empty, not be guessed: %v", long)
	}
	if Fetch(t.Context(), "instagram", "30").State != "error" {
		t.Error("a failing 30-day total must surface as an error")
	}
}
