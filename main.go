package main

import (
	"log"
	"os"
	"path"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/webhook"
)

var configFileName = path.Join("config", "config.yaml")

func getConfig() (*config.Config, error) {
	yamlFile, err := os.ReadFile(configFileName)
	if err != nil {
		return nil, err
	}
	return config.NewConfigFromYaml(yamlFile)
}

func main() {
	cfg, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Process config once and exit (suitable for Kubernetes Job execution)
	webhook.NewWebhookService(cfg).Run()
}
