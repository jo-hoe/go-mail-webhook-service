package selector

import (
	"regexp"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

// regexTarget is an internal enum representing which mail field to evaluate.
type regexTarget int

const (
	targetSubject regexTarget = iota
	targetBody
	targetSender
)

// RegexSelectorPrototype is an immutable, reusable configuration for a regex selector.
// It holds compiled regex and static attributes. Safe to share across goroutines.
type RegexSelectorPrototype struct {
	name         string
	selType      string // "subjectRegex" | "bodyRegex" | "senderRegex"
	target       regexTarget
	scope        bool
	captureGroup int
	re           *regexp.Regexp
}

// RegexSelector is a stateful instance created from a RegexSelectorPrototype.
// It evaluates a single mail and stores the selectedValue if it matches.
type RegexSelector struct {
	proto         *RegexSelectorPrototype
	selectedValue string
}

func (p *RegexSelectorPrototype) NewInstance() Selector {
	return &RegexSelector{
		proto:         p,
		selectedValue: "",
	}
}

func (s *RegexSelector) Name() string {
	return s.proto.name
}

func (s *RegexSelector) Type() string {
	return s.proto.selType
}

func (s *RegexSelector) IsScope() bool {
	return s.proto.scope
}

// Evaluate applies the regex against the configured target of the mail.
// If it matches, it stores either the full match (captureGroup == 0)
// or the specified capture group (>0) as selectedValue.
func (s *RegexSelector) Evaluate(m mail.Mail) bool {
	var input string
	switch s.proto.target {
	case targetSubject:
		input = m.Subject
	case targetBody:
		input = m.Body
	case targetSender:
		input = m.Sender
	default:
		// Unknown target; treat as non-match
		return false
	}

	// Using FindStringSubmatch to have access to capture groups
	submatches := s.proto.re.FindStringSubmatch(input)
	if len(submatches) == 0 {
		return false
	}

	// captureGroup 0 means full match
	if s.proto.captureGroup == 0 {
		s.selectedValue = submatches[0]
		return true
	}

	// captureGroup > 0 must be within bounds
	if s.proto.captureGroup > 0 && s.proto.captureGroup < len(submatches) {
		s.selectedValue = submatches[s.proto.captureGroup]
		return true
	}

	// group requested but not available -> no match
	return false
}

func (s *RegexSelector) SelectedValue() string {
	return s.selectedValue
}
