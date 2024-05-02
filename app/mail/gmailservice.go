package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"

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
	_, err := service.getGmailService(context, gmail.GmailModifyScope)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (service *GmailService) MarkMailAsRead(context context.Context, mail Mail) error {
	_, err := service.getGmailService(context, gmail.GmailModifyScope)
	if err != nil {
		return err
	}

	return nil
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
		token, err = GetTokenFromWeb(context, config)
		if err != nil {
			return nil, err
		}
		err = SaveToken(tokenFilePath, token)
		if err != nil {
			return nil, err
		}
	}
	return config.Client(context, token), nil
}

// Request a token from the web, then returns the retrieved token.
func GetTokenFromWeb(context context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	fmt.Printf("Enter the authorization code: ")
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, err
	}

	token, err := config.Exchange(context, authCode)
	if err != nil {
		return nil, err
	}
	return token, nil
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

// Saves a token to a file path.
func SaveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	err = json.NewEncoder(file).Encode(token)
	if err != nil {
		return err
	}

	return nil
}
