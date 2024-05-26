package mail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailService struct {
	credentialsPath string
}

const (
	CredentialsFileName = "client_secret.json"
	TokenFileName       = "request.token"
)

var GMailDomainNames = []string{"googlemail.com", "gmail.com"}

func NewGmailService(credentialsPath string) *GmailService {
	return &GmailService{
		credentialsPath: credentialsPath,
	}
}

func (service *GmailService) GetAllUnreadMail(context context.Context) ([]Mail, error) {
	result := make([]Mail, 0)
	gmailService, err := service.getGmailService(context, gmail.GmailModifyScope)
	if err != nil {
		return result, err
	}

	// Since the client config contains is associated with the mail name
	// 'me' can be used here. This implies that the config file actually
	// does not even need the field 'mailClientConfig.mail'.
	// It is kept in the config.go file to keep the interface generic for
	// other mail clients and authentication mechanisms.
	user := "me"
	listCall := gmailService.Users.Messages.List(user).Q("is:unread")
	resp, err := listCall.Do()
	if err != nil {
		return result, fmt.Errorf("unable to retrieve messages: %v", err)
	}

	for _, message := range resp.Messages {
		fullMessage, err := gmailService.Users.Messages.Get(user, message.Id).Format("full").Do()
		if err != nil {
			return result, err
		}

		subject := extractSubject(fullMessage.Payload.Headers)
		body := extractPlainTextBody(fullMessage.Payload.Parts)

		result = append(result, Mail{
			Id:      message.Id,
			Subject: subject,
			Body:    body,
		})
	}

	return result, nil
}

func (service *GmailService) MarkMailAsRead(context context.Context, mail Mail) error {
	gmailService, err := service.getGmailService(context, gmail.GmailModifyScope)
	if err != nil {
		return err
	}

	req := &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}
	_, err = gmailService.Users.Messages.Modify("me", mail.Id, req).Do()
	if err != nil {
		return err
	}

	return nil
}

func extractSubject(headers []*gmail.MessagePartHeader) string {
	for _, header := range headers {
		if header.Name == "Subject" {
			return header.Value
		}
	}
	return ""
}

func extractPlainTextBody(parts []*gmail.MessagePart) string {
	for _, part := range parts {
		if part.MimeType == "text/plain" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				log.Printf("Error decoding body data: %v", err)
				continue
			}
			return string(data)
		}
		// Handle multipart email: recursively check for plain text
		if len(part.Parts) > 0 {
			body := extractPlainTextBody(part.Parts)
			if body != "" {
				return body
			}
		}
	}
	return ""
}

func (service *GmailService) getGmailService(context context.Context, scope ...string) (*gmail.Service, error) {
	config, err := GetGmailConfig(service.credentialsPath, scope...)
	if err != nil {
		return nil, err
	}

	client, err := service.getClient(context, config)
	if err != nil {
		return nil, err
	}

	return gmail.NewService(context, option.WithHTTPClient(client))
}

func GetGmailConfig(credentialsPath string, scope ...string) (*oauth2.Config, error) {
	b, err := os.ReadFile(path.Join(credentialsPath, CredentialsFileName))
	if err != nil {
		return nil, err
	}

	// If modifying these scopes, delete your previously saved token.json.
	return google.ConfigFromJSON(b, scope...)
}

// Retrieve a token, saves the token, then returns the generated client.
func (service *GmailService) getClient(context context.Context, config *oauth2.Config) (*http.Client, error) {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokenFilePath := path.Join(service.credentialsPath, TokenFileName)
	token, err := tokenFromFile(tokenFilePath)
	if err != nil {
		return nil, err
	}

	// check if token is already expired or about to expire (within minutes)
	if token.Expiry.Before(time.Now().Add(4 * time.Minute)) {
		// refresh token
		token, err = config.TokenSource(context, token).Token()
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(tokenFilePath, []byte(token.AccessToken), 0600)
		if err != nil {
			return nil, err
		}
	}

	return config.Client(context, token), nil
}

// Retrieves a token from a local file.
func tokenFromFile(filePath string) (*oauth2.Token, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	token := &oauth2.Token{}
	err = json.NewDecoder(file).Decode(token)
	return token, err
}
