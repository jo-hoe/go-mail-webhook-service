package webhook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jo-hoe/gohook"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
)

const (
	gmailClientType = "gmail"
)

type WebhookService struct {
	config *config.Config
}

type selectedMail struct {
	Mail     mail.Mail
	Selected map[string]string
}

func NewWebhookService(cfg *config.Config) *WebhookService {
	return &WebhookService{
		config: cfg,
	}
}

func (webhookService *WebhookService) Run() {
	processWebhook(webhookService.config)
}

func processWebhook(cfg *config.Config) {
	var clientType string
	if cfg.MailClient.Gmail.Enabled {
		clientType = gmailClientType
	} else {
		slog.Error("no mail client enabled in configuration")
		return
	}
	mailService, err := mail.NewMailClientService(clientType)
	if err != nil {
		slog.Error("could not create mail service", "error", err)
		return
	}

	// Build a gohook executor using the library's Config directly.
	// Pass nil as client so gohook constructs a default client honoring Timeout/InsecureSkipVerify.
	exec, err := gohook.NewHookExecutor(cfg.Callback, nil)
	if err != nil {
		slog.Error("could not create gohook executor", "error", err)
		return
	}

	processMails(context.Background(), exec, cfg, mailService)
}

func processMails(ctx context.Context, exec gohook.HookExecutor, cfg *config.Config, mailService mail.MailClientService) {
	slog.Info("start reading mails")
	allMails, err := mailService.GetAllUnreadMail(ctx)
	if err != nil {
		slog.Error("error while reading all mails", "error", err)
		return
	}
	// Log the total count of unread mails before applying any selectors
	slog.Info(fmt.Sprintf("number of unread mails is: %d", len(allMails)))

	allProtos, err := buildSelectorPrototypes(cfg)
	if err != nil {
		slog.Error("could not build selector prototypes", "error", err)
		return
	}
	if len(allProtos) == 0 {
		slog.Warn("no selectors configured; no mails will be processed")
	}

	filteredMails := filterMailsBySelectors(allMails, allProtos)
	slog.Info(fmt.Sprintf("number of unread mails matching all selectors is: %d", len(filteredMails)))

	var wg sync.WaitGroup
	for _, sm := range filteredMails {
		wg.Add(1)
		go processMail(ctx, exec, mailService, sm.Mail, cfg, sm.Selected, &wg)
	}
	wg.Wait()
}

func processMail(ctx context.Context, exec gohook.HookExecutor, mailService mail.MailClientService,
	m mail.Mail, cfg *config.Config, selected map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Execute webhook using gohook with template data from selectors
	resp, _, err := exec.Execute(ctx, gohook.TemplateData{Values: selected})
	if err != nil {
		// gohook already handled retries/backoff according to cfg.Callback
		slog.Error("webhook execution failed", "mailId", m.Id, "subject", m.Subject, "error", err)
		return
	}

	// Success: apply processed action and log success
	action, aErr := mail.NewProcessedAction(cfg.Processing.ProcessedAction)
	if aErr != nil {
		// No legacy support needed, but fallback to markRead to be safe
		slog.Error("invalid processed action; falling back to markRead", "configured", cfg.Processing.ProcessedAction, "error", aErr)
		action, _ = mail.NewProcessedAction("markRead")
	}
	if err := action.Apply(ctx, mailService, m); err != nil {
		slog.Error("could not apply processed action", "action", action.Name(), "error", err, "mailId", m.Id)
	}

	// Log request/response meta if available
	method := ""
	url := ""
	status := 0
	if resp != nil {
		status = resp.StatusCode
		if resp.Request != nil {
			method = resp.Request.Method
			if resp.Request.URL != nil {
				url = resp.Request.URL.String()
			}
		}
	}
	slog.Info("successfully processed mail", "mailId", m.Id, "subject", m.Subject, "status_code", status, "method", method, "url", url, "body_prefix", getPrefix(m.Body, 100))
}

func getPrefix(input string, prefixLength int) string {
	if len(input) > prefixLength {
		return fmt.Sprintf("%s...", input[0:prefixLength])
	}
	return input
}

func evaluateSelectorsCore(m mail.Mail, protos []selector.SelectorPrototype, collectValues bool) (map[string]string, error) {
	var result map[string]string
	if collectValues {
		result = map[string]string{}
	}

	for _, proto := range protos {
		sel := proto.NewInstance()
		v, err := sel.SelectValue(m)
		if err != nil {
			// Explicitly log non-match vs other evaluation errors
			if errors.Is(err, selector.ErrNotMatched) {
				slog.Info("selector not matched", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id)
			} else {
				slog.Error("selector evaluation failed", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id, "error", err)
			}
			return nil, fmt.Errorf("selector '%s' did not apply: %w", sel.Name(), err)
		}
		// Matched
		slog.Info("selector matched", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id)

		if collectValues {
			result[sel.Name()] = v
		}
	}

	return result, nil
}

func filterMailsBySelectors(mails []mail.Mail, protos []selector.SelectorPrototype) []selectedMail {
	result := make([]selectedMail, 0)

	// If no selectors defined, process nothing by default
	if len(protos) == 0 {
		return result
	}

	for _, m := range mails {
		if selected, err := evaluateSelectorsCore(m, protos, true); err == nil {
			result = append(result, selectedMail{Mail: m, Selected: selected})
		}
	}

	return result
}

func buildSelectorPrototypes(cfg *config.Config) ([]selector.SelectorPrototype, error) {
	return selector.NewSelectorPrototypes(cfg.MailSelectors)
}