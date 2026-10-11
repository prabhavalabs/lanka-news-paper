package frontpage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	EditorialDraft     = "draft"
	EditorialPublished = "published"
	EditorialArchived  = "archived"
)

type EditorialSummary struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Standfirst  string    `json:"standfirst"`
	AuthorName  string    `json:"author_name"`
	PublishedAt time.Time `json:"published_at"`
}

type Editorial struct {
	EditorialSummary
	Body      string    `json:"body"`
	EventID   *string   `json:"event_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminEditorial includes drafts, whose published_at is the zero time until published.
type AdminEditorial struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Standfirst  string     `json:"standfirst"`
	Body        string     `json:"body"`
	AuthorName  string     `json:"author_name"`
	EventID     *string    `json:"event_id"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type EditorialInput struct {
	Title      string  `json:"title"`
	Standfirst string  `json:"standfirst"`
	Body       string  `json:"body"`
	AuthorName string  `json:"author_name"`
	EventID    *string `json:"event_id"`
}

func (input EditorialInput) clean() (EditorialInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Standfirst = strings.TrimSpace(input.Standfirst)
	input.Body = strings.TrimSpace(strings.ReplaceAll(input.Body, "\r\n", "\n"))
	input.AuthorName = strings.TrimSpace(input.AuthorName)
	if input.EventID != nil && strings.TrimSpace(*input.EventID) == "" {
		input.EventID = nil
	}
	switch {
	case input.Title == "" || utf8.RuneCountInString(input.Title) > 200:
		return input, fmt.Errorf("%w: a title of up to 200 characters is required", ErrInvalid)
	case utf8.RuneCountInString(input.Standfirst) > 400:
		return input, fmt.Errorf("%w: the standfirst is limited to 400 characters", ErrInvalid)
	case input.Body == "" || utf8.RuneCountInString(input.Body) > 20000:
		return input, fmt.Errorf("%w: a body of up to 20,000 characters is required", ErrInvalid)
	case input.AuthorName == "" || utf8.RuneCountInString(input.AuthorName) > 120:
		return input, fmt.Errorf("%w: an author name of up to 120 characters is required", ErrInvalid)
	}
	if input.EventID != nil {
		if _, err := uuid.Parse(*input.EventID); err != nil {
			return input, fmt.Errorf("%w: invalid story id", ErrInvalid)
		}
	}
	return input, nil
}

func (store *Store) publishedEditorials(ctx context.Context, limit int) ([]EditorialSummary, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, title, standfirst, author_name, published_at FROM editorials
		WHERE status = 'published' ORDER BY published_at DESC, id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("load editorials: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (EditorialSummary, error) {
		var item EditorialSummary
		err := row.Scan(&item.ID, &item.Title, &item.Standfirst, &item.AuthorName, &item.PublishedAt)
		return item, err
	})
}

func (service *Service) Editorials(ctx context.Context) ([]EditorialSummary, error) {
	return service.store.publishedEditorials(ctx, 50)
}

func (service *Service) Editorial(ctx context.Context, id string) (Editorial, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Editorial{}, ErrNotFound
	}
	var item Editorial
	err := service.store.pool.QueryRow(ctx, `
		SELECT id::text, title, standfirst, author_name, published_at, body, event_id::text, updated_at
		FROM editorials WHERE id = $1 AND status = 'published'`, id).
		Scan(&item.ID, &item.Title, &item.Standfirst, &item.AuthorName, &item.PublishedAt, &item.Body, &item.EventID, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Editorial{}, ErrNotFound
	}
	return item, err
}

const adminEditorialColumns = `id::text, title, standfirst, body, author_name, event_id::text, status, published_at, created_at, updated_at`

