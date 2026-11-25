package alerts

import (
	"arecosys-dashboard/internal/adapters/gmail"
	"arecosys-dashboard/internal/adapters/upstream"
	"arecosys-dashboard/internal/platform/httpx"
	"arecosys-dashboard/internal/services/design"
	"arecosys-dashboard/internal/services/leads"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type fleetFeature struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Metric string `json:"metric"`
	Value  any    `json:"value"`
	Status string `json:"status"`
}

// dueAt is one business day after a lead arrived, skipping weekends. It matches the
// response timer shown in the Leads view.
func dueAt(created string) (time.Time, bool) {
	due, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return time.Time{}, false
	}
	for {
		due = due.AddDate(0, 0, 1)
		if due.Weekday() != time.Saturday && due.Weekday() != time.Sunday {
			return due, true
		}
	}
}

type alertState struct {
	Seeded     bool            `json:"seeded"`
	Flagged    map[string]bool `json:"flagged"`
	LastReport string          `json:"lastReport"`
}

// Alerter watches leads and fleet status and tells the team when something needs attention.
type Alerter struct {
	store     *leads.Store
	design    *design.Store
	fleet     func(ctx context.Context) ([]fleetFeature, bool)
	number    func(ctx context.Context, source, field string) (float64, bool)
	send      func(ctx context.Context, subject, body string) error
	statePath string
	mu        sync.Mutex
	state     alertState
}

func AlertsConfigured() bool {
	return os.Getenv("ARECOSYS_ALERT_WEBHOOK_URL") != "" || (os.Getenv("ARECOSYS_ALERT_EMAIL") != "" && gmail.Configured())
}

func NewAlerter(store *leads.Store, design *design.Store) *Alerter {
	a := &Alerter{store: store, design: design, send: sendNotification}
	a.fleet = func(ctx context.Context) ([]fleetFeature, bool) {
		result := upstream.Fetch(ctx, "fleet", "30")
		data, ok := result.Data.(map[string]json.RawMessage)
		if result.State != "live" || !ok {
			return nil, false
		}
		var features []fleetFeature
		if json.Unmarshal(data["features"], &features) != nil {
			return nil, false
		}
		return features, true
	}
	a.number = func(ctx context.Context, name, field string) (float64, bool) {
		result := upstream.Fetch(ctx, name, "7")
		data, ok := result.Data.(map[string]json.RawMessage)
		if result.State != "live" || !ok {
			return 0, false
		}
		var value float64
		return value, json.Unmarshal(data[field], &value) == nil
	}
	a.state.Flagged = map[string]bool{}
	if store != nil && store.Path != "" {
		a.statePath = store.Path + ".alerts"
		if raw, err := os.ReadFile(a.statePath); err == nil {
			json.Unmarshal(raw, &a.state)
			if a.state.Flagged == nil {
				a.state.Flagged = map[string]bool{}
			}
		}
	}
	return a
}

func (a *Alerter) persist() {
	if a.statePath == "" {
		return
	}
	raw, _ := json.Marshal(a.state)
	if err := os.WriteFile(a.statePath, raw, 0o600); err != nil {
		log.Print("alert state could not be saved")
	}
}

func (a *Alerter) Run() {
	for range time.Tick(5 * time.Minute) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		a.evaluate(ctx, time.Now())
		a.weeklyReport(ctx, time.Now())
		cancel()
	}
}

