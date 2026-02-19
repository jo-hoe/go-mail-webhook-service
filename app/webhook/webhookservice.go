package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	callbackField "github.com/jo-hoe/go-mail-webhook-service/app/callbackfield"

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

func processWebhook(config *config.Config) {
	var clientType string
	if config.MailClient.Gmail.Enabled {
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

	client, err := createHttpClient(config)
	if err != nil {
		slog.Error("could not create http client", "error", err)
		return
	}

	processMails(context.Background(), client, config, mailService)
}

func createHttpClient(config *config.Config) (client *http.Client, err error) {
	timeout, err := time.ParseDuration(config.Callback.Timeout)
	if err != nil {
		return nil, err
	}
	client = &http.Client{
		Timeout: timeout,
	}

	return client, nil
}

func processMails(ctx context.Context, client *http.Client, config *config.Config, mailService mail.MailClientService) {
	slog.Info("start reading mails")
	allMails, err := mailService.GetAllUnreadMail(ctx)
	if err != nil {
		slog.Error("error while reading all mails", "error", err)
		return
	}
	// Log the total count of unread mails before applying any selectors
	slog.Info(fmt.Sprintf("number of unread mails is: %d", len(allMails)))

	allProtos, err := buildSelectorPrototypes(config)
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
		go processMail(ctx, client, mailService, sm.Mail, config, sm.Selected, &wg)
	}
	wg.Wait()
}

func processMail(ctx context.Context, client *http.Client, mailService mail.MailClientService,
	m mail.Mail, config *config.Config, selected map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	var lastErr error
	for attempts := 0; attempts < config.Callback.Retries+1; attempts++ {
		// Reconstruct request for each attempt so the body reader is fresh
		request, err := constructRequest(m, config, selected)
		if err != nil {
			slog.Error("could not create request", "mailId", m.Id, "error", err)
			return
		}

		lastErr = sendRequest(request, client, m.Id)
		if lastErr == nil {
			// Success: apply processed action and log success
			action, aErr := mail.NewProcessedAction(config.Processing.ProcessedAction)
			if aErr != nil {
				slog.Error("invalid processed action; falling back to markRead", "configured", config.Processing.ProcessedAction, "error", aErr)
				action, _ = mail.NewProcessedAction("markRead")
			}
			if err := action.Apply(ctx, mailService, m); err != nil {
				slog.Error("could not apply processed action", "action", action.Name(), "error", err, "mailId", m.Id)
			}
			slog.Info("successfully processed mail", "mailId", m.Id, "subject", m.Subject, "body_prefix", getPrefix(m.Body, 100))
			return
		}
		slog.Error("could not send request", "mailId", m.Id, "attempt", attempts+1, "max_attempts", config.Callback.Retries+1, "error", lastErr)
	}

	// Exhausted retries: do not mark as read
	slog.Warn("exhausted retries for mail; leaving unread", "mailId", m.Id, "subject", m.Subject)
}

func getPrefix(input string, prefixLength int) string {
	if len(input) > prefixLength {
		return fmt.Sprintf("%s...", input[0:prefixLength])
	}
	return input
}

func sendRequest(request *http.Request, client *http.Client, mailId string) error {
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	// Ensure response body is closed to avoid resource leaks and enable connection reuse
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			slog.Warn("failed to close response body", "mailId", mailId, "error", cerr)
		}
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("status: %d - %s for request: %s - %s", resp.StatusCode, resp.Status, request.Method, request.URL.String())
	} else {
		slog.Info("request success", "mailId", mailId, "status_code", resp.StatusCode, "method", request.Method, "url", request.URL.String())
	}

	return nil
}

func constructRequest(m mail.Mail, cfg *config.Config, selected map[string]string) (request *http.Request, err error) {
	// Start with a base request without body; we'll attach body/headers/query per sections
	request, err = http.NewRequest(cfg.Callback.Method, cfg.Callback.Url, nil)
	if err != nil {
		return nil, err
	}
	// Ensure header map exists before setting any headers
	if request.Header == nil {
		request.Header = make(http.Header)
	}

	// Apply query parameters
	q := request.URL.Query()
	for _, kv := range cfg.Callback.QueryParams {
		v := callbackField.ExpandPlaceholders(kv.Value, selected)
		q.Add(kv.Key, v)
	}
	request.URL.RawQuery = q.Encode()

	// Apply headers
	for _, kv := range cfg.Callback.Headers {
		v := callbackField.ExpandPlaceholders(kv.Value, selected)
		request.Header.Set(kv.Key, v)
	}

	// Determine whether to build multipart/form-data:
	// - if form fields exist
	// - or if attachments forwarding is enabled and there are attachments
	attachmentsEnabled := cfg.Callback.Attachments.Enabled
	hasAttachments := attachmentsEnabled && len(m.Attachments) > 0
	if len(cfg.Callback.Form) > 0 || hasAttachments {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)

		// Write configured form fields
		for _, kv := range cfg.Callback.Form {
			v := callbackField.ExpandPlaceholders(kv.Value, selected)
			_ = w.WriteField(kv.Key, v)
		}

		// Append attachments if enabled
		if hasAttachments {
			prefix := cfg.Callback.Attachments.FieldPrefix
			maxSizeBytes := cfg.Callback.Attachments.MaxSizeBytes

			for i, a := range m.Attachments {
				// Enforce max size if configured
				if maxSizeBytes > 0 && int64(len(a.Content)) > maxSizeBytes {
					slog.Warn("skipping attachment due to size limit", "name", a.Name, "size_bytes", len(a.Content), "max_bytes", maxSizeBytes)
					continue
				}

				// Field name and filename (always original when present)
				fieldName := fmt.Sprintf("%s_%d", prefix, i)
				filename := a.Name
				if filename == "" {
					filename = fmt.Sprintf("%s_%d", prefix, i)
				}
				// Sanitize filename to base name
				filename = filepath.Base(filename)

				fw, err := w.CreateFormFile(fieldName, filename)
				if err != nil {
					return nil, err
				}
				if _, err := fw.Write(a.Content); err != nil {
					return nil, err
				}
			}
		}

		if err := w.Close(); err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(bytes.NewReader(body.Bytes()))
		request.ContentLength = int64(body.Len())
		// Set Content-Type with boundary if not already provided
		if request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", w.FormDataContentType())
		}
		return request, nil
	}

	// Attach raw body if provided
	if cfg.Callback.Body != "" {
		bodyStr := callbackField.ExpandPlaceholders(cfg.Callback.Body, selected)
		bodyBytes := []byte(bodyStr)
		request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		request.ContentLength = int64(len(bodyBytes))
		// Do not set Content-Type automatically for raw body; user can supply via headers
	}

	return request, nil
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

func buildSelectorPrototypes(config *config.Config) ([]selector.SelectorPrototype, error) {
	return selector.NewSelectorPrototypes(config.MailSelectors)
}
