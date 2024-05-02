package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/webhook"
)

var configFileName = path.Join("config", "config.yaml")

func getConfig() (*[]config.Config, error) {
	yamlFile, err := os.ReadFile(configFileName)
	if err != nil {
		return nil, err
	}
	return config.NewConfigsFromYaml(yamlFile)
}

func main() {
	configs, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	webhook.NewWebhookService(configs).Run(context.Background(), http.DefaultClient)
}
