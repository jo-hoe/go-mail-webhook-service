package main

import (
	"log/slog"
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
	// initialize slog default logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := getConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Reconfigure slog level based on config (default is info)
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	// Process config once and exit (suitable for Kubernetes Job execution)
	webhook.NewWebhookService(cfg).Run()
}
