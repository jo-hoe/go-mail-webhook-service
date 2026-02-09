package selector

import (
	"fmt"
	"regexp"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
)

// NewSelectorPrototypes constructs immutable selector prototypes from configuration.
// Supports "subjectRegex", "bodyRegex", "senderRegex", and "attachmentNameRegex".
func NewSelectorPrototypes(cfgs []config.MailSelectorConfig) ([]SelectorPrototype, error) {
	prototypes := make([]SelectorPrototype, 0, len(cfgs))
	for _, c := range cfgs {
		switch c.Type {
		case "subjectRegex", "bodyRegex", "senderRegex":
			re, err := regexp.Compile(c.Pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex for selector '%s': %w", c.Name, err)
			}
			var target regexTarget
			if c.Type == "subjectRegex" {
				target = targetSubject
			} else if c.Type == "bodyRegex" {
				target = targetBody
			} else {
				target = targetSender
			}
			prototypes = append(prototypes, &RegexSelectorPrototype{
				name:         c.Name,
				selType:      c.Type,
				target:       target,
				scope:        c.Scope,
				captureGroup: c.CaptureGroup,
				re:           re,
			})
		case "attachmentNameRegex":
			re, err := regexp.Compile(c.Pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex for selector '%s': %w", c.Name, err)
			}
			prototypes = append(prototypes, &AttachmentNameRegexSelectorPrototype{
				name:  c.Name,
				re:    re,
				scope: c.Scope,
			})
		default:
			return nil, fmt.Errorf("unsupported selector type '%s' for selector '%s'", c.Type, c.Name)
		}
	}
	return prototypes, nil
}
