package config

import (
	"reflect"
	"testing"

	"github.com/jo-hoe/gohook"
)

func TestNewConfig(t *testing.T) {
	type args struct {
		yamlBytes []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *Config
		wantErr bool
	}{{
		name: "positive test",
		args: args{
			yamlBytes: []byte(`
mailSelectors:
- name: "subjectScope"
  type: "subjectRegex"
  pattern: ".*"
- name: "test"
  type: "bodyRegex"
  pattern: "[a-z]{0,6}"
- name: "test2"
  type: "bodyRegex"
  pattern: ".*"
callback:
  url: "https://example.com/callback"
  method: "POST"
  timeout: 8s
  maxRetries: 10`),
		},
		want: &Config{
			LogLevel: "info",
			MailClient: MailClient{
				Gmail: GmailClient{
					Enabled: true,
				},
			},
			MailSelectors: []MailSelectorConfig{
				{
					Name:         "subjectScope",
					Type:         "subjectRegex",
					Pattern:      ".*",
					CaptureGroup: 0,
				},
				{
					Name:         "test",
					Type:         "bodyRegex",
					Pattern:      "[a-z]{0,6}",
					CaptureGroup: 0,
				},
				{
					Name:         "test2",
					Type:         "bodyRegex",
					Pattern:      ".*",
					CaptureGroup: 0,
				},
			},
			Callback: gohook.Config{
				URL:       "https://example.com/callback",
				Method:    "POST",
				Timeout:   "8s",
				MaxRetries: 10,
			},
			Processing: Processing{
				ProcessedAction: "markRead",
			},
		},
		wantErr: false,
	}, {
		name: "negative test - missing callback url",
		args: args{
			yamlBytes: []byte(`
mailSelectors:
- name: "subjectScope"
  type: "subjectRegex"
  pattern: ".*"
callback:
  method: "POST"`),
		},
		want:    nil,
		wantErr: true,
	}, {
		name: "test defaults",
		args: args{
			yamlBytes: []byte(`
mailSelectors:
- name: "subjectScope"
  type: "subjectRegex"
  pattern: ".*"
callback:
  url: "https://example.com/callback"
  method: "POST"`),
		},
		want: &Config{
			LogLevel: "info",
			MailClient: MailClient{
				Gmail: GmailClient{
					Enabled: true,
				},
			},
			MailSelectors: []MailSelectorConfig{
				{
					Name:         "subjectScope",
					Type:         "subjectRegex",
					Pattern:      ".*",
					CaptureGroup: 0,
				},
			},
			Callback: gohook.Config{
				URL:     "https://example.com/callback",
				Method:  "POST",
				Timeout: "24s",
			},
			Processing: Processing{
				ProcessedAction: "markRead",
			},
		},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConfigFromYaml(tt.args.yamlBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigFromYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConfigFromYaml() = %#v, want %#v", got, tt.want)
			}
		})
	}
}