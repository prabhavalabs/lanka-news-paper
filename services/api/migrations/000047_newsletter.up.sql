BEGIN;

CREATE TABLE newsletter_settings (
  singleton boolean PRIMARY KEY DEFAULT true,
  enabled boolean NOT NULL DEFAULT false,
  timezone text NOT NULL DEFAULT 'Asia/Colombo',
  send_hour smallint NOT NULL DEFAULT 8,
  configured_recipient_imported boolean NOT NULL DEFAULT false,
  updated_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT newsletter_settings_singleton CHECK (singleton),
  CONSTRAINT newsletter_send_hour_valid CHECK (send_hour BETWEEN 0 AND 23)
);

INSERT INTO newsletter_settings (singleton)
VALUES (true);

CREATE TABLE newsletter_subscribers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email citext NOT NULL UNIQUE,
  name text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active',
  consent_source text NOT NULL,
  consented_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  unsubscribe_token uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT newsletter_subscriber_status_valid CHECK (status IN ('active', 'paused', 'unsubscribed')),
  CONSTRAINT newsletter_subscriber_consent_source_not_blank CHECK (length(btrim(consent_source)) > 0),
  CONSTRAINT newsletter_subscriber_name_length CHECK (length(name) <= 160)
);

CREATE TRIGGER newsletter_subscribers_set_updated_at
BEFORE UPDATE ON newsletter_subscribers
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX newsletter_subscribers_status_created
  ON newsletter_subscribers (status, created_at DESC);

CREATE TABLE newsletter_editions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  edition_date date NOT NULL UNIQUE,
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  subject text NOT NULL,
  preheader text NOT NULL,
  digest jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT newsletter_edition_window_valid CHECK (window_start < window_end),
  CONSTRAINT newsletter_edition_digest_object CHECK (jsonb_typeof(digest) = 'object')
);

CREATE TABLE newsletter_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  edition_id uuid NOT NULL REFERENCES newsletter_editions(id) ON DELETE CASCADE,
  subscriber_id uuid NOT NULL REFERENCES newsletter_subscribers(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'pending',
  attempt_count integer NOT NULL DEFAULT 0,
  provider_message_id text,
  last_error text,
  sent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (edition_id, subscriber_id),
  CONSTRAINT newsletter_delivery_status_valid CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped')),
  CONSTRAINT newsletter_delivery_attempt_count_valid CHECK (attempt_count >= 0)
);

CREATE TRIGGER newsletter_deliveries_set_updated_at
BEFORE UPDATE ON newsletter_deliveries
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX newsletter_deliveries_status_created
  ON newsletter_deliveries (status, created_at DESC);

COMMIT;
