package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jo-hoe/gohook"
	"gopkg.in/yaml.v2"
)

type Config struct {
	MailClient    MailClient           `yaml:"mailClient"`
	MailSelectors []MailSelectorConfig `yaml:"mailSelectors"`
	Callback      gohook.Config        `yaml:"callback"`
	LogLevel      string               `yaml:"logLevel"` // logging level: "debug" | "info" | "warn" | "error"
	Processing    Processing           `yaml:"processing"`
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

func NewConfigFromYaml(yamlBytes []byte) (*Config, error) {
	var cfg Config
	err := yaml.Unmarshal(yamlBytes, &cfg)
	if err != nil {
		return nil, err
	}

	setDefaults(&cfg)
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(config *Config) {
	// Default log level
	if strings.TrimSpace(config.LogLevel) == "" {
		config.LogLevel = "info"
	}
	// Default mail client: enable gmail by default
	if !config.MailClient.Gmail.Enabled {
		config.MailClient.Gmail.Enabled = true
	}
	// Default processing action
	if strings.TrimSpace(config.Processing.ProcessedAction) == "" {
		config.Processing.ProcessedAction = "markRead"
	}
	// Keep previous default behavior for timeout unless overridden
	if strings.TrimSpace(config.Callback.Timeout) == "" {
		config.Callback.Timeout = "24s"
	}
}

func validateConfig(config *Config) error {
	// Normalize and validate log level
	level := strings.ToLower(strings.TrimSpace(config.LogLevel))
	switch level {
	case "debug", "info", "warn", "error":
		config.LogLevel = level
	default:
		return fmt.Errorf("invalid logLevel '%s' (supported: debug, info, warn, error)", config.LogLevel)
	}

	// Validate selectors
	for _, sel := range config.MailSelectors {
		if err := validateMailSelectorConfig(&sel); err != nil {
			return err
		}
	}

	// Validate mail client enablement
	if !config.MailClient.Gmail.Enabled {
		return fmt.Errorf("no mail client enabled; set mailClient.gmail.enabled: true")
	}

	// Minimal callback validation; detailed validation is performed by gohook.New / NewHookExecutor
	if strings.TrimSpace(config.Callback.URL) == "" {
		return fmt.Errorf("callback.url is empty")
	}

	// Validate processing behavior (no legacy support)
	action := strings.TrimSpace(config.Processing.ProcessedAction)
	switch action {
	case "markRead", "delete":
		// ok
	default:
		return fmt.Errorf("invalid processing.processedAction '%s' (supported: markRead, delete)", config.Processing.ProcessedAction)
	}

	return nil
}

func validateMailSelectorConfig(sel *MailSelectorConfig) error {
	// name: alphanumeric only
	nameRegex := regexp.MustCompile("^[0-9A-Za-z]+$")
	if !nameRegex.MatchString(sel.Name) {
		return fmt.Errorf("mailSelectors.name must match ^[0-9A-Za-z]+$: '%s'", sel.Name)
	}

	// type: support "subjectRegex", "bodyRegex", "attachmentNameRegex", "senderRegex", and "recipientRegex"
	if sel.Type != "subjectRegex" && sel.Type != "bodyRegex" && sel.Type != "attachmentNameRegex" && sel.Type != "senderRegex" && sel.Type != "recipientRegex" {
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
	// NumSubexp is the number of capturing groups; captureGroup == 0 is full match
	if sel.CaptureGroup > re.NumSubexp() {
		return fmt.Errorf("mailSelectors.captureGroup (%d) exceeds number of groups (%d) in pattern '%s'", sel.CaptureGroup, re.NumSubexp(), sel.Pattern)
	}

	return nil
}