package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const gmailBase = "https://gmail.googleapis.com/gmail/v1/users/me/"

var gmailID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
var messageID = regexp.MustCompile(`<[^<>\s]+>`)

type gmailPart struct {
	MimeType string
	Headers  []struct{ Name, Value string }
	Body     struct{ Data string }
	Parts    []gmailPart
}

type gmailMessage struct {
	ID, ThreadID, InternalDate, Snippet string
	LabelIDs                            []string
	Payload                             gmailPart
}

type inboxMessage struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Body    string `json:"body"`
	Unread  bool   `json:"unread"`
}

func (m gmailMessage) header(name string) string {
	for _, header := range m.Payload.Headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value
		}
	}
	return ""
}

func (p gmailPart) plainText() string {
	if p.MimeType == "text/plain" && p.Body.Data != "" {
		data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(p.Body.Data, "="))
		if err == nil {
			return string(data)
		}
	}
	for _, part := range p.Parts {
		if text := part.plainText(); text != "" {
			return text
		}
	}
	return ""
}

func gmailConfigured() bool { return os.Getenv("ARECOSYS_GMAIL_REFRESH_TOKEN") != "" }

func gmailToken(ctx context.Context) (string, error) {
	form := url.Values{
		"client_id":     {os.Getenv("ARECOSYS_GMAIL_CLIENT_ID")},
		"client_secret": {os.Getenv("ARECOSYS_GMAIL_CLIENT_SECRET")},
		"refresh_token": {os.Getenv("ARECOSYS_GMAIL_REFRESH_TOKEN")},
		"grant_type":    {"refresh_token"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := requestJSON(request, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", errors.New("missing Gmail access token")
	}
	return result.AccessToken, nil
}

func gmailRequest(ctx context.Context, token, method, path, body string, result any) error {
	request, err := http.NewRequestWithContext(ctx, method, gmailBase+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return requestJSON(request, result)
}

func gmailInbox(ctx context.Context) source {
	if !gmailConfigured() {
		return source{State: "demo"}
	}
	ctx, cancel := context.WithTimeout(ctx, 28*time.Second)
	defer cancel()
	token, err := gmailToken(ctx)
	if err != nil {
		return source{State: "error"}
	}
	var result struct{ Messages []struct{ ID string } }
	if err := gmailRequest(ctx, token, http.MethodGet, "messages?labelIds=INBOX&maxResults=15", "", &result); err != nil {
		return source{State: "error"}
	}
	rows := make([]inboxMessage, len(result.Messages))
	completed := make(chan error, len(rows))
	for index, entry := range result.Messages {
		go func() {
			if !gmailID.MatchString(entry.ID) {
				completed <- errors.New("invalid Gmail message ID")
				return
			}
			var message gmailMessage
			if err := gmailRequest(ctx, token, http.MethodGet, "messages/"+entry.ID+"?format=full", "", &message); err != nil {
				completed <- err
				return
			}
			name, email := message.header("From"), ""
			if address, err := mail.ParseAddress(name); err == nil {
				name, email = address.Name, address.Address
				if name == "" {
					name = email
				}
			}
			body := message.Payload.plainText()
			if body == "" {
				body = message.Snippet
			}
			date := ""
			if millis, err := strconv.ParseInt(message.InternalDate, 10, 64); err == nil {
				date = time.UnixMilli(millis).Format("02 Jan 2006 15:04 MST")
			}
			rows[index] = inboxMessage{ID: entry.ID, Name: name, Email: email, Subject: message.header("Subject"), Date: date, Body: body, Unread: slices.Contains(message.LabelIDs, "UNREAD")}
			completed <- nil
		}()
	}
	for range rows {
		if err := <-completed; err != nil {
			return source{State: "error"}
		}
	}
	return source{State: "live", Data: rows}
}

func makeReply(message gmailMessage, from, body string) (string, error) {
	parent := messageID.FindString(message.header("Message-ID"))
	recipient := message.header("Reply-To")
	if recipient == "" {
		recipient = message.header("From")
	}
	address, err := mail.ParseAddress(recipient)
	sender, senderErr := mail.ParseAddress(from)
	if err != nil || senderErr != nil || parent == "" || !gmailID.MatchString(message.ThreadID) || !slices.Contains(message.LabelIDs, "INBOX") {
		return "", errors.New("message cannot be replied to")
	}
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(message.header("Subject"))
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	refs := messageID.FindAllString(message.header("References"), -1)
	if len(refs) > 5 {
		refs = refs[len(refs)-5:]
	}
	refs = append(refs, parent)
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	var lines strings.Builder
	for len(encoded) > 76 {
		lines.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	lines.WriteString(encoded + "\r\n")
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nIn-Reply-To: %s\r\nReferences: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s", sender.String(), address.String(), mime.QEncoding.Encode("UTF-8", subject), parent, strings.Join(refs, "\r\n "), lines.String()), nil
}

func gmailReply(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if !gmailConfigured() || !gmailID.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	var reply struct {
		Body string `json:"body"`
	}
	if !readJSON(w, r, &reply) {
		return
	}
	if reply.Body = strings.TrimSpace(reply.Body); reply.Body == "" || len(reply.Body) > 48000 {
		http.Error(w, "invalid reply", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	token, err := gmailToken(ctx)
	if err != nil {
		http.Error(w, "Gmail is unavailable", http.StatusBadGateway)
		return
	}
	var message gmailMessage
	if err := gmailRequest(ctx, token, http.MethodGet, "messages/"+id+"?format=metadata", "", &message); err != nil {
		http.Error(w, "Gmail message is unavailable", http.StatusBadGateway)
		return
	}
	var profile struct{ EmailAddress string }
	if err := gmailRequest(ctx, token, http.MethodGet, "profile", "", &profile); err != nil {
		http.Error(w, "Gmail address is unavailable", http.StatusBadGateway)
		return
	}
	mimeReply, err := makeReply(message, profile.EmailAddress, reply.Body)
	if err != nil {
		http.Error(w, "Gmail message cannot be replied to", http.StatusUnprocessableEntity)
		return
	}
	payload, _ := json.Marshal(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(mimeReply)), "threadId": message.ThreadID})
	if err := gmailRequest(ctx, token, http.MethodPost, "messages/send", string(payload), nil); err != nil {
		http.Error(w, "Gmail did not confirm the reply", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
