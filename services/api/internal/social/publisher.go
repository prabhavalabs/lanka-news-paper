package social

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/meta"
)

type historyReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (store *Store) publishHistory(ctx context.Context, now time.Time) ([]time.Time, error) {
	return publishHistory(ctx, store.pool, now)
}

func publishHistory(ctx context.Context, reader historyReader, now time.Time) ([]time.Time, error) {
	rows, err := reader.Query(ctx, `SELECT published_at FROM social_posts WHERE status='published' AND published_at>=$1 ORDER BY published_at`, now.Add(-48*time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]time.Time, 0)
	for rows.Next() {
		var at time.Time
		if err := rows.Scan(&at); err != nil {
			return nil, err
		}
		result = append(result, at)
	}
	return result, rows.Err()
}

func (service *Service) Status(ctx context.Context) (Status, error) {
	result := Status{Configured: service.config.Configured(), Counts: map[string]int{
		"pending_review": 0, "approved": 0, "publishing": 0, "published": 0, "rejected": 0, "skipped": 0, "failed": 0, "needs_attention": 0,
	}}
	var err error
	result.Settings, err = service.store.GetSettings(ctx)
	if err != nil {
		return result, err
	}
	result.Account, err = service.store.Account(ctx)
	if err != nil {
		return result, err
	}
	result.Connected = result.Account != nil && result.Account.Status == "connected"
	rows, err := service.store.pool.Query(ctx, `SELECT status,count(*)::int FROM social_posts GROUP BY status`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return result, err
		}
		result.Counts[status] = count
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if result.Settings.Enabled && result.Configured && result.Connected {
		history, err := service.store.publishHistory(ctx, service.now())
		if err != nil {
			return result, err
		}
		start := service.now()
		var retryAt *time.Time
		if err = service.store.pool.QueryRow(ctx, `SELECT (SELECT publish_retry_at FROM social_accounts WHERE status='connected')`).Scan(&retryAt); err != nil {
			return result, err
		}
		if retryAt != nil && retryAt.After(start) {
			start = *retryAt
		}
		next, err := NextEligible(start, result.Settings, history)
		if err != nil {
			return result, err
		}
		var scheduled *time.Time
		if err = service.store.pool.QueryRow(ctx, `SELECT min(COALESCE(scheduled_for,$1)) FROM social_posts WHERE status='approved'`, service.now()).Scan(&scheduled); err != nil {
			return result, err
		}
		if scheduled != nil && scheduled.After(next) {
			next = *scheduled
			next, err = NextEligible(next, result.Settings, history)
			if err != nil {
				return result, err
			}
		}
		result.NextPublishEligibleAt = &next
	}
	return result, nil
}

