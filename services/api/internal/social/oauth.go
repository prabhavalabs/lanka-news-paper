package social

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/meta"
)

type PendingPage struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (service *Service) clearExpiredStates(ctx context.Context) error {
	_, err := service.store.pool.Exec(ctx, `UPDATE social_oauth_states SET pending_pages_ciphertext=NULL,pending_pages_nonce=NULL WHERE expires_at<=$1 AND pending_pages_ciphertext IS NOT NULL`, service.now())
	return err
}

func (service *Service) Connect(ctx context.Context, actor uuid.UUID) (string, error) {
	if !service.config.Configured() {
		return "", ErrNotConfigured
	}
	if err := service.clearExpiredStates(ctx); err != nil {
		return "", err
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(value)
	hash := sha256.Sum256([]byte(state))
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	id := uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO social_oauth_states (id,state_hash,admin_user_id,expires_at) VALUES ($1,$2,$3,$4)`, id, hash[:], actor, service.now().Add(10*time.Minute)); err != nil {
		return "", err
	}
	if err = audit(ctx, tx, actor, "social_facebook_connect", id); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return service.graph.AuthorizeURL(state), nil
}

func (service *Service) consumeState(ctx context.Context, state string, actor uuid.UUID) (string, error) {
	hash := sha256.Sum256([]byte(state))
	var id string
	err := service.store.pool.QueryRow(ctx, `UPDATE social_oauth_states SET used_at=$3 WHERE state_hash=$1 AND admin_user_id=$2 AND used_at IS NULL AND expires_at>$3 RETURNING id::text`, hash[:], actor, service.now()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrOAuthState
	}
	return id, err
}

func (service *Service) Callback(ctx context.Context, code, state, providerError string, actor uuid.UUID) (string, string, error) {
	if !service.config.Configured() {
		return "", "", ErrNotConfigured
	}
	if err := service.clearExpiredStates(ctx); err != nil {
		return "", "", err
	}
	id, err := service.consumeState(ctx, state, actor)
	if err != nil {
		return "", "", err
	}
	if providerError != "" || code == "" {
		return "", "", fmt.Errorf("%w: Facebook authorization was declined", ErrInvalid)
	}
	short, err := service.graph.ExchangeCode(ctx, code)
	if err != nil {
		return "", "", err
	}
	long, err := service.graph.ExchangeLongLived(ctx, short.AccessToken)
	if err != nil {
		return "", "", err
	}
	info, err := service.graph.DebugToken(ctx, long.AccessToken)
	if err != nil {
		return "", "", err
	}
	if !service.validToken(info, "") {
		return "", "", fmt.Errorf("%w: Facebook permissions are missing", ErrInvalid)
	}
	pages, err := service.graph.ListPages(ctx, long.AccessToken)
	if err != nil {
		return "", "", err
	}
	if len(pages) == 0 {
		return "", "", fmt.Errorf("%w: no eligible managed Page", ErrInvalid)
	}
	if len(pages) == 1 {
		if err := service.connectPage(ctx, pages[0], actor, id); err != nil {
			return "", "", err
		}
		return "connected", "", nil
	}
	data, err := json.Marshal(pages)
	if err != nil {
		return "", "", err
	}
	encrypted, nonce, err := service.cipher.encrypt(data, "social:oauth:"+id)
	if err != nil {
		return "", "", err
	}
	result, err := service.store.pool.Exec(ctx, `UPDATE social_oauth_states SET pending_pages_ciphertext=$2,pending_pages_nonce=$3 WHERE id=$1 AND expires_at>$4`, id, encrypted, nonce, service.now())
	if err != nil {
		return "", "", err
	}
	if result.RowsAffected() != 1 {
		return "", "", ErrOAuthState
	}
	return "choose_page", id, nil
}

func (service *Service) validToken(info meta.TokenInfo, pageID string) bool {
	if !info.IsValid || info.AppID != service.config.AppID || (info.ExpiresAt > 0 && info.ExpiresAt <= service.now().Unix()) {
		return false
	}
	scopes := map[string]bool{}
	for _, scope := range info.Scopes {
		scopes[scope] = true
	}
	for _, scope := range []string{"pages_read_engagement", "pages_manage_posts"} {
		if !scopes[scope] {
			return false
		}
	}
	if pageID == "" {
		return scopes["pages_show_list"]
	}
	if info.ProfileID != "" && info.ProfileID != pageID {
		return false
	}
	for _, scope := range info.GranularScopes {
		if scope.Scope != "pages_manage_posts" && scope.Scope != "pages_read_engagement" {
			continue
		}
		if len(scope.TargetIDs) > 0 {
			found := false
			for _, target := range scope.TargetIDs {
				if target == pageID {
					found = true
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func (service *Service) loadPending(ctx context.Context, row scanner, id string) ([]meta.Page, error) {
	var encrypted, nonce []byte
	if err := row.Scan(&encrypted, &nonce); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOAuthState
		}
		return nil, err
	}
	if len(encrypted) == 0 {
		return nil, ErrOAuthState
	}
	data, err := service.cipher.decrypt(encrypted, nonce, "social:oauth:"+id)
	if err != nil {
		return nil, err
	}
	var pages []meta.Page
	if err = json.Unmarshal(data, &pages); err != nil {
		return nil, errors.New("saved Facebook Page selection is invalid")
	}
	return pages, nil
}

func (service *Service) PendingPages(ctx context.Context, id string, actor uuid.UUID) ([]PendingPage, error) {
	if !service.config.Configured() {
		return nil, ErrNotConfigured
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalid
	}
	if err := service.clearExpiredStates(ctx); err != nil {
		return nil, err
	}
	pages, err := service.loadPending(ctx, service.store.pool.QueryRow(ctx, `SELECT pending_pages_ciphertext,pending_pages_nonce FROM social_oauth_states WHERE id=$1 AND admin_user_id=$2 AND used_at IS NOT NULL AND expires_at>$3`, id, actor, service.now()), id)
	if err != nil {
		return nil, err
	}
	result := make([]PendingPage, 0, len(pages))
	for _, p := range pages {
		result = append(result, PendingPage{ID: p.ID, Name: p.Name, Picture: p.Picture.Data.URL})
	}
	return result, nil
}

func (service *Service) SelectPage(ctx context.Context, id, pageID string, actor uuid.UUID) error {
	if !service.config.Configured() {
		return ErrNotConfigured
	}
	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalid
	}
	if err := service.clearExpiredStates(ctx); err != nil {
		return err
	}
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT singleton FROM social_settings WHERE singleton FOR UPDATE`); err != nil {
		return err
	}
	pages, err := service.loadPending(ctx, tx.QueryRow(ctx, `SELECT pending_pages_ciphertext,pending_pages_nonce FROM social_oauth_states WHERE id=$1 AND admin_user_id=$2 AND used_at IS NOT NULL AND expires_at>$3 FOR UPDATE`, id, actor, service.now()), id)
	if err != nil {
		return err
	}
	var chosen *meta.Page
	for i := range pages {
		if pages[i].ID == pageID {
			chosen = &pages[i]
			break
		}
	}
	if chosen == nil {
		return ErrInvalid
	}
	info, err := service.graph.DebugToken(ctx, chosen.AccessToken)
	if err != nil {
		return err
	}
	if !service.validToken(info, chosen.ID) {
		return fmt.Errorf("%w: Facebook Page token is invalid or lacks permissions", ErrInvalid)
	}
	if err = service.savePage(ctx, tx, *chosen, info, actor, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service *Service) connectPage(ctx context.Context, page meta.Page, actor uuid.UUID, stateID string) error {
	info, err := service.graph.DebugToken(ctx, page.AccessToken)
	if err != nil {
		return err
	}
	if !service.validToken(info, page.ID) {
		return fmt.Errorf("%w: Facebook Page token is invalid or lacks permissions", ErrInvalid)
	}
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = service.savePage(ctx, tx, page, info, actor, stateID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service *Service) savePage(ctx context.Context, tx pgx.Tx, page meta.Page, info meta.TokenInfo, actor uuid.UUID, stateID string) error {
	// A settings-row lock serializes connects and disconnects, including when no account exists yet.
	if _, err := tx.Exec(ctx, `SELECT singleton FROM social_settings WHERE singleton FOR UPDATE`); err != nil {
		return err
	}
	if stateID != "" {
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT expires_at>$2 FROM social_oauth_states WHERE id=$1 AND admin_user_id=$3 FOR UPDATE`, stateID, service.now(), actor).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return ErrOAuthState
		}
	}
	encrypted, nonce, err := service.cipher.encrypt([]byte(page.AccessToken), "social:"+page.ID)
	if err != nil {
		return err
	}
	var expires *time.Time
	if info.ExpiresAt > 0 {
		value := time.Unix(info.ExpiresAt, 0)
		expires = &value
	}
	if _, err = tx.Exec(ctx, `UPDATE social_accounts SET status='disconnected',token_ciphertext=NULL,token_nonce=NULL,key_version=NULL WHERE status<>'disconnected'`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO social_accounts (page_id,page_name,page_picture_url,token_ciphertext,token_nonce,key_version,granted_scopes,status,connected_by,connected_at,last_verified_at,token_expires_at)
 VALUES ($1,$2,$3,$4,$5,'v1',$6,'connected',$7,$8,$8,$9)
 ON CONFLICT (page_id) DO UPDATE SET page_name=EXCLUDED.page_name,page_picture_url=EXCLUDED.page_picture_url,
 token_ciphertext=EXCLUDED.token_ciphertext,token_nonce=EXCLUDED.token_nonce,key_version=EXCLUDED.key_version,
 granted_scopes=EXCLUDED.granted_scopes,status='connected',connected_by=EXCLUDED.connected_by,
 connected_at=EXCLUDED.connected_at,last_verified_at=EXCLUDED.last_verified_at,token_expires_at=EXCLUDED.token_expires_at,last_error=NULL`,
		page.ID, page.Name, page.Picture.Data.URL, encrypted, nonce, info.Scopes, actor, service.now(), expires); err != nil {
		return err
	}
	if stateID != "" {
		if _, err = tx.Exec(ctx, `UPDATE social_oauth_states SET pending_pages_ciphertext=NULL,pending_pages_nonce=NULL WHERE id=$1`, stateID); err != nil {
			return err
		}
	}
	return audit(ctx, tx, actor, "social_facebook_connected", page.ID)
}

func (service *Service) Disconnect(ctx context.Context, actor uuid.UUID) error {
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT singleton FROM social_settings WHERE singleton FOR UPDATE`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE social_accounts SET status='disconnected',token_ciphertext=NULL,token_nonce=NULL,key_version=NULL`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE social_oauth_states SET pending_pages_ciphertext=NULL,pending_pages_nonce=NULL,used_at=COALESCE(used_at,$1),expires_at=LEAST(expires_at,$1)`, service.now()); err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "social_facebook_disconnected", "account"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (store *Store) Account(ctx context.Context) (*Account, error) {
	var a Account
	err := store.pool.QueryRow(ctx, `SELECT id::text,page_id,page_name,page_picture_url,status,connected_at,last_verified_at,token_expires_at,last_error FROM social_accounts ORDER BY (status='connected') DESC,updated_at DESC LIMIT 1`).Scan(&a.ID, &a.PageID, &a.PageName, &a.Picture, &a.Status, &a.ConnectedAt, &a.LastVerifiedAt, &a.TokenExpiresAt, &a.LastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &a, err
}
