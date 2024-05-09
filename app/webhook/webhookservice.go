package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
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
	filteredMails := filterMailsBySubject(allMails, config.SubjectSelectorRegex)
	log.Printf("number of mails that fit to subject selector '%s' is: %d\n", config.SubjectSelectorRegex, len(filteredMails))

	var wg sync.WaitGroup
	for _, mail := range filteredMails {
		wg.Add(1)
		go processMail(ctx, client, mailService, mail, config, &wg)
	}
	wg.Wait()
}

func processMail(ctx context.Context, client *http.Client, mailService mail.MailClientService,
	mail mail.Mail, config *config.Config, wg *sync.WaitGroup) {
	defer wg.Done()

	request, err := constructRequest(mail, config)
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

	err = mailService.MarkMailAsRead(ctx, mail)
	if err != nil {
		log.Printf("could not mark mails as read - error: %s", err)
	}

	log.Printf("successfully processed mail with subject: '%s' and body: '%s'\n", mail.Subject, getPrefix(mail.Body, 100))
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

func constructRequest(mail mail.Mail, config *config.Config) (request *http.Request, err error) {
	bodyBytes := getRequestBody(mail, config.BodySelectorRegexList)
	if len(bodyBytes) > 0 {
		request, err = http.NewRequest(config.Callback.Method, config.Callback.Url, bytes.NewReader(bodyBytes))
		if err == nil {
			request.Header.Set("Content-Type", "application/json")
		}
	} else {
		request, err = http.NewRequest(config.Callback.Method, config.Callback.Url, nil)
	}
	return request, err
}

func getRequestBody(mail mail.Mail, selectors []config.BodySelectorRegex) (result []byte) {
	data := selectFromBody(mail, selectors)

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

func selectFromBody(mail mail.Mail, selectors []config.BodySelectorRegex) (result map[string]string) {
	result = map[string]string{}

	if len(selectors) == 0 {
		return result
	}

	for _, bodySelectorRegex := range selectors {
		regex, err := regexp.Compile(bodySelectorRegex.Regex)
		if err != nil {
			log.Printf("regex: %s cannot be compiled. error: %s", bodySelectorRegex.Regex, err)
			continue
		}
		result[bodySelectorRegex.Name] = regex.FindString(mail.Body)
	}

	return result
}

func filterMailsBySubject(mails []mail.Mail, regex string) []mail.Mail {
	result := make([]mail.Mail, 0)

	for _, mail := range mails {
		match, err := regexp.MatchString(regex, mail.Subject)
		if err != nil {
			log.Printf("regex: %s cannot be compiled. error: %s", regex, err)
			continue
		}
		if match {
			result = append(result, mail)
		}
	}

	return result
}
