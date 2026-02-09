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

// CallbackField describes how to contribute to the outgoing request.
type CallbackField struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`  // "jsonValue" | "headerValue" | "queryParamValue"
	Value string `yaml:"value"` // may contain placeholders like ${selectorName}
}

func validateCallbackFields(fields []CallbackField) error {
	for _, f := range fields {
		switch f.Type {
		case "jsonValue", "queryParamValue":
			// Name must be alphanumeric
			nameRegex := regexp.MustCompile("^[0-9A-Za-z]+$")
			if !nameRegex.MatchString(f.Name) {
				return fmt.Errorf("callback.fields[%s] invalid name for type %s: must match ^[0-9A-Za-z]+$", f.Name, f.Type)
			}
		case "headerValue":
			// Allow hyphens in header names
			nameRegex := regexp.MustCompile("^[0-9A-Za-z-]+$")
			if !nameRegex.MatchString(f.Name) {
				return fmt.Errorf("callback.fields[%s] invalid header name: must match ^[0-9A-Za-z-]+$", f.Name)
			}
		default:
			return fmt.Errorf("callback.fields[%s] invalid type: %s (supported: jsonValue, headerValue, queryParamValue)", f.Name, f.Type)
		}
		// f.Value can be any string; placeholders are validated at runtime
	}
	return nil
}

type Config struct {
	MailClientConfig          MailClientConfig        `yaml:"mailClientConfig"`
	MailSelectors             []MailSelectorConfig    `yaml:"mailSelectors"`
	Callback                  Callback                `yaml:"callback"`
	IntervalBetweenExecutions string                  `yaml:"intervalBetweenExecutions"` // default is "0s"
	RunOnce                   bool                    `yaml:"runOnce"`
}

type MailClientConfig struct {
	Mail            string `yaml:"mail"`
	CredentialsPath string `yaml:"credentialsPath"`
}

type MailSelectorConfig struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`         // "subjectRegex" | "bodyRegex"
	Pattern      string `yaml:"pattern"`      // regex pattern
	CaptureGroup int    `yaml:"captureGroup"` // default 0 (full match)
	Scope        bool   `yaml:"scope"`        // default false
}

type Callback struct {
	Url     string          `yaml:"url"`
	Method  string          `yaml:"method"`
	Timeout string          `yaml:"timeout"` // default is "24s"
	Retries int             `yaml:"retries"` // default is "0"
	Fields  []CallbackField `yaml:"fields"`
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
		if config.IntervalBetweenExecutions == "" {
			(*output)[i].IntervalBetweenExecutions = "0s"
		}
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

	// Validate interval
	if _, err := time.ParseDuration(config.IntervalBetweenExecutions); err != nil {
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

	// type: only "subjectRegex" or "bodyRegex" supported currently
	if sel.Type != "subjectRegex" && sel.Type != "bodyRegex" {
		return fmt.Errorf("mailSelectors.type not supported: '%s' (supported: 'subjectRegex','bodyRegex')", sel.Type)
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

	// Validate fields, if provided
	if err := validateCallbackFields(callback.Fields); err != nil {
		return err
	}

	return nil
}