func scanAdminEditorial(row pgx.Row) (AdminEditorial, error) {
	var item AdminEditorial
	err := row.Scan(&item.ID, &item.Title, &item.Standfirst, &item.Body, &item.AuthorName, &item.EventID, &item.Status, &item.PublishedAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (service *Service) AdminEditorials(ctx context.Context) ([]AdminEditorial, error) {
	rows, err := service.store.pool.Query(ctx, `SELECT `+adminEditorialColumns+` FROM editorials
		ORDER BY CASE status WHEN 'draft' THEN 0 WHEN 'published' THEN 1 ELSE 2 END, COALESCE(published_at, updated_at) DESC, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (AdminEditorial, error) { return scanAdminEditorial(row) })
}

func (service *Service) CreateEditorial(ctx context.Context, actor uuid.UUID, input EditorialInput) (AdminEditorial, error) {
	input, err := input.clean()
	if err != nil {
		return AdminEditorial{}, err
	}
	return service.editorialWrite(ctx, actor, "editorial_create", func(tx pgx.Tx) (AdminEditorial, error) {
		return scanAdminEditorial(tx.QueryRow(ctx, `
			INSERT INTO editorials (title, standfirst, body, author_name, event_id, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $6) RETURNING `+adminEditorialColumns,
			input.Title, input.Standfirst, input.Body, input.AuthorName, input.EventID, actor))
	})
}

func (service *Service) UpdateEditorial(ctx context.Context, actor uuid.UUID, id string, input EditorialInput) (AdminEditorial, error) {
	if _, err := uuid.Parse(id); err != nil {
		return AdminEditorial{}, ErrNotFound
	}
	input, err := input.clean()
	if err != nil {
		return AdminEditorial{}, err
	}
	return service.editorialWrite(ctx, actor, "editorial_update", func(tx pgx.Tx) (AdminEditorial, error) {
		return scanAdminEditorial(tx.QueryRow(ctx, `
			UPDATE editorials SET title = $2, standfirst = $3, body = $4, author_name = $5, event_id = $6,
			       updated_by = $7, updated_at = clock_timestamp()
			WHERE id = $1 RETURNING `+adminEditorialColumns,
			id, input.Title, input.Standfirst, input.Body, input.AuthorName, input.EventID, actor))
	})
}

// SetEditorialStatus publishes, unpublishes (back to draft) or archives an editorial. The first
// publication date is kept when an editorial is republished.
func (service *Service) SetEditorialStatus(ctx context.Context, actor uuid.UUID, id, action string) (AdminEditorial, error) {
	if _, err := uuid.Parse(id); err != nil {
		return AdminEditorial{}, ErrNotFound
	}
	status := map[string]string{"publish": EditorialPublished, "unpublish": EditorialDraft, "archive": EditorialArchived}[action]
	if status == "" {
		return AdminEditorial{}, fmt.Errorf("%w: unknown action %q", ErrInvalid, action)
	}
	return service.editorialWrite(ctx, actor, "editorial_"+action, func(tx pgx.Tx) (AdminEditorial, error) {
		return scanAdminEditorial(tx.QueryRow(ctx, `
			UPDATE editorials SET status = $2,
			       published_at = CASE WHEN $2 = 'published' THEN COALESCE(published_at, clock_timestamp()) ELSE published_at END,
			       updated_by = $3, updated_at = clock_timestamp()
			WHERE id = $1 RETURNING `+adminEditorialColumns, id, status, actor))
	})
}

// DeleteEditorial removes a draft. Published and archived editorials are archived instead.
func (service *Service) DeleteEditorial(ctx context.Context, actor uuid.UUID, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrNotFound
	}
	_, err := service.editorialWrite(ctx, actor, "editorial_delete", func(tx pgx.Tx) (AdminEditorial, error) {
		tag, err := tx.Exec(ctx, `DELETE FROM editorials WHERE id = $1 AND status = 'draft'`, id)
		if err != nil {
			return AdminEditorial{}, err
		}
		if tag.RowsAffected() == 0 {
			return AdminEditorial{}, fmt.Errorf("%w: only drafts can be deleted; archive published editorials", ErrInvalid)
		}
		return AdminEditorial{ID: id}, nil
	})
	return err
}

func (service *Service) editorialWrite(ctx context.Context, actor uuid.UUID, action string, write func(pgx.Tx) (AdminEditorial, error)) (AdminEditorial, error) {
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return AdminEditorial{}, err
	}
	defer tx.Rollback(ctx)
	item, err := write(tx)
	if err != nil {
		if strings.Contains(err.Error(), "editorials_event_id_fkey") {
			return AdminEditorial{}, fmt.Errorf("%w: the linked story does not exist", ErrInvalid)
		}
		return AdminEditorial{}, err
	}
	if err := writeAudit(ctx, tx, actor, action, "editorial", item.ID); err != nil {
		return AdminEditorial{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminEditorial{}, err
	}
	service.invalidate()
	return item, nil
}
