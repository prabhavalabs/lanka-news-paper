package watchtower

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) CreateThread(ctx context.Context, userID uuid.UUID, title string) (Thread, error) {
	var thread Thread
	err := store.pool.QueryRow(ctx, `
		INSERT INTO watch_tower_threads (user_id, title)
		VALUES ($1, $2)
		RETURNING id, user_id, title, created_at, updated_at
	`, userID, title).Scan(&thread.ID, &thread.UserID, &thread.Title, &thread.CreatedAt, &thread.UpdatedAt)
	return thread, err
}

func (store *Store) ListThreads(ctx context.Context, userID uuid.UUID) ([]Thread, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT thread.id, thread.user_id, thread.title,
		       count(message.id)::int,
		       COALESCE(last_message.content, ''),
		       thread.created_at, thread.updated_at
		FROM watch_tower_threads thread
		LEFT JOIN watch_tower_messages message ON message.thread_id = thread.id
		LEFT JOIN LATERAL (
		  SELECT content
		  FROM watch_tower_messages
		  WHERE thread_id = thread.id
		  ORDER BY created_at DESC, id DESC
		  LIMIT 1
		) last_message ON true
		WHERE thread.user_id = $1
		GROUP BY thread.id, last_message.content
		ORDER BY thread.updated_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	threads := make([]Thread, 0)
	for rows.Next() {
		var thread Thread
		if err := rows.Scan(&thread.ID, &thread.UserID, &thread.Title, &thread.MessageCount, &thread.LastMessage, &thread.CreatedAt, &thread.UpdatedAt); err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}
	return threads, rows.Err()
}

