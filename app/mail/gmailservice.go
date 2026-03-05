package mail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	gomail "net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	CredentialsFileName = "client_secret.json"
	TokenFileName       = "request.token"
)

// GmailService implements MailClientService using the Gmail API.
type GmailService struct {
	credentialsPath string
}

// NewGmailService creates a GmailService that reads credentials from credentialsPath.
func NewGmailService(credentialsPath string) *GmailService {
	return &GmailService{credentialsPath: credentialsPath}
}

func (s *GmailService) GetAllUnreadMail(ctx context.Context) ([]Mail, error) {
	svc, err := s.getGmailService(ctx, gmail.GmailModifyScope)
	if err != nil {
		return nil, err
	}

	resp, err := svc.Users.Messages.List("me").Q("is:unread").Do()
	if err != nil {
		return nil, s.wrapGmailError(err, "list unread messages", "")
	}

	result := make([]Mail, 0, len(resp.Messages))
	for _, msg := range resp.Messages {
		full, err := svc.Users.Messages.Get("me", msg.Id).Format("full").Do()
		if err != nil {
			return nil, err
		}
		result = append(result, Mail{
			Id:          msg.Id,
			Sender:      extractSender(full.Payload.Headers),
			Recipients:  extractRecipients(full.Payload.Headers),
			Subject:     extractSubject(full.Payload.Headers),
			Body:        extractPlainTextBody(full.Payload.Parts),
			Attachments: extractAttachments(svc, "me", msg.Id, full.Payload.Parts),
		})
	}
	return result, nil
}

func (s *GmailService) MarkMailAsRead(ctx context.Context, mail Mail) error {
	svc, err := s.getGmailService(ctx, gmail.GmailModifyScope)
	if err != nil {
		return err
	}
	req := &gmail.ModifyMessageRequest{RemoveLabelIds: []string{"UNREAD"}}
	_, err = svc.Users.Messages.Modify("me", mail.Id, req).Do()
	if err != nil {
		return s.wrapGmailError(err, "mark message as read", mail.Id)
	}
	return nil
}

func (s *GmailService) DeleteMail(ctx context.Context, mail Mail) error {
	svc, err := s.getGmailService(ctx, gmail.GmailModifyScope)
	if err != nil {
		return err
	}
	if err := svc.Users.Messages.Delete("me", mail.Id).Do(); err != nil {
		return s.wrapGmailError(err, "delete message", mail.Id)
	}
	return nil
}

// wrapGmailError returns a descriptive error, identifying auth failures (401/403) separately.
func (s *GmailService) wrapGmailError(err error, action, mailID string) error {
	tokenPath := filepath.Join(s.credentialsPath, TokenFileName)
	ctx := action
	if mailID != "" {
		ctx = fmt.Sprintf("%s (mail %s)", action, mailID)
	}
	var gErr *googleapi.Error
	if errors.As(err, &gErr) && (gErr.Code == 401 || gErr.Code == 403) {
		return fmt.Errorf("%s: gmail API returned %d — OAuth token at %s may be invalid or revoked; regenerate using cli/gmail: %w",
			ctx, gErr.Code, tokenPath, err)
	}
	return fmt.Errorf("%s: %w", ctx, err)
}

func extractSubject(headers []*gmail.MessagePartHeader) string {
	return findHeader(headers, "Subject")
}

func extractSender(headers []*gmail.MessagePartHeader) string {
	raw := findHeader(headers, "From")
	if addr, err := gomail.ParseAddress(raw); err == nil && addr != nil {
		return addr.Address
	}
	return raw
}

// extractRecipients collects unique recipient addresses from Delivered-To, To, and Cc headers.
// Bcc is intentionally excluded as it is typically not visible to recipients.
func extractRecipients(headers []*gmail.MessagePartHeader) []string {
	seen := make(map[string]bool)
	var recipients []string

	addAddress := func(raw string) {
		if addr, err := gomail.ParseAddress(raw); err == nil && addr != nil && addr.Address != "" {
			if !seen[addr.Address] {
				seen[addr.Address] = true
				recipients = append(recipients, addr.Address)
			}
			return
		}
		v := strings.TrimSpace(raw)
		if v != "" && !seen[v] {
			seen[v] = true
			recipients = append(recipients, v)
		}
	}

	addAddressList := func(raw string) {
		if list, err := gomail.ParseAddressList(raw); err == nil {
			for _, a := range list {
				if a != nil && a.Address != "" && !seen[a.Address] {
					seen[a.Address] = true
					recipients = append(recipients, a.Address)
				}
			}
			return
		}
		// Fallback: comma-separated raw strings.
		for _, p := range strings.Split(raw, ",") {
			addAddress(p)
		}
	}

	for _, h := range headers {
		switch h.Name {
		case "Delivered-To":
			addAddress(h.Value)
		case "To", "Cc":
			addAddressList(h.Value)
		}
	}
	return recipients
}

