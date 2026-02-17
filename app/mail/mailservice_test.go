package mail

import (
	"reflect"
	"testing"
)

func TestNewMailClientService(t *testing.T) {
	tests := []struct {
		name    string
		want    MailClientService
		wantErr bool
	}{
		{
			name:    "returns gmail service with default credentials path",
			want:    &GmailService{credentialsPath: DefaultCredentialsPath},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMailClientService()
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