package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *leadStore {
	t.Helper()
	t.Setenv("ARECOSYS_LEAD_TOKEN", "secret")
	return &leadStore{path: filepath.Join(t.TempDir(), "leads.json"), rows: []lead{}}
}

func postLead(store *leadStore, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/leads", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	store.ingest(w, r)
	return w
}

func TestIngestRequiresToken(t *testing.T) {
	store := newStore(t)
	for _, token := range []string{"", "wrong"} {
		if w := postLead(store, token, `{"email":"a@example.test"}`); w.Code != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", token, w.Code)
		}
	}
}

func TestIngestValidatesAndDeduplicates(t *testing.T) {
	store := newStore(t)
	if w := postLead(store, "secret", `{"email":"not-an-email"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad email: status %d, want 400", w.Code)
	}
	if w := postLead(store, "secret", `{"email":"a@example.test","extra":1}`); w.Code != http.StatusBadRequest {
		t.Errorf("unknown field: status %d, want 400", w.Code)
	}
	body := `{"id":"x1","email":"a@example.test","name":"Ann"}`
	for i := 0; i < 2; i++ {
		if w := postLead(store, "secret", body); w.Code != http.StatusOK {
			t.Fatalf("submit %d: status %d", i, w.Code)
		}
	}
	if len(store.rows) != 1 || store.rows[0].Status != "New" || store.rows[0].Source != "Website" {
		t.Fatalf("unexpected rows: %+v", store.rows)
	}
}

func TestUpdateStage(t *testing.T) {
	store := newStore(t)
	postLead(store, "secret", `{"id":"x1","email":"a@example.test"}`)
	patch := func(id, body string) int {
		mux := http.NewServeMux()
		mux.HandleFunc("PATCH /api/leads/{id}", store.update)
		r := httptest.NewRequest(http.MethodPatch, "/api/leads/"+id, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	if c := patch("x1", `{"status":"Qualified"}`); c != http.StatusNoContent {
		t.Fatalf("valid stage: %d", c)
	}
	if store.rows[0].Status != "Qualified" {
		t.Errorf("stage not saved: %s", store.rows[0].Status)
	}
	if store.rows[0].Responded == "" {
		t.Error("moving out of New should record the first response")
	}
	if c := patch("x1", `{"assignee":"  Meera  "}`); c != http.StatusNoContent || store.rows[0].Assignee != "Meera" {
		t.Errorf("assignee update: %d %q", c, store.rows[0].Assignee)
	}
	if c := patch("x1", `{}`); c != http.StatusBadRequest {
		t.Errorf("empty update: %d", c)
	}
	if c := patch("x1", `{"assignee":"`+strings.Repeat("a", 81)+`"}`); c != http.StatusBadRequest {
		t.Errorf("long assignee: %d", c)
	}
	if c := patch("x1", `{"status":"Bogus"}`); c != http.StatusBadRequest {
		t.Errorf("invalid stage: %d", c)
	}
	if c := patch("missing", `{"status":"New"}`); c != http.StatusNotFound {
		t.Errorf("missing lead: %d", c)
	}
}

func TestLeadTimelineAndPhone(t *testing.T) {
	store := newStore(t)
	if w := postLead(store, "secret", `{"id":"w1","phone":"+91 98765 43210","source":"WhatsApp"}`); w.Code != http.StatusOK {
		t.Fatalf("phone-only lead: %d", w.Code)
	}
	if w := postLead(store, "secret", `{"phone":"abc"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad phone: %d", w.Code)
	}
	if w := postLead(store, "secret", `{"name":"No contact"}`); w.Code != http.StatusBadRequest {
		t.Errorf("no email or phone: %d", w.Code)
	}
	if w := postLead(store, "secret", `{"id":"w2","email":"a@example.test","assignee":"Eve","responded":"2020-01-01T00:00:00Z"}`); w.Code != http.StatusOK {
		t.Errorf("lead with server fields: %d", w.Code)
	}
	if got := store.rows[0]; got.ID != "w2" || got.Assignee != "" || got.Responded != "" {
		t.Errorf("submitter must not set server-managed fields: %+v", got)
	}
	row := store.rows[1]
	if row.Name != "+91 98765 43210" || len(row.Activity) != 1 || row.Activity[0].Text != "Received via WhatsApp" {
		t.Fatalf("stored lead: %+v", row)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/leads/{id}", store.update)
	patch := func(body string) int {
		r := withAccount(httptest.NewRequest(http.MethodPatch, "/api/leads/w1", strings.NewReader(body)), account{Name: "meera", Role: "editor"})
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	if c := patch(`{"status":"Contacted","assignee":"Meera","note":"Called, sending quote"}`); c != http.StatusNoContent {
		t.Fatalf("update: %d", c)
	}
	texts := []string{}
	for _, entry := range store.rows[1].Activity {
		texts = append(texts, entry.By+": "+entry.Text)
	}
	want := []string{"system: Received via WhatsApp", "meera: Stage: New to Contacted", "meera: Assigned to Meera", "meera: Called, sending quote"}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Errorf("timeline = %v", texts)
	}
	if c := patch(`{"note":"   "}`); c != http.StatusBadRequest {
		t.Errorf("blank note: %d", c)
	}
}

func TestWhatsAppWebhook(t *testing.T) {
	store := newStore(t)
	t.Setenv("ARECOSYS_WHATSAPP_VERIFY_TOKEN", "verify")
	t.Setenv("ARECOSYS_WHATSAPP_APP_SECRET", "appsecret")
	send := func(body, signature string) int {
		r := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
		if signature == "" {
			mac := hmac.New(sha256.New, []byte("appsecret"))
			mac.Write([]byte(body))
			signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
		}
		r.Header.Set("X-Hub-Signature-256", signature)
		w := httptest.NewRecorder()
		store.whatsapp(w, r)
		return w.Code
	}
	message := func(id, text string) string {
		return `{"entry":[{"changes":[{"value":{"contacts":[{"wa_id":"919876543210","profile":{"name":"Priya"}}],"messages":[{"from":"919876543210","id":"` + id + `","type":"text","text":{"body":"` + text + `"}}]}}]}]}`
	}
	if c := send(message("m1", "Hi"), "sha256=bad"); c != http.StatusUnauthorized {
		t.Errorf("bad signature: %d", c)
	}
	if len(store.rows) != 0 {
		t.Fatal("unsigned payload created a lead")
	}
	if c := send(message("m1", "Do you cover Ward 4?"), ""); c != http.StatusOK {
		t.Fatalf("first message: %d", c)
	}
	send(message("m1", "Do you cover Ward 4?"), "") // Meta retries deliveries
	send(message("m2", "Also EV pricing please"), "")
	if len(store.rows) != 1 {
		t.Fatalf("one conversation should be one lead, got %d", len(store.rows))
	}
	row := store.rows[0]
	if row.Name != "Priya" || row.Phone != "+919876543210" || row.Source != "WhatsApp" || len(row.Activity) != 2 || row.Activity[1].Text != "Also EV pricing please" {
		t.Errorf("lead: %+v", row)
	}
	verify := httptest.NewRecorder()
	store.whatsapp(verify, httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=verify&hub.challenge=12345", nil))
	if verify.Code != http.StatusOK || verify.Body.String() != "12345" {
		t.Errorf("verification handshake: %d %q", verify.Code, verify.Body.String())
	}
	wrong := httptest.NewRecorder()
	store.whatsapp(wrong, httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=nope&hub.challenge=1", nil))
	if wrong.Code != http.StatusForbidden {
		t.Errorf("wrong verify token: %d", wrong.Code)
	}
}
