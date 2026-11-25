package main

import (
	"cmp"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

var phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 ()-]{5,19}$`)

type LeadStore struct {
	Mu   sync.Mutex
	Path string
	Rows []Lead
}

func (s *LeadStore) save(rows []Lead) error {
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.Path), ".leads-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(data)
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), s.Path); err != nil {
		return err
	}
	s.Rows = rows
	return nil
}

// addLead stores a validated lead. A repeated ID returns the stored lead unchanged, so
// retries from the website or a webhook never create duplicates.
func (s *LeadStore) addLead(item Lead) (Lead, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for _, existing := range s.Rows {
		if item.ID == existing.ID {
			return existing, nil
		}
	}
	item.Status = "New"
	item.Created = time.Now().Format(time.RFC3339)
	item.Activity = []Activity{{At: item.Created, By: "system", Text: "Received via " + item.Source}}
	if err := s.save(append([]Lead{item}, s.Rows...)); err != nil {
		return Lead{}, err
	}
	return item, nil
}

func (s *LeadStore) Ingest(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("ARECOSYS_LEAD_TOKEN")
	if s.Path == "" || secret == "" || !Same(r.Header.Get("Authorization"), "Bearer "+secret) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var item Lead
	if !ReadJSON(w, r, &item) {
		return
	}
	item.Email = strings.TrimSpace(item.Email)
	item.Phone = strings.TrimSpace(item.Phone)
	if item.Email != "" {
		if address, err := mail.ParseAddress(item.Email); err != nil || address.Address != item.Email || len(item.Email) > 254 {
			http.Error(w, "invalid contact email", http.StatusBadRequest)
			return
		}
	}
	if item.Phone != "" && !phonePattern.MatchString(item.Phone) {
		http.Error(w, "invalid phone number", http.StatusBadRequest)
		return
	}
	if item.Email == "" && item.Phone == "" || len(item.Note) > 8000 {
		http.Error(w, "invalid contact email or message", http.StatusBadRequest)
		return
	}
	for _, field := range []*string{&item.ID, &item.Name, &item.Company, &item.Interest, &item.Source, &item.Campaign} {
		*field = strings.TrimSpace(*field)
		if len(*field) > 160 {
			http.Error(w, "contact field too long", http.StatusBadRequest)
			return
		}
	}
	if item.ID == "" {
		item.ID = rand.Text()
	}
	if item.Name == "" {
		item.Name = cmp.Or(item.Email, item.Phone)
	}
	if item.Source == "" {
		item.Source = "Website"
	}
	// Server-managed fields cannot be set by the submitter.
	item.Assignee, item.Responded, item.Activity = "", "", nil
	saved, err := s.addLead(item)
	if err != nil {
		http.Error(w, "lead could not be saved", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, saved)
}

func (s *LeadStore) Update(w http.ResponseWriter, r *http.Request) {
	var item struct {
		Status   *string `json:"status"`
		Assignee *string `json:"assignee"`
		Note     *string `json:"note"`
	}
	if !ReadJSON(w, r, &item) {
		return
	}
	if item.Status == nil && item.Assignee == nil && item.Note == nil {
		http.Error(w, "nothing to update", http.StatusBadRequest)
		return
	}
	if item.Status != nil {
		switch *item.Status {
		case "New", "Contacted", "Qualified", "Closed":
		default:
			http.Error(w, "invalid stage", http.StatusBadRequest)
			return
		}
	}
	if item.Assignee != nil {
		trimmed := strings.TrimSpace(*item.Assignee)
		item.Assignee = &trimmed
		if len(trimmed) > 80 {
			http.Error(w, "assignee too long", http.StatusBadRequest)
			return
		}
	}
	if item.Note != nil {
		trimmed := strings.TrimSpace(*item.Note)
		item.Note = &trimmed
		if trimmed == "" || len(trimmed) > 1000 {
			http.Error(w, "a note must be 1 to 1000 characters", http.StatusBadRequest)
			return
		}
	}
	who := CurrentAccount(r).Name
	if who == "" {
		who = "system"
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for i, row := range s.Rows {
		if row.ID != r.PathValue("id") {
			continue
		}
		rows := append([]Lead{}, s.Rows...)
		now := time.Now().Format(time.RFC3339)
		entries := slices.Clone(row.Activity)
		if item.Status != nil && *item.Status != row.Status {
			entries = append(entries, Activity{At: now, By: who, Text: "Stage: " + row.Status + " to " + *item.Status})
		}
		if item.Status != nil {
			rows[i].Status = *item.Status
			// The first move out of New records when the lead was first answered.
			if *item.Status != "New" && rows[i].Responded == "" {
				rows[i].Responded = now
			}
		}
		if item.Assignee != nil && *item.Assignee != row.Assignee {
			text := "Assigned to " + *item.Assignee
			if *item.Assignee == "" {
				text = "Unassigned"
			}
			entries = append(entries, Activity{At: now, By: who, Text: text})
			rows[i].Assignee = *item.Assignee
		}
		if item.Note != nil {
			entries = append(entries, Activity{At: now, By: who, Text: *item.Note})
		}
		rows[i].Activity = entries
		if err := s.save(rows); err != nil {
			http.Error(w, "lead could not be saved", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.NotFound(w, r)
}
