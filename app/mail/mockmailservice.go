package mail

import (
	"context"
	"fmt"
)

type MailClientServiceMock struct {
	ReturnErrorsOnly bool
	Mails            []Mail
}

func (service MailClientServiceMock) GetAllUnreadMail(context context.Context) ([]Mail, error) {
	if service.ReturnErrorsOnly {
		return nil, fmt.Errorf("dummy error")
	}

	return service.Mails, nil
}

func (service MailClientServiceMock) MarkMailAsRead(context context.Context, mail Mail) error {
	if service.ReturnErrorsOnly {
		return fmt.Errorf("dummy error")
	}
	return nil
}
