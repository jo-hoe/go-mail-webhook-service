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
	"sync/atomic"
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
					{Name: "subjectScope", Type: "subjectRegex", Pattern: "^includethis$"},
				}),
			},
			want: []mail.Mail{
				{Subject: "includethis"},
			},
		},
		{
			name: "no selectors returns empty result",
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

func Test_processOneMail_requestBodyTemplating(t *testing.T) {
	testMethod := "POST"
	testURL := "http://example.com"
	expectedBody := `{"testKey":"testValue"}`

	var gotMethod, gotURL, gotBody string
	var gotHeaders http.Header

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
			URL:    testURL,
			Method: testMethod,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"testKey":"{{ .testKey }}"}`,
		},
	}
	selected := map[string]string{"testKey": "testValue"}

	var fc atomic.Int64
	processOneMail(context.Background(), client, mock, m, cfg, selected, &fc)

	if gotMethod != testMethod {
		t.Errorf("method = %s, want %s", gotMethod, testMethod)
	}
	if gotURL != testURL {
		t.Errorf("url = %s, want %s", gotURL, testURL)
	}
	if gotBody != expectedBody {
		t.Errorf("body = %s, want %s", gotBody, expectedBody)
	}
	if gotHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", gotHeaders.Get("Content-Type"))
	}
}

func Test_processOneMail(t *testing.T) {
	var logBuffer bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug})))

	allProtos, err := selector.NewSelectorPrototypes([]config.MailSelectorConfig{
		{Name: "subjectScope", Type: "subjectRegex", Pattern: "testSubject"},
		{Name: "testKey", Type: "bodyRegex", Pattern: "testValue"},
	})
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}

	tests := []struct {
		name           string
		m              mail.Mail
		cfg            *config.Config
		wantSuccessLog bool
	}{
		{
			name: "matching mail is processed and logged",
			m:    mail.Mail{Subject: "testSubject", Body: "testValue"},
			cfg: &config.Config{
				Callback: goback.Config{URL: "http://example.com", Method: "POST"},
			},
			wantSuccessLog: true,
		},
		{
			name: "non-matching mail body skips processing",
			m:    mail.Mail{Subject: "testSubject", Body: "noMatch"},
			cfg: &config.Config{
				Callback: goback.Config{URL: "http://example.com", Method: "POST"},
			},
			wantSuccessLog: false,
		},
	}

	okClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := selectMailValues(tt.m, allProtos)
			if err != nil {
				if tt.wantSuccessLog {
					t.Errorf("selectMailValues() unexpected error: %v", err)
				}
				return
			}
			var fc atomic.Int64
			processOneMail(context.Background(), okClient, &mail.MailClientServiceMock{}, tt.m, tt.cfg, selected, &fc)
			if tt.wantSuccessLog && !strings.Contains(logBuffer.String(), "successfully processed mail") {
				t.Errorf("expected success log; got: %s", logBuffer.String())
			}
		})
	}
}

func Test_processMails(t *testing.T) {
	var logBuffer bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug})))

	tests := []struct {
		name        string
		cfg         *config.Config
		mailService mail.MailClientService
	}{
		{
			name: "mails are fetched and selector count is logged",
			mailService: &mail.MailClientServiceMock{
				Mails: []mail.Mail{
					{Subject: "testSubject", Body: "testValue"},
				},
			},
			cfg: &config.Config{
				MailSelectors: []config.MailSelectorConfig{
					{Name: "subjectScope", Type: "subjectRegex", Pattern: "testSubject"},
					{Name: "testKey", Type: "bodyRegex", Pattern: "testValue"},
				},
				Callback: goback.Config{URL: "http://example.com", Method: "POST"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: &http.Transport{}}
			var fc atomic.Int64
			processMails(context.Background(), client, tt.cfg, tt.mailService, &fc)

			allMails, err := tt.mailService.GetAllUnreadMail(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			expectedLog := fmt.Sprintf("count=%d", len(allMails))
			if !strings.Contains(logBuffer.String(), expectedLog) {
				t.Errorf("expected log containing %q; got: %s", expectedLog, logBuffer.String())
			}
		})
	}
}

func Test_truncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "shorter than limit",
			input:  "testValue",
			maxLen: 100,
			want:   "testValue",
		},
		{
			name:   "exactly at limit",
			input:  createString('a', 100),
			maxLen: 100,
			want:   createString('a', 100),
		},
		{
			name:   "over limit gets ellipsis",
			input:  createString('a', 200),
			maxLen: 100,
			want:   createString('a', 100) + "...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.input, tt.maxLen); got != tt.want {
				t.Errorf("truncate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createString(ch rune, n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteRune(ch)
	}
	return sb.String()
}

func mustPrototypes(t *testing.T, cfgs []config.MailSelectorConfig) []selector.SelectorPrototype {
	t.Helper()
	protos, err := selector.NewSelectorPrototypes(cfgs)
	if err != nil {
		t.Fatalf("failed to build selector prototypes: %v", err)
	}
	return protos
}