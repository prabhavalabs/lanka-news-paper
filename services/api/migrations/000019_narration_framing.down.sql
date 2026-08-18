DROP INDEX article_political_analysis_frame;

CREATE INDEX article_political_analysis_frame
  ON article_political_analysis (economic_frame)
  WHERE confidence >= 0.45;

ALTER TABLE article_political_analysis
  DROP CONSTRAINT article_political_label_valid,
  DROP COLUMN provider_model,
  DROP COLUMN provider_id,
  DROP COLUMN evidence,
  DROP COLUMN rationale,
  DROP COLUMN label,
  DROP COLUMN relevant;
