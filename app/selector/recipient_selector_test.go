package selector

import (
	"testing"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

func TestRecipientRegexSelector_MatchFirstRecipient(t *testing.T) {
	// Arrange: mail with multiple recipients
	m := mail.Mail{
		Recipients: []string{"sales@example.com", "team@example.com"},
	}

	// Build a recipientRegex selector that matches the first recipient
	protos, err := NewSelectorPrototypes([]config.MailSelectorConfig{
		{
			Name:         "toAddress",
			Type:         "recipientRegex",
			Pattern:      "^sales@example.com$",
			CaptureGroup: 0,
		},
	})
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}
	if len(protos) != 1 {
		t.Fatalf("expected 1 prototype, got %d", len(protos))
	}

	sel := protos[0].NewInstance()

	// Act
	val, err := sel.SelectValue(m)
	if err != nil {
		t.Fatalf("SelectValue returned error: %v", err)
	}

	// Assert
	if val != "sales@example.com" {
		t.Fatalf("expected match 'sales@example.com', got '%s'", val)
	}
}

func TestRecipientRegexSelector_NoMatch(t *testing.T) {
	// Arrange: mail with recipients not matching pattern
	m := mail.Mail{
		Recipients: []string{"sales@example.com", "team@example.com"},
	}

	// Build a recipientRegex selector that matches none
	protos, err := NewSelectorPrototypes([]config.MailSelectorConfig{
		{
			Name:         "toAddress",
			Type:         "recipientRegex",
			Pattern:      "^nobody@example.com$",
			CaptureGroup: 0,
		},
	})
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}
	if len(protos) != 1 {
		t.Fatalf("expected 1 prototype, got %d", len(protos))
	}

	sel := protos[0].NewInstance()

	// Act
	_, err = sel.SelectValue(m)

	// Assert: should be ErrNotMatched
	if err == nil {
		t.Fatalf("expected ErrNotMatched, got nil")
	}
	if err != ErrNotMatched {
		t.Fatalf("expected ErrNotMatched, got %v", err)
	}
}