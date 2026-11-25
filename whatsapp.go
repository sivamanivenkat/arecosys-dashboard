package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type waMessage struct {
	From string `json:"from"`
	ID   string `json:"id"`
	Type string `json:"type"`
	Text struct {
		Body string `json:"body"`
	} `json:"text"`
}

type waPayload struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Contacts []struct {
					WaID    string `json:"wa_id"`
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"contacts"`
				Messages []waMessage `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

func whatsappConfigured() bool {
	return os.Getenv("ARECOSYS_WHATSAPP_VERIFY_TOKEN") != "" && os.Getenv("ARECOSYS_WHATSAPP_APP_SECRET") != ""
}

// whatsapp handles the WhatsApp Cloud API webhook. GET answers Meta's verification
// handshake. POST must carry a valid X-Hub-Signature-256 over the raw body.
func (s *leadStore) whatsapp(w http.ResponseWriter, r *http.Request) {
	if s.path == "" || !whatsappConfigured() {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		query := r.URL.Query()
		if query.Get("hub.mode") == "subscribe" && same(query.Get("hub.verify_token"), os.Getenv("ARECOSYS_WHATSAPP_VERIFY_TOKEN")) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			io.WriteString(w, query.Get("hub.challenge"))
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	mac := hmac.New(sha256.New, []byte(os.Getenv("ARECOSYS_WHATSAPP_APP_SECRET")))
	mac.Write(body)
	if !hmac.Equal([]byte(r.Header.Get("X-Hub-Signature-256")), []byte("sha256="+hex.EncodeToString(mac.Sum(nil)))) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var payload waPayload
	if json.Unmarshal(body, &payload) != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			names := map[string]string{}
			for _, contact := range change.Value.Contacts {
				names[contact.WaID] = contact.Profile.Name
			}
			for _, message := range change.Value.Messages {
				text := strings.TrimSpace(message.Text.Body)
				if message.Type != "text" || text == "" {
					text = "[" + message.Type + " message]"
				}
				if err := s.whatsappMessage(message.From, names[message.From], message.ID, text); err != nil {
					http.Error(w, "message could not be saved", http.StatusInternalServerError)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

// whatsappMessage adds a message to the sender's open lead, or opens a new lead. Meta
// retries deliveries, so a message ID that was already recorded is ignored.
func (s *leadStore) whatsappMessage(from, name, messageID, text string) error {
	phone := "+" + from
	if !phonePattern.MatchString(phone) || messageID == "" || len(messageID) > 160 {
		return nil
	}
	if len(text) > 4000 {
		text = text[:4000]
	}
	now := time.Now().Format(time.RFC3339)
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := append([]lead{}, s.rows...)
	for i, row := range rows {
		for _, entry := range row.Activity {
			if entry.Ref == messageID {
				return nil
			}
		}
		if row.Phone == phone && row.Status != "Closed" {
			rows[i].Activity = append(append([]activity{}, row.Activity...), activity{At: now, By: "WhatsApp", Text: text, Ref: messageID})
			return s.save(rows)
		}
	}
	if name == "" {
		name = phone
	}
	item := lead{ID: "wa-" + messageID, Name: name, Phone: phone, Source: "WhatsApp", Status: "New", Note: text, Created: now,
		Activity: []activity{{At: now, By: "system", Text: "Received via WhatsApp", Ref: messageID}}}
	return s.save(append([]lead{item}, rows...))
}
