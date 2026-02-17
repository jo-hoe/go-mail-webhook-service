package mail

import (
	"reflect"
	"testing"
)

func TestNewMailClientService(t *testing.T) {
	tests := []struct {
		name    string
		argType string
		want    MailClientService
		wantErr bool
	}{
		{
			name:    "returns gmail service with default credentials path",
			argType: "gmail",
			want:    &GmailService{credentialsPath: DefaultCredentialsPath},
			wantErr: false,
		},
		{
			name:    "empty type defaults to gmail",
			argType: "",
			want:    &GmailService{credentialsPath: DefaultCredentialsPath},
			wantErr: false,
		},
		{
			name:    "unsupported type returns error",
			argType: "imap",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMailClientService(tt.argType)
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
