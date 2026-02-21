package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/jo-hoe/goback"

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

	// Let goback construct its own http.Client honoring cfg.Callback.Timeout and InsecureSkipVerify.
	processMails(context.Background(), nil, cfg, mailService)
}

func processMails(ctx context.Context, client *http.Client, cfg *config.Config, mailService mail.MailClientService) {
	slog.Info("start reading mails")
	allMails, err := mailService.GetAllUnreadMail(ctx)
	if err != nil {
		slog.Error("error while reading all mails", "error", err)
		return
	}
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
		go processMail(ctx, client, mailService, sm.Mail, cfg, sm.Selected, &wg)
	}
	wg.Wait()
}

func processMail(ctx context.Context, client *http.Client, mailService mail.MailClientService,
	m mail.Mail, cfg *config.Config, selected map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	hookCfg := buildHookConfig(cfg, m)
	exec, err := goback.NewCallbackExecutor(hookCfg, client)
	if err != nil {
		slog.Error("could not create webhook executor", "mailId", m.Id, "error", err)
		return
	}

	resp, _, err := exec.Execute(ctx, goback.TemplateData{Values: selected})
	if err != nil {
		slog.Error("webhook execution failed", "mailId", m.Id, "error", err)
		return
	}
	if resp != nil {
		slog.Info("request success", "mailId", m.Id, "status_code", resp.StatusCode, "method", hookCfg.Method, "url", hookCfg.URL)
	}

	// Success: apply processed action and log success
	action, aErr := mail.NewProcessedAction(cfg.Processing.ProcessedAction)
	if aErr != nil {
		slog.Error("invalid processed action; falling back to markRead", "configured", cfg.Processing.ProcessedAction, "error", aErr)
		action, _ = mail.NewProcessedAction("markRead")
	}
	if err := action.Apply(ctx, mailService, m); err != nil {
		slog.Error("could not apply processed action", "action", action.Name(), "error", err, "mailId", m.Id)
	}
	slog.Info("successfully processed mail", "mailId", m.Id, "subject", m.Subject, "body_prefix", getPrefix(m.Body, 100))
}

func getPrefix(input string, prefixLength int) string {
	if len(input) > prefixLength {
		return fmt.Sprintf("%s...", input[0:prefixLength])
	}
	return input
}

// buildHookConfig starts from cfg.Callback and augments it with runtime attachment files.
// When no ExpectedStatus is configured, defaults to 2xx/3xx as success to mirror prior behavior.
func buildHookConfig(cfg *config.Config, m mail.Mail) goback.Config {
	// Start from the configured hook
	h := cfg.Callback

	// Ensure multipart exists when we need to attach fields or files
	if h.Multipart == nil && (cfg.Attachments.Enabled && len(m.Attachments) > 0) {
		h.Multipart = &goback.Multipart{
			Fields: nil,
			Files:  nil,
		}
	}

	// Append files for attachments if enabled
	if h.Multipart != nil && cfg.Attachments.Enabled && len(m.Attachments) > 0 {
		h.Multipart.Files = append(h.Multipart.Files, buildAttachmentFiles(cfg, m)...)
	}

	// Apply a default ExpectedStatus if not set
	if len(h.ExpectedStatus) == 0 {
		h.ExpectedStatus = successStatusCodes()
	}

	return h
}

func buildAttachmentFiles(cfg *config.Config, m mail.Mail) []goback.ByteFile {
	if !cfg.Attachments.Enabled || len(m.Attachments) == 0 {
		return nil
	}
	prefix := cfg.Attachments.FieldPrefix
	if prefix == "" {
		prefix = "attachment"
	}
	maxSize := cfg.Attachments.MaxSizeBytes

	files := make([]goback.ByteFile, 0, len(m.Attachments))
	for i, a := range m.Attachments {
		if maxSize > 0 && int64(len(a.Content)) > maxSize {
			slog.Warn("skipping attachment due to size limit", "name", a.Name, "size_bytes", len(a.Content), "max_bytes", maxSize)
			continue
		}
		field := fmt.Sprintf("%s_%d", prefix, i)
		name := a.Name
		if name == "" {
			name = field
		}
		name = filepath.Base(name)
		files = append(files, goback.ByteFile{
            Field:    field,
            FileName: name,
            Data:     a.Content,
        })
	}
	return files
}

func successStatusCodes() []int {
	// Accept 2xx and 3xx as success (mirror previous behavior that treated >= 400 as error)
	codes := make([]int, 0, 200)
	for c := 200; c < 400; c++ {
		codes = append(codes, c)
	}
	return codes
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
			if err == selector.ErrNotMatched {
				slog.Info("selector not matched", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id)
			} else {
				slog.Error("selector evaluation failed", "name", sel.Name(), "type", sel.Type(), "mailId", m.Id, "error", err)
			}
			return nil, fmt.Errorf("selector '%s' did not apply: %w", sel.Name(), err)
		}
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