// evaluate sends one message listing everything newly flagged since the last check. The
// first run only records what is already wrong, so a restart does not flood the team.
func (a *Alerter) evaluate(ctx context.Context, now time.Time) {
	current := map[string]string{}
	if a.store != nil {
		a.store.Mu.Lock()
		for _, row := range a.store.Rows {
			if due, ok := dueAt(row.Created); ok && row.Status == "New" && row.Responded == "" && now.After(due) {
				current["lead:"+row.ID] = fmt.Sprintf("Lead %s (%s) is past the one-business-day response target.", row.Name, row.Source)
			}
		}
		a.store.Mu.Unlock()
	}
	features, fleetOK := a.fleet(ctx)
	for _, feature := range features {
		if feature.Status == "Needs attention" {
			current["fleet:"+feature.ID] = fmt.Sprintf("Fleet: %s needs attention (%s: %v).", feature.Name, feature.Metric, feature.Value)
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var fresh, freshKeys []string
	for _, key := range keys {
		if !a.state.Flagged[key] {
			fresh, freshKeys = append(fresh, current[key]), append(freshKeys, key)
		}
	}
	if !a.state.Seeded {
		for _, key := range keys {
			a.state.Flagged[key] = true
		}
		a.state.Seeded = true
	} else if len(fresh) > 0 {
		if err := a.send(ctx, "AR ECOSYS: attention needed", strings.Join(fresh, "\n")); err != nil {
			log.Print("alert could not be sent")
			return // keep them unflagged so the next check retries
		}
		for _, key := range freshKeys {
			a.state.Flagged[key] = true
		}
	}
	for key := range a.state.Flagged {
		if _, still := current[key]; !still && (fleetOK || !strings.HasPrefix(key, "fleet:")) {
			delete(a.state.Flagged, key)
		}
	}
	a.persist()
}

// weeklyReport sends a plain-text summary on Monday morning, once per day.
func (a *Alerter) weeklyReport(ctx context.Context, now time.Time) {
	today := now.Format("2006-01-02")
	a.mu.Lock()
	skip := now.Weekday() != time.Monday || now.Hour() < 8 || a.state.LastReport == today
	a.mu.Unlock()
	if skip {
		return
	}
	var out strings.Builder
	fmt.Fprintf(&out, "AR ECOSYS weekly summary, week ending %s\n\n", today)
	if a.store != nil {
		a.store.Mu.Lock()
		stages := map[string]int{}
		overdue, received := 0, 0
		for _, row := range a.store.Rows {
			stages[row.Status]++
			if due, ok := dueAt(row.Created); ok && row.Status == "New" && row.Responded == "" && now.After(due) {
				overdue++
			}
			if created, err := time.Parse(time.RFC3339, row.Created); err == nil && now.Sub(created) < 7*24*time.Hour {
				received++
			}
		}
		a.store.Mu.Unlock()
		fmt.Fprintf(&out, "Leads: %d received this week. New %d, Contacted %d, Qualified %d, Closed %d. %d past the response target.\n", received, stages["New"], stages["Contacted"], stages["Qualified"], stages["Closed"], overdue)
	}
	if sessions, ok := a.number(ctx, "website", "sessions"); ok {
		fmt.Fprintf(&out, "Website: %.0f sessions in the last 7 days.\n", sessions)
	}
	if reach, ok := a.number(ctx, "instagram", "reach"); ok {
		fmt.Fprintf(&out, "Instagram: %.0f accounts reached in the last 7 days.\n", reach)
	}
	if a.design != nil && a.design.Dir != "" {
		published := 0
		a.design.Mu.Lock()
		for _, p := range a.design.Data.Posts {
			if at, err := time.Parse(time.RFC3339, p.PublishedAt); err == nil && p.Status == "Published" && now.Sub(at) < 7*24*time.Hour {
				published++
			}
		}
		a.design.Mu.Unlock()
		fmt.Fprintf(&out, "Instagram posts published this week: %d.\n", published)
	}
	if features, ok := a.fleet(ctx); ok {
		var needing []string
		for _, feature := range features {
			if feature.Status == "Needs attention" {
				needing = append(needing, feature.Name)
			}
		}
		if len(needing) > 0 {
			fmt.Fprintf(&out, "Fleet needing attention: %s.\n", strings.Join(needing, ", "))
		} else {
			out.WriteString("Fleet: every monitored feature is on track.\n")
		}
	}
	if err := a.send(ctx, "AR ECOSYS weekly summary", out.String()); err != nil {
		log.Print("weekly report could not be sent")
		return
	}
	a.mu.Lock()
	a.state.LastReport = today
	a.persist()
	a.mu.Unlock()
}

// sendNotification delivers a message by email (through the connected Gmail mailbox) and,
// when configured, to a webhook. It fails only if every configured channel fails.
func sendNotification(ctx context.Context, subject, body string) error {
	var failures, attempts int
	if to := os.Getenv("ARECOSYS_ALERT_EMAIL"); to != "" && gmail.Configured() {
		attempts++
		if err := gmailSend(ctx, to, subject, body); err != nil {
			failures++
		}
	}
	if hook := os.Getenv("ARECOSYS_ALERT_WEBHOOK_URL"); hook != "" {
		attempts++
		if err := postWebhook(ctx, hook, subject+"\n"+body); err != nil {
			failures++
		}
	}
	if attempts == 0 || failures == attempts {
		return errors.New("no notification channel accepted the message")
	}
	return nil
}

func postWebhook(ctx context.Context, address, text string) error {
	endpoint, err := url.Parse(address)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return errors.New("webhook must be an HTTPS URL")
	}
	payload, _ := json.Marshal(map[string]string{"text": text})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return httpx.RequestJSON(request, nil)
}

func gmailSend(ctx context.Context, to, subject, body string) error {
	recipient, err := mail.ParseAddress(to)
	if err != nil {
		return err
	}
	token, err := gmail.Token(ctx)
	if err != nil {
		return err
	}
	var profile struct{ EmailAddress string }
	if err := gmail.Request(ctx, token, http.MethodGet, "profile", "", &profile); err != nil {
		return err
	}
	sender, err := mail.ParseAddress(profile.EmailAddress)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	var lines strings.Builder
	for len(encoded) > 76 {
		lines.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	lines.WriteString(encoded + "\r\n")
	subject = strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	raw := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s", sender.String(), recipient.String(), mime.QEncoding.Encode("UTF-8", subject), lines.String())
	payload, _ := json.Marshal(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(raw))})
	return gmail.Request(ctx, token, http.MethodPost, "messages/send", string(payload), nil)
}
