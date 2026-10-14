package mail

import (
	"context"
	"fmt"
)

// ClientServiceMock is a test double for ClientService that records calls.
type ClientServiceMock struct {
	ReturnErrorsOnly bool
	Mails            []Mail
	MarkReadCalls    int
	DeleteCalls      int
}

// GetAllUnreadMail returns the configured mails, or an error when ReturnErrorsOnly is set.
func (service *ClientServiceMock) GetAllUnreadMail(_ context.Context) ([]Mail, error) {
	if service.ReturnErrorsOnly {
		return nil, fmt.Errorf("dummy error")
	}

	return service.Mails, nil
}

// MarkMailAsRead records the call, or returns an error when ReturnErrorsOnly is set.
func (service *ClientServiceMock) MarkMailAsRead(_ context.Context, _ Mail) error {
	if service.ReturnErrorsOnly {
		return fmt.Errorf("dummy error")
	}
	service.MarkReadCalls++
	return nil
}

// DeleteMail records the call, or returns an error when ReturnErrorsOnly is set.
func (service *ClientServiceMock) DeleteMail(_ context.Context, _ Mail) error {
	if service.ReturnErrorsOnly {
		return fmt.Errorf("dummy error")
	}
	service.DeleteCalls++
	return nil
}
