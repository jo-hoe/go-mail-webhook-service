package mail

import (
	"reflect"
	"testing"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
)

func TestNewMailClientService(t *testing.T) {
	type args struct {
		mailClientConfig *config.MailClientConfig
	}
	tests := []struct {
		name    string
		args    args
		want    MailClientService
		wantErr bool
	}{
		{
			name: "generate gmail service",
			args: args{
				mailClientConfig: &config.MailClientConfig{
					Mail:     "test@gmail.com",
					CredentialsPath: "testPath",
				},
			},
			want:    &GmailService{
				credentialsPath: "testPath",
			},
			wantErr: false,
		}, {
			name: "call with unsupported mail address",
			args: args{
				mailClientConfig: &config.MailClientConfig{Mail: "unsupported@unsupported.com"},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMailClientService(tt.args.mailClientConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMailClientService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMailClientService() = %v, want %v", got, tt.want)
			}
		})
	}
}
