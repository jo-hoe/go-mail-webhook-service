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

// Service orchestrates mail fetching, selector evaluation, and webhook delivery.
type Service struct {
	config *config.Config
}

// NewWebhookService creates a Service for the given configuration.
func NewWebhookService(cfg *config.Config) *Service {
	return &Service{config: cfg}
}

// Run fetches unread mails, evaluates selectors, dispatches webhooks, and returns the failure count.
func (s *Service) Run() int {
	mailService, err := mail.NewMailClientService(mail.GmailClientType)
	if err != nil {
		slog.Error("could not create mail service", "error", err)
		return 0
	}
	var failureCount atomic.Int64
	processMails(context.Background(), nil, s.config, mailService, &failureCount)
	return int(failureCount.Load())
}

func processMails(ctx context.Context, client *http.Client, cfg *config.Config, mailService mail.ClientService, failureCounter *atomic.Int64) {
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
	mailService mail.ClientService,
	m mail.Mail,
	cfg *config.Config,
	selected map[string]string,
	failureCounter *atomic.Int64,
) {
	strategy := NewAttachmentDeliveryStrategy(cfg.Attachments.Strategy)
	slog.Info("start processing mail", "mailId", m.ID, "subject", m.Subject, "body_prefix", truncate(m.Body, 100), "received_at", m.ReceivedAt)
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
	slog.Info("successfully processed mail", "mailId", m.ID)
}

func applyProcessedAction(ctx context.Context, mailService mail.ClientService, m mail.Mail, actionName string) {
	action, err := mail.NewProcessedAction(actionName)
	if err != nil {
		slog.Error("invalid processed action; falling back to markRead", "configured", actionName, "error", err)
		action, _ = mail.NewProcessedAction("markRead")
	}
	if err := action.Apply(ctx, mailService, m); err != nil {
		slog.Error("could not apply processed action", "action", action.Name(), "mailId", m.ID, "error", err)
	}
}

func sendRequest(ctx context.Context, client *http.Client, h goback.Config, selected map[string]string, m mail.Mail) error {
	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	slog.Debug("sending webhook request", "mailId", m.ID, "method", h.Method, "url", h.URL, "templateKeys", keys)
	exec, err := goback.NewCallbackExecutor(h, client)
	if err != nil {
		slog.Error("could not create webhook executor", "mailId", m.ID, "error", err)
		return fmt.Errorf("create webhook executor: %w", err)
	}
	resp, _, err := exec.Execute(ctx, goback.TemplateData{Values: selected})
	if resp != nil && resp.Body != nil {
		if cerr := resp.Body.Close(); cerr != nil {
			slog.Error("error closing webhook response body", "mailId", m.ID, "error", cerr)
		}
	}
	if err != nil {
		slog.Error("webhook execution failed", "mailId", m.ID, "error", err)
		return fmt.Errorf("execute webhook: %w", err)
	}
	if resp != nil {
		slog.Info("webhook request sent", "mailId", m.ID, "status_code", resp.StatusCode, "method", h.Method, "url", h.URL)
	}
	return nil
}

// selectMailValues evaluates every prototype against m and returns the collected values.
// Returns an error as soon as any selector does not match or fails.
func selectMailValues(m mail.Mail, prototypes []selector.Prototype) (map[string]string, error) {
	result := make(map[string]string, len(prototypes))
	for _, proto := range prototypes {
		sel := proto.NewInstance()
		v, err := sel.SelectValue(m)
		if err != nil {
			if errors.Is(err, selector.ErrNotMatched) {
				slog.Info("selector not matched", "name", sel.Name(), "type", sel.Type(), "mailId", m.ID)
			} else {
				slog.Error("selector evaluation failed", "name", sel.Name(), "type", sel.Type(), "mailId", m.ID, "error", err)
			}
			return nil, fmt.Errorf("selector %q did not apply: %w", sel.Name(), err)
		}
		slog.Info("selector matched", "name", sel.Name(), "type", sel.Type(), "mailId", m.ID)
		result[sel.Name()] = v
	}
	return result, nil
}

func filterMailsBySelectors(mails []mail.Mail, prototypes []selector.Prototype) []selectedMail {
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