func (store *Store) Conversation(ctx context.Context, userID, threadID uuid.UUID) (Conversation, error) {
	var conversation Conversation
	err := store.pool.QueryRow(ctx, `
		SELECT id, user_id, title, created_at, updated_at
		FROM watch_tower_threads
		WHERE id = $1 AND user_id = $2
	`, threadID, userID).Scan(
		&conversation.Thread.ID, &conversation.Thread.UserID, &conversation.Thread.Title,
		&conversation.Thread.CreatedAt, &conversation.Thread.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	if err != nil {
		return Conversation{}, err
	}

	rows, err := store.pool.Query(ctx, `
		SELECT id, role, content, citations, suggestions,
		       COALESCE(provider_id, ''), COALESCE(provider_model, ''),
		       search_label, search_from, search_to, search_article_count, created_at
		FROM watch_tower_messages
		WHERE thread_id = $1
		ORDER BY created_at, id
	`, threadID)
	if err != nil {
		return Conversation{}, err
	}
	defer rows.Close()
	conversation.Messages = make([]Message, 0)
	for rows.Next() {
		message, scanErr := scanMessage(rows)
		if scanErr != nil {
			return Conversation{}, scanErr
		}
		conversation.Messages = append(conversation.Messages, message)
	}
	conversation.Thread.MessageCount = len(conversation.Messages)
	if len(conversation.Messages) > 0 {
		conversation.Thread.LastMessage = conversation.Messages[len(conversation.Messages)-1].Content
	}
	return conversation, rows.Err()
}

func (store *Store) SearchArticles(ctx context.Context, scope SearchScope) ([]ArticleEvidence, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT article.id, article.headline, source.name,
		       COALESCE(category.name_en, category.slug, 'Uncategorized'),
		       article.published_at, article.original_url,
		       COALESCE(event.display_title, ''),
		       COALESCE(NULLIF(document.summary_text, ''), NULLIF(article.description, ''),
		                LEFT(NULLIF(document.cleaned_text, ''), 1800),
		                LEFT(NULLIF(content.body_text, ''), 1800), ''),
		       article.public_status
		FROM articles article
		JOIN sources source ON source.id = article.source_id
		LEFT JOIN categories category ON category.id = article.category_id
		LEFT JOIN event_clusters event ON event.id = article.event_id
		LEFT JOIN article_analysis_documents document ON document.article_id = article.id
		LEFT JOIN article_contents content ON content.article_id = article.id AND content.current
		WHERE article.public_status NOT IN ('removed', 'quarantined')
		  AND article.published_at >= $1 AND article.published_at <= $2
		  AND ($3 = '' OR category.slug = $3)
		  AND (NOT $6 OR category.slug IS DISTINCT FROM 'world')
		  AND (
		    $5
		    OR cardinality($4::text[]) = 0
		    OR EXISTS (
		      SELECT 1 FROM unnest($4::text[]) term
		      WHERE lower(article.headline) LIKE '%' || lower(term) || '%'
		         OR lower(COALESCE(article.description, '')) LIKE '%' || lower(term) || '%'
		         OR lower(COALESCE(document.summary_text, '')) LIKE '%' || lower(term) || '%'
		         OR lower(COALESCE(document.cleaned_text, '')) LIKE '%' || lower(term) || '%'
		         OR lower(source.name) LIKE '%' || lower(term) || '%'
		         OR lower(COALESCE(category.name_en, '')) LIKE '%' || lower(term) || '%'
		         OR lower(COALESCE(event.display_title, '')) LIKE '%' || lower(term) || '%'
		    )
		  )
		ORDER BY (
		  SELECT count(*) FROM unnest($4::text[]) term
		  WHERE lower(article.headline || ' ' || COALESCE(article.description, '') || ' ' || COALESCE(document.summary_text, '') || ' ' || source.name || ' ' || COALESCE(category.name_en, ''))
		        LIKE '%' || lower(term) || '%'
		) DESC, article.published_at DESC
		LIMIT 40
	`, scope.From, scope.To, scope.Category, scope.Terms, scope.CategoryOnly, scope.ExcludeWorld)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	articles := make([]ArticleEvidence, 0)
	for rows.Next() {
		var article ArticleEvidence
		if err := rows.Scan(
			&article.ID, &article.Headline, &article.Source, &article.Category,
			&article.PublishedAt, &article.OriginalURL, &article.EventTitle,
			&article.Summary, &article.PublicStatus,
		); err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	return articles, rows.Err()
}

func (store *Store) SaveExchange(ctx context.Context, userID, threadID uuid.UUID, question string, assistant MessageDraft) (Conversation, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return Conversation{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var lockedThreadID uuid.UUID
	if err := transaction.QueryRow(ctx, `
		SELECT id FROM watch_tower_threads
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, threadID, userID).Scan(&lockedThreadID); errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrNotFound
	} else if err != nil {
		return Conversation{}, err
	}
	citations, err := json.Marshal(assistant.Citations)
	if err != nil {
		return Conversation{}, err
	}
	suggestions, err := json.Marshal(assistant.Suggestions)
	if err != nil {
		return Conversation{}, err
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO watch_tower_messages (thread_id, role, content)
		VALUES ($1, 'user', $2)
	`, threadID, question)
	if err != nil {
		return Conversation{}, err
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO watch_tower_messages (
		  thread_id, role, content, citations, suggestions, provider_id, provider_model,
		  search_label, search_from, search_to, search_article_count
		) VALUES ($1, 'assistant', $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9, $10)
	`, threadID, assistant.Content, citations, suggestions, assistant.Provider, assistant.Model,
		assistant.Search.Label, assistant.Search.From, assistant.Search.To, assistant.Search.ArticleCount)
	if err != nil {
		return Conversation{}, err
	}
	if _, err := transaction.Exec(ctx, `UPDATE watch_tower_threads SET updated_at = clock_timestamp() WHERE id = $1`, threadID); err != nil {
		return Conversation{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Conversation{}, err
	}
	return store.Conversation(ctx, userID, threadID)
}

func (store *Store) DeleteThread(ctx context.Context, userID, threadID uuid.UUID) error {
	command, err := store.pool.Exec(ctx, `DELETE FROM watch_tower_threads WHERE id = $1 AND user_id = $2`, threadID, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type messageScanner interface {
	Scan(...any) error
}

func scanMessage(scanner messageScanner) (Message, error) {
	var message Message
	var citations, suggestions []byte
	var searchLabel *string
	var searchFrom, searchTo *time.Time
	var searchCount *int
	if err := scanner.Scan(
		&message.ID, &message.Role, &message.Content, &citations, &suggestions,
		&message.Provider, &message.Model, &searchLabel, &searchFrom, &searchTo, &searchCount, &message.CreatedAt,
	); err != nil {
		return Message{}, err
	}
	message.Citations = make([]Citation, 0)
	message.Suggestions = make([]string, 0)
	if err := json.Unmarshal(citations, &message.Citations); err != nil {
		return Message{}, fmt.Errorf("decode watch tower citations: %w", err)
	}
	if err := json.Unmarshal(suggestions, &message.Suggestions); err != nil {
		return Message{}, fmt.Errorf("decode watch tower suggestions: %w", err)
	}
	if searchLabel != nil && searchFrom != nil && searchTo != nil && searchCount != nil {
		message.Search = &SearchSummary{Label: *searchLabel, From: *searchFrom, To: *searchTo, ArticleCount: *searchCount}
	}
	return message, nil
}
