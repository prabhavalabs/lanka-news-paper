package social

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/loomcast"
)

const (
	// Loomcast schedules posts at least two minutes ahead; the rest covers the upload.
	loomcastLeadTime = 150 * time.Second
	// A post handed to Loomcast that is still not live after this long needs an editor.
	handoffStaleAfter = 6 * time.Hour
	// Loomcast failures that may pass are retried automatically while a post has fewer attempts.
	maxPublishAttempts = 5
	// Captions longer than Loomcast accepts are cut; the publisher never sends more.
	maxCaptionRunes = 2200
)

type historyReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (store *Store) publishHistory(ctx context.Context, now time.Time) ([]time.Time, error) {
	return publishHistory(ctx, store.pool, now)
}

// publishHistory returns when recent posts went out. A post handed to Loomcast counts from its
// handoff time, so the caps and the minimum gap hold before Loomcast reports it live.
func publishHistory(ctx context.Context, reader historyReader, now time.Time) ([]time.Time, error) {
	rows, err := reader.Query(ctx, `SELECT COALESCE(published_at,handed_off_at) FROM social_posts
 WHERE (status='published' AND published_at>=$1) OR (status='publishing' AND handed_off_at IS NOT NULL) ORDER BY 1`, now.Add(-48*time.Hour))
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
	lookup := service.publishChannel(ctx, false)
	result.Channel, result.Problem, result.Warning, result.Ready = publishChannelOf(lookup), lookup.problem, lookup.warning, lookup.ready()
	result.PublishRetryAt, err = service.store.publishRetryAt(ctx, service.store.pool)
	if err != nil {
		return result, err
	}
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
	if result.Settings.Enabled && result.Ready {
		history, err := service.store.publishHistory(ctx, service.now())
		if err != nil {
			return result, err
		}
		start := service.now()
		if result.PublishRetryAt != nil && result.PublishRetryAt.After(start) {
			start = *result.PublishRetryAt
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

// Publish first settles posts already handed to Loomcast, then hands over the next post in the
// queue when the caps, the minimum gap and quiet hours allow. Loomcast publishes it on the
// Facebook Page a little over two minutes later; a later run records the outcome.
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
	// Retain this session lock across the network requests, not only the claim transaction.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := connection.Exec(cleanup, `SELECT pg_advisory_unlock(540054)`); err != nil {
			_ = connection.Conn().Close(cleanup)
		}
	}()
	if err = service.syncHandedOff(ctx, connection); err != nil {
		return err
	}
	settings, err := scanSettings(connection.QueryRow(ctx, `SELECT `+settingsColumns+` FROM social_settings WHERE singleton`))
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	retryAt, err := service.store.publishRetryAt(ctx, connection)
	if err != nil {
		return err
	}
	if retryAt != nil && retryAt.After(service.now()) {
		return nil
	}
	lookup := service.publishChannel(ctx, true)
	if !lookup.ready() {
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
	image, readErr := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(image) == 0 || len(image) > 4<<20 {
		// Fail the post rather than the job so one bad image cannot hold up the queue.
		if _, err = tx.Exec(ctx, `UPDATE social_posts SET status='failed',last_error='Rendered image could not be read or exceeds the 4 MB Facebook photo limit; render this post again and retry' WHERE id=$1`, p.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	at := service.now().Add(loomcastLeadTime).UTC().Truncate(time.Second)
	var attempt int
	if err = tx.QueryRow(ctx, `UPDATE social_posts SET status='publishing',attempts=attempts+1,last_error=NULL,loomcast_post_id=NULL,handed_off_at=$2
 WHERE id=$1 RETURNING attempts`, p.ID, at).Scan(&attempt); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return service.handOff(ctx, connection, p, attempt, image, at, lookup.channel.ID)
}

// handOff creates the post in Loomcast, uploads the card and finalizes it. Nothing is published
// before finalize, so a handoff that fails part way is cancelled and queued again.
func (service *Service) handOff(ctx context.Context, connection *pgxpool.Conn, p Post, attempt int, image []byte, at time.Time, channelID string) error {
	digest := sha256.Sum256(image)
	file := loomcast.Media{Filename: "thulawa-" + p.ID + ".png", MIME: "image/png", Bytes: len(image), SHA256: hex.EncodeToString(digest[:]), AltText: truncate(p.HeadlineSI, 1000)}
	if config, err := png.DecodeConfig(bytes.NewReader(image)); err == nil {
		file.Width, file.Height = config.Width, config.Height
	}
	request := loomcast.NewPost{Format: "image", Title: truncate(p.HeadlineSI, 200), Caption: limitCaption(p.Caption),
		Media: []loomcast.Media{file}, Channels: []loomcast.ChannelRef{{ChannelID: channelID}}, ScheduledAt: at, ExternalRef: p.ID}
	created, err := service.loomcast.CreatePost(ctx, fmt.Sprintf("thulawa-social-%s-%d", p.ID, attempt), request)
	// Persist a known outcome even if the River context was canceled.
	resultContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err != nil {
		return service.handOffFailed(resultContext, connection, p.ID, "", err)
	}
	if _, err = connection.Exec(resultContext, `UPDATE social_posts SET loomcast_post_id=$2 WHERE id=$1 AND status='publishing'`, p.ID, created.ID); err != nil {
		return err
	}
	for _, upload := range created.Uploads {
		if err = service.loomcast.Upload(ctx, upload, image); err != nil {
			return service.handOffFailed(resultContext, connection, p.ID, created.ID, err)
		}
	}
	final, err := service.loomcast.Finalize(ctx, created.ID)
	if err != nil {
		var transport *loomcast.TransportError
		if errors.As(err, &transport) {
			// Finalize may have taken effect; the next run reads the post back before acting.
			_, err = connection.Exec(resultContext, `UPDATE social_posts SET last_error='Handed to Loomcast; waiting to confirm it is scheduled' WHERE id=$1 AND status='publishing'`, p.ID)
			return err
		}
		return service.handOffFailed(resultContext, connection, p.ID, created.ID, err)
	}
	return service.applyRemote(resultContext, connection, p.ID, final, channelID)
}

// handOffFailed records a handoff that did not reach finalize. The unfinished Loomcast post is
// withdrawn, and the post is queued again when the failure may pass.
func (service *Service) handOffFailed(ctx context.Context, connection *pgxpool.Conn, id, remoteID string, failure error) error {
	if remoteID != "" {
		_, _ = service.loomcast.Cancel(ctx, remoteID)
	}
	message := truncate("Loomcast: "+failure.Error(), 1000)
	var apiError *loomcast.APIError
	var transport *loomcast.TransportError
	retryIn := time.Duration(0)
	switch {
	case errors.As(failure, &apiError) && apiError.Unauthorized():
		// The key or its grants need the owner; hold every post until then.
		service.forgetChannel()
		if err := service.pausePublishing(ctx, connection, 30*time.Minute); err != nil {
			return err
		}
		retryIn = time.Minute
	case errors.As(failure, &apiError) && apiError.Status == 429:
		if err := service.pausePublishing(ctx, connection, max(apiError.RetryAfter, time.Minute)); err != nil {
			return err
		}
		retryIn = time.Minute
	case errors.As(failure, &apiError) && apiError.Temporary(), errors.As(failure, &transport):
		retryIn = 5 * time.Minute
	}
	if retryIn > 0 {
		_, err := connection.Exec(ctx, `UPDATE social_posts SET status=CASE WHEN attempts<$4 THEN 'approved' ELSE 'failed' END,
 scheduled_for=CASE WHEN attempts<$4 THEN $2 ELSE scheduled_for END,loomcast_post_id=NULL,handed_off_at=NULL,last_error=$3
 WHERE id=$1 AND status='publishing'`, id, service.now().Add(retryIn), message, maxPublishAttempts)
		return err
	}
	_, err := connection.Exec(ctx, `UPDATE social_posts SET status='failed',loomcast_post_id=NULL,handed_off_at=NULL,last_error=$2 WHERE id=$1 AND status='publishing'`, id, message)
	return err
}

func (service *Service) pausePublishing(ctx context.Context, connection *pgxpool.Conn, wait time.Duration) error {
	_, err := connection.Exec(ctx, `UPDATE social_settings SET publish_retry_at=$1 WHERE singleton`, service.now().Add(wait))
	return err
}

// syncHandedOff reads back every post handed to Loomcast and records how it went.
func (service *Service) syncHandedOff(ctx context.Context, connection *pgxpool.Conn) error {
	// The process stopped before Loomcast had the post id saved, so nothing was finalized.
	if _, err := connection.Exec(ctx, `UPDATE social_posts SET status='approved',handed_off_at=NULL,last_error='The handoff to Loomcast was interrupted; queued again'
 WHERE status='publishing' AND loomcast_post_id IS NULL AND updated_at<$1`, service.now().Add(-10*time.Minute)); err != nil {
		return err
	}
	rows, err := connection.Query(ctx, `SELECT id::text,loomcast_post_id,handed_off_at FROM social_posts
 WHERE status='publishing' AND loomcast_post_id IS NOT NULL ORDER BY handed_off_at LIMIT 50`)
	if err != nil {
		return err
	}
	type handedOff struct {
		id, remoteID string
		at           *time.Time
	}
	pending := make([]handedOff, 0)
	for rows.Next() {
		var item handedOff
		if err := rows.Scan(&item.id, &item.remoteID, &item.at); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	channelID := service.config.LoomcastChannelID
	if lookup := service.publishChannel(ctx, false); lookup.channel != nil {
		channelID = lookup.channel.ID
	}
	for _, item := range pending {
		remote, err := service.loomcast.Post(ctx, item.remoteID)
		var apiError *loomcast.APIError
		switch {
		case errors.As(err, &apiError) && apiError.Status == 404:
			if _, err = connection.Exec(ctx, `UPDATE social_posts SET status='needs_attention',last_error='Loomcast no longer shows this post; check the Page before retrying' WHERE id=$1 AND status='publishing'`, item.id); err != nil {
				return err
			}
			continue
		case errors.As(err, &apiError) && apiError.Unauthorized():
			service.forgetChannel()
			return nil
		case err != nil:
			// Loomcast is busy or unreachable; the next run tries again.
			continue
		}
		if remote.Status == "awaiting_media" {
			// The earlier handoff stopped before finalize; withdraw it and start over.
			if _, err := service.loomcast.Cancel(ctx, remote.ID); err != nil {
				continue
			}
			if _, err := connection.Exec(ctx, `UPDATE social_posts SET status='approved',scheduled_for=$2,loomcast_post_id=NULL,handed_off_at=NULL,
 last_error='The handoff to Loomcast did not finish; queued again' WHERE id=$1 AND status='publishing'`, item.id, service.now()); err != nil {
				return err
			}
			continue
		}
		if err := service.applyRemote(ctx, connection, item.id, remote, channelID); err != nil {
			return err
		}
		if item.at != nil && item.at.Before(service.now().Add(-handoffStaleAfter)) {
			if _, err := connection.Exec(ctx, `UPDATE social_posts SET status='needs_attention',last_error='Loomcast has not published this post after six hours; check Loomcast and the Page before retrying'
 WHERE id=$1 AND status='publishing'`, item.id); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyRemote records what Loomcast reports for a handed-off post.
func (service *Service) applyRemote(ctx context.Context, connection *pgxpool.Conn, id string, remote loomcast.Post, channelID string) error {
	target := remote.Target(channelID)
	if target == nil && len(remote.Targets) > 0 {
		target = &remote.Targets[len(remote.Targets)-1]
	}
	targetStatus := ""
	if target != nil && target.Status != nil {
		targetStatus = *target.Status
	}
	targetError := ""
	if target != nil && target.LastError != nil {
		targetError = target.LastError.Message
	}
	var err error
	switch {
	case targetStatus == "published" || (target == nil && remote.Status == "published"):
		if target != nil && target.DryRun {
			_, err = connection.Exec(ctx, `UPDATE social_posts SET status='failed',last_error=$2 WHERE id=$1 AND status='publishing'`, id,
				"Loomcast published this as a dry run, so nothing reached Facebook. Turn dry run off for the channel in Loomcast, then retry.")
			return err
		}
		publishedAt := service.now()
		var externalID, permalink *string
		if target != nil {
			if target.PublishedAt != nil {
				publishedAt = *target.PublishedAt
			}
			externalID, permalink = target.ExternalID, target.Permalink
		}
		_, err = connection.Exec(ctx, `UPDATE social_posts SET status='published',published_at=$2,fb_post_id=$3,fb_permalink=$4,last_error=NULL WHERE id=$1 AND status='publishing'`,
			id, publishedAt, externalID, permalink)
	case remote.Status == "pending_approval":
		_, err = connection.Exec(ctx, `UPDATE social_posts SET last_error='Waiting for the owner to approve it in Loomcast' WHERE id=$1 AND status='publishing'`, id)
	case remote.Status == "cancelled" || targetStatus == "cancelled":
		_, err = connection.Exec(ctx, `UPDATE social_posts SET status='failed',last_error='Cancelled in Loomcast: withdrawn or rejected by the owner' WHERE id=$1 AND status='publishing'`, id)
	case targetStatus == "failed" || targetStatus == "missed" || remote.Status == "failed" || remote.Status == "partially_published":
		message := "Loomcast could not publish it"
		if targetStatus == "missed" {
			message = "Missed its time in Loomcast while it waited for approval"
		}
		if targetError != "" {
			message += ": " + targetError
		}
		retry := target != nil && target.LastError != nil && target.LastError.Retryable && targetStatus == "failed"
		wait := 10 * time.Minute
		if target != nil && target.LastError != nil && time.Duration(target.LastError.RetryAfterMs)*time.Millisecond > wait {
			wait = time.Duration(target.LastError.RetryAfterMs) * time.Millisecond
		}
		_, err = connection.Exec(ctx, `UPDATE social_posts SET status=CASE WHEN $3 AND attempts<$5 THEN 'approved' ELSE 'failed' END,
 scheduled_for=CASE WHEN $3 AND attempts<$5 THEN $4 ELSE scheduled_for END,
 loomcast_post_id=CASE WHEN $3 AND attempts<$5 THEN NULL ELSE loomcast_post_id END,
 handed_off_at=CASE WHEN $3 AND attempts<$5 THEN NULL ELSE handed_off_at END,last_error=$2 WHERE id=$1 AND status='publishing'`,
			id, truncate(message, 1000), retry, service.now().Add(wait), maxPublishAttempts)
	default:
		// Scheduled or publishing: Loomcast holds it, possibly for a Facebook rate-limit window.
		var note *string
		if targetError != "" {
			value := truncate("Loomcast is holding it for a publishing window: "+targetError, 1000)
			note = &value
		}
		_, err = connection.Exec(ctx, `UPDATE social_posts SET last_error=$2 WHERE id=$1 AND status='publishing' AND last_error IS DISTINCT FROM $2`, id, note)
	}
	return err
}

// limitCaption cuts a caption to Loomcast's limit and keeps its line breaks.
func limitCaption(caption string) string {
	runes := []rune(caption)
	if len(runes) <= maxCaptionRunes {
		return caption
	}
	return strings.TrimSpace(string(runes[:maxCaptionRunes-1])) + "…"
}
