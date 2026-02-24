package webhook

import (
	"path/filepath"

	"github.com/jo-hoe/goback"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

// AttachmentDeliveryStrategy defines how to modify the base webhook request(s) for attachments.
// It does NOT send the requests; it only returns the concrete request(s) to be sent.
type AttachmentDeliveryStrategy interface {
	// BuildRequests receives a base request (cfg.Callback) and returns one or more concrete requests
	// to be sent by the caller (webhookservice). This preserves separation of concerns.
	BuildRequests(base goback.Config, cfg *config.Config, m mail.Mail, selected map[string]string) []goback.Config
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

// ignoreStrategy returns the base request as-is (no attachments).
type ignoreStrategy struct{}

func (st *ignoreStrategy) BuildRequests(base goback.Config, _ *config.Config, _ mail.Mail, _ map[string]string) []goback.Config {
	return []goback.Config{base}
}

// multipartBundleStrategy returns a single request with all qualifying attachments as multipart files.
type multipartBundleStrategy struct{}

func (st *multipartBundleStrategy) BuildRequests(base goback.Config, cfg *config.Config, m mail.Mail, selected map[string]string) []goback.Config {
	h := base
	if len(m.Attachments) > 0 {
		if h.Multipart == nil {
			h.Multipart = &goback.Multipart{Fields: nil, Files: nil}
		}
		h.Multipart.Files = append(h.Multipart.Files, buildRequestFiles(cfg, m, selected)...)
	}
	return []goback.Config{h}
}

// multipartPerAttachmentStrategy returns one request per qualifying attachment (single file each).
// If no attachments qualify, it returns the base request unchanged.
type multipartPerAttachmentStrategy struct{}

func (st *multipartPerAttachmentStrategy) BuildRequests(base goback.Config, cfg *config.Config, m mail.Mail, selected map[string]string) []goback.Config {
	valid := filterAttachmentsBySize(m.Attachments, cfg.Attachments.MaxSizeBytes)
	if len(valid) == 0 {
		return []goback.Config{base}
	}

	requests := make([]goback.Config, 0, len(valid))
	for i, a := range valid {
		h := base
		var fields map[string]string
		if base.Multipart != nil {
			fields = base.Multipart.Fields
		}
		h.Multipart = &goback.Multipart{Fields: fields, Files: nil}

		field := renderFieldName(cfg.Attachments.FieldName, i, a.Name, selected)
		filename := filepath.Base(a.Name)
		if filename == "" {
			filename = field
		}
		h.Multipart.Files = []goback.ByteFile{{
			Field:    field,
			FileName: filename,
			Data:     a.Content,
		}}
		requests = append(requests, h)
	}
	return requests
}
