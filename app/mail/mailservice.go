package mail

import (
	"context"
	"fmt"
	"strings"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
)

type MailClientService interface {
	GetAllUnreadMail(context context.Context) ([]Mail, error)
	SetMailAsRead(context context.Context, mail Mail) error
}

type Mail struct {
	Subject string
	Body    string
}

func NewMailClientService(mailClientConfig *config.MailClientConfig) (MailClientService, error) {
	for _, domainName := range GMailDomainNames {
		if strings.HasSuffix(mailClientConfig.Mail, fmt.Sprintf("@%s", domainName)) {
			return NewGmailService(mailClientConfig.CredentialsPath), nil
		}
	}

	return nil, fmt.Errorf("%s has an unsupported domain name", mailClientConfig.Mail)
}
