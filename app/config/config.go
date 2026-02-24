package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jo-hoe/goback"
	"gopkg.in/yaml.v2"
)

// Config is the application configuration.
//   - Callback contains the goback.Config used to execute webhooks (URL, method, headers, query, body, multipart, etc.).
//   - Attachments controls forwarding email attachments into the webhook's multipart request.
type Config struct {
	LogLevel      string               `yaml:"logLevel"`
	MailClient    MailClient           `yaml:"mailClient"`
	MailSelectors []MailSelectorConfig `yaml:"mailSelectors"`

	// Callback contains the strongly-typed goback configuration for outgoing webhook calls.
	Callback goback.Config `yaml:"callback"`

	// Attachments controls how email attachments are forwarded via multipart file parts.
	Attachments AttachmentsConfig `yaml:"attachments"`

	// Processing controls what to do with a mail after successful webhook execution.
	Processing Processing `yaml:"processing"`
}

type MailSelectorConfig struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`         // "subjectRegex" | "bodyRegex" | "attachmentNameRegex" | "senderRegex" | "recipientRegex"
	Pattern      string `yaml:"pattern"`      // regex pattern
	CaptureGroup int    `yaml:"captureGroup"` // default 0 (full match)
}

type GmailClient struct {
	Enabled bool `yaml:"enabled"`
}

type MailClient struct {
	Gmail GmailClient `yaml:"gmail"`
}

type Processing struct {
	// ProcessedAction controls how a mail is marked as processed after a successful webhook call.
	// Supported values:
	// - "markRead" (default): remove UNREAD label
	// - "delete": delete the mail using the mail client (be aware this is permanent for Gmail)
	ProcessedAction string `yaml:"processedAction"`
}

 // AttachmentStrategy is a strongly-typed enum for attachment handling behavior.
type AttachmentStrategy string

const (
	StrategyIgnore                 AttachmentStrategy = "ignore"
	StrategyMultipartBundle        AttachmentStrategy = "multipartBundle"
	StrategyMultipartPerAttachment AttachmentStrategy = "multipartPerAttachment"
)

// AttachmentsConfig controls forwarding of attachments in webhook requests.
 // Strategy defines how to handle attachments: "ignore", "multipartBundle", or "multipartPerAttachment".
// FieldName is the multipart form field name for files. It can be a static string or a Go text/template.
// MaxSize is optional per-attachment size limit (e.g., "200Mi", "1MiB", "500MB"); empty or "0" means no limit.
type AttachmentsConfig struct {
	Strategy     AttachmentStrategy `yaml:"strategy"`
	FieldName    string             `yaml:"fieldName"`
	MaxSize      string             `yaml:"maxSize"`
	MaxSizeBytes int64              `yaml:"-"`
}

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
	// Default log level
	if strings.TrimSpace(cfg.LogLevel) == "" {
		cfg.LogLevel = "info"
	}

	// Default mail client: enable gmail by default
	if !cfg.MailClient.Gmail.Enabled {
		cfg.MailClient.Gmail.Enabled = true
	}

	// Default processing action
	if strings.TrimSpace(cfg.Processing.ProcessedAction) == "" {
		cfg.Processing.ProcessedAction = "markRead"
	}

	// Attachments defaults
	if strings.TrimSpace(cfg.Attachments.FieldName) == "" {
		cfg.Attachments.FieldName = "attachment"
	}
	if strings.TrimSpace(string(cfg.Attachments.Strategy)) == "" {
		cfg.Attachments.Strategy = StrategyMultipartBundle
	}
}

func validateConfig(cfg *Config) error {
	// log level
	level := strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	switch level {
	case "debug", "info", "warn", "error":
		cfg.LogLevel = level
	default:
		return fmt.Errorf("invalid logLevel '%s' (supported: debug, info, warn, error)", cfg.LogLevel)
	}

	// selectors
	for _, sel := range cfg.MailSelectors {
		if err := validateMailSelectorConfig(&sel); err != nil {
			return err
		}
	}

	// mail client
	if !cfg.MailClient.Gmail.Enabled {
		return fmt.Errorf("no mail client enabled; set mailClient.gmail.enabled: true")
	}

	// minimal hook validation: URL is required
	if strings.TrimSpace(cfg.Callback.URL) == "" {
		return fmt.Errorf("hook.url is empty")
	}

	// processing behavior
	switch strings.ToLower(strings.TrimSpace(cfg.Processing.ProcessedAction)) {
	case "markRead", "markread", "":
		cfg.Processing.ProcessedAction = "markRead"
	case "delete":
		cfg.Processing.ProcessedAction = "delete"
	default:
		return fmt.Errorf("invalid processing.processedAction '%s' (supported: markRead, delete)", cfg.Processing.ProcessedAction)
	}

	// attachments
	if err := validateAttachments(&cfg.Attachments); err != nil {
		return err
	}

	return nil
}

func validateMailSelectorConfig(sel *MailSelectorConfig) error {
	// name: alphanumeric only
	nameRegex := regexp.MustCompile("^[0-9A-Za-z]+$")
	if !nameRegex.MatchString(sel.Name) {
		return fmt.Errorf("mailSelectors.name must match ^[0-9A-Za-z]+$: '%s'", sel.Name)
	}

	// type
	switch sel.Type {
	case "subjectRegex", "bodyRegex", "attachmentNameRegex", "senderRegex", "recipientRegex":
	default:
		return fmt.Errorf("mailSelectors.type not supported: '%s' (supported: 'subjectRegex','bodyRegex','attachmentNameRegex','senderRegex','recipientRegex')", sel.Type)
	}

	// pattern must compile
	re, err := regexp.Compile(sel.Pattern)
	if err != nil {
		return fmt.Errorf("mailSelectors.pattern cannot be compiled: '%s' - error: %s", sel.Pattern, err)
	}

	// captureGroup must be >= 0 and not exceed available subexpressions
	if sel.CaptureGroup < 0 {
		return fmt.Errorf("mailSelectors.captureGroup must be >= 0 (got %d)", sel.CaptureGroup)
	}
	if sel.CaptureGroup > re.NumSubexp() {
		return fmt.Errorf("mailSelectors.captureGroup (%d) exceeds number of groups (%d) in pattern '%s'", sel.CaptureGroup, re.NumSubexp(), sel.Pattern)
	}

	return nil
}

func validateAttachments(att *AttachmentsConfig) error {
	// Normalize and validate strategy
	switch strings.ToLower(strings.TrimSpace(string(att.Strategy))) {
	case "ignore":
		att.Strategy = StrategyIgnore
	case "multipartbundle", "":
		att.Strategy = StrategyMultipartBundle
	case "multipartperattachment":
		att.Strategy = StrategyMultipartPerAttachment
	default:
		return fmt.Errorf("attachments.strategy invalid '%s' (supported: ignore, multipartBundle, multipartPerAttachment)", string(att.Strategy))
	}

	// FieldName: allow static or templated names; no strict validation needed.
	att.FieldName = strings.TrimSpace(att.FieldName)
	if att.FieldName == "" {
		att.FieldName = "attachment"
	}

	// Parse MaxSize (string) into bytes; empty or "0" means no limit
	sizeStr := strings.TrimSpace(att.MaxSize)
	if sizeStr == "" || sizeStr == "0" {
		att.MaxSizeBytes = 0
		return nil
	}
	bytes, err := parseSizeString(sizeStr)
	if err != nil {
		return fmt.Errorf("attachments.maxSize invalid '%s': %v", att.MaxSize, err)
	}
	if bytes < 0 {
		return fmt.Errorf("attachments.maxSize must be >= 0")
	}
	att.MaxSizeBytes = bytes
	return nil
}