package mail

import (
	"testing"
	"time"

	"google.golang.org/api/gmail/v1"
)

func Test_extractReceivedAt(t *testing.T) {
	tests := []struct {
		name           string
		internalDateMs int64
		want           time.Time
	}{
		{
			name:           "zero value",
			internalDateMs: 0,
			want:           time.UnixMilli(0).UTC(),
		},
		{
			name:           "known timestamp",
			internalDateMs: 1_700_000_000_000,
			want:           time.Date(2023, 11, 14, 22, 13, 20, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReceivedAt(tt.internalDateMs)
			if !got.Equal(tt.want) {
				t.Errorf("extractReceivedAt(%d) = %v, want %v", tt.internalDateMs, got, tt.want)
			}
			if got.Location() != time.UTC {
				t.Errorf("extractReceivedAt() location = %v, want UTC", got.Location())
			}
		})
	}
}

func Test_extractSubject(t *testing.T) {
	tests := []struct {
		name    string
		headers []*gmail.MessagePartHeader
		want    string
	}{
		{
			name:    "subject present",
			headers: []*gmail.MessagePartHeader{{Name: "Subject", Value: "Hello"}},
			want:    "Hello",
		},
		{
			name:    "subject absent",
			headers: []*gmail.MessagePartHeader{{Name: "From", Value: "a@b.com"}},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSubject(tt.headers); got != tt.want {
				t.Errorf("extractSubject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_extractSender(t *testing.T) {
	tests := []struct {
		name    string
		headers []*gmail.MessagePartHeader
		want    string
	}{
		{
			name:    "RFC 5322 display name plus address",
			headers: []*gmail.MessagePartHeader{{Name: "From", Value: "Alice <alice@example.com>"}},
			want:    "alice@example.com",
		},
		{
			name:    "bare address",
			headers: []*gmail.MessagePartHeader{{Name: "From", Value: "alice@example.com"}},
			want:    "alice@example.com",
		},
		{
			name:    "From header absent",
			headers: []*gmail.MessagePartHeader{},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSender(tt.headers); got != tt.want {
				t.Errorf("extractSender() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_extractRecipients(t *testing.T) {
	tests := []struct {
		name    string
		headers []*gmail.MessagePartHeader
		want    []string
	}{
		{
			name: "To and Cc deduplicated",
			headers: []*gmail.MessagePartHeader{
				{Name: "To", Value: "a@example.com, b@example.com"},
				{Name: "Cc", Value: "b@example.com, c@example.com"},
			},
			want: []string{"a@example.com", "b@example.com", "c@example.com"},
		},
		{
			name: "Delivered-To included",
			headers: []*gmail.MessagePartHeader{
				{Name: "Delivered-To", Value: "me@example.com"},
				{Name: "To", Value: "me@example.com"},
			},
			want: []string{"me@example.com"},
		},
		{
			name:    "no recipient headers",
			headers: []*gmail.MessagePartHeader{{Name: "Subject", Value: "hi"}},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRecipients(tt.headers)
			if len(got) != len(tt.want) {
				t.Errorf("extractRecipients() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractRecipients()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
