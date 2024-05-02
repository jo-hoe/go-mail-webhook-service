package router

import (
	"github.com/jo-hoe/go-mail-webhook-service/app/config"
)

type WebhookRouter struct {
	config *config.Config
}

func NewRouter(config *config.Config) *WebhookRouter {
	return &WebhookRouter{
		config: config,
	}
}
