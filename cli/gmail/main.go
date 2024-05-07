package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"golang.org/x/oauth2"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Printf("provide path to client credentials in quotes")
		return
	}
	generateToken(args[1])
}

func generateToken(pathToClientCredentials string) {
	config, err := mail.GetGmailConfig(pathToClientCredentials)
	if err != nil {
		fmt.Printf("%v", err.Error())
		return
	}

	token, err := getTokenFromWeb(context.Background(), config)
	if err != nil {
		fmt.Printf("%v", err.Error())
		return
	}
	err = saveToken(path.Join(pathToClientCredentials, mail.TokenFileName), token)
	if err != nil {
		fmt.Printf("%v", err.Error())
		return
	}
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(context context.Context, config *oauth2.Config) (*oauth2.Token, error) {
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

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
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
