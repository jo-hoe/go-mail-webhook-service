package main

import (
	"context"
	"os"
	"path"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		println("provide path to client credentials in quotes")
		return
	}
	generateToken(args[1])
}

func generateToken(pathToClientCredentials string) {
	config, err := mail.GetGmailConfig(pathToClientCredentials)
	if err != nil {
		println(err.Error())
		return
	}

	token, err := mail.GetTokenFromWeb(context.Background(), config)
	if err != nil {
		println(err.Error())
		return
	}
	err = mail.SaveToken(path.Join(pathToClientCredentials, mail.TokenFileName), token)
	if err != nil {
		println(err.Error())
		return
	}
}
