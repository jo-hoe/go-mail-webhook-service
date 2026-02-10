package webhook

import (
	"bytes"
	"context"
	"fmt"
	callbackField "github.com/jo-hoe/go-mail-webhook-service/app/callbackfield"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
)

type WebhookService struct {
	config *config.Config
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
	mailService, err := mail.NewMailClientService(&config.MailClientConfig)
	if err != nil {
		slog.Error("could not create mail service", "error", err)
	}

	client, err := createHttpClient(config)
	if err != nil {
		slog.Error("could not create http client", "error", err)
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

	allProtos, err := buildSelectorPrototypes(config)
	if err != nil {
		slog.Error("could not build selector prototypes", "error", err)
		return
	}

	filteredMails := filterMailsBySelectors(allMails, allProtos)
	slog.Info(fmt.Sprintf("number of unread mails that are in scope is: %d", len(filteredMails)), "count", len(filteredMails))

	var wg sync.WaitGroup
	for _, m := range filteredMails {
		wg.Add(1)
		go processMail(ctx, client, mailService, m, config, allProtos, &wg)
	}
	wg.Wait()
}

func processMail(ctx context.Context, client *http.Client, mailService mail.MailClientService,
	m mail.Mail, config *config.Config, allProtos []selector.SelectorPrototype, wg *sync.WaitGroup) {
	defer wg.Done()

	request, err := constructRequest(m, config, allProtos)
	if err != nil {
		slog.Error("could not create request", "error", err)
		return
	}

	var lastErr error
	for attempts := 0; attempts < config.Callback.Retries+1; attempts++ {
		lastErr = sendRequest(request, client)
		if lastErr == nil {
			// Success: mark as read and log success
			if err := mailService.MarkMailAsRead(ctx, m); err != nil {
				slog.Error("could not mark mails as read", "error", err, "mailId", m.Id)
			}
			slog.Info("successfully processed mail", "subject", m.Subject, "body_prefix", getPrefix(m.Body, 100))
			return
		}
		slog.Error("could not send request", "attempt", attempts+1, "max_attempts", config.Callback.Retries+1, "error", lastErr)
	}

	// Exhausted retries: do not mark as read
	slog.Warn("exhausted retries for mail; leaving unread", "subject", m.Subject)
}

func getPrefix(input string, prefixLength int) string {
	if len(input) > prefixLength {
		return fmt.Sprintf("%s...", input[0:prefixLength])
	}
	return input
}

func sendRequest(request *http.Request, client *http.Client) error {
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("status: %d - %s for request: %s - %s", resp.StatusCode, resp.Status, request.Method, request.URL.String())
	} else {
		slog.Info("request success", "status_code", resp.StatusCode, "method", request.Method, "url", request.URL.String())
	}

	return nil
}

func constructRequest(m mail.Mail, cfg *config.Config, allProtos []selector.SelectorPrototype) (request *http.Request, err error) {
	// Evaluate all selectors to build placeholder map; require all to apply
	selected, err := evaluateSelectorsStrict(m, allProtos)
	if err != nil {
		return nil, err
	}

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
		request.Body = ioNopCloser(bytes.NewReader(body.Bytes()))
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
		request.Body = ioNopCloser(bytes.NewReader(bodyBytes))
		request.ContentLength = int64(len(bodyBytes))
		// Do not set Content-Type automatically for raw body; user can supply via headers
	}

	return request, nil
}



func filterMailsBySelectors(mails []mail.Mail, protos []selector.SelectorPrototype) []mail.Mail {
	result := make([]mail.Mail, 0)

	// If no selectors defined, process nothing by default
	if len(protos) == 0 {
		return result
	}

	for _, m := range mails {
		inScope := true
		for _, proto := range protos {
			sel := proto.NewInstance()
			if _, err := sel.SelectValue(m); err != nil {
				inScope = false
				break
			}
		}
		if inScope {
			result = append(result, m)
		}
	}

	return result
}

func buildSelectorPrototypes(config *config.Config) ([]selector.SelectorPrototype, error) {
	return selector.NewSelectorPrototypes(config.MailSelectors)
}


// evaluateSelectorsStrict ensures every selector applies; returns error if any selector doesn't match.
func evaluateSelectorsStrict(m mail.Mail, allProtos []selector.SelectorPrototype) (map[string]string, error) {
	result := map[string]string{}
	for _, proto := range allProtos {
		sel := proto.NewInstance()
		v, err := sel.SelectValue(m)
		if err != nil {
			return nil, fmt.Errorf("selector '%s' did not apply: %w", sel.Name(), err)
		}
		result[sel.Name()] = v
	}
	return result, nil
}

// ioNopCloser wraps a Reader to satisfy io.ReadCloser without allocating a full NopCloser dependency.
type nopCloser struct{ *bytes.Reader }

func (nc nopCloser) Close() error { return nil }

func ioNopCloser(r *bytes.Reader) nopCloser {
	return nopCloser{r}
}
