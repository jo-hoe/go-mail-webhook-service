package router

import (
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)


type WebhookRouter struct {}

func NewRouter() *WebhookRouter {
	return &WebhookRouter{}
}

func (r *WebhookRouter) Handle(mail *mail.Mail) error {
 	return nil
}