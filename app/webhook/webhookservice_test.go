package webhook

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

func Test_filterMailsBySubject(t *testing.T) {
	type args struct {
		mails []mail.Mail
		regex string
	}
	tests := []struct {
		name string
		args args
		want []mail.Mail
	}{
		{
			name: "filter mails by subject",
			args: args{
				mails: []mail.Mail{
					{
						Subject: "includethis",
					}, {
						Subject: "donotincludethis",
					},
				},
				regex: "^includethis$",
			},
			want: []mail.Mail{
				{
					Subject: "includethis",
				},
			},
		},
		{
			name: "filter mails with invalid regex",
			args: args{
				mails: []mail.Mail{
					{
						Subject: "",
					},
				},
				regex: "(invalidRegex",
			},
			want: make([]mail.Mail, 0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterMailsBySubject(tt.args.mails, tt.args.regex); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterMailsBySubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_selectFromBody(t *testing.T) {
	type args struct {
		mail      mail.Mail
		selectors []config.BodySelectorRegex
	}
	tests := []struct {
		name       string
		args       args
		wantResult map[string]string
	}{
		{
			name: "convert to map",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				selectors: []config.BodySelectorRegex{
					{
						Regex: "testValue",
						Name:  "testKey",
					},
					{
						Regex: "(invalidRegex",
						Name:  "invalidRegex",
					},
				},
			},
			wantResult: map[string]string{
				"testKey": "testValue",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := selectFromBody(tt.args.mail, tt.args.selectors); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("selectFromBody() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func Test_getRequestBody(t *testing.T) {
	type args struct {
		mail      mail.Mail
		selectors []config.BodySelectorRegex
	}
	tests := []struct {
		name       string
		args       args
		wantResult []byte
	}{
		{
			name: "get body",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				selectors: []config.BodySelectorRegex{
					{
						Regex: "testValue",
						Name:  "testKey",
					},
				},
			},
			wantResult: []byte("{\"testKey\":\"testValue\"}"),
		},
		{
			name: "get body without selectors",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				selectors: []config.BodySelectorRegex{},
			},
			wantResult: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := getRequestBody(tt.args.mail, tt.args.selectors); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("getRequestBody() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func Test_constructRequest(t *testing.T) {
	testMethod := "POST"
	testUrl := "http://example.com"
	testBody := []byte("{\"testKey\":\"testValue\"}")
	testRequestWithoutBody, err := http.NewRequest(testMethod, testUrl, nil)
	if err != nil {
		t.Error(err)
	}

	testRequestWithBody, err := http.NewRequest(testMethod, testUrl, bytes.NewReader(testBody))
	if err != nil {
		t.Error(err)
	}
	testRequestWithBody.Header.Set("Content-Type", "application/json")

	type args struct {
		mail   mail.Mail
		config *config.Config
	}
	tests := []struct {
		name    string
		args    args
		want    *http.Request
		wantErr bool
	}{
		{
			name: "construct request without body",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				config: &config.Config{
					BodySelectorRegexList: nil,
					Callback: config.Callback{
						Url:    testUrl,
						Method: testMethod,
					},
				},
			},
			want:    testRequestWithoutBody,
			wantErr: false,
		}, {
			name: "construct request",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				config: &config.Config{
					BodySelectorRegexList: []config.BodySelectorRegex{
						{
							Regex: "testValue",
							Name:  "testKey",
						},
					},
					Callback: config.Callback{
						Url:    testUrl,
						Method: testMethod,
					},
				},
			},
			want:    testRequestWithBody,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := constructRequest(tt.args.mail, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("constructRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Method != tt.want.Method {
				t.Errorf("constructRequest() got = %v, want %v", got.Method, tt.want.Method)
			}
			if got.URL.String() != tt.want.URL.String() {
				t.Errorf("constructRequest() got = %v, want %v", got.URL.String(), tt.want.URL.String())
			}
			if !reflect.DeepEqual(got.Header, tt.want.Header) {
				t.Errorf("constructRequest() got = %v, want %v", got.Header, tt.want.Header)
			}
			if !reflect.DeepEqual(got.Body, tt.want.Body) {
				t.Errorf("constructRequest() got = %v, want %v", got.Body, tt.want.Body)
			}
		})
	}
}

func Test_processMail(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer func() {
		log.SetOutput(os.Stderr)
	}()
	t.Log(logBuffer.String())

	type args struct {
		ctx         context.Context
		client      *http.Client
		mailService mail.MailClientService
		mail        mail.Mail
		config      *config.Config
		wantErrLog  bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "process mail",
			args: args{
				ctx: context.Background(),
				client: &http.Client{
					Transport: &http.Transport{},
				},
				mailService: &mail.MailClientServiceMock{},
				mail: mail.Mail{
					Subject: "testSubject",
					Body:    "testValue",
				},
				config: &config.Config{
					SubjectSelectorRegex: "testSubject",
					BodySelectorRegexList: []config.BodySelectorRegex{
						{
							Regex: "testValue",
							Name:  "testKey",
						},
					},
					Callback: config.Callback{
						Url:    "http://example.com",
						Method: "POST",
					},
				},
				wantErrLog: false,
			},
		},
	}
	for _, tt := range tests {
		var wg sync.WaitGroup
		wg.Add(1)
		t.Run(tt.name, func(t *testing.T) {
			processMail(tt.args.ctx, tt.args.client, tt.args.mailService, tt.args.mail, tt.args.config, &wg)
		})
		bufferString := logBuffer.String()
		if tt.args.wantErrLog && len(bufferString) == 0 {
			t.Error("Did not find expected log")
		}
		if !tt.args.wantErrLog && len(bufferString) > 0 {
			t.Error("Found unexpected log")
		}
	}
}

func Test_processMails(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer func() {
		log.SetOutput(os.Stderr)
	}()
	t.Log(logBuffer.String())

	type args struct {
		ctx         context.Context
		client      *http.Client
		config      *config.Config
		mailService mail.MailClientService
		wantErrLog  bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "process mails",
			args: args{
				ctx: context.Background(),
				client: &http.Client{
					Transport: &http.Transport{},
				},
				mailService: &mail.MailClientServiceMock{
					ReturnErrorsOnly: false,
					Mails: []mail.Mail{
						{
							Subject: "testSubject",
							Body:    "testValue",
						},
					},
				},
				config: &config.Config{
					SubjectSelectorRegex: "testSubject",
					BodySelectorRegexList: []config.BodySelectorRegex{
						{
							Regex: "testValue",
							Name:  "testKey",
						},
					},
				},
				wantErrLog: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processMails(tt.args.ctx, tt.args.client, tt.args.config, tt.args.mailService)
		})
		bufferString := logBuffer.String()
		if tt.args.wantErrLog && len(bufferString) == 0 {
			t.Error("Did not find expected log")
		}
		if !tt.args.wantErrLog && len(bufferString) > 0 {
			t.Error("Found unexpected log")
		}
	}
}

func Test_getPrefix(t *testing.T) {
	type args struct {
		input string
		length int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "get short prefix",
			args: args{
				input: "testValue",
				length: 100,
			},
			want: "testValue",
		}, {
			name: "exactly on limit",
			args: args{
				input: createString('a', 100),
				length: 100,
			},
			want: createString('a', 100),
		}, {
			name: "over limit",
			args: args{
				input: createString('a', 200),
				length: 100,
			},
			want: fmt.Sprintf("%s...", createString('a', 100)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPrefix(tt.args.input, tt.args.length); got != tt.want {
				t.Errorf("getPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createString(character rune, length int) string {
	var sb strings.Builder
	for i := 0; i < length; i++ {
		sb.WriteRune(character)
	}
	return sb.String()
}
