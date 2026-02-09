package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
)

func Test_filterMailsByScopeSelectors(t *testing.T) {
	type args struct {
		mails      []mail.Mail
		scopeProtos []selector.SelectorPrototype
	}
	tests := []struct {
		name string
		args args
		want []mail.Mail
	}{
		{
			name: "filter mails by subject scope selector",
			args: args{
				mails: []mail.Mail{
					{Subject: "includethis"},
					{Subject: "donotincludethis"},
				},
				scopeProtos: mustScopePrototypes(t, []config.MailSelectorConfig{
					{
						Name:    "subjectScope",
						Type:    "subjectRegex",
						Pattern: "^includethis$",
						Scope:   true,
					},
				}),
			},
			want: []mail.Mail{
				{Subject: "includethis"},
			},
		},
		{
			name: "no scope selectors -> empty result",
			args: args{
				mails:       []mail.Mail{{Subject: "anything"}},
				scopeProtos: []selector.SelectorPrototype{},
			},
			want: []mail.Mail{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterMailsByScopeSelectors(tt.args.mails, tt.args.scopeProtos); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterMailsByScopeSelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_collectSelectorValues(t *testing.T) {
	type args struct {
		mail          mail.Mail
		nonScopeProtos []selector.SelectorPrototype
	}
	tests := []struct {
		name       string
		args       args
		wantResult map[string]string
	}{
		{
			name: "collect body value to map",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				nonScopeProtos: mustNonScopePrototypes(t, []config.MailSelectorConfig{
					{
						Name:         "testKey",
						Type:         "bodyRegex",
						Pattern:      "testValue",
						CaptureGroup: 0,
						Scope:        false,
					},
				}),
			},
			wantResult: map[string]string{
				"testKey": "testValue",
			},
		},
		{
			name: "collect link to map",
			args: args{
				mail: mail.Mail{
					Body: "https://youtu.be/DucriSA8ukw?feature=shared\r",
				},
				nonScopeProtos: mustNonScopePrototypes(t, []config.MailSelectorConfig{
					{
						Name:    "url",
						Type:    "bodyRegex",
						Pattern: "https?://(www.)?[-a-zA-Z0-9@:%._+~#=]{1,256}.[a-zA-Z0-9()]{1,6}([-a-zA-Z0-9()@:%_+.~#?&//=]*)",
						Scope:   false,
					},
				}),
			},
			wantResult: map[string]string{
				"url": "https://youtu.be/DucriSA8ukw?feature=shared",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := collectSelectorValues(tt.args.mail, tt.args.nonScopeProtos); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("collectSelectorValues() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func Test_getRequestBody(t *testing.T) {
	type args struct {
		mail           mail.Mail
		nonScopeProtos []selector.SelectorPrototype
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
				nonScopeProtos: mustNonScopePrototypes(t, []config.MailSelectorConfig{
					{
						Name:    "testKey",
						Type:    "bodyRegex",
						Pattern: "testValue",
						Scope:   false,
					},
				}),
			},
			wantResult: []byte("{\"testKey\":\"testValue\"}"),
		},
		{
			name: "get body without selectors",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				nonScopeProtos: []selector.SelectorPrototype{},
			},
			wantResult: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := getRequestBody(tt.args.mail, tt.args.nonScopeProtos); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("getRequestBody() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func Test_constructRequest(t *testing.T) {
	testMethod := "POST"
	testUrl := "http://example.com"
	testBody := []byte("{\"testKey\":\"testValue\"}")

	type args struct {
		mail           mail.Mail
		config         *config.Config
		nonScopeProtos []selector.SelectorPrototype
	}
	tests := []struct {
		name         string
		args         args
		wantMethod   string
		wantURL      string
		wantHeaders  http.Header
		wantBodyText string
		wantErr      bool
	}{
		{
			name: "construct request without body",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				config: &config.Config{
					MailClientConfig:          config.MailClientConfig{},
					MailSelectors:             nil,
					IntervalBetweenExecutions: "",
					RunOnce:                   false,
					Callback: config.Callback{
						Url:    testUrl,
						Method: testMethod,
					},
				},
				nonScopeProtos: []selector.SelectorPrototype{},
			},
			wantMethod:   testMethod,
			wantURL:      testUrl,
			wantHeaders:  http.Header{},
			wantBodyText: "",
			wantErr:      false,
		},
		{
			name: "construct request with body",
			args: args{
				mail: mail.Mail{
					Body: "testValue",
				},
				config: &config.Config{
					Callback: config.Callback{
						Url:    testUrl,
						Method: testMethod,
						Headers: []config.KeyValue{
							{Key: "Content-Type", Value: "application/json"},
						},
						Body: "{\"testKey\":\"${testKey}\"}",
					},
				},
				nonScopeProtos: mustNonScopePrototypes(t, []config.MailSelectorConfig{
					{
						Name:    "testKey",
						Type:    "bodyRegex",
						Pattern: "testValue",
						Scope:   false,
					},
				}),
			},
			wantMethod:   testMethod,
			wantURL:      testUrl,
			wantHeaders:  http.Header{"Content-Type": []string{"application/json"}},
			wantBodyText: string(testBody),
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := constructRequest(tt.args.mail, tt.args.config, tt.args.nonScopeProtos)
			if (err != nil) != tt.wantErr {
				t.Errorf("constructRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Method != tt.wantMethod {
				t.Errorf("constructRequest() got method = %v, want %v", got.Method, tt.wantMethod)
			}
			if got.URL.String() != tt.wantURL {
				t.Errorf("constructRequest() got url = %v, want %v", got.URL.String(), tt.wantURL)
			}
			if !headersEqual(got.Header, tt.wantHeaders) {
				t.Errorf("constructRequest() got headers = %v, want %v", got.Header, tt.wantHeaders)
			}
			var bodyText string
			if got.Body != nil {
				b, _ := io.ReadAll(got.Body)
				bodyText = string(b)
			} else {
				bodyText = ""
			}
			if bodyText != tt.wantBodyText {
				t.Errorf("constructRequest() got body = %s, want %s", bodyText, tt.wantBodyText)
			}
		})
	}
}

func headersEqual(a, b http.Header) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func Test_processMail(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer func() {
		log.SetOutput(os.Stderr)
	}()
	t.Log(logBuffer.String())

	// Build non-scope prototypes for body
	_, nonScopeProtos, err := buildSelectorPrototypes(&config.Config{
		MailSelectors: []config.MailSelectorConfig{
			{
				Name:    "subjectScope",
				Type:    "subjectRegex",
				Pattern: "testSubject",
				Scope:   true,
			},
			{
				Name:    "testKey",
				Type:    "bodyRegex",
				Pattern: "testValue",
				Scope:   false,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}

	type args struct {
		ctx            context.Context
		client         *http.Client
		mailService    mail.MailClientService
		mail           mail.Mail
		config         *config.Config
		wantSuccessLog bool
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
					Callback: config.Callback{
						Url:    "http://example.com",
						Method: "POST",
					},
				},
				wantSuccessLog: true,
			},
		}, {
			name: "process mail without body selector values",
			args: args{
				ctx: context.Background(),
				client: &http.Client{
					Transport: &http.Transport{},
				},
				mailService: &mail.MailClientServiceMock{},
				mail: mail.Mail{
					Subject: "testSubject",
					Body:    "noMatch",
				},
				config: &config.Config{
					Callback: config.Callback{
						Url:    "http://example.com",
						Method: "POST",
					},
				},
				wantSuccessLog: true,
			},
		},
	}
	for _, tt := range tests {
		var wg sync.WaitGroup
		wg.Add(1)
		processMail(tt.args.ctx, tt.args.client, tt.args.mailService, tt.args.mail, tt.args.config, nonScopeProtos, &wg)
		wg.Wait()
		bufferString := logBuffer.String()
		if tt.args.wantSuccessLog && !strings.Contains(bufferString, "successfully processed mail") {
			t.Errorf("Did not find expected log, log was'%s'", bufferString)
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
					MailSelectors: []config.MailSelectorConfig{
						{
							Name:    "subjectScope",
							Type:    "subjectRegex",
							Pattern: "testSubject",
							Scope:   true,
						},
						{
							Name:    "testKey",
							Type:    "bodyRegex",
							Pattern: "testValue",
							Scope:   false,
						},
					},
					Callback: config.Callback{
						Url:    "http://example.com",
						Method: "POST",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processMails(tt.args.ctx, tt.args.client, tt.args.config, tt.args.mailService)
		})
		bufferString := logBuffer.String()
		numberOfUnreadMails, err := tt.args.mailService.GetAllUnreadMail(context.Background())
		if err != nil {
			t.Error(err)
		}
		expectedLog := fmt.Sprintf("number of unread mails that are in scope is: %d", len(numberOfUnreadMails))
		if !strings.Contains(bufferString, expectedLog) {
			t.Errorf("Did not find expected log '%s'", expectedLog)
		}
	}
}

func Test_getPrefix(t *testing.T) {
	type args struct {
		input  string
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
				input:  "testValue",
				length: 100,
			},
			want: "testValue",
		}, {
			name: "exactly on limit",
			args: args{
				input:  createString('a', 100),
				length: 100,
			},
			want: createString('a', 100),
		}, {
			name: "over limit",
			args: args{
				input:  createString('a', 200),
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

func mustScopePrototypes(t *testing.T, cfgs []config.MailSelectorConfig) []selector.SelectorPrototype {
	all, err := selector.NewSelectorPrototypes(cfgs)
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}
	scope := make([]selector.SelectorPrototype, 0)
	for _, p := range all {
		if p.NewInstance().IsScope() {
			scope = append(scope, p)
		}
	}
	return scope
}

func mustNonScopePrototypes(t *testing.T, cfgs []config.MailSelectorConfig) []selector.SelectorPrototype {
	all, err := selector.NewSelectorPrototypes(cfgs)
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}
	nonScope := make([]selector.SelectorPrototype, 0)
	for _, p := range all {
		if !p.NewInstance().IsScope() {
			nonScope = append(nonScope, p)
		}
	}
	return nonScope
}