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

	"github.com/jo-hoe/gohook"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
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

func Test_processMail(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Log(logBuffer.String())

	// Build selector prototypes
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

	tests := []struct {
		name            string
		mail            mail.Mail
		cfg             *config.Config
		client          *http.Client
		wantSuccessLog  bool
		shouldCallHook  bool
	}{
		{
			name: "process mail",
			mail: mail.Mail{
				Subject: "testSubject",
				Body:    "testValue",
			},
			cfg: &config.Config{
				Callback: gohook.Config{
					URL:    "http://example.com",
					Method: "POST",
				},
			},
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
			wantSuccessLog: true,
			shouldCallHook: true,
		},
		{
			name: "process mail without body selector values",
			mail: mail.Mail{
				Subject: "testSubject",
				Body:    "noMatch",
			},
			cfg: &config.Config{
				Callback: gohook.Config{
					URL:    "http://example.com",
					Method: "POST",
				},
			},
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
			wantSuccessLog: false,
			shouldCallHook: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := evaluateSelectorsCore(tt.mail, allProtos, true)
			var wg sync.WaitGroup
			if err == nil && tt.shouldCallHook {
				exec, err := gohook.NewHookExecutor(tt.cfg.Callback, tt.client)
				if err != nil {
					t.Fatalf("failed to create gohook executor: %v", err)
				}
				wg.Add(1)
				processMail(context.Background(), exec, &mail.MailClientServiceMock{}, tt.mail, tt.cfg, selected, &wg)
				wg.Wait()
			}
			bufferString := logBuffer.String()
			if tt.wantSuccessLog && !strings.Contains(bufferString, "successfully processed mail") {
				t.Errorf("Did not find expected log, log was'%s'", bufferString)
			}
		})
	}
}

func Test_processMails(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	t.Log(logBuffer.String())

	type args struct {
		ctx         context.Context
		exec        gohook.HookExecutor
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
				exec: func() gohook.HookExecutor {
					client := &http.Client{Transport: &http.Transport{}}
					exec, err := gohook.NewHookExecutor(gohook.Config{
						URL:    "http://example.com",
						Method: "POST",
					}, client)
					if err != nil {
						t.Fatalf("failed to create gohook executor: %v", err)
					}
					return exec
				}(),
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
					Callback: gohook.Config{
						URL:    "http://example.com",
						Method: "POST",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processMails(tt.args.ctx, tt.args.exec, tt.args.config, tt.args.mailService)
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