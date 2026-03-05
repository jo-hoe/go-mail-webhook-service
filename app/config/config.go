package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jo-hoe/goback"
	"gopkg.in/yaml.v2"
)

// Config is the top-level application configuration.
type Config struct {
	LogLevel      string               `yaml:"logLevel"`
	MailClient    MailClient           `yaml:"mailClient"`
	MailSelectors []MailSelectorConfig `yaml:"mailSelectors"`

	// Callback is the strongly-typed goback configuration for outgoing webhook calls.
	Callback goback.Config `yaml:"callback"`

	// Attachments controls how email attachments are forwarded via multipart file parts.
	Attachments AttachmentsConfig `yaml:"attachments"`

	// Processing controls what to do with a mail after a successful webhook call.
	Processing Processing `yaml:"processing"`
}

// MailSelectorConfig defines a single mail selector rule.
type MailSelectorConfig struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`         // "subjectRegex" | "bodyRegex" | "attachmentNameRegex" | "senderRegex" | "recipientRegex"
	Pattern      string `yaml:"pattern"`      // regex pattern
	CaptureGroup int    `yaml:"captureGroup"` // 0 = full match (default)
}

// GmailClient holds Gmail-specific client configuration.
type GmailClient struct {
	Enabled bool `yaml:"enabled"`
}

// MailClient holds mail client configuration.
type MailClient struct {
	Gmail GmailClient `yaml:"gmail"`
}

// Processing controls post-processing behaviour after a successful webhook call.
type Processing struct {
	// ProcessedAction determines how a mail is marked after processing.
	// Supported: "markRead" (default), "delete".
	ProcessedAction string `yaml:"processedAction"`
}

// AttachmentStrategy is a strongly-typed enum for attachment handling behaviour.
type AttachmentStrategy string

const (
	StrategyIgnore                 AttachmentStrategy = "ignore"
	StrategyMultipartBundle        AttachmentStrategy = "multipartBundle"
	StrategyMultipartPerAttachment AttachmentStrategy = "multipartPerAttachment"
)

// AttachmentsConfig controls forwarding of attachments in webhook requests.
type AttachmentsConfig struct {
	// Strategy defines how to handle attachments.
	Strategy AttachmentStrategy `yaml:"strategy"`
	// FieldName is the multipart form field name; supports Go text/template variables.
	FieldName string `yaml:"fieldName"`
	// MaxSize is an optional per-attachment size limit (e.g. "200Mi"); empty or "0" means no limit.
	MaxSize      string `yaml:"maxSize"`
	MaxSizeBytes int64  `yaml:"-"`
}

// selectorNameRegex is compiled once and reused for every selector name validation.
var selectorNameRegex = regexp.MustCompile(`^[0-9A-Za-z]+$`)

// NewConfigFromYaml parses yamlBytes, applies defaults, and validates the result.
func NewConfigFromYaml(yamlBytes []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		return nil, err
	}
	setDefaults(&cfg)
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.LogLevel) == "" {
		cfg.LogLevel = "info"
	}
	if !cfg.MailClient.Gmail.Enabled {
		cfg.MailClient.Gmail.Enabled = true
	}
	if strings.TrimSpace(cfg.Processing.ProcessedAction) == "" {
		cfg.Processing.ProcessedAction = "markRead"
	}
	if strings.TrimSpace(cfg.Attachments.FieldName) == "" {
		cfg.Attachments.FieldName = "attachment"
	}
	if strings.TrimSpace(string(cfg.Attachments.Strategy)) == "" {
		cfg.Attachments.Strategy = StrategyMultipartBundle
	}
}

func validateConfig(cfg *Config) error {
	if err := validateLogLevel(cfg); err != nil {
		return err
	}
	for i := range cfg.MailSelectors {
		if err := validateMailSelectorConfig(&cfg.MailSelectors[i]); err != nil {
			return err
		}
	}
	if !cfg.MailClient.Gmail.Enabled {
		return fmt.Errorf("no mail client enabled; set mailClient.gmail.enabled: true")
	}
	if strings.TrimSpace(cfg.Callback.URL) == "" {
		return fmt.Errorf("callback.url is required")
	}
	if err := validateProcessedAction(cfg); err != nil {
		return err
	}
	return validateAttachments(&cfg.Attachments)
}

func validateLogLevel(cfg *Config) error {
	level := strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	switch level {
	case "debug", "info", "warn", "error":
		cfg.LogLevel = level
		return nil
	default:
		return fmt.Errorf("invalid logLevel %q (supported: debug, info, warn, error)", cfg.LogLevel)
	}
}

func validateProcessedAction(cfg *Config) error {
	switch strings.ToLower(strings.TrimSpace(cfg.Processing.ProcessedAction)) {
	case "markread":
		cfg.Processing.ProcessedAction = "markRead"
	case "delete":
		// already canonical
	default:
		return fmt.Errorf("invalid processing.processedAction %q (supported: markRead, delete)", cfg.Processing.ProcessedAction)
	}
	return nil
}

func validateMailSelectorConfig(sel *MailSelectorConfig) error {
	if !selectorNameRegex.MatchString(sel.Name) {
		return fmt.Errorf("mailSelectors.name must match ^[0-9A-Za-z]+$: %q", sel.Name)
	}
	switch sel.Type {
	case "subjectRegex", "bodyRegex", "attachmentNameRegex", "senderRegex", "recipientRegex":
	default:
		return fmt.Errorf("mailSelectors.type %q not supported (supported: subjectRegex, bodyRegex, attachmentNameRegex, senderRegex, recipientRegex)", sel.Type)
	}
	re, err := regexp.Compile(sel.Pattern)
	if err != nil {
		return fmt.Errorf("mailSelectors.pattern %q cannot be compiled: %w", sel.Pattern, err)
	}
	if sel.CaptureGroup < 0 {
		return fmt.Errorf("mailSelectors.captureGroup must be >= 0 (got %d)", sel.CaptureGroup)
	}
	if sel.CaptureGroup > re.NumSubexp() {
		return fmt.Errorf("mailSelectors.captureGroup (%d) exceeds number of groups (%d) in pattern %q", sel.CaptureGroup, re.NumSubexp(), sel.Pattern)
	}
	return nil
}

func validateAttachments(att *AttachmentsConfig) error {
	switch strings.ToLower(strings.TrimSpace(string(att.Strategy))) {
	case "ignore":
		att.Strategy = StrategyIgnore
	case "multipartbundle":
		att.Strategy = StrategyMultipartBundle
	case "multipartperattachment":
		att.Strategy = StrategyMultipartPerAttachment
	default:
		return fmt.Errorf("attachments.strategy %q is invalid (supported: ignore, multipartBundle, multipartPerAttachment)", att.Strategy)
	}

	sizeStr := strings.TrimSpace(att.MaxSize)
	if sizeStr == "" || sizeStr == "0" {
		att.MaxSizeBytes = 0
		return nil
	}
	n, err := parseSizeString(sizeStr)
	if err != nil {
		return fmt.Errorf("attachments.maxSize %q is invalid: %w", att.MaxSize, err)
	}
	if n < 0 {
		return fmt.Errorf("attachments.maxSize must be >= 0")
	}
	att.MaxSizeBytes = n
	return nil
}