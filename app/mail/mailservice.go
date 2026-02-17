package mail

import (
	"context"
	"fmt"
	"strings"
)

type MailClientService interface {
	GetAllUnreadMail(context context.Context) ([]Mail, error)
	MarkMailAsRead(context context.Context, mail Mail) error
}

type Attachment struct {
	Name    string
	Content []byte
}

type Mail struct {
	Id          string
	Sender      string
	Recipients  []string
	Subject     string
	Body        string
	Attachments []Attachment
}

const DefaultCredentialsPath = "/secrets/mail"

// NewMailClientService is a factory that creates a concrete MailClientService based on the provided client type.
// Currently supported types: "gmail"
func NewMailClientService(clientType string) (MailClientService, error) {
	switch strings.ToLower(strings.TrimSpace(clientType)) {
	case "", "gmail":
		// Gmail client: use default mounted credentials path
		return NewGmailService(DefaultCredentialsPath), nil
	default:
		return nil, fmt.Errorf("unsupported mail client type: %s", clientType)
	}
}
