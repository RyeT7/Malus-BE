package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

	"malus-be/internal/kernel"
	"malus-be/internal/platform/cosmosdb"
	"malus-be/internal/realtime/domain"
)

const (
	PartitionKeyPath  = "/pk"
	sessionsPartition = "sessions"
)

type sessionDoc struct {
	ID         string     `json:"id"`
	PK         string     `json:"pk"`
	SlideCount int        `json:"slideCount"`
	Slide      int        `json:"slide"`
	Version    int64      `json:"version"`
	StartedAt  time.Time  `json:"startedAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
	ETag       string     `json:"_etag,omitempty"`
}

type SessionRepository struct {
	container *azcosmos.ContainerClient
	pk        azcosmos.PartitionKey
}

func NewSessionRepository(container *azcosmos.ContainerClient) *SessionRepository {
	return &SessionRepository{container: container, pk: azcosmos.NewPartitionKeyString(sessionsPartition)}
}

func (r *SessionRepository) Get(ctx context.Context, id kernel.ID) (*domain.Session, error) {
	resp, err := r.container.ReadItem(ctx, r.pk, id.String(), nil)
	if cosmosdb.StatusCode(err) == http.StatusNotFound {
		return nil, kernel.NotFound("session %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	var doc sessionDoc
	if err := json.Unmarshal(resp.Value, &doc); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	doc.ETag = string(resp.ETag)
	return toDomain(doc), nil
}

func (r *SessionRepository) FindActive(ctx context.Context) (*domain.Session, error) {
	pager := r.container.NewQueryItemsPager("SELECT * FROM c WHERE NOT IS_DEFINED(c.endedAt) OR IS_NULL(c.endedAt)", r.pk, nil)
	var latest *sessionDoc
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("query live session: %w", err)
		}
		for _, item := range page.Items {
			var doc sessionDoc
			if err := json.Unmarshal(item, &doc); err != nil {
				return nil, fmt.Errorf("decode session: %w", err)
			}
			if latest == nil || doc.StartedAt.After(latest.StartedAt) {
				d := doc
				latest = &d
			}
		}
	}
	if latest == nil {
		return nil, kernel.NotFound("no live session")
	}
	return toDomain(*latest), nil
}

func (r *SessionRepository) Save(ctx context.Context, s *domain.Session) error {
	snap := s.Snapshot()
	body, err := json.Marshal(sessionDoc{
		ID:         snap.ID.String(),
		PK:         sessionsPartition,
		SlideCount: snap.SlideCount,
		Slide:      snap.Slide,
		Version:    snap.Version,
		StartedAt:  snap.StartedAt,
		UpdatedAt:  snap.UpdatedAt,
		EndedAt:    snap.EndedAt,
	})
	if err != nil {
		return err
	}

	if snap.ETag == "" {
		_, err = r.container.CreateItem(ctx, r.pk, body, nil)
	} else {
		etag := azcore.ETag(snap.ETag)
		_, err = r.container.ReplaceItem(ctx, r.pk, snap.ID.String(), body, &azcosmos.ItemOptions{IfMatchEtag: &etag})
	}
	switch cosmosdb.StatusCode(err) {
	case 0:
		if err != nil {
			return fmt.Errorf("save session: %w", err)
		}
		return nil
	case http.StatusConflict, http.StatusPreconditionFailed:
		return kernel.ErrConcurrentUpdate
	case http.StatusNotFound:
		return kernel.NotFound("session %s", snap.ID)
	default:
		return fmt.Errorf("save session: %w", err)
	}
}

func (r *SessionRepository) Ping(ctx context.Context) error {
	_, err := r.container.Read(ctx, nil)
	return err
}

func toDomain(doc sessionDoc) *domain.Session {
	return domain.Restore(domain.Snapshot{
		ID:         kernel.ID(doc.ID),
		SlideCount: doc.SlideCount,
		Slide:      doc.Slide,
		Version:    doc.Version,
		StartedAt:  doc.StartedAt.UTC(),
		UpdatedAt:  doc.UpdatedAt.UTC(),
		EndedAt:    doc.EndedAt,
		ETag:       doc.ETag,
	})
}
