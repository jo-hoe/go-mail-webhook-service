package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Printf("provide path to client credentials in quotes")
		return
	}
	generateToken(args[1])
}

func generateToken(pathToClientCredentials string) {
	config, err := mail.GetGmailConfig(pathToClientCredentials, gmail.GmailModifyScope)
	if err != nil {
		log.Printf("%v", err.Error())
		return
	}

	getTokenFromWeb(context.Background(), pathToClientCredentials, config)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(context context.Context, pathToClientCredentials string, config *oauth2.Config) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	log.Printf("go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		authCode := req.URL.Query().Get("code")
		if authCode == "" {
			log.Printf("authCode was empty")
			return
		}
		token, err := config.Exchange(context, authCode)
		if err != nil {
			log.Printf("%v", err.Error())
			return
		}

		err = saveToken(path.Join(pathToClientCredentials, mail.TokenFileName), token)
		if err != nil {
			log.Printf("%v", err.Error())
			return
		}
	})
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	log.Printf("saving credential file to: %s\n", path)
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
