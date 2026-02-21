package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

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

	client, err := createHttpClient(cfg)
	if err != nil {
		slog.Error("could not create http client", "error", err)
		return
	}

	processMails(context.Background(), client, cfg, mailService)
}

func createHttpClient(cfg *config.Config) (client *http.Client, err error) {
	timeout, err := time.ParseDuration(cfg.Callback.Timeout)
	if err != nil {
		return nil, err
	}
	client = &http.Client{
		Timeout: timeout,
	}

	return client, nil
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
	exec, err := gohook.NewHookExecutor(hookCfg, client)
	if err != nil {
		slog.Error("could not create webhook executor", "mailId", m.Id, "error", err)
		return
	}

	resp, _, err := exec.Execute(ctx, gohook.TemplateData{Values: selected})
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

// buildHookConfig maps our config into a gohook Config, including attachments/form mapping.
// ExpectedStatus is set to accept 2xx and 3xx to mirror previous behavior of treating >= 400 as error.
func buildHookConfig(cfg *config.Config, m mail.Mail) gohook.Config {
	headers := make(map[string]string, len(cfg.Callback.Headers))
	for _, kv := range cfg.Callback.Headers {
		headers[kv.Key] = kv.Value
	}
	query := make(map[string]string, len(cfg.Callback.QueryParams))
	for _, kv := range cfg.Callback.QueryParams {
		query[kv.Key] = kv.Value
	}

	var mp *gohook.Multipart
	if len(cfg.Callback.Form) > 0 || (cfg.Callback.Attachments.Enabled && len(m.Attachments) > 0) {
		fields := make(map[string]string, len(cfg.Callback.Form))
		for _, kv := range cfg.Callback.Form {
			fields[kv.Key] = kv.Value
		}
		files := buildAttachmentFiles(cfg, m)
		mp = &gohook.Multipart{
			Fields: fields,
			Files:  files,
		}
	}

	return gohook.Config{
		URL:            cfg.Callback.Url,
		Method:         cfg.Callback.Method,
		Headers:        headers,
		Query:          query,
		Body:           cfg.Callback.Body,
		Multipart:      mp,
		Timeout:        cfg.Callback.Timeout,
		StrictTemplates: false, // previous placeholder expansion tolerated missing keys
		ExpectedStatus: successStatusCodes(),
		MaxRetries:     cfg.Callback.Retries,
		Backoff:        "", // no backoff in legacy config
	}
}

func buildAttachmentFiles(cfg *config.Config, m mail.Mail) []gohook.ByteFile {
	if !cfg.Callback.Attachments.Enabled || len(m.Attachments) == 0 {
		return nil
	}
	prefix := cfg.Callback.Attachments.FieldPrefix
	if prefix == "" {
		prefix = "attachment"
	}
	maxSize := cfg.Callback.Attachments.MaxSizeBytes

	files := make([]gohook.ByteFile, 0, len(m.Attachments))
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
		files = append(files, gohook.ByteFile{
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