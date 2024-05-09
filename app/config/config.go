package config

import (
	"fmt"
	"regexp"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	MailClientConfig          MailClientConfig    `yaml:"mailClientConfig"`
	SubjectSelectorRegex      string              `yaml:"subjectSelectorRegex"`
	BodySelectorRegexList     []BodySelectorRegex `yaml:"bodySelectorRegexList"`
	Callback                  Callback            `yaml:"callback"`
	IntervalBetweenExecutions string              `yaml:"intervalBetweenExecutions"`
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
	Timeout string `yaml:"timeout"`
	Retries int    `yaml:"retries"`
}

func NewConfigsFromYaml(yamlBytes []byte) (*[]Config, error) {
	var configs []Config
	err := yaml.Unmarshal(yamlBytes, &configs)
	if err != nil {
		return nil, err
	}
	if err := validateConfigs(&configs); err != nil {
		return nil, err
	}
	return &configs, nil
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
	if callback.Method != "POST" && callback.Method != "GET" && callback.Method != "PUT" && callback.Method != "DELETE" {
		return fmt.Errorf("callback.method not supported: %s", callback.Method)
	}

	if _, err := time.ParseDuration(callback.Timeout); err != nil {
		return err
	}

	if callback.Retries < 0 {
		return fmt.Errorf("callback.retries must be greater than or equal to 0")
	}
	
	return nil
}
