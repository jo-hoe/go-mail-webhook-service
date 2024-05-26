package mail

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestIntegrationGmailService_getGmailService(t *testing.T) {
	testRootDirectory, err := os.MkdirTemp(os.TempDir(), "testDir")
	defer os.RemoveAll(testRootDirectory)
	if err != nil {
		t.Error("could not create folder")
	}
	assetsDirectory, err := filepath.Abs(filepath.Join("..", "..", "assets", "test"))
	if err != nil {
		t.Error("could not get resources directory")
	}
	copyFile(filepath.Join(assetsDirectory, CredentialsFileName), filepath.Join(testRootDirectory, CredentialsFileName), t)
	copyFile(filepath.Join(assetsDirectory, TokenFileName), filepath.Join(testRootDirectory, TokenFileName), t)

	type args struct {
		context context.Context
		scope   []string
	}
	tests := []struct {
		name    string
		service *GmailService
		args    args
		wantErr bool
	}{
		{
			name: "Test getGmailService with invalid credentials path",
			service: &GmailService{
				credentialsPath: "invalid",
			},
			args: args{
				context: context.Background(),
				scope:   []string{"https://www.googleapis.com/auth/gmail.readonly"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.service.getGmailService(tt.args.context, tt.args.scope...)
			if tt.wantErr && err == nil {
				t.Errorf("GmailService.getGmailService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == false && got == nil {
				t.Error("GmailService.getGmailService() returned nil")
			}
		})
	}
}

func copyFile(src, dst string, t *testing.T) {
	inputFile, err := os.Open(src)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(dst)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, inputFile)

	if err != nil {
		t.Errorf("Error: %v", err)
	}
}
