package selector

import (
	"fmt"
	"regexp"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

 // NewSelectorPrototypes constructs immutable selector prototypes from configuration.
 // Supports "subjectRegex", "bodyRegex", "senderRegex", "recipientRegex", and "attachmentNameRegex".
func NewSelectorPrototypes(cfgs []config.MailSelectorConfig) ([]SelectorPrototype, error) {
	prototypes := make([]SelectorPrototype, 0, len(cfgs))
	for _, c := range cfgs {
		switch c.Type {
		case "subjectRegex", "bodyRegex", "senderRegex", "recipientRegex":
			re, err := regexp.Compile(c.Pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex for selector '%s': %w", c.Name, err)
			}
			var getValues func(mail.Mail) []string
			switch c.Type {
			case "subjectRegex":
				getValues = func(m mail.Mail) []string { return []string{m.Subject} }
			case "bodyRegex":
				getValues = func(m mail.Mail) []string { return []string{m.Body} }
			case "senderRegex":
				getValues = func(m mail.Mail) []string { return []string{m.Sender} }
			case "recipientRegex":
				getValues = func(m mail.Mail) []string { return m.Recipients }
			}
			prototypes = append(prototypes, &RegexSelectorPrototype{
				name:         c.Name,
				selType:      c.Type,
				captureGroup: c.CaptureGroup,
				re:           re,
				getValues:    getValues,
			})
		case "attachmentNameRegex":
			re, err := regexp.Compile(c.Pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex for selector '%s': %w", c.Name, err)
			}
			prototypes = append(prototypes, &AttachmentNameRegexSelectorPrototype{
				name:  c.Name,
				re:    re,
			})
		default:
			return nil, fmt.Errorf("unsupported selector type '%s' for selector '%s'", c.Type, c.Name)
		}
	}
	return prototypes, nil
}
