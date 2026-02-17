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
	"path"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
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

	// Gmail API allows using the literal "me" to refer to the authenticated user.
	// Credentials and token are loaded from the mounted credentials path.
	user := "me"
	listCall := gmailService.Users.Messages.List(user).Q("is:unread")
	resp, err := listCall.Do()
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && (gErr.Code == 401 || gErr.Code == 403) {
			return result, fmt.Errorf("gmail API returned %d unauthorized/forbidden. The OAuth token at %s may be invalid, expired, or revoked. Re-generate the token using the CLI (cli/gmail) and ensure it is mounted at the configured credentialsPath. Original error: %v", gErr.Code, path.Join(service.credentialsPath, TokenFileName), err)
		}
		return result, fmt.Errorf("unable to retrieve messages from Gmail: %v", err)
	}

		for _, message := range resp.Messages {
		fullMessage, err := gmailService.Users.Messages.Get(user, message.Id).Format("full").Do()
		if err != nil {
			return result, err
		}

		subject := extractSubject(fullMessage.Payload.Headers)
		sender := extractSender(fullMessage.Payload.Headers)
		recipients := extractRecipients(fullMessage.Payload.Headers)
		body := extractPlainTextBody(fullMessage.Payload.Parts)
		attachments := extractAttachments(gmailService, user, message.Id, fullMessage.Payload.Parts)

		result = append(result, Mail{
			Id:          message.Id,
			Sender:      sender,
			Recipients:  recipients,
			Subject:     subject,
			Body:        body,
			Attachments: attachments,
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
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && (gErr.Code == 401 || gErr.Code == 403) {
			return fmt.Errorf("gmail API returned %d unauthorized/forbidden while marking message %s as read. The OAuth token at %s may be invalid, expired, or revoked. Re-generate the token using the CLI (cli/gmail) and ensure it is mounted at the configured credentialsPath. Original error: %v", gErr.Code, mail.Id, path.Join(service.credentialsPath, TokenFileName), err)
		}
		return fmt.Errorf("unable to mark message %s as read: %v", mail.Id, err)
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

func extractSender(headers []*gmail.MessagePartHeader) string {
	for _, header := range headers {
		if header.Name == "From" {
			if addr, err := gomail.ParseAddress(header.Value); err == nil && addr != nil {
				return addr.Address
			}
			return header.Value
		}
	}
	return ""
}

// extractRecipients builds a list of recipient addresses from common headers:
// - Delivered-To (primary target mailbox; may appear multiple times)
// - To and Cc (may contain multiple addresses)
// Bcc is typically not visible to recipients, so it is not considered here.
func extractRecipients(headers []*gmail.MessagePartHeader) []string {
	var recipients []string
	seen := map[string]bool{}

	// Delivered-To may appear multiple times
	for _, header := range headers {
		if header.Name == "Delivered-To" {
			if addr, err := gomail.ParseAddress(header.Value); err == nil && addr != nil {
				if addr.Address != "" && !seen[addr.Address] {
					seen[addr.Address] = true
					recipients = append(recipients, addr.Address)
				}
			} else {
				v := strings.TrimSpace(header.Value)
				if v != "" && !seen[v] {
					seen[v] = true
					recipients = append(recipients, v)
				}
			}
		}
	}

	// Parse To and Cc lists
	for _, header := range headers {
		if header.Name == "To" || header.Name == "Cc" {
			if list, err := gomail.ParseAddressList(header.Value); err == nil {
				for _, a := range list {
					if a != nil && a.Address != "" && !seen[a.Address] {
						seen[a.Address] = true
						recipients = append(recipients, a.Address)
					}
				}
			} else {
				// Fallback: comma-separated
				parts := strings.Split(header.Value, ",")
				for _, p := range parts {
					v := strings.TrimSpace(p)
					if v != "" && !seen[v] {
						seen[v] = true
						recipients = append(recipients, v)
					}
				}
			}
		}
	}

	return recipients
}

func extractPlainTextBody(parts []*gmail.MessagePart) string {
	for _, part := range parts {
		if part.MimeType == "text/plain" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				slog.Error("Error decoding body data", "error", err)
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

// extractAttachments walks the message parts to collect attachments (filename + raw bytes).
func extractAttachments(svc *gmail.Service, user, msgID string, parts []*gmail.MessagePart) []Attachment {
	var result []Attachment
	for _, part := range parts {
		// If this part has a filename, it's typically an attachment part
		if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
			att, err := svc.Users.Messages.Attachments.Get(user, msgID, part.Body.AttachmentId).Do()
			if err != nil {
				slog.Error("Error retrieving attachment", "filename", part.Filename, "error", err)
				continue
			}
			// Gmail returns URL-safe base64 for attachments
			data, err := base64.URLEncoding.DecodeString(att.Data)
			if err != nil {
				slog.Error("Error decoding attachment", "filename", part.Filename, "error", err)
				continue
			}
			result = append(result, Attachment{
				Name:    part.Filename,
				Content: data,
			})
		}
		// Recurse into nested parts
		if len(part.Parts) > 0 {
			sub := extractAttachments(svc, user, msgID, part.Parts)
			if len(sub) > 0 {
				result = append(result, sub...)
			}
		}
	}
	return result
}

func (service *GmailService) getGmailService(context context.Context, scope ...string) (*gmail.Service, error) {
	config, err := GetGmailConfig(service.credentialsPath, scope...)
	if err != nil {
		return nil, err
	}

	ts, err := service.getTokenSource(context, config)
	if err != nil {
		return nil, err
	}

	return gmail.NewService(context, option.WithTokenSource(ts))
}

func GetGmailConfig(credentialsPath string, scope ...string) (*oauth2.Config, error) {
	b, err := os.ReadFile(path.Join(credentialsPath, CredentialsFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to read Gmail client credentials at %s: %w", path.Join(credentialsPath, CredentialsFileName), err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	return google.ConfigFromJSON(b, scope...)
}


 // Retrieves a token from a local file.
func tokenFromFile(filePath string) (*oauth2.Token, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Error("Error closing file", "error", cerr)
		}
	}()
	token := &oauth2.Token{}
	err = json.NewDecoder(file).Decode(token)
	return token, err
}

// tokenSavingSource persists any refreshed token to disk to keep the on-disk cache in sync.
type tokenSavingSource struct {
	oauth2.TokenSource
	path string
}

func (s *tokenSavingSource) Token() (*oauth2.Token, error) {
	t, err := s.TokenSource.Token()
	if err != nil {
		return nil, err
	}
	// Persist the full token JSON (including refresh_token) to file
	file, err := os.OpenFile(s.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		slog.Error("failed to open token file for writing", "path", s.path, "error", err)
		return t, nil // return token even if persisting failed
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Error("Error closing token file", "error", cerr)
		}
	}()
	if err := json.NewEncoder(file).Encode(t); err != nil {
		slog.Error("failed to encode token JSON", "path", s.path, "error", err)
	}
	return t, nil
}

// getTokenSource returns a TokenSource that auto-refreshes and saves refreshed tokens back to request.token
func (service *GmailService) getTokenSource(ctx context.Context, config *oauth2.Config) (oauth2.TokenSource, error) {
	tokenFilePath := path.Join(service.credentialsPath, TokenFileName)
	token, err := tokenFromFile(tokenFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OAuth token from %s: %w. If the file is missing or corrupted, re-generate it using the CLI tool (cli/gmail)", tokenFilePath, err)
	}

	// If the token is already expired and there's no refresh token, provide a helpful error
	if token.Expiry.Before(time.Now().Add(-1*time.Minute)) && token.RefreshToken == "" {
		return nil, fmt.Errorf("stored OAuth token in %s is expired and has no refresh_token to refresh it. Please re-authorize to generate a new token using the CLI tool (cli/gmail)", tokenFilePath)
	}

	ts := config.TokenSource(ctx, token)
	return &tokenSavingSource{TokenSource: ts, path: tokenFilePath}, nil
}
