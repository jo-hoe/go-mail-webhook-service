package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

func main() {
	// initialize slog default logger for CLI
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	args := os.Args
	directory_of_client_secret_json := ""
	if len(args) < 2 {
		directory_of_client_secret_json = "."
	} else {
		directory_of_client_secret_json = args[1]
	}
	generateToken(directory_of_client_secret_json)
}

func generateToken(pathToClientCredentials string) {
	config, err := mail.GetGmailConfig(pathToClientCredentials, gmail.GmailModifyScope)
	if err != nil {
		slog.Error("error getting Gmail config", "error", err)
		return
	}

	getTokenFromWeb(context.Background(), pathToClientCredentials, config)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(context context.Context, pathToClientCredentials string, config *oauth2.Config) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	slog.Info("Open browser to authorize and paste the code", "auth_url", authURL)

	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		authCode := req.URL.Query().Get("code")
		if authCode == "" {
			slog.Warn("authCode was empty")
			return
		}
		token, err := config.Exchange(context, authCode)
		if err != nil {
			slog.Error("token exchange failed", "error", err)
			return
		}

		err = saveToken(filepath.Join(pathToClientCredentials, mail.TokenFileName), token)
		if err != nil {
			slog.Error("failed to save token", "error", err)
			return
		}
	})
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           nil, // default mux
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("HTTP server error", "error", err)
	}
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	slog.Info("saving credential file", "path", path)
	file, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600) // #nosec G304 -- writing to fixed filename joined under user-provided directory
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Error("Error closing file", "error", cerr)
		}
	}()
	err = json.NewEncoder(file).Encode(token)
	if err != nil {
		return err
	}

	return nil
}
