package webhook

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jo-hoe/gohook"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
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

func Test_processMail_ProcessedAction_markRead(t *testing.T) {
	ctx := context.Background()
	client := successHTTPClient()
	mock := &mail.MailClientServiceMock{}
	m := mail.Mail{Subject: "s", Body: "b"}
	cfg := &config.Config{
		Callback: gohook.Config{
			URL:    "http://example.com",
			Method: "POST",
		},
		Processing: config.Processing{
			ProcessedAction: "markRead",
		},
	}

	exec, err := gohook.NewHookExecutor(cfg.Callback, client)
	if err != nil {
		t.Fatalf("failed to create gohook executor: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	processMail(ctx, exec, mock, m, cfg, map[string]string{}, &wg)
	wg.Wait()

	if mock.MarkReadCalls != 1 {
		t.Fatalf("expected MarkMailAsRead to be called once, got %d", mock.MarkReadCalls)
	}
	if mock.DeleteCalls != 0 {
		t.Fatalf("expected DeleteMail to not be called, got %d", mock.DeleteCalls)
	}
}

func Test_processMail_ProcessedAction_delete(t *testing.T) {
	ctx := context.Background()
	client := successHTTPClient()
	mock := &mail.MailClientServiceMock{}
	m := mail.Mail{Subject: "s", Body: "b"}
	cfg := &config.Config{
		Callback: gohook.Config{
			URL:    "http://example.com",
			Method: "POST",
		},
		Processing: config.Processing{
			ProcessedAction: "delete",
		},
	}

	exec, err := gohook.NewHookExecutor(cfg.Callback, client)
	if err != nil {
		t.Fatalf("failed to create gohook executor: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	processMail(ctx, exec, mock, m, cfg, map[string]string{}, &wg)
	wg.Wait()

	if mock.DeleteCalls != 1 {
		t.Fatalf("expected DeleteMail to be called once, got %d", mock.DeleteCalls)
	}
	if mock.MarkReadCalls != 0 {
		t.Fatalf("expected MarkMailAsRead to not be called, got %d", mock.MarkReadCalls)
	}
}