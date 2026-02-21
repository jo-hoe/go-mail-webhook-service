package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
	"github.com/jo-hoe/goback"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func Test_filterMailsBySelectors(t *testing.T) {
	type args struct {
		mails  []mail.Mail
		protos []selector.SelectorPrototype
	}
	tests := []struct {
		name string
		args args
		want []mail.Mail
	}{
		{
			name: "filter mails by subject selector",
			args: args{
				mails: []mail.Mail{
					{Subject: "includethis"},
					{Subject: "donotincludethis"},
				},
				protos: mustPrototypes(t, []config.MailSelectorConfig{
					{
						Name:    "subjectScope",
						Type:    "subjectRegex",
						Pattern: "^includethis$",
					},
				}),
			},
			want: []mail.Mail{
				{Subject: "includethis"},
			},
		},
		{
			name: "no selectors -> empty result",
			args: args{
				mails:  []mail.Mail{{Subject: "anything"}},
				protos: []selector.SelectorPrototype{},
			},
			want: []mail.Mail{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSelected := filterMailsBySelectors(tt.args.mails, tt.args.protos)
			got := make([]mail.Mail, len(gotSelected))
			for i, sm := range gotSelected {
				got[i] = sm.Mail
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterMailsBySelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_processMail_requestBodyTemplating(t *testing.T) {
	testMethod := "POST"
	testUrl := "http://example.com"
	expectedBody := "{\"testKey\":\"testValue\"}"

	// Capture sent request details
	var gotMethod string
	var gotURL string
	var gotHeaders http.Header
	var gotBody string

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotMethod = req.Method
			gotURL = req.URL.String()
			gotHeaders = req.Header.Clone()
			if req.Body != nil {
				b, _ := io.ReadAll(req.Body)
				gotBody = string(b)
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	mock := &mail.MailClientServiceMock{}
	m := mail.Mail{Subject: "s", Body: "b"}

	cfg := &config.Config{
		Callback: goback.Config{
			URL:    testUrl,
			Method: testMethod,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: "{\"testKey\":\"{{ .testKey }}\"}",
		},
	}

	selected := map[string]string{"testKey": "testValue"}

	var wg sync.WaitGroup
	wg.Add(1)
	processMail(context.Background(), client, mock, m, cfg, selected, &wg)
	wg.Wait()

	if gotMethod != testMethod {
		t.Errorf("method = %s, want %s", gotMethod, testMethod)
	}
	if gotURL != testUrl {
		t.Errorf("url = %s, want %s", gotURL, testUrl)
	}
	if gotBody != expectedBody {
		t.Errorf("body = %s, want %s", gotBody, expectedBody)
	}
	// Optional: check header presence
	if gotHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header = %s, want application/json", gotHeaders.Get("Content-Type"))
	}
}


func Test_processMail(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Log(logBuffer.String())

	// Build non-scope prototypes for body
	allProtos, err := buildSelectorPrototypes(&config.Config{
		MailSelectors: []config.MailSelectorConfig{
			{
				Name:    "subjectScope",
				Type:    "subjectRegex",
				Pattern: "testSubject",
			},
			{
				Name:    "testKey",
				Type:    "bodyRegex",
				Pattern: "testValue",
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
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(strings.NewReader("")),
							Header:     make(http.Header),
							Request:    req,
						}, nil
					}),
				},
				mailService: &mail.MailClientServiceMock{},
				mail: mail.Mail{
					Subject: "testSubject",
					Body:    "testValue",
				},
				config: &config.Config{
					Callback: goback.Config{
						URL:    "http://example.com",
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
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(strings.NewReader("")),
							Header:     make(http.Header),
							Request:    req,
						}, nil
					}),
				},
				mailService: &mail.MailClientServiceMock{},
				mail: mail.Mail{
					Subject: "testSubject",
					Body:    "noMatch",
				},
				config: &config.Config{
					Callback: goback.Config{
						URL:    "http://example.com",
						Method: "POST",
					},
				},
				wantSuccessLog: false,
			},
		},
	}
	for _, tt := range tests {
		var wg sync.WaitGroup
		selected, err := evaluateSelectorsCore(tt.args.mail, allProtos, true)
		if err == nil {
			wg.Add(1)
			processMail(tt.args.ctx, tt.args.client, tt.args.mailService, tt.args.mail, tt.args.config, selected, &wg)
			wg.Wait()
		}
		bufferString := logBuffer.String()
		if tt.args.wantSuccessLog && !strings.Contains(bufferString, "successfully processed mail") {
			t.Errorf("Did not find expected log, log was'%s'", bufferString)
		}
	}
}

func Test_processMails(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
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
						},
						{
							Name:    "testKey",
							Type:    "bodyRegex",
							Pattern: "testValue",
						},
					},
					Callback: goback.Config{
						URL:    "http://example.com",
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
		expectedLog := fmt.Sprintf("number of unread mails matching all selectors is: %d", len(numberOfUnreadMails))
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

func mustPrototypes(t *testing.T, cfgs []config.MailSelectorConfig) []selector.SelectorPrototype {
	all, err := selector.NewSelectorPrototypes(cfgs)
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}
	return all
}
