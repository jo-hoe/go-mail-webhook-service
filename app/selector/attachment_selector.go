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
	proto         *AttachmentNameRegexSelectorPrototype
	selectedValue string
}

func (p *AttachmentNameRegexSelectorPrototype) NewInstance() Selector {
	return &AttachmentNameRegexSelector{
		proto:         p,
		selectedValue: "",
	}
}

func (s *AttachmentNameRegexSelector) Name() string {
	return s.proto.name
}

func (s *AttachmentNameRegexSelector) Type() string {
	return "attachmentNameRegex"
}

func (s *AttachmentNameRegexSelector) IsScope() bool {
	return s.proto.scope
}

// Evaluate scans attachments by filename; on first match sets selectedValue to base64 of content.
func (s *AttachmentNameRegexSelector) Evaluate(m mail.Mail) bool {
	for _, att := range m.Attachments {
		if s.proto.re.MatchString(att.Name) {
			s.selectedValue = base64.StdEncoding.EncodeToString(att.Content)
			return true
		}
	}
	return false
}

func (s *AttachmentNameRegexSelector) SelectedValue() string {
	return s.selectedValue
}
