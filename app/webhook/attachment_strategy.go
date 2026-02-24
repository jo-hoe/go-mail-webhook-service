package webhook

import (
	"context"
	"net/http"

	"github.com/jo-hoe/goback"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

// AttachmentDeliveryStrategy defines how to deliver attachments for a mail.
type AttachmentDeliveryStrategy interface {
	Deliver(ctx context.Context, client *http.Client, cfg *config.Config, m mail.Mail, selected map[string]string) error
}

// NewAttachmentDeliveryStrategy creates a concrete strategy from config.
func NewAttachmentDeliveryStrategy(s config.AttachmentStrategy) AttachmentDeliveryStrategy {
	switch s {
	case config.StrategyMultipartPerAttachment:
		return &multipartPerAttachmentStrategy{}
	case config.StrategyIgnore:
		return &ignoreStrategy{}
	case config.StrategyMultipartBundle:
		fallthrough
	default:
		return &multipartBundleStrategy{}
	}
}

// ignoreStrategy sends a single request without attachments.
type ignoreStrategy struct{}

func (st *ignoreStrategy) Deliver(ctx context.Context, client *http.Client, cfg *config.Config, m mail.Mail, selected map[string]string) error {
	h := cfg.Callback
	// Ensure default expected status range if unset
	if len(h.ExpectedStatus) == 0 {
		h.ExpectedStatus = successStatusCodes()
	}
	return sendRequest(ctx, client, h, selected, m)
}

// multipartBundleStrategy sends one request with all qualifying attachments as multipart files.
type multipartBundleStrategy struct{}

func (st *multipartBundleStrategy) Deliver(ctx context.Context, client *http.Client, cfg *config.Config, m mail.Mail, selected map[string]string) error {
	h := cfg.Callback
	if len(m.Attachments) > 0 {
		if h.Multipart == nil {
			h.Multipart = &goback.Multipart{Fields: nil, Files: nil}
		}
		h.Multipart.Files = append(h.Multipart.Files, buildRequestFiles(cfg, m, selected)...)
	}
	// Ensure default expected status range if unset
	if len(h.ExpectedStatus) == 0 {
		h.ExpectedStatus = successStatusCodes()
	}
	return sendRequest(ctx, client, h, selected, m)
}

// multipartPerAttachmentStrategy sends one request per qualifying attachment.
type multipartPerAttachmentStrategy struct{}

func (st *multipartPerAttachmentStrategy) Deliver(ctx context.Context, client *http.Client, cfg *config.Config, m mail.Mail, selected map[string]string) error {
	valid := filterAttachmentsBySize(m.Attachments, cfg.Attachments.MaxSizeBytes)
	// No attachments: send a single request without files
	if len(valid) == 0 {
		h := cfg.Callback
		if len(h.ExpectedStatus) == 0 {
			h.ExpectedStatus = successStatusCodes()
		}
		return sendRequest(ctx, client, h, selected, m)
	}

	for i, a := range valid {
		h := cfg.Callback
		if h.Multipart == nil {
			h.Multipart = &goback.Multipart{Fields: nil, Files: nil}
		}
		field := renderFieldName(cfg.Attachments.FieldName, i, a.Name, selected)
		filename := a.Name
		if filename == "" {
			filename = field
		}
		h.Multipart.Files = []goback.ByteFile{{
			Field:    field,
			FileName: filename,
			Data:     a.Content,
		}}
		if len(h.ExpectedStatus) == 0 {
			h.ExpectedStatus = successStatusCodes()
		}
		if err := sendRequest(ctx, client, h, selected, m); err != nil {
			return err
		}
	}
	return nil
}