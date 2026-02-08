package selector

import "github.com/jo-hoe/go-mail-webhook-service/app/mail"

// Selector represents a stateful selector instance for a single mail evaluation.
// It holds a selected value if the evaluation matched.
type Selector interface {
	// Name returns the configured selector name.
	Name() string
	// Type returns the selector type (e.g., "regex").
	Type() string
	// IsScope indicates if this selector participates in scope filtering.
	IsScope() bool
	// Evaluate checks if the selector matches the given mail and stores the selected value when it does.
	Evaluate(mail.Mail) bool
	// SelectedValue returns the value captured during Evaluate when it matched.
	SelectedValue() string
}

// SelectorPrototype represents an immutable, reusable selector configuration.
// It can produce fresh stateful Selector instances for evaluation.
type SelectorPrototype interface {
	// NewInstance creates a new stateful selector instance.
	NewInstance() Selector
}