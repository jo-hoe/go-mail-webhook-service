package mail

import (
	"context"
	"fmt"
	"strings"
)

// ProcessedAction defines the strategy interface for marking a mail as processed.
type ProcessedAction interface {
	// Apply performs the action (e.g., mark as read, delete) on the given mail using the provided service.
	Apply(ctx context.Context, svc MailClientService, mail Mail) error
	// Name returns the identifier of the action (e.g., "mark_read", "delete").
	Name() string
}

// markReadAction marks the mail as read
type markReadAction struct{}

func (a *markReadAction) Apply(ctx context.Context, svc MailClientService, m Mail) error {
	return svc.MarkMailAsRead(ctx, m)
}
func (a *markReadAction) Name() string { return "markRead" }

// deleteAction deletes the mail
type deleteAction struct{}

func (a *deleteAction) Apply(ctx context.Context, svc MailClientService, m Mail) error {
	return svc.DeleteMail(ctx, m)
}
func (a *deleteAction) Name() string { return "delete" }

	// NewProcessedAction creates a ProcessedAction strategy from the provided name.
	// Supported: "markRead" (default), "delete".
	// Backward-compatible: accepts legacy "mark_read".
func NewProcessedAction(name string) (ProcessedAction, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "", "markread", "mark_read":
		return &markReadAction{}, nil
	case "delete":
		return &deleteAction{}, nil
	default:
		return nil, fmt.Errorf("unsupported processed action: %s", name)
	}
}
