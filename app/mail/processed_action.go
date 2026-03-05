package mail

import (
	"context"
	"fmt"
	"strings"
)

// ProcessedAction is the strategy interface for marking a mail as processed.
type ProcessedAction interface {
	// Apply performs the action on the given mail via the provided service.
	Apply(ctx context.Context, svc MailClientService, mail Mail) error
	// Name returns the canonical identifier of the action (e.g. "markRead", "delete").
	Name() string
}

type markReadAction struct{}

func (a *markReadAction) Apply(ctx context.Context, svc MailClientService, m Mail) error {
	return svc.MarkMailAsRead(ctx, m)
}
func (a *markReadAction) Name() string { return "markRead" }

type deleteAction struct{}

func (a *deleteAction) Apply(ctx context.Context, svc MailClientService, m Mail) error {
	return svc.DeleteMail(ctx, m)
}
func (a *deleteAction) Name() string { return "delete" }

// NewProcessedAction creates a ProcessedAction from the given name.
// Supported: "markRead" (default), "delete".
func NewProcessedAction(name string) (ProcessedAction, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "markread":
		return &markReadAction{}, nil
	case "delete":
		return &deleteAction{}, nil
	default:
		return nil, fmt.Errorf("unsupported processed action: %q", name)
	}
}