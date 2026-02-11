package selector

import (
	"regexp"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)


// RegexSelectorPrototype is an immutable, reusable configuration for a regex selector.
// It holds compiled regex and static attributes. Safe to share across goroutines.
type RegexSelectorPrototype struct {
	name         string
	selType      string // "subjectRegex" | "bodyRegex" | "senderRegex" | "recipientRegex"
	scope        bool
	captureGroup int
	re           *regexp.Regexp
	getValues    func(mail.Mail) []string
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
	if s.proto.getValues == nil {
		return "", ErrNotMatched
	}
	values := s.proto.getValues(m)
	if len(values) == 0 {
		return "", ErrNotMatched
	}
	return s.selectFromValues(values)
}


// selectFromValues tries to match the regex against the provided values and returns the selected capture.
func (s *RegexSelector) selectFromValues(values []string) (string, error) {
	for _, v := range values {
		if v == "" {
			continue
		}
		sub := s.proto.re.FindStringSubmatch(v)
		if len(sub) == 0 {
			continue
		}
		// captureGroup 0 means full match
		if s.proto.captureGroup == 0 {
			return sub[0], nil
		}
		// captureGroup > 0 must be within bounds
		if s.proto.captureGroup > 0 && s.proto.captureGroup < len(sub) {
			return sub[s.proto.captureGroup], nil
		}
		// capture group out of bounds for this value; try next value
	}
	return "", ErrNotMatched
}

