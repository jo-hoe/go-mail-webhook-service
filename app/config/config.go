package config

import (
	"fmt"
	"net/http"
	"regexp"
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
	Enabled       bool   `yaml:"enabled"`
	FieldPrefix   string `yaml:"fieldPrefix"` // prefix for multipart field names
	MaxSize       int    `yaml:"maxSize"`     // bytes; <=0 means no limit
	IncludeInline bool   `yaml:"includeInline"`
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
	MailClientConfig  MailClientConfig     `yaml:"mailClientConfig"`
	MailSelectors     []MailSelectorConfig `yaml:"mailSelectors"`
	Callback          Callback             `yaml:"callback"`
}

type MailClientConfig struct {
	Mail            string `yaml:"mail"`
	CredentialsPath string `yaml:"credentialsPath"`
}

type MailSelectorConfig struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`         // "subjectRegex" | "bodyRegex" | "attachmentNameRegex" | "senderRegex"
	Pattern      string `yaml:"pattern"`      // regex pattern
	CaptureGroup int    `yaml:"captureGroup"` // default 0 (full match)
	Scope        bool   `yaml:"scope"`        // default false
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

func NewConfigsFromYaml(yamlBytes []byte) (*[]Config, error) {
	var configs []Config
	err := yaml.Unmarshal(yamlBytes, &configs)
	if err != nil {
		return nil, err
	}

	configs = *setDefaults(&configs)
	if err := validateConfigs(&configs); err != nil {
		return nil, err
	}
	return &configs, nil
}

func setDefaults(input *[]Config) (output *[]Config) {
	output = input
	for i, config := range *output {
		if config.Callback.Timeout == "" {
			(*output)[i].Callback.Timeout = "24s"
		}
		// CaptureGroup and Scope default via zero-values; nothing to set here
	}
	return output
}

func validateConfigs(configs *[]Config) error {
	for _, config := range *configs {
		if err := validateConfig(&config); err != nil {
			return err
		}
	}
	return nil
}

func validateConfig(config *Config) error {
	// Validate selectors
	for _, sel := range config.MailSelectors {
		if err := validateMailSelectorConfig(&sel); err != nil {
			return err
		}
	}

	// Validate callback
	if err := validateCallback(&config.Callback); err != nil {
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

	// type: support "subjectRegex", "bodyRegex", "attachmentNameRegex", and "senderRegex"
	if sel.Type != "subjectRegex" && sel.Type != "bodyRegex" && sel.Type != "attachmentNameRegex" && sel.Type != "senderRegex" {
		return fmt.Errorf("mailSelectors.type not supported: '%s' (supported: 'subjectRegex','bodyRegex','attachmentNameRegex','senderRegex')", sel.Type)
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
	// MaxSize must be >= 0
	if att.MaxSize < 0 {
		return fmt.Errorf("callback.attachments.maxSize must be >= 0")
	}
	return nil
}
