package config

import (
	"gopkg.in/yaml.v2"
)

type Config struct {
	MailServiceConfig     MailServiceConfig   `yaml:"mailServiceConfig"`
	SubjectSelectorRegex  string              `yaml:"subjectSelectorRegex"`
	BodySelectorRegexList []BodySelectorRegex `yaml:"bodySelectorRegexList"`
	Callback              Callback            `yaml:"callback"`
}

type MailServiceConfig struct {
	Mail            string `yaml:"mail"`
	CredentialsPath string `yaml:"credentialsPath"`
}

type BodySelectorRegex struct {
	Name  string `yaml:"name"`
	Regex string `yaml:"regex"`
}

type Callback struct {
	Url    string `yaml:"url"`
	Method string `yaml:"method"`
}

func NewConfigsFromYaml(yamlBytes []byte) (*[]Config, error) {
	var configs []Config
	err := yaml.Unmarshal(yamlBytes, &configs)
	if err != nil {
		return nil, err
	}
	return &configs, nil
}
