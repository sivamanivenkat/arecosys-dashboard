package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func testAlerter(t *testing.T) (*Alerter, *[]string) {
	t.Helper()
	store := newStore(t)
	sent := []string{}
	a := NewAlerter(store, nil)
	a.send = func(_ context.Context, subject, body string) error {
		sent = append(sent, subject+": "+body)
		return nil
	}
	a.fleet = func(context.Context) ([]fleetFeature, bool) { return nil, true }
	a.number = func(context.Context, string, string) (float64, bool) { return 0, false }
	return a, &sent
}

func TestDueAtSkipsWeekends(t *testing.T) {
	friday := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC) // a Friday
	due, ok := dueAt(friday.Format(time.RFC3339))
	if !ok || due.Weekday() != time.Monday || due.Day() != 23 {
		t.Errorf("due = %v", due)
	}
	if _, ok := dueAt("garbage"); ok {
		t.Error("invalid timestamp accepted")
	}
}

func TestAlertsSeedThenReportOnlyNewProblems(t *testing.T) {
	a, sent := testAlerter(t)
	old := time.Now().AddDate(0, 0, -10).Format(time.RFC3339)
	a.store.Rows = []Lead{{ID: "a", Name: "Old lead", Source: "Website", Status: "New", Created: old}}
	ctx := context.Background()
	a.evaluate(ctx, time.Now())
	if len(*sent) != 0 {
		t.Fatalf("first run must only record existing problems: %v", *sent)
	}
	a.evaluate(ctx, time.Now())
	if len(*sent) != 0 {
		t.Fatal("known problem was reported again")
	}
	a.store.Rows = append(a.store.Rows, Lead{ID: "b", Name: "Second", Source: "Email", Status: "New", Created: old})
	a.fleet = func(context.Context) ([]fleetFeature, bool) {
		return []fleetFeature{{ID: "gps", Name: "Live GPS telemetry", Metric: "Vehicles reporting", Value: "46 / 48", Status: "Needs attention"}}, true
	}
	a.evaluate(ctx, time.Now())
	if len(*sent) != 1 || !strings.Contains((*sent)[0], "Second") || !strings.Contains((*sent)[0], "Live GPS telemetry") || strings.Contains((*sent)[0], "Old lead") {
		t.Fatalf("alert = %v", *sent)
	}
	a.evaluate(ctx, time.Now())
	if len(*sent) != 1 {
		t.Error("alert repeated")
	}
	// The GPS problem clears, then returns: it must alert again.
	a.fleet = func(context.Context) ([]fleetFeature, bool) { return nil, true }
	a.evaluate(ctx, time.Now())
	a.fleet = func(context.Context) ([]fleetFeature, bool) {
		return []fleetFeature{{ID: "gps", Name: "Live GPS telemetry", Status: "Needs attention"}}, true
	}
	a.evaluate(ctx, time.Now())
	if len(*sent) != 2 {
		t.Errorf("recurring problem should alert again: %v", *sent)
	}
}

func TestFailedSendIsRetriedAndFleetOutageDoesNotResetState(t *testing.T) {
	a, sent := testAlerter(t)
	ctx := context.Background()
	a.evaluate(ctx, time.Now()) // seed
	attempts := 0
	a.send = func(context.Context, string, string) error {
		attempts++
		if attempts == 1 {
			return errors.New("down")
		}
		*sent = append(*sent, "ok")
		return nil
	}
	a.fleet = func(context.Context) ([]fleetFeature, bool) {
		return []fleetFeature{{ID: "fuel", Name: "Diesel", Status: "Needs attention"}}, true
	}
	a.evaluate(ctx, time.Now())
	a.evaluate(ctx, time.Now())
	if attempts != 2 || len(*sent) != 1 {
		t.Fatalf("attempts %d sent %v", attempts, *sent)
	}
	a.fleet = func(context.Context) ([]fleetFeature, bool) { return nil, false } // source outage
	a.evaluate(ctx, time.Now())
	a.fleet = func(context.Context) ([]fleetFeature, bool) {
		return []fleetFeature{{ID: "fuel", Name: "Diesel", Status: "Needs attention"}}, true
	}
	a.evaluate(ctx, time.Now())
	if attempts != 2 {
		t.Error("an outage of the fleet source must not re-trigger a known alert")
	}
}

func TestWeeklyReportOncePerMonday(t *testing.T) {
	a, sent := testAlerter(t)
	a.number = func(_ context.Context, source, _ string) (float64, bool) {
		return map[string]float64{"website": 1200, "instagram": 3400}[source], true
	}
	a.store.Rows = []Lead{{ID: "a", Name: "Ann", Status: "New", Created: time.Now().Format(time.RFC3339)}}
	ctx := context.Background()
	monday := time.Date(2026, 11, 23, 9, 0, 0, 0, time.Local)
	a.weeklyReport(ctx, monday.Add(-24*time.Hour)) // Sunday
	a.weeklyReport(ctx, monday.Add(-2*time.Hour))  // Monday before 08:00
	if len(*sent) != 0 {
		t.Fatalf("report sent at the wrong time: %v", *sent)
	}
	a.weeklyReport(ctx, monday)
	a.weeklyReport(ctx, monday.Add(time.Hour))
	if len(*sent) != 1 || !strings.Contains((*sent)[0], "1200 sessions") || !strings.Contains((*sent)[0], "3400 accounts") || !strings.Contains((*sent)[0], "1 received this week") {
		t.Fatalf("report = %v", *sent)
	}
}

func TestWebhookMustBeHTTPS(t *testing.T) {
	if err := postWebhook(context.Background(), "http://hooks.example.test/x", "hi"); err == nil {
		t.Error("plain HTTP webhook accepted")
	}
}
