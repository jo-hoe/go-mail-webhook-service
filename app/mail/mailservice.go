package mail

import "context"


type MailService interface {
	GetAllUnreadMail(context context.Context) ([]Mail, error)
	SetMailAsRead(context context.Context, mail Mail) error
}

type Mail struct {
	Subject string
	Body string	
}