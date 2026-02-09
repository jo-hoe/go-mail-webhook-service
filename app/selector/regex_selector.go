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

// RegexSelector is a stateless instance created from a RegexSelectorPrototype.
// It evaluates a mail and returns the selected value if it matches.
type RegexSelector struct {
	proto *RegexSelectorPrototype
}

func (p *RegexSelectorPrototype) NewInstance() Selector {
	return &RegexSelector{
		proto: p,
	}
}

func (s *RegexSelector) Name() string {
	return s.proto.name
}

func (s *RegexSelector) Type() string {
	return s.proto.selType
}

// SelectValue applies the regex against the configured target of the mail.
// If it matches, it returns either the full match (captureGroup == 0)
// or the specified capture group (>0). Otherwise returns ErrNotMatched.
func (s *RegexSelector) SelectValue(m mail.Mail) (string, error) {
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
		return "", ErrNotMatched
	}

	// Using FindStringSubmatch to have access to capture groups
	submatches := s.proto.re.FindStringSubmatch(input)
	if len(submatches) == 0 {
		return "", ErrNotMatched
	}

	// captureGroup 0 means full match
	if s.proto.captureGroup == 0 {
		return submatches[0], nil
	}

	// captureGroup > 0 must be within bounds
	if s.proto.captureGroup > 0 && s.proto.captureGroup < len(submatches) {
		return submatches[s.proto.captureGroup], nil
	}

	// group requested but not available -> no match
	return "", ErrNotMatched
}