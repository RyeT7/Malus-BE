package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

	"malus-be/internal/interaction/domain"
	"malus-be/internal/kernel"
	"malus-be/internal/platform/cosmosdb"
)

const (
	PartitionKeyPath   = "/pk"
	questionsPartition = "questions"
)

type questionDoc struct {
	ID         string     `json:"id"`
	PK         string     `json:"pk"`
	Text       string     `json:"text"`
	Author     string     `json:"author,omitempty"`
	Voters     []string   `json:"voters"`
	AskedAt    time.Time  `json:"askedAt"`
	AnsweredAt *time.Time `json:"answeredAt,omitempty"`
	ETag       string     `json:"_etag,omitempty"`
}

type QuestionRepository struct {
	container *azcosmos.ContainerClient
	pk        azcosmos.PartitionKey
}

func NewQuestionRepository(container *azcosmos.ContainerClient) *QuestionRepository {
	return &QuestionRepository{container: container, pk: azcosmos.NewPartitionKeyString(questionsPartition)}
}

func (r *QuestionRepository) Get(ctx context.Context, id kernel.ID) (*domain.Question, error) {
	resp, err := r.container.ReadItem(ctx, r.pk, id.String(), nil)
	if cosmosdb.StatusCode(err) == http.StatusNotFound {
		return nil, kernel.NotFound("question %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("read question: %w", err)
	}
	var doc questionDoc
	if err := json.Unmarshal(resp.Value, &doc); err != nil {
		return nil, fmt.Errorf("decode question: %w", err)
	}
	doc.ETag = string(resp.ETag)
	return toDomain(doc), nil
}

func (r *QuestionRepository) List(ctx context.Context) ([]*domain.Question, error) {
	pager := r.container.NewQueryItemsPager("SELECT * FROM c", r.pk, nil)
	var questions []*domain.Question
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("query questions: %w", err)
		}
		for _, item := range page.Items {
			var doc questionDoc
			if err := json.Unmarshal(item, &doc); err != nil {
				return nil, fmt.Errorf("decode question: %w", err)
			}
			questions = append(questions, toDomain(doc))
		}
	}
	return questions, nil
}

func (r *QuestionRepository) Save(ctx context.Context, q *domain.Question) error {
	snap := q.Snapshot()
	body, err := json.Marshal(questionDoc{
		ID:         snap.ID.String(),
		PK:         questionsPartition,
		Text:       snap.Text,
		Author:     snap.Author,
		Voters:     snap.Voters,
		AskedAt:    snap.AskedAt,
		AnsweredAt: snap.AnsweredAt,
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
			return fmt.Errorf("save question: %w", err)
		}
		return nil
	case http.StatusConflict, http.StatusPreconditionFailed:
		return kernel.ErrConcurrentUpdate
	case http.StatusNotFound:
		return kernel.NotFound("question %s", snap.ID)
	default:
		return fmt.Errorf("save question: %w", err)
	}
}

func (r *QuestionRepository) Ping(ctx context.Context) error {
	_, err := r.container.Read(ctx, nil)
	return err
}

func toDomain(doc questionDoc) *domain.Question {
	var answeredAt *time.Time
	if doc.AnsweredAt != nil {
		t := doc.AnsweredAt.UTC()
		answeredAt = &t
	}
	return domain.Restore(domain.Snapshot{
		ID:         kernel.ID(doc.ID),
		Text:       doc.Text,
		Author:     doc.Author,
		Voters:     doc.Voters,
		AskedAt:    doc.AskedAt.UTC(),
		AnsweredAt: answeredAt,
		ETag:       doc.ETag,
	})
}
