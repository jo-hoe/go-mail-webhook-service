package selector

import (
	"errors"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

// ErrNotMatched indicates that a selector did not apply (no match) to the given mail.
// Callers can use errors.Is(err, ErrNotMatched) to distinguish non-match from other errors.
var ErrNotMatched = errors.New("selector not matched")

// Selector represents a selector that can evaluate a mail and return a selected value.
// Implementations should be stateless with respect to evaluation results.
type Selector interface {
	// Name returns the configured selector name.
	Name() string
	// Type returns the selector type (e.g., "regex").
	Type() string
	// SelectValue evaluates the selector against the given mail and returns the selected value.
	// Returns ErrNotMatched when the selector does not apply to the mail.
	// Returns a different error when evaluation fails for operational reasons.
	SelectValue(mail.Mail) (string, error)
}

// SelectorPrototype represents an immutable, reusable selector configuration.
// It can produce fresh Selector instances for evaluation.
type SelectorPrototype interface {
	// NewInstance creates a new selector instance.
	NewInstance() Selector
}
