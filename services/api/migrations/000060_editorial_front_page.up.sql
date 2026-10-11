-- The chief editor curates the public front page and writes the editorial (කතුවැකිය).
ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_valid;
ALTER TABLE admin_users ADD CONSTRAINT admin_users_role_valid CHECK (role IN (
  'administrator', 'chief_editor', 'source_manager', 'editor', 'compliance_reviewer', 'operations_engineer'
));

-- Sources the chief editor allows on the front page. Excluded sources still appear in
-- section pages, search and story pages.
ALTER TABLE sources ADD COLUMN front_page boolean NOT NULL DEFAULT true;

CREATE TABLE front_page_placements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slot text NOT NULL CHECK (slot IN ('lead', 'top', 'more')),
  rank integer NOT NULL CHECK (rank >= 0),
  event_id uuid REFERENCES event_clusters(id) ON DELETE CASCADE,
  article_id uuid REFERENCES articles(id) ON DELETE CASCADE,
  note text CHECK (note IS NULL OR char_length(note) <= 280),
  expires_at timestamptz,
  placed_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  placed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK ((event_id IS NULL) <> (article_id IS NULL))
);
CREATE UNIQUE INDEX front_page_placements_event ON front_page_placements (event_id) WHERE event_id IS NOT NULL;
CREATE UNIQUE INDEX front_page_placements_article ON front_page_placements (article_id) WHERE article_id IS NOT NULL;
CREATE UNIQUE INDEX front_page_one_lead ON front_page_placements (slot) WHERE slot = 'lead';

CREATE TABLE editorials (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
  standfirst text NOT NULL DEFAULT '' CHECK (char_length(standfirst) <= 400),
  body text NOT NULL CHECK (char_length(btrim(body)) BETWEEN 1 AND 20000),
  author_name text NOT NULL CHECK (char_length(btrim(author_name)) BETWEEN 1 AND 120),
  event_id uuid REFERENCES event_clusters(id) ON DELETE SET NULL,
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at timestamptz,
  created_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE INDEX editorials_published ON editorials (published_at DESC) WHERE status = 'published';
