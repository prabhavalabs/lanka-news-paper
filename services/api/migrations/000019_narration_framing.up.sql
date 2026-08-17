ALTER TABLE article_political_analysis
  ADD COLUMN relevant boolean NOT NULL DEFAULT false,
  ADD COLUMN label text NOT NULL DEFAULT 'unclear',
  ADD COLUMN rationale text NOT NULL DEFAULT '',
  ADD COLUMN evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN provider_id text NOT NULL DEFAULT '',
  ADD COLUMN provider_model text NOT NULL DEFAULT '',
  ADD CONSTRAINT article_political_label_valid
    CHECK (label IN ('left', 'center_left', 'neutral', 'center_right', 'right', 'unclear'));

DROP INDEX article_political_analysis_frame;

CREATE INDEX article_political_analysis_frame
  ON article_political_analysis (economic_frame)
  WHERE relevant AND confidence >= 0.6;
