package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"mime/multipart"
	"github.com/jo-hoe/go-mail-webhook-service/app/callbackfield"
	"sync"
	"time"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
)

type WebhookService struct {
	configs *[]config.Config
}

func NewWebhookService(configs *[]config.Config) *WebhookService {
	return &WebhookService{
		configs: configs,
	}
}

func (webhookService *WebhookService) Run() {
	for _, config := range *webhookService.configs {
		go createWebhook(&config)
	}
}

func createWebhook(config *config.Config) {
	for {
		processWebhook(config)
		wait(config.IntervalBetweenExecutions)

		if config.RunOnce {
			log.Print("'runOnce' is set to true. exiting")
			break
		}
	}
}

func processWebhook(config *config.Config) {
	mailService, err := mail.NewMailClientService(&config.MailClientConfig)
	if err != nil {
		log.Printf("could not create mail service - error: %s", err)
	}

	client, err := createHttpClient(config)
	if err != nil {
		log.Printf("could not create http client - error: %s", err)
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

func wait(durationString string) {
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		log.Printf("could parse time - error: %s", err)
		return
	}
	time.Sleep(duration)
}

func processMails(ctx context.Context, client *http.Client, config *config.Config, mailService mail.MailClientService) {
	log.Print("start reading mails\n")
	allMails, err := mailService.GetAllUnreadMail(ctx)
	if err != nil {
		log.Printf("read all mails - error: %s", err)
		return
	}

	scopeProtos, nonScopeProtos, err := buildSelectorPrototypes(config)
	if err != nil {
		log.Printf("could not build selector prototypes - error: %s", err)
		return
	}

	filteredMails := filterMailsByScopeSelectors(allMails, scopeProtos)
	log.Printf("number of unread mails that are in scope is: %d\n", len(filteredMails))

	var wg sync.WaitGroup
	// build combined prototypes for placeholder evaluation (scope + non-scope)
	allProtos := make([]selector.SelectorPrototype, 0, len(scopeProtos)+len(nonScopeProtos))
	allProtos = append(allProtos, scopeProtos...)
	allProtos = append(allProtos, nonScopeProtos...)

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
		log.Printf("could not create request - error: %s", err)
		return
	}

	for range config.Callback.Retries + 1 {
		err = sendRequest(request, client)
		if err == nil {
			break
		}
		log.Printf("could not send request - error: %s", err)
	}

	err = mailService.MarkMailAsRead(ctx, m)
	if err != nil {
		log.Printf("could not mark mails as read - error: %s", err)
	}

	log.Printf("successfully processed mail with subject: '%s' and body: '%s'\n", m.Subject, getPrefix(m.Body, 100))
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
		return fmt.Errorf("status code: %d for request: %s - %s", resp.StatusCode, request.Method, request.URL.String())
	} else {
		log.Printf(
			"status code: %d for request: %s - %s\n",
			resp.StatusCode,
			request.Method,
			request.URL.String())
	}

	return nil
}

