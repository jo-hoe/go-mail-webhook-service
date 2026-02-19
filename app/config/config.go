package config

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

var supportHttpMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
}

// KeyValue represents a simple key/value pair in the callback config.
type KeyValue struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"` // may contain placeholders like ${SelectorName}
}

// AttachmentsConfig controls forwarding of attachments in callback requests.
type AttachmentsConfig struct {
	Enabled      bool   `yaml:"enabled"`
	FieldPrefix  string `yaml:"fieldPrefix"` // prefix for multipart field names
	MaxSize      string `yaml:"maxSize"`     // size string (e.g., "200Mi", "1MiB", "500MB"); empty or "0" means no limit
	MaxSizeBytes int64  `yaml:"-"`           // parsed bytes from MaxSize; 0 means no limit
}

func validateKeyValueList(list []KeyValue, allowHyphens bool, context string) error {
	for _, kv := range list {
		var nameRegex *regexp.Regexp
		if allowHyphens {
			nameRegex = regexp.MustCompile("^[0-9A-Za-z-]+$")
		} else {
			nameRegex = regexp.MustCompile("^[0-9A-Za-z]+$")
		}
		if !nameRegex.MatchString(kv.Key) {
			if allowHyphens {
				return fmt.Errorf("%s[%s] invalid key: must match ^[0-9A-Za-z-]+$", context, kv.Key)
			}
			return fmt.Errorf("%s[%s] invalid key: must match ^[0-9A-Za-z]+$", context, kv.Key)
		}
	}
	return nil
}

type Config struct {
	MailClient    MailClient           `yaml:"mailClient"`
	MailSelectors []MailSelectorConfig `yaml:"mailSelectors"`
	Callback      Callback             `yaml:"callback"`
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

type Callback struct {
	Url         string            `yaml:"url"`
	Method      string            `yaml:"method"`
	Timeout     string            `yaml:"timeout"` // default is "24s"
	Retries     int               `yaml:"retries"` // default is "0"
	Headers     []KeyValue        `yaml:"headers"`
	QueryParams []KeyValue        `yaml:"queryParams"`
	Form        []KeyValue        `yaml:"form"`
	Body        string            `yaml:"body"` // raw string body; user can build JSON themselves if desired
	Attachments AttachmentsConfig `yaml:"attachments"`
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
	if config.Callback.Timeout == "" {
		config.Callback.Timeout = "24s"
	}
	// Default FieldPrefix for attachments
	if config.Callback.Attachments.FieldPrefix == "" {
		config.Callback.Attachments.FieldPrefix = "attachment"
	}
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
	// CaptureGroup defaults via zero-values; nothing to set here
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

	// Validate callback
	if err := validateCallback(&config.Callback); err != nil {
		return err
	}

	// Validate processing behavior
	// Accept both "markRead" and legacy "mark_read" for backward compatibility; normalize to "markRead".
	action := strings.TrimSpace(config.Processing.ProcessedAction)
	switch strings.ToLower(action) {
	case "markread", "mark_read", "":
		config.Processing.ProcessedAction = "markRead"
	case "delete":
		config.Processing.ProcessedAction = "delete"
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

func validateCallback(callback *Callback) error {
	if callback.Url == "" {
		return fmt.Errorf("callback.url is empty")
	}
	if _, ok := supportHttpMethods[callback.Method]; !ok {
		allSupportHttpMethods := make([]string, len(supportHttpMethods))
		i := 0
		for method := range supportHttpMethods {
			allSupportHttpMethods[i] = method
			i++
		}
		return fmt.Errorf("callback.method not supported: %s, supported methods: %v", callback.Method, allSupportHttpMethods)
	}

	if _, err := time.ParseDuration(callback.Timeout); err != nil {
		return err
	}

	if callback.Retries < 0 {
		return fmt.Errorf("callback.retries must be greater than or equal to 0")
	}

	// Validate structured callback sections
	if err := validateKeyValueList(callback.Headers, true, "callback.headers"); err != nil {
		return err
	}
	if err := validateKeyValueList(callback.QueryParams, false, "callback.queryParams"); err != nil {
		return err
	}
	if err := validateKeyValueList(callback.Form, false, "callback.form"); err != nil {
		return err
	}
	if err := validateAttachments(&callback.Attachments); err != nil {
		return err
	}

	return nil
}

// validateAttachments checks optional attachment forwarding config.
func validateAttachments(att *AttachmentsConfig) error {
	// FieldPrefix must be alphanumeric if provided
	if att.FieldPrefix != "" {
		nameRegex := regexp.MustCompile("^[0-9A-Za-z]+$")
		if !nameRegex.MatchString(att.FieldPrefix) {
			return fmt.Errorf("callback.attachments.fieldPrefix must match ^[0-9A-Za-z]+$ (got '%s')", att.FieldPrefix)
		}
	}
	// Parse MaxSize (string) into bytes; empty or "0" means no limit
	sizeStr := strings.TrimSpace(att.MaxSize)
	if sizeStr == "" || sizeStr == "0" {
		att.MaxSizeBytes = 0
		return nil
	}
	bytes, err := parseSizeString(sizeStr)
	if err != nil {
		return fmt.Errorf("callback.attachments.maxSize invalid '%s': %v", att.MaxSize, err)
	}
	if bytes < 0 {
		return fmt.Errorf("callback.attachments.maxSize must be >= 0")
	}
	att.MaxSizeBytes = bytes
	return nil
}