// nextInQueue returns the approved post that should go out next: the highest ranked, after
// dropping any automatic post that repeats one published since it was queued.
func (service *Service) nextInQueue(ctx context.Context, reader historyReader) (queuedPost, bool, error) {
	now := service.now()
	recent, err := service.store.recentPosts(ctx, now)
	if err != nil {
		return queuedPost{}, false, err
	}
	published := make([]recentPost, 0, len(recent))
	for _, post := range recent {
		if post.Status == "published" || post.Status == "publishing" {
			published = append(published, post)
		}
	}
	rows, err := reader.Query(ctx, `SELECT p.id::text,p.kind='breaking',p.score,COALESCE(c.slug,''),p.headline_si,p.summary_si,p.created_at,p.approved_by IS NULL,
 COALESCE(p.repeat_checked_at,p.approved_at,p.created_at),
 `+storylineJSON("p.event_id")+`,p.kind,
 (SELECT count(*) FROM social_posts prior JOIN storyline_events member ON member.event_id=prior.event_id
 WHERE member.storyline_id=(SELECT storyline_id FROM storyline_events WHERE event_id=p.event_id)
 AND prior.status='published' AND prior.kind='update' AND prior.published_at >= $1::timestamptz-interval '24 hours')
 FROM social_posts p LEFT JOIN event_clusters e ON e.id=p.event_id LEFT JOIN articles a ON a.id=p.article_id
 LEFT JOIN categories c ON c.id=COALESCE(e.category_id,a.category_id)
 WHERE p.status='approved' AND COALESCE(p.scheduled_for,$1)<=$1 ORDER BY p.created_at LIMIT 100`, now)
	if err != nil {
		return queuedPost{}, false, err
	}
	var best queuedPost
	found, repeats := false, make([]string, 0)
	for rows.Next() {
		var post queuedPost
		post.Now = now
		if err := rows.Scan(&post.ID, &post.Breaking, &post.Score, &post.CategorySlug, &post.Headline, &post.Summary, &post.CreatedAt, &post.Automatic, &post.CheckedSince, &post.Storyline, &post.Kind, &post.Updates24h); err != nil {
			rows.Close()
			return queuedPost{}, false, err
		}
		// Spacing and the daily update cap defer a post; they never mark it skipped.
		blocked := post.Kind == "update" && post.Updates24h >= storylineDailyUpdateCap
		for _, prior := range published {
			blocked = blocked || storylineBlocked(post.Storyline, post.Breaking, prior, now)
		}
		if blocked {
			continue
		}
		if post.Automatic && repeatsPublished(post, published) != nil {
			repeats = append(repeats, post.ID)
			continue
		}
		if !found || post.rank(published, now) > best.rank(published, now) {
			best, found = post, true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return queuedPost{}, false, err
	}
	for _, id := range repeats {
		if _, err := service.store.pool.Exec(ctx, `UPDATE social_posts SET status='skipped',last_error='Not published: a post on the same story went out first' WHERE id=$1 AND status='approved'`, id); err != nil {
			return queuedPost{}, false, err
		}
	}
	return best, found, nil
}

func (service *Service) Publish(ctx context.Context) error {
	if !service.config.Configured() {
		return nil
	}
	connection, err := service.store.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	var locked bool
	if err = connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(540054)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	// Retain this session lock across the network request, not only the claim transaction.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := connection.Exec(cleanup, `SELECT pg_advisory_unlock(540054)`); err != nil {
			_ = connection.Conn().Close(cleanup)
		}
	}()
	// A process may have died after Meta accepted the upload. Never reclaim it for an automatic retry.
	if _, err = connection.Exec(ctx, `UPDATE social_posts SET status='needs_attention',last_error='Publisher stopped before recording the outcome; check the Page before retrying' WHERE status='publishing' AND updated_at<$1`, service.now().Add(-10*time.Minute)); err != nil {
		return err
	}
	settings, err := scanSettings(connection.QueryRow(ctx, `SELECT `+settingsColumns+` FROM social_settings WHERE singleton`))
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	history, err := publishHistory(ctx, connection, service.now())
	if err != nil {
		return err
	}
	// Posts approved automatically go stale if caps kept them queued; an editor's approval stands.
	if _, err = connection.Exec(ctx, `UPDATE social_posts SET status='skipped',last_error='Not published: the story was more than 12 hours old when a publishing slot opened'
 WHERE status='approved' AND approved_by IS NULL AND created_at<$1`, service.now().Add(-autoPublishMaxAge)); err != nil {
		return err
	}
	var next queuedPost
	for {
		var found bool
		next, found, err = service.nextInQueue(ctx, connection)
		if err != nil || !found {
			return err
		}
		eligible, err := NextEligible(service.now(), limitsFor(next, settings), history)
		if err != nil {
			return err
		}
		if eligible.After(service.now()) {
			return nil
		}
		repeated, err := service.guardRepeat(ctx, next)
		if err != nil {
			return err
		}
		if !repeated {
			break
		}
	}
	tx, err := connection.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var accountID, pageID, keyVersion string
	var encrypted, nonce []byte
	var expires, retryAt *time.Time
	err = tx.QueryRow(ctx, `SELECT id::text,page_id,token_ciphertext,token_nonce,COALESCE(key_version,''),token_expires_at,publish_retry_at FROM social_accounts WHERE status='connected' FOR UPDATE SKIP LOCKED`).Scan(&accountID, &pageID, &encrypted, &nonce, &keyVersion, &expires, &retryAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if retryAt != nil && retryAt.After(service.now()) {
		return nil
	}
	if expires != nil && !expires.After(service.now()) {
		if _, err = tx.Exec(ctx, `UPDATE social_accounts SET status='needs_reconnect',last_error='Facebook Page token has expired; reconnect' WHERE id=$1`, accountID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if keyVersion != "v1" {
		return errors.New("unsupported social token key version; reconnect the Page")
	}
	token, err := service.cipher.decrypt(encrypted, nonce, "social:"+pageID)
	if err != nil {
		if _, updateErr := tx.Exec(ctx, `UPDATE social_accounts SET status='needs_reconnect',last_error='Page token could not be decrypted; reconnect' WHERE id=$1`, accountID); updateErr != nil {
			return updateErr
		}
		return tx.Commit(ctx)
	}
	p, err := scanPost(tx.QueryRow(ctx, postSelect+`WHERE p.id=$1 AND p.status='approved' FOR UPDATE OF p SKIP LOCKED`, next.ID))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	rightsEligible, err := publishablePost(ctx, tx, p, service.now())
	if err != nil {
		return err
	}
	if !rightsEligible {
		if _, err = tx.Exec(ctx, `UPDATE social_posts SET status='failed',last_error='Source rights or category availability changed; reject this post and prepare it again from current evidence' WHERE id=$1`, p.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if p.ImageKey == nil || service.media == nil {
		if _, err = tx.Exec(ctx, `UPDATE social_posts SET status='failed',last_error='A rendered image is required before publishing; render this post again and retry' WHERE id=$1`, p.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	file, _, err := service.media.Open(ctx, *p.ImageKey)
	if err != nil {
		if _, err = tx.Exec(ctx, `UPDATE social_posts SET status='failed',image_key=NULL,last_error='Rendered image could not be read; render this post again and retry' WHERE id=$1`, p.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	png, readErr := io.ReadAll(io.LimitReader(file, (20<<20)+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(png) == 0 || len(png) > 20<<20 {
		// Fail the post rather than the job so one bad image cannot hold up the queue.
		if _, err = tx.Exec(ctx, `UPDATE social_posts SET status='failed',last_error='Rendered image could not be read or exceeds 20 MB; render this post again and retry' WHERE id=$1`, p.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE social_posts SET status='publishing',account_id=$2,attempts=attempts+1,last_error=NULL WHERE id=$1`, p.ID, accountID); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	photo, publishErr := service.graph.PublishPhoto(ctx, pageID, string(token), png, p.Caption)
	// Persist a known outcome even if the River/request context was canceled.
	resultContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if publishErr == nil {
		// Reserve time to save a confirmed upload even if the optional lookup times out.
		linkContext, cancelLink := context.WithTimeout(resultContext, 10*time.Second)
		permalink, linkErr := service.graph.Permalink(linkContext, photo.PostID, string(token))
		cancelLink()
		var lastError *string
		if linkErr != nil {
			value := "Published successfully; permalink lookup failed"
			lastError = &value
		}
		_, err = connection.Exec(resultContext, `UPDATE social_posts SET status='published',published_at=$2,fb_post_id=$3,fb_permalink=NULLIF($4,''),last_error=$5 WHERE id=$1 AND status='publishing'`, p.ID, service.now(), photo.PostID, permalink, lastError)
		return err
	}
	message := truncate(meta.Redact(publishErr.Error(), string(token), service.config.AppSecret), 1000)
	status := "failed"
	var scheduled *time.Time
	var graphError *meta.Error
	var transport *meta.TransportError
	if errors.As(publishErr, &graphError) {
		switch graphError.Classification() {
		case "needs_reconnect":
			status = "approved"
			if _, err = connection.Exec(resultContext, `UPDATE social_accounts SET status='needs_reconnect',last_error=$2 WHERE id=$1 AND status='connected' AND token_ciphertext=$3`, accountID, message, encrypted); err != nil {
				return err
			}
		case "rate_limited":
			status = "approved"
			value := service.now().Add(30 * time.Minute)
			scheduled = &value
			// Meta limits apply to the account, including other queued and newly selected posts.
			if _, err = connection.Exec(resultContext, `UPDATE social_accounts SET publish_retry_at=$2,last_error=$3 WHERE id=$1 AND status='connected' AND token_ciphertext=$4`, accountID, value, message, encrypted); err != nil {
				return err
			}
		}
	} else if errors.As(publishErr, &transport) {
		if transport.Sent {
			status = "needs_attention"
		} else {
			status = "approved"
			value := service.now().Add(5 * time.Minute)
			scheduled = &value
		}
	}
	_, err = connection.Exec(resultContext, `UPDATE social_posts SET status=$2,scheduled_for=COALESCE($3,scheduled_for),last_error=$4 WHERE id=$1 AND status='publishing'`, p.ID, status, scheduled, message)
	if err != nil {
		return fmt.Errorf("save social publish outcome: %w", err)
	}
	return nil
}
