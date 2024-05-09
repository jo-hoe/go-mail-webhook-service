package config

import (
	"reflect"
	"testing"
)

func TestNewConfig(t *testing.T) {
	type args struct {
		yamlBytes []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *[]Config
		wantErr bool
	}{
		{
			name: "positive test",
			args: args{
				yamlBytes: []byte(`
- mailClientConfig: 
    mail: "example@gmail.com"
    credentialsPath: "/path/to/client_secrets/file/"
  subjectSelectorRegex: ".*"
  bodySelectorRegexList:
  - name: "test"
    regex: "[a-z]{0,6}"
  - name: "test2"
    regex: ".*"
  intervalBetweenExecutions: 20s
  callback:
    url: "https://example.com/callback"
    method: "POST"
    timeout: 8s
    retries: 10`),
			},
			want: &[]Config{
				{
					MailClientConfig: MailClientConfig{
						Mail:            "example@gmail.com",
						CredentialsPath: "/path/to/client_secrets/file/",
					},
					IntervalBetweenExecutions: "20s",
					SubjectSelectorRegex:      ".*",
					BodySelectorRegexList: []BodySelectorRegex{
						{
							Name:  "test",
							Regex: "[a-z]{0,6}",
						},
						{
							Name:  "test2",
							Regex: ".*",
						},
					},
					Callback: Callback{
						Url:     "https://example.com/callback",
						Method:  "POST",
						Timeout: "8s",
						Retries: 10,
					},
				},
			},
			wantErr: false,
		}, {
			name: "negative test",
			args: args{
				yamlBytes: []byte(`invalid yaml`),
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "test defaults",
			args: args{
				yamlBytes: []byte(`
- mailClientConfig: 
    mail: "example@gmail.com"
    credentialsPath: "/path/to/client_secrets/file/"
  subjectSelectorRegex: ".*"
  callback:
    url: "https://example.com/callback"
    method: "POST"`),
			},
			want: &[]Config{
				{
					MailClientConfig: MailClientConfig{
						Mail:            "example@gmail.com",
						CredentialsPath: "/path/to/client_secrets/file/",
					},
					IntervalBetweenExecutions: "0s",
					SubjectSelectorRegex:      ".*",
					BodySelectorRegexList:     nil,
					Callback: Callback{
						Url:     "https://example.com/callback",
						Method:  "POST",
						Timeout: "24s",
						Retries: 0,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConfigsFromYaml(tt.args.yamlBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
