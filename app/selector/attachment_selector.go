package selector

import (
	"encoding/base64"
	"regexp"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

// AttachmentNameRegexSelectorPrototype is an immutable configuration for an attachment name regex selector.
type AttachmentNameRegexSelectorPrototype struct {
	name string
	re   *regexp.Regexp
}

// AttachmentNameRegexSelector is a stateless instance created from an
// AttachmentNameRegexSelectorPrototype.
type AttachmentNameRegexSelector struct {
	proto *AttachmentNameRegexSelectorPrototype
}

// NewInstance creates a new attachment name regex selector instance.
func (p *AttachmentNameRegexSelectorPrototype) NewInstance() Selector {
	return &AttachmentNameRegexSelector{
		proto: p,
	}
}

// Name returns the configured selector name.
func (s *AttachmentNameRegexSelector) Name() string {
	return s.proto.name
}

// Type returns the selector type.
func (s *AttachmentNameRegexSelector) Type() string {
	return "attachmentNameRegex"
}

// SelectValue scans attachments by filename; on first match returns base64 of content.
func (s *AttachmentNameRegexSelector) SelectValue(m mail.Mail) (string, error) {
	for _, att := range m.Attachments {
		if s.proto.re.MatchString(att.Name) {
			return base64.StdEncoding.EncodeToString(att.Content), nil
		}
	}
	return "", ErrNotMatched
}
