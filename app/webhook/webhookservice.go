package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"

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

func (webhookService *WebhookService) Run(ctx context.Context, client *http.Client) {
	var wg sync.WaitGroup
	for _, config := range *webhookService.configs {
		wg.Add(1)
		go createWebhook(ctx, client, &config, &wg)
	}
	wg.Wait()
}

func createWebhook(ctx context.Context, client *http.Client, config *config.Config, wg *sync.WaitGroup) {
	defer wg.Done()
	mailService, err := mail.NewMailClientService(&config.MailClientConfig)
	if err != nil {
		fmt.Println(err)
		return
	}
	processMails(ctx, client, config, mailService)
}

func processMails(ctx context.Context, client *http.Client, config *config.Config, mailService mail.MailClientService) {
	allMails, err := mailService.GetAllUnreadMail(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	filteredMails := filterMailsBySubject(allMails, config.SubjectSelectorRegex)
	if len(filteredMails) != 0 {
		fmt.Printf("%d mails fit to subject selector: '%s'\n", len(filteredMails), config.SubjectSelectorRegex)
	}
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
		fmt.Println(err)
		return
	}
	err = sendRequest(request, client)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = mailService.MarkMailAsRead(ctx, mail)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("successfully processed mail with subject: %s and body: %s\n", mail.Subject, mail.Body)
}

func sendRequest(request *http.Request, client *http.Client) error {
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status code: %d for request: %s - %s", resp.StatusCode, request.Method, request.URL.String())
	} else {
		fmt.Printf(
			"status code: %d for request: %s - %s\n",
			resp.StatusCode,
			request.Method,
			request.URL.String(),
		)
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
		fmt.Println(err)
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
			fmt.Printf("regex: %s cannot be compiled. error: %s", bodySelectorRegex.Regex, err)
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
			fmt.Printf("regex: %s cannot be compiled. error: %s", regex, err)
			continue
		}
		if match {
			result = append(result, mail)
		}
	}

	return result
}
