package mail

import (
	"context"
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

func NewMailClientService() (MailClientService, error) {
	// Only Gmail is supported; use default mounted credentials path
	return NewGmailService(DefaultCredentialsPath), nil
}
