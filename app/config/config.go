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

type Config struct {
	MailClientConfig          MailClientConfig    `yaml:"mailClientConfig"`
	SubjectSelectorRegex      string              `yaml:"subjectSelectorRegex"`
	BodySelectorRegexList     []BodySelectorRegex `yaml:"bodySelectorRegexList"`
	Callback                  Callback            `yaml:"callback"`
	IntervalBetweenExecutions string              `yaml:"intervalBetweenExecutions"` // default is "0s"
	RunOnce                   bool                `yaml:"runOnce"`
}

type MailClientConfig struct {
	Mail            string `yaml:"mail"`
	CredentialsPath string `yaml:"credentialsPath"`
}

type BodySelectorRegex struct {
	Name  string `yaml:"name"`
	Regex string `yaml:"regex"`
}

type Callback struct {
	Url     string `yaml:"url"`
	Method  string `yaml:"method"`
	Timeout string `yaml:"timeout"` // default is "24s"
	Retries int    `yaml:"retries"` // default is "0"
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
	_, err := regexp.Compile(config.SubjectSelectorRegex)
	if err != nil {
		return err
	}
	for _, bodySelectorRegex := range config.BodySelectorRegexList {
		_, err := regexp.Compile(bodySelectorRegex.Regex)
		if err != nil {
			return err
		}
	}

	if err := validateCallback(&config.Callback); err != nil {
		return err
	}

	if _, err = time.ParseDuration(config.IntervalBetweenExecutions); err != nil {
		return err
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

	return nil
}
