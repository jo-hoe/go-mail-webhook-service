package selector

import (
	"encoding/base64"
	"regexp"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

// AttachmentNameRegexSelectorPrototype is an immutable configuration for an attachment name regex selector.
type AttachmentNameRegexSelectorPrototype struct {
	name  string
	re    *regexp.Regexp
	scope bool
}

type AttachmentNameRegexSelector struct {
	proto *AttachmentNameRegexSelectorPrototype
}

func (p *AttachmentNameRegexSelectorPrototype) NewInstance() Selector {
	return &AttachmentNameRegexSelector{
		proto: p,
	}
}

func (s *AttachmentNameRegexSelector) Name() string {
	return s.proto.name
}

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