func extractPlainTextBody(parts []*gmail.MessagePart) string {
	for _, part := range parts {
		if part.MimeType == "text/plain" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				slog.Error("error decoding body data", "error", err)
				continue
			}
			return string(data)
		}
		if len(part.Parts) > 0 {
			if body := extractPlainTextBody(part.Parts); body != "" {
				return body
			}
		}
	}
	return ""
}

// extractAttachments walks message parts recursively and collects file attachments.
func extractAttachments(svc *gmail.Service, user, msgID string, parts []*gmail.MessagePart) []Attachment {
	var result []Attachment
	for _, part := range parts {
		if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
			att, err := svc.Users.Messages.Attachments.Get(user, msgID, part.Body.AttachmentId).Do()
			if err != nil {
				slog.Error("error retrieving attachment", "filename", part.Filename, "error", err)
				continue
			}
			data, err := base64.URLEncoding.DecodeString(att.Data)
			if err != nil {
				slog.Error("error decoding attachment", "filename", part.Filename, "error", err)
				continue
			}
			result = append(result, Attachment{Name: part.Filename, Content: data})
		}
		if len(part.Parts) > 0 {
			result = append(result, extractAttachments(svc, user, msgID, part.Parts)...)
		}
	}
	return result
}

// findHeader returns the value of the first header matching name, or "" if absent.
func findHeader(headers []*gmail.MessagePartHeader, name string) string {
	for _, h := range headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}

func (s *GmailService) getGmailService(ctx context.Context, scope ...string) (*gmail.Service, error) {
	cfg, err := GetGmailConfig(s.credentialsPath, scope...)
	if err != nil {
		return nil, err
	}
	ts, err := s.getTokenSource(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return gmail.NewService(ctx, option.WithTokenSource(ts))
}

// GetGmailConfig reads the OAuth client configuration from the credentials directory.
func GetGmailConfig(credentialsPath string, scope ...string) (*oauth2.Config, error) {
	credFile := filepath.Join(credentialsPath, CredentialsFileName)
	b, err := os.ReadFile(credFile) // #nosec G304 -- path is built from a configured directory and fixed filename
	if err != nil {
		return nil, fmt.Errorf("failed to read Gmail credentials at %s: %w", credFile, err)
	}
	return google.ConfigFromJSON(b, scope...)
}

// tokenFromFile loads an OAuth token from disk.
func tokenFromFile(path string) (*oauth2.Token, error) {
	f, err := os.Open(path) // #nosec G304 -- path is built from a configured directory and fixed filename
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			slog.Error("error closing token file", "error", cerr)
		}
	}()
	token := &oauth2.Token{}
	return token, json.NewDecoder(f).Decode(token)
}

// tokenSavingSource wraps a TokenSource and persists refreshed tokens to disk.
type tokenSavingSource struct {
	oauth2.TokenSource
	path string
}

func (s *tokenSavingSource) Token() (*oauth2.Token, error) {
	t, err := s.TokenSource.Token()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "read-only file system") || os.IsPermission(err) {
			slog.Warn("token not persisted (read-only or permission denied)", "path", s.path, "error", err)
		} else {
			slog.Error("failed to open token file for writing", "path", s.path, "error", err)
		}
		return t, nil
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			slog.Error("error closing token file", "error", cerr)
		}
	}()
	if err := json.NewEncoder(f).Encode(t); err != nil {
		slog.Error("failed to encode token", "path", s.path, "error", err)
	}
	return t, nil
}

func (s *GmailService) getTokenSource(ctx context.Context, cfg *oauth2.Config) (oauth2.TokenSource, error) {
	tokenPath := filepath.Join(s.credentialsPath, TokenFileName)
	token, err := tokenFromFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OAuth token from %s: %w; regenerate using cli/gmail", tokenPath, err)
	}
	if token.Expiry.Before(time.Now().Add(-time.Minute)) && token.RefreshToken == "" {
		return nil, fmt.Errorf("OAuth token at %s is expired with no refresh_token; re-authorize using cli/gmail", tokenPath)
	}
	return &tokenSavingSource{TokenSource: cfg.TokenSource(ctx, token), path: tokenPath}, nil
}