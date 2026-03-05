package webhook

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/goback"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func successHTTPClient() *http.Client {
	return &http.Client{
		Transport: rtFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
}

func Test_processOneMail_ProcessedAction_markRead(t *testing.T) {
	mock := &mail.MailClientServiceMock{}
	cfg := &config.Config{
		Callback:   goback.Config{URL: "http://example.com", Method: "POST"},
		Processing: config.Processing{ProcessedAction: "markRead"},
	}

	var fc atomic.Int64
	processOneMail(context.Background(), successHTTPClient(), mock, mail.Mail{Subject: "s", Body: "b"}, cfg, map[string]string{}, &fc)

	if mock.MarkReadCalls != 1 {
		t.Fatalf("MarkMailAsRead called %d times, want 1", mock.MarkReadCalls)
	}
	if mock.DeleteCalls != 0 {
		t.Fatalf("DeleteMail called %d times, want 0", mock.DeleteCalls)
	}
}

func Test_processOneMail_ProcessedAction_delete(t *testing.T) {
	mock := &mail.MailClientServiceMock{}
	cfg := &config.Config{
		Callback:   goback.Config{URL: "http://example.com", Method: "POST"},
		Processing: config.Processing{ProcessedAction: "delete"},
	}

	var fc atomic.Int64
	processOneMail(context.Background(), successHTTPClient(), mock, mail.Mail{Subject: "s", Body: "b"}, cfg, map[string]string{}, &fc)

	if mock.DeleteCalls != 1 {
		t.Fatalf("DeleteMail called %d times, want 1", mock.DeleteCalls)
	}
	if mock.MarkReadCalls != 0 {
		t.Fatalf("MarkMailAsRead called %d times, want 0", mock.MarkReadCalls)
	}
}