func constructRequest(m mail.Mail, config *config.Config, allProtos []selector.SelectorPrototype) (request *http.Request, err error) {
	// Evaluate all selectors to build placeholder map
	selected := evaluateSelectors(m, allProtos)

	// Build field prototypes and accumulate request parts
	fieldProtos, err := callbackfield.NewFieldPrototypes(config.Callback.Fields)
	if err != nil {
		return nil, err
	}
	parts := callbackfield.NewRequestBuildParts()
	for _, p := range fieldProtos {
		f := p.NewInstance()
		f.Apply(selected, parts)
	}

	// Start with a base request without body; we'll attach body/headers/query per fields
	request, err = http.NewRequest(config.Callback.Method, config.Callback.Url, nil)
	if err != nil {
		return nil, err
	}
	// Ensure header map exists before setting any headers
	if request.Header == nil {
		request.Header = make(http.Header)
	}

	// Apply query parameters
	q := request.URL.Query()
	for name, values := range parts.Query {
		for _, v := range values {
			q.Add(name, v)
		}
	}
	request.URL.RawQuery = q.Encode()

	// Apply headers
	for name, value := range parts.Headers {
		request.Header.Set(name, value)
	}

	// If form fields exist, build multipart/form-data body
	if len(parts.Form) > 0 {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		for name, values := range parts.Form {
			for _, v := range values {
				_ = w.WriteField(name, v)
			}
		}
		// Also include JSON fields as simple form fields if any
		for name, value := range parts.JSON {
			_ = w.WriteField(name, value)
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

	// Attach JSON body if any jsonValue fields were provided
	if len(parts.JSON) > 0 {
		bodyBytes, mErr := json.Marshal(parts.JSON)
		if mErr != nil {
			return nil, mErr
		}
		request.Body = ioNopCloser(bytes.NewReader(bodyBytes))
		request.ContentLength = int64(len(bodyBytes))
		// Ensure Content-Type is set for JSON body if not provided by fields
		if request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", "application/json")
		}
	}

	return request, nil
}

func getRequestBody(m mail.Mail, nonScopeProtos []selector.SelectorPrototype) (result []byte) {
	data := collectSelectorValues(m, nonScopeProtos)
	if len(data) == 0 {
		return result
	}

	result, err := json.Marshal(data)
	if err != nil {
		log.Printf("could not marshal data - error: %s", err)
		result = make([]byte, 0)
	}

	return result
}

func collectSelectorValues(m mail.Mail, nonScopeProtos []selector.SelectorPrototype) map[string]string {
	result := map[string]string{}

	if len(nonScopeProtos) == 0 {
		return result
	}

	for _, proto := range nonScopeProtos {
		sel := proto.NewInstance()
		// only non-scope selectors should be in this slice already, but keep check for safety
		if sel.IsScope() {
			continue
		}
		if sel.Evaluate(m) {
			if v := sel.SelectedValue(); v != "" {
				result[sel.Name()] = v
			}
		}
	}

	return result
}

func filterMailsByScopeSelectors(mails []mail.Mail, scopeProtos []selector.SelectorPrototype) []mail.Mail {
	result := make([]mail.Mail, 0)

	// If no scope selectors defined, process nothing by default
	if len(scopeProtos) == 0 {
		return result
	}

	for _, m := range mails {
		inScope := true
		for _, proto := range scopeProtos {
			sel := proto.NewInstance()
			// ensure it's a scope selector
			if !sel.IsScope() {
				continue
			}
			if !sel.Evaluate(m) {
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

func buildSelectorPrototypes(config *config.Config) (scope []selector.SelectorPrototype, nonScope []selector.SelectorPrototype, err error) {
	all, err := selector.NewSelectorPrototypes(config.MailSelectors)
	if err != nil {
		return nil, nil, err
	}

	scope = make([]selector.SelectorPrototype, 0)
	nonScope = make([]selector.SelectorPrototype, 0)
	for _, p := range all {
		inst := p.NewInstance()
		if inst.IsScope() {
			scope = append(scope, p)
		} else {
			nonScope = append(nonScope, p)
		}
	}
	return scope, nonScope, nil
}

// evaluateSelectors runs all selector prototypes against a mail and returns matched values by selector name.
func evaluateSelectors(m mail.Mail, allProtos []selector.SelectorPrototype) map[string]string {
	result := map[string]string{}
	for _, proto := range allProtos {
		sel := proto.NewInstance()
		if sel.Evaluate(m) {
			if v := sel.SelectedValue(); v != "" {
				result[sel.Name()] = v
			}
		}
	}
	return result
}


// ioNopCloser wraps a Reader to satisfy io.ReadCloser without allocating a full NopCloser dependency.
type nopCloser struct{ *bytes.Reader }

func (nc nopCloser) Close() error { return nil }

func ioNopCloser(r *bytes.Reader) nopCloser {
	return nopCloser{r}
}
