BEGIN;
-- Trend radar: outside signals of what Sri Lanka is watching, searching and discussing, merged
-- with our own storylines into ranked topics that feed the Facebook Reel drafts.

-- YouTube channels whose uploads are watched for unusually fast-growing videos.
CREATE TABLE trend_youtube_channels (
  id text PRIMARY KEY CHECK (id ~ '^UC[A-Za-z0-9_-]{22}$'),
  title text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO trend_youtube_channels (id, title) VALUES
  ('UCCK3OZi788Ok44K97WAhLKQ', 'Ada Derana'),
  ('UCckltLEhFLv8Xz_lQhYfwmg', 'Hiru News'),
  ('UCgnFSj7jQffD5V5m05j4dPw', 'Newsfirst Sri Lanka'),
  ('UCV66ictLH2DZehG1tH0G5KA', 'ITN News'),
  ('UCGB1U55Jf7bp7knbUhCCTMA', 'Swarnavahini News'),
  ('UC8lKgUHo-2mSR5cRRXO9xow', 'Siyatha News'),
  ('UCeQSkHuVMGmuqLxq5JVHkNg', 'Neth News'),
  ('UCPXSUKHTNc1_hnHuj-UiQWQ', 'Rupavahini News'),
  ('UCOZfX56m_e3zcBQ2tkQxxIQ', 'Dasatha News'),
  ('UCcg4tDKgPLIc_x3lhWdogLg', 'LankaAnews'),
  ('UC2gl0cPNslZ6xkyc8RYa_xg', 'Lankadeepa'),
  ('UCjLHcArC62qBXayYl_TAN1A', 'Gossip Lanka News'),
  ('UCfdBd-9WWZUv1h7d80Omy8g', 'Truth with Chamuditha'),
  ('UCqEJfuHH218M_n7DkjIj3VA', 'Neth FM Balumgala');

-- Latest statistics of recent videos from the watched channels and YouTube's Sri Lanka chart.
-- Thirty days of uploads give each channel its normal view level (the baseline).
CREATE TABLE trend_youtube_videos (
  id text PRIMARY KEY CHECK (id ~ '^[A-Za-z0-9_-]{6,20}$'),
  channel_id text NOT NULL,
  channel_title text NOT NULL DEFAULT '',
  title text NOT NULL,
  published_at timestamptz NOT NULL,
  views bigint NOT NULL DEFAULT 0 CHECK (views >= 0),
  likes bigint NOT NULL DEFAULT 0 CHECK (likes >= 0),
  comments bigint NOT NULL DEFAULT 0 CHECK (comments >= 0),
  chart_rank integer CHECK (chart_rank > 0),
  first_seen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX trend_youtube_videos_channel ON trend_youtube_videos (channel_id, published_at DESC);
CREATE INDEX trend_youtube_videos_recent ON trend_youtube_videos (published_at DESC);

CREATE TABLE trend_radar_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'ready', 'failed')),
  research jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(research) = 'array'),
  signals jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(signals) = 'object'),
  error text,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  finished_at timestamptz
);
CREATE INDEX trend_radar_runs_recent ON trend_radar_runs (created_at DESC);

CREATE TABLE trend_topics (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL REFERENCES trend_radar_runs(id) ON DELETE CASCADE,
  rank integer NOT NULL CHECK (rank > 0),
  title_si text NOT NULL DEFAULT '',
  title_en text NOT NULL DEFAULT '',
  category text NOT NULL DEFAULT 'other',
  storyline_id uuid REFERENCES storylines(id) ON DELETE SET NULL,
  status text NOT NULL CHECK (status IN ('ready', 'watch', 'skip')),
  score numeric(6, 1) NOT NULL DEFAULT 0,
  components jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(components) = 'object'),
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(evidence) = 'object'),
  reason text NOT NULL DEFAULT '',
  UNIQUE (run_id, rank)
);

INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled) VALUES
  ('trend_research', 'codex_cli', 'gpt-6.1-sol', 'medium', 600, true),
  ('trend_radar', 'codex_cli', 'gpt-6.1-sol', 'medium', 300, true)
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience) VALUES
  ('trend_research', 'Trend research', 'Searches the web for what people in Sri Lanka are discussing in the last 24 hours, with evidence links.',
   'Social publishing', 'Sharp Sri Lankan social media researcher who checks every claim against sources.', 'precise', 'en', 'Thulawa editors'),
  ('trend_radar', 'Trend radar', 'Groups YouTube, Google Trends, web research and newsroom storylines into ranked topics for Facebook Reels.',
   'Social publishing', 'Trend editor who separates real public interest from noise.', 'decisive and neutral', 'en', 'Thulawa editors')
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task IN ('trend_research', 'trend_radar')
ON CONFLICT DO NOTHING;
COMMIT;
