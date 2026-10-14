package mail

import (
	"context"
	"fmt"
	"strings"
)

// ProcessedAction is the strategy interface for marking a mail as processed.
type ProcessedAction interface {
	// Apply performs the action on the given mail via the provided service.
	Apply(ctx context.Context, svc ClientService, mail Mail) error
	// Name returns the canonical identifier of the action (e.g. "markRead", "delete").
	Name() string
}

type markReadAction struct{}

func (a *markReadAction) Apply(ctx context.Context, svc ClientService, m Mail) error {
	if err := svc.MarkMailAsRead(ctx, m); err != nil {
		return fmt.Errorf("mark mail as read: %w", err)
	}
	return nil
}
func (a *markReadAction) Name() string { return "markRead" }

type deleteAction struct{}

func (a *deleteAction) Apply(ctx context.Context, svc ClientService, m Mail) error {
	if err := svc.DeleteMail(ctx, m); err != nil {
		return fmt.Errorf("delete mail: %w", err)
	}
	return nil
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