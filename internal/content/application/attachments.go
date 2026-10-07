package application

import (
	"context"
	"errors"
	"slices"
	"time"

	"malus-be/internal/content/domain"
	"malus-be/internal/kernel"
)

const (
	uploadLinkTTL   = 10 * time.Minute
	downloadLinkTTL = 5 * time.Minute
)

type BlobStore interface {
	UploadURL(ctx context.Context, blobName, contentType string, ttl time.Duration) (string, error)
	DownloadURL(ctx context.Context, blobName, fileName, contentType string, ttl time.Duration) (string, error)
	Size(ctx context.Context, blobName string) (int64, error)
	ReadHead(ctx context.Context, blobName string, n int) ([]byte, error)
	Delete(ctx context.Context, blobName string) error
}

type AttachmentView struct {
	ID          string
	FileName    string
	ContentType string
	Size        int64
	Ready       bool
	CreatedAt   time.Time
}

type UploadTicket struct {
	Attachment AttachmentView
	UploadURL  string
	Headers    map[string]string
	ExpiresAt  time.Time
}

func toAttachmentView(a *domain.Attachment) AttachmentView {
	return AttachmentView{
		ID:          a.ID().String(),
		FileName:    a.FileName(),
		ContentType: a.ContentType(),
		Size:        a.Size(),
		Ready:       a.Ready(),
		CreatedAt:   a.CreatedAt(),
	}
}

type RequestUpload struct {
	FileName    string
	ContentType string
	Size        int64
}

func (s *Service) RequestUpload(ctx context.Context, cmd RequestUpload) (UploadTicket, error) {
	now := s.now()
	attachment, err := domain.NewAttachment(cmd.FileName, cmd.ContentType, cmd.Size, now)
	if err != nil {
		return UploadTicket{}, err
	}
	link, err := s.blobs.UploadURL(ctx, attachment.BlobName(), attachment.ContentType(), uploadLinkTTL)
	if err != nil {
		return UploadTicket{}, err
	}
	if err := s.attachments.Save(ctx, attachment); err != nil {
		return UploadTicket{}, err
	}
	return UploadTicket{
		Attachment: toAttachmentView(attachment),
		UploadURL:  link,
		Headers:    map[string]string{"x-ms-blob-type": "BlockBlob", "Content-Type": attachment.ContentType()},
		ExpiresAt:  now.Add(uploadLinkTTL),
	}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, id kernel.ID) (AttachmentView, error) {
	attachment, err := s.attachments.Get(ctx, id)
	if err != nil {
		return AttachmentView{}, err
	}
	if attachment.Ready() {
		return toAttachmentView(attachment), nil
	}
	size, err := s.blobs.Size(ctx, attachment.BlobName())
	if errors.Is(err, kernel.ErrNotFound) {
		return AttachmentView{}, kernel.Invalid("the file has not been uploaded yet")
	}
	if err != nil {
		return AttachmentView{}, err
	}
	head, err := s.blobs.ReadHead(ctx, attachment.BlobName(), domain.SniffBytes)
	if err != nil {
		return AttachmentView{}, err
	}
	if err := attachment.MarkReady(size, head, s.now()); err != nil {
		_ = s.blobs.Delete(ctx, attachment.BlobName())
		return AttachmentView{}, err
	}
	if err := s.attachments.Save(ctx, attachment); err != nil {
		return AttachmentView{}, err
	}
	return toAttachmentView(attachment), nil
}

func (s *Service) ListAttachments(ctx context.Context) ([]AttachmentView, error) {
	attachments, err := s.attachments.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]AttachmentView, 0, len(attachments))
	for _, a := range attachments {
		views = append(views, toAttachmentView(a))
	}
	return views, nil
}

func (s *Service) AttachmentLink(ctx context.Context, id kernel.ID, admin bool) (string, error) {
	attachment, err := s.attachments.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if !attachment.Ready() {
		return "", kernel.NotFound("attachment %s", id)
	}
	if !admin {
		published, err := s.publishedAttachmentIDs(ctx)
		if err != nil {
			return "", err
		}
		if !slices.Contains(published, id) {
			return "", kernel.NotFound("attachment %s", id)
		}
	}
	return s.blobs.DownloadURL(ctx, attachment.BlobName(), attachment.FileName(), attachment.ContentType(), downloadLinkTTL)
}

func (s *Service) publishedAttachmentIDs(ctx context.Context) ([]kernel.ID, error) {
	sections, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	var ids []kernel.ID
	for _, section := range sections {
		if v, ok := section.Published(); ok {
			ids = append(ids, v.Content.AttachmentIDs()...)
		}
	}
	return ids, nil
}

func (s *Service) checkAttachments(ctx context.Context, content domain.Content) error {
	ids := content.AttachmentIDs()
	if len(ids) == 0 {
		return nil
	}
	found, err := s.attachments.GetMany(ctx, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if a, ok := found[id]; !ok || !a.Ready() {
			return kernel.Invalid("attachment %s does not exist or has not finished uploading", id)
		}
	}
	return nil
}

func (s *Service) attachmentIndex(ctx context.Context, contents ...domain.Content) (map[kernel.ID]AttachmentView, error) {
	var ids []kernel.ID
	for _, c := range contents {
		for _, id := range c.AttachmentIDs() {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	index := make(map[kernel.ID]AttachmentView, len(ids))
	if len(ids) == 0 {
		return index, nil
	}
	found, err := s.attachments.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, a := range found {
		index[id] = toAttachmentView(a)
	}
	return index, nil
}

func sectionContents(s *domain.Section, withVersions bool) []domain.Content {
	contents := []domain.Content{s.Draft()}
	if withVersions {
		for _, v := range s.Versions() {
			contents = append(contents, v.Content)
		}
	} else if v, ok := s.Published(); ok {
		contents = append(contents, v.Content)
	}
	return contents
}
