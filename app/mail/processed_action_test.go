package mail

import (
	"context"
	"testing"
)

func TestNewProcessedAction_MarkReadAndDelete(t *testing.T) {
	ctx := context.Background()
	mock := &MailClientServiceMock{}

	// markRead (preferred K8s-style)
	a, err := NewProcessedAction("markRead")
	if err != nil {
		t.Fatalf("NewProcessedAction(markRead) error = %v", err)
	}
	if a.Name() != "markRead" {
		t.Fatalf("markRead Name() = %s, want markRead", a.Name())
	}
	if err := a.Apply(ctx, mock, Mail{}); err != nil {
		t.Fatalf("Apply(markRead) error = %v", err)
	}
	if mock.MarkReadCalls != 1 {
		t.Fatalf("MarkReadCalls = %d, want 1", mock.MarkReadCalls)
	}

	// legacy: markRead should map to markRead
	a2, err := NewProcessedAction("markRead")
	if err != nil {
		t.Fatalf("NewProcessedAction(markRead) error = %v", err)
	}
	if a2.Name() != "markRead" {
		t.Fatalf("markRead Name() = %s, want markRead", a2.Name())
	}
	if err := a2.Apply(ctx, mock, Mail{}); err != nil {
		t.Fatalf("Apply(markRead) error = %v", err)
	}
	if mock.MarkReadCalls != 2 {
		t.Fatalf("MarkReadCalls = %d, want 2", mock.MarkReadCalls)
	}

	// delete
	a3, err := NewProcessedAction("delete")
	if err != nil {
		t.Fatalf("NewProcessedAction(delete) error = %v", err)
	}
	if a3.Name() != "delete" {
		t.Fatalf("delete Name() = %s, want delete", a3.Name())
	}
	if err := a3.Apply(ctx, mock, Mail{}); err != nil {
		t.Fatalf("Apply(delete) error = %v", err)
	}
	if mock.DeleteCalls != 1 {
		t.Fatalf("DeleteCalls = %d, want 1", mock.DeleteCalls)
	}
}

func TestNewProcessedAction_Invalid(t *testing.T) {
	if _, err := NewProcessedAction("unknown-action"); err == nil {
		t.Fatalf("expected error for unknown action, got nil")
	}
}