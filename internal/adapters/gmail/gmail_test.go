package gmail

import (
	"encoding/base64"
	"io"
	"net/mail"
	"strings"
	"testing"
)

func TestReplyPreservesGmailThread(t *testing.T) {
	message := gmailMessage{ThreadID: "test-thread", LabelIDs: []string{"INBOX"}}
	message.Payload.Headers = []struct{ Name, Value string }{
		{"From", "sender@example.test"}, {"Reply-To", "contact@example.test"},
		{"Subject", "Ward pilot"}, {"Message-ID", "<parent@example.test>"},
		{"References", "<previous@example.test>"},
	}
	body := strings.Repeat("Pickup details ✓ ", 16)
	raw, err := makeReply(message, "operations@example.test", body)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"From": "<operations@example.test>", "To": "<contact@example.test>",
		"Subject": "Re: Ward pilot", "In-Reply-To": "<parent@example.test>",
		"References": "<previous@example.test> <parent@example.test>",
	} {
		if got := reply.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, reply.Body))
	if err != nil || string(decoded) != body {
		t.Fatalf("reply body changed or failed to decode: %v", err)
	}
}
