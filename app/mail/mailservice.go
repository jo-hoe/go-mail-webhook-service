package mail

import (
	"context"
	"fmt"
)

// ClientType identifies a mail client backend.
type ClientType string

const (
	// GmailClientType selects the Google Gmail backend.
	GmailClientType ClientType = "gmail"

	// DefaultCredentialsPath is the default path for mounted OAuth credentials.
	DefaultCredentialsPath = "/secrets/mail"
)

// MailClientService defines the operations required by the application.
type MailClientService interface {
	GetAllUnreadMail(ctx context.Context) ([]Mail, error)
	MarkMailAsRead(ctx context.Context, mail Mail) error
	DeleteMail(ctx context.Context, mail Mail) error
}

// Attachment represents a single email attachment.
type Attachment struct {
	Name    string
	Content []byte
}

// Mail represents an email message.
type Mail struct {
	Id          string
	Sender      string
	Recipients  []string
	Subject     string
	Body        string
	Attachments []Attachment
}

// NewMailClientService returns a MailClientService for the given client type.
// An empty ClientType defaults to GmailClientType.
func NewMailClientService(clientType ClientType) (MailClientService, error) {
	switch clientType {
	case GmailClientType, "":
		return NewGmailService(DefaultCredentialsPath), nil
	default:
		return nil, fmt.Errorf("unsupported mail client type: %s", clientType)
	}
}