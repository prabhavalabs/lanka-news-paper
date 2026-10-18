BEGIN;
CREATE TABLE social_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  platform text NOT NULL DEFAULT 'facebook_page' CHECK (platform = 'facebook_page'),
  page_id text NOT NULL UNIQUE,
  page_name text NOT NULL,
  page_picture_url text NOT NULL DEFAULT '',
  token_ciphertext bytea,
  token_nonce bytea,
  key_version text,
  granted_scopes text[] NOT NULL DEFAULT '{}',
  status text NOT NULL CHECK (status IN ('connected', 'needs_reconnect', 'disconnected')),
  connected_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  connected_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  last_verified_at timestamptz,
  token_expires_at timestamptz,
  last_error text,
  publish_retry_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK ((token_ciphertext IS NULL) = (token_nonce IS NULL)),
  CHECK (status <> 'connected' OR (token_ciphertext IS NOT NULL AND key_version IS NOT NULL))
);
CREATE UNIQUE INDEX social_one_connected_account ON social_accounts ((true)) WHERE status = 'connected';
CREATE TRIGGER social_accounts_set_updated_at BEFORE UPDATE ON social_accounts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TABLE social_oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  state_hash bytea NOT NULL UNIQUE CHECK (octet_length(state_hash) = 32),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL DEFAULT clock_timestamp() + interval '10 minutes',
  used_at timestamptz,
  pending_pages_ciphertext bytea,
  pending_pages_nonce bytea,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK ((pending_pages_ciphertext IS NULL) = (pending_pages_nonce IS NULL))
);
CREATE INDEX social_oauth_expiry ON social_oauth_states (expires_at);
ALTER TABLE social_settings DROP COLUMN publish_retry_at;
DROP INDEX social_posts_loomcast_post;
ALTER TABLE social_posts
  DROP COLUMN handed_off_at,
  DROP COLUMN loomcast_post_id,
  ADD COLUMN account_id uuid REFERENCES social_accounts(id) ON DELETE SET NULL;
COMMIT;
