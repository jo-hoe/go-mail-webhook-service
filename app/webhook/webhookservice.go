package webhook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/jo-hoe/goback"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
)

// defaultSuccessStatusCodes contains the HTTP 2xx–3xx range treated as success.
var defaultSuccessStatusCodes = func() []int {
	codes := make([]int, 0, 200)
	for c := 200; c < 400; c++ {
		codes = append(codes, c)
	}
	return codes
}()

// selectedMail pairs a mail with the values extracted by the matching selectors.
type selectedMail struct {
	Mail     mail.Mail
	Selected map[string]string
}

// WebhookService orchestrates mail fetching, selector evaluation, and webhook delivery.
type WebhookService struct {
	config *config.Config
}

// NewWebhookService creates a WebhookService for the given configuration.
func NewWebhookService(cfg *config.Config) *WebhookService {
	return &WebhookService{config: cfg}
}

// Run fetches unread mails, evaluates selectors, dispatches webhooks, and returns the failure count.
func (s *WebhookService) Run() int {
	mailService, err := mail.NewMailClientService(mail.GmailClientType)
	if err != nil {
		slog.Error("could not create mail service", "error", err)
		return 0
	}
	var failureCount atomic.Int64
	processMails(context.Background(), nil, s.config, mailService, &failureCount)
	return int(failureCount.Load())
}

func processMails(ctx context.Context, client *http.Client, cfg *config.Config, mailService mail.MailClientService, failureCounter *atomic.Int64) {
	slog.Info("start reading mails")
	allMails, err := mailService.GetAllUnreadMail(ctx)
	if err != nil {
		slog.Error("error reading mails", "error", err)
		return
	}
	slog.Info("unread mails fetched", "count", len(allMails))

	prototypes, err := selector.NewSelectorPrototypes(cfg.MailSelectors)
	if err != nil {
		slog.Error("could not build selector prototypes", "error", err)
		return
	}
	if len(prototypes) == 0 {
		slog.Warn("no selectors configured; no mails will be processed")
	}

	matched := filterMailsBySelectors(allMails, prototypes)
	slog.Info("mails matching all selectors", "count", len(matched))

	var wg sync.WaitGroup
	for _, sm := range matched {
		wg.Add(1)
		go func(m mail.Mail, sel map[string]string) {
			defer wg.Done()
			processOneMail(ctx, client, mailService, m, cfg, sel, failureCounter)
		}(sm.Mail, sm.Selected)
	}
	wg.Wait()
}

func processOneMail(
	ctx context.Context,
	client *http.Client,
	mailService mail.MailClientService,
	m mail.Mail,
	cfg *config.Config,
	selected map[string]string,
	failureCounter *atomic.Int64,
) {
	strategy := NewAttachmentDeliveryStrategy(cfg.Attachments.Strategy)
	slog.Info("start processing mail", "mailId", m.Id, "subject", m.Subject, "body_prefix", truncate(m.Body, 100), "received_at", m.ReceivedAt)
	for _, req := range strategy.BuildRequests(cfg.Callback, cfg, m, selected) {
		if len(req.ExpectedStatus) == 0 {
			req.ExpectedStatus = defaultSuccessStatusCodes
		}
		if err := sendRequest(ctx, client, req, selected, m); err != nil {
			failureCounter.Add(1)
			return
		}
	}
	applyProcessedAction(ctx, mailService, m, cfg.Processing.ProcessedAction)
	slog.Info("successfully processed mail", "mailId", m.Id)
}

func applyProcessedAction(ctx context.Context, mailService mail.MailClientService, m mail.Mail, actionName string) {
	action, err := mail.NewProcessedAction(actionName)
	if err != nil {
		slog.Error("invalid processed action; falling back to markRead", "configured", actionName, "error", err)
		action, _ = mail.NewProcessedAction("markRead")
	}
	if err := action.Apply(ctx, mailService, m); err != nil {
		slog.Error("could not apply processed action", "action", action.Name(), "mailId", m.Id, "error", err)
	}
}

func sendRequest(ctx context.Context, client *http.Client, h goback.Config, selected map[string]string, m mail.Mail) error {
	exec, err := goback.NewCallbackExecutor(h, client)
	if err != nil {
		slog.Error("could not create webhook executor", "mailId", m.Id, "error", err)
		return err
	}
	resp, _, err := exec.Execute(ctx, goback.TemplateData{Values: selected})
	if err != nil {
		slog.Error("webhook execution failed", "mailId", m.Id, "error", err)
		return err
	}
	if resp != nil {
		slog.Info("webhook request sent", "mailId", m.Id, "status_code", resp.StatusCode, "method", h.Method, "url", h.URL)
	}
	return nil
}

// selectMailValues evaluates every prototype against m and returns the collected values.
// Returns an error as soon as any selector does not match or fails.
func selectMailValues(m mail.Mail, prototypes []selector.SelectorPrototype) (map[string]string, error) {
	result := make(map[string]string, len(prototypes))
	for _, proto := range prototypes {
		sel := proto.NewInstance()
		v, err := sel.SelectValue(m)
		if err != nil {
			if errors.Is(err, selector.ErrNotMatched) {
				slog.Info("selector not matched", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id)
			} else {
				slog.Error("selector evaluation failed", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id, "error", err)
			}
			return nil, fmt.Errorf("selector %q did not apply: %w", sel.Name(), err)
		}
		slog.Info("selector matched", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id)
		result[sel.Name()] = v
	}
	return result, nil
}

func filterMailsBySelectors(mails []mail.Mail, prototypes []selector.SelectorPrototype) []selectedMail {
	if len(prototypes) == 0 {
		return nil
	}
	result := make([]selectedMail, 0, len(mails))
	for _, m := range mails {
		if selected, err := selectMailValues(m, prototypes); err == nil {
			result = append(result, selectedMail{Mail: m, Selected: selected})
		}
	}
	return result
}

// truncate returns input truncated to maxLen characters with a "..." suffix when truncated.
func truncate(input string, maxLen int) string {
	if len(input) <= maxLen {
		return input
	}
	return input[:maxLen] + "..."
}