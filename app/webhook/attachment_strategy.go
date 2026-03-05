package webhook

import (
	"bytes"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jo-hoe/goback"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
)

// AttachmentDeliveryStrategy builds the concrete webhook request(s) for a mail's attachments.
// Implementations must not send the requests themselves; they only return the configs to be sent.
type AttachmentDeliveryStrategy interface {
	BuildRequests(base goback.Config, cfg *config.Config, m mail.Mail, selected map[string]string) []goback.Config
}

// NewAttachmentDeliveryStrategy returns the strategy corresponding to s.
func NewAttachmentDeliveryStrategy(s config.AttachmentStrategy) AttachmentDeliveryStrategy {
	switch s {
	case config.StrategyMultipartPerAttachment:
		return &multipartPerAttachmentStrategy{}
	case config.StrategyIgnore:
		return &ignoreStrategy{}
	default:
		return &multipartBundleStrategy{}
	}
}

// ignoreStrategy forwards the base request without any attachments.
type ignoreStrategy struct{}

func (st *ignoreStrategy) BuildRequests(base goback.Config, _ *config.Config, _ mail.Mail, _ map[string]string) []goback.Config {
	return []goback.Config{base}
}

// multipartBundleStrategy bundles all qualifying attachments into one request.
type multipartBundleStrategy struct{}

func (st *multipartBundleStrategy) BuildRequests(base goback.Config, cfg *config.Config, m mail.Mail, selected map[string]string) []goback.Config {
	h := base
	valid := filterAttachmentsBySize(m.Attachments, cfg.Attachments.MaxSizeBytes)
	if len(valid) > 0 {
		if h.Multipart == nil {
			h.Multipart = &goback.Multipart{}
		}
		h.Multipart.Files = append(h.Multipart.Files, buildRequestFiles(cfg.Attachments.FieldName, valid, selected)...)
	}
	return []goback.Config{h}
}

// multipartPerAttachmentStrategy sends one request per qualifying attachment.
// When no attachments qualify, the base request is returned unchanged.
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
		h.Multipart = &goback.Multipart{
			Fields: fields,
			Files:  []goback.ByteFile{buildSingleRequestFile(cfg.Attachments.FieldName, i, a, selected)},
		}
		requests = append(requests, h)
	}
	return requests
}

// filterAttachmentsBySize returns the subset of atts whose content size does not exceed max.
// When max <= 0, all attachments are returned unchanged.
func filterAttachmentsBySize(atts []mail.Attachment, max int64) []mail.Attachment {
	if max <= 0 {
		return atts
	}
	result := make([]mail.Attachment, 0, len(atts))
	for _, a := range atts {
		if int64(len(a.Content)) > max {
			slog.Warn("skipping attachment: exceeds size limit", "name", a.Name, "size_bytes", len(a.Content), "max_bytes", max)
			continue
		}
		result = append(result, a)
	}
	return result
}

// buildRequestFiles converts a slice of attachments into goback.ByteFile entries.
func buildRequestFiles(fieldTpl string, attachments []mail.Attachment, selected map[string]string) []goback.ByteFile {
	files := make([]goback.ByteFile, 0, len(attachments))
	for i, a := range attachments {
		files = append(files, buildSingleRequestFile(fieldTpl, i, a, selected))
	}
	return files
}

// buildSingleRequestFile creates a goback.ByteFile from one attachment.
func buildSingleRequestFile(fieldTpl string, idx int, a mail.Attachment, selected map[string]string) goback.ByteFile {
	field := renderFieldName(fieldTpl, idx, a.Name, selected)
	name := filepath.Base(a.Name)
	if name == "" {
		name = field
	}
	return goback.ByteFile{Field: field, FileName: name, Data: a.Content}
}

// renderFieldName evaluates fieldTpl as a Go text/template or returns a positional default.
func renderFieldName(fieldTpl string, idx int, filename string, selected map[string]string) string {
	if strings.TrimSpace(fieldTpl) == "" {
		return fmt.Sprintf("attachment_%d", idx)
	}
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	data := map[string]any{
		"index":       idx,
		"filename":    base,
		"basename":    strings.TrimSuffix(base, ext),
		"ext":         ext,
		"contentType": mime.TypeByExtension(ext),
	}
	for k, v := range selected {
		data[k] = v
	}
	t, err := template.New("field").Parse(fieldTpl)
	if err != nil {
		return fieldTpl
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fieldTpl
	}
	return buf.String()
}