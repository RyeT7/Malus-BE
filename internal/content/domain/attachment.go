package domain

import (
	"bytes"
	"context"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"malus-be/internal/kernel"
)

const (
	MaxAttachmentBytes = 10 << 20
	maxFileNameRunes   = 200
)

type AttachmentStatus string

const (
	AttachmentPending AttachmentStatus = "pending"
	AttachmentReady   AttachmentStatus = "ready"
)

var allowedContentTypes = map[string]func([]byte) bool{
	"application/pdf": func(b []byte) bool { return bytes.HasPrefix(b, []byte("%PDF-")) },
	"image/png":       func(b []byte) bool { return bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) },
	"image/jpeg":      func(b []byte) bool { return bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}) },
	"image/webp": func(b []byte) bool {
		return len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP"))
	},
}

const SniffBytes = 16

type Attachment struct {
	id          kernel.ID
	fileName    string
	contentType string
	size        int64
	status      AttachmentStatus
	createdAt   time.Time
	readyAt     *time.Time
}

func NewAttachment(fileName, contentType string, size int64, now time.Time) (*Attachment, error) {
	name := cleanFileName(fileName)
	if name == "" {
		return nil, kernel.Invalid("file name is required")
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if _, ok := allowedContentTypes[contentType]; !ok {
		return nil, kernel.Invalid("files must be PDF, PNG, JPEG or WebP")
	}
	if size < 1 || size > MaxAttachmentBytes {
		return nil, kernel.Invalid("files must be between 1 byte and %d MB", MaxAttachmentBytes>>20)
	}
	return &Attachment{
		id:          kernel.NewID(),
		fileName:    name,
		contentType: contentType,
		size:        size,
		status:      AttachmentPending,
		createdAt:   now,
	}, nil
}

func cleanFileName(raw string) string {
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == "/" {
		return ""
	}
	for utf8.RuneCountInString(name) > maxFileNameRunes {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

func (a *Attachment) ID() kernel.ID            { return a.id }
func (a *Attachment) FileName() string         { return a.fileName }
func (a *Attachment) ContentType() string      { return a.contentType }
func (a *Attachment) Size() int64              { return a.size }
func (a *Attachment) Status() AttachmentStatus { return a.status }
func (a *Attachment) CreatedAt() time.Time     { return a.createdAt }
func (a *Attachment) Ready() bool              { return a.status == AttachmentReady }

func (a *Attachment) BlobName() string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		}
		return '-'
	}, a.fileName)
	return a.id.String() + "/" + safe
}

func (a *Attachment) MarkReady(actualSize int64, head []byte, now time.Time) error {
	if a.status == AttachmentReady {
		return nil
	}
	if actualSize != a.size {
		return kernel.Invalid("uploaded file is %d bytes, expected %d", actualSize, a.size)
	}
	if !allowedContentTypes[a.contentType](head) {
		return kernel.Invalid("uploaded file content does not match %s", a.contentType)
	}
	a.status = AttachmentReady
	a.readyAt = &now
	return nil
}

type AttachmentSnapshot struct {
	ID          kernel.ID
	FileName    string
	ContentType string
	Size        int64
	Status      AttachmentStatus
	CreatedAt   time.Time
	ReadyAt     *time.Time
}

func (a *Attachment) Snapshot() AttachmentSnapshot {
	return AttachmentSnapshot{
		ID:          a.id,
		FileName:    a.fileName,
		ContentType: a.contentType,
		Size:        a.size,
		Status:      a.status,
		CreatedAt:   a.createdAt,
		ReadyAt:     a.readyAt,
	}
}

func RestoreAttachment(s AttachmentSnapshot) *Attachment {
	return &Attachment{
		id:          s.ID,
		fileName:    s.FileName,
		contentType: s.ContentType,
		size:        s.Size,
		status:      s.Status,
		createdAt:   s.CreatedAt,
		readyAt:     s.ReadyAt,
	}
}

type AttachmentRepository interface {
	Get(ctx context.Context, id kernel.ID) (*Attachment, error)
	GetMany(ctx context.Context, ids []kernel.ID) (map[kernel.ID]*Attachment, error)
	List(ctx context.Context) ([]*Attachment, error)
	Save(ctx context.Context, a *Attachment) error
}
