package config

import (
	"reflect"
	"testing"

	"github.com/jo-hoe/goback"
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
	}{
		{
			name: "positive test with hook config",
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
  timeout: "8s"
processing:
  processedAction: "markRead"
`),
			},
			want: &Config{
				LogLevel: "info",
				MailClient: MailClient{
					Gmail: GmailClient{Enabled: true},
				},
				MailSelectors: []MailSelectorConfig{
					{Name: "subjectScope", Type: "subjectRegex", Pattern: ".*", CaptureGroup: 0},
					{Name: "test", Type: "bodyRegex", Pattern: "[a-z]{0,6}", CaptureGroup: 0},
					{Name: "test2", Type: "bodyRegex", Pattern: ".*", CaptureGroup: 0},
				},
				Callback: goback.Config{
					URL:    "https://example.com/callback",
					Method: "POST",
					Timeout: "8s",
				},
				Attachments: AttachmentsConfig{
					FieldPrefix:  "attachment",
					MaxSize:      "",
					MaxSizeBytes: 0,
				},
				Processing: Processing{
					ProcessedAction: "markRead",
				},
			},
			wantErr: false,
		},
		{
			name: "negative test invalid yaml",
			args: args{
				yamlBytes: []byte(`invalid yaml`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test defaults",
			args: args{
				yamlBytes: []byte(`
mailSelectors:
- name: "subjectScope"
  type: "subjectRegex"
  pattern: ".*"
callback:
  url: "https://example.com/callback"
`),
			},
			want: &Config{
				LogLevel: "info",
				MailClient: MailClient{
					Gmail: GmailClient{Enabled: true},
				},
				MailSelectors: []MailSelectorConfig{
					{Name: "subjectScope", Type: "subjectRegex", Pattern: ".*", CaptureGroup: 0},
				},
				Callback: goback.Config{
					URL: "https://example.com/callback",
				},
				Attachments: AttachmentsConfig{
					FieldPrefix:  "attachment",
					MaxSize:      "",
					MaxSizeBytes: 0,
				},
				Processing: Processing{
					ProcessedAction: "markRead",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConfigFromYaml(tt.args.yamlBytes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewConfigFromYaml() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			// Compare fields that are deterministic and relevant
			if got.LogLevel != tt.want.LogLevel {
				t.Errorf("LogLevel = %v, want %v", got.LogLevel, tt.want.LogLevel)
			}
			if !reflect.DeepEqual(got.MailClient, tt.want.MailClient) {
				t.Errorf("MailClient = %v, want %v", got.MailClient, tt.want.MailClient)
			}
			if !reflect.DeepEqual(got.MailSelectors, tt.want.MailSelectors) {
				t.Errorf("MailSelectors = %v, want %v", got.MailSelectors, tt.want.MailSelectors)
			}
			if got.Callback.URL != tt.want.Callback.URL || got.Callback.Method != tt.want.Callback.Method || got.Callback.Timeout != tt.want.Callback.Timeout {
				t.Errorf("Callback = %+v, want URL=%s Method=%s Timeout=%s", got.Callback, tt.want.Callback.URL, tt.want.Callback.Method, tt.want.Callback.Timeout)
			}
			if !reflect.DeepEqual(got.Attachments, tt.want.Attachments) {
				t.Errorf("Attachments = %+v, want %+v", got.Attachments, tt.want.Attachments)
			}
			if got.Processing.ProcessedAction != tt.want.Processing.ProcessedAction {
				t.Errorf("ProcessedAction = %s, want %s", got.Processing.ProcessedAction, tt.want.Processing.ProcessedAction)
			}
		})
	}
}