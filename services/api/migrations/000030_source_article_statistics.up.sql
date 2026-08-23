BEGIN;

CREATE TABLE source_article_statistics (
  source_id uuid PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
  published_article_count bigint NOT NULL DEFAULT 0,
  latest_published_at timestamptz,
  CONSTRAINT source_article_statistics_count_nonnegative CHECK (published_article_count >= 0)
);

CREATE INDEX articles_published_by_source
  ON articles (source_id, published_at DESC)
  WHERE public_status = 'published';

INSERT INTO source_article_statistics (source_id, published_article_count, latest_published_at)
SELECT source_id, count(*), max(published_at)
FROM articles
WHERE public_status = 'published'
GROUP BY source_id;

CREATE FUNCTION add_published_article_stat(article_source_id uuid, article_published_at timestamptz)
RETURNS void
LANGUAGE sql
AS $$
  INSERT INTO source_article_statistics (source_id, published_article_count, latest_published_at)
  VALUES (article_source_id, 1, article_published_at)
  ON CONFLICT (source_id) DO UPDATE
  SET published_article_count = source_article_statistics.published_article_count + 1,
      latest_published_at = GREATEST(
        source_article_statistics.latest_published_at,
        EXCLUDED.latest_published_at
      );
$$;

CREATE FUNCTION remove_published_article_stat(article_source_id uuid, article_published_at timestamptz)
RETURNS void
LANGUAGE sql
AS $$
  UPDATE source_article_statistics AS statistics
  SET published_article_count = GREATEST(statistics.published_article_count - 1, 0),
      latest_published_at = CASE
        WHEN statistics.latest_published_at <= article_published_at THEN (
          SELECT max(article.published_at)
          FROM articles AS article
          WHERE article.source_id = article_source_id
            AND article.public_status = 'published'
        )
        ELSE statistics.latest_published_at
      END
  WHERE statistics.source_id = article_source_id;
$$;

CREATE FUNCTION maintain_source_article_statistics()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    IF NEW.public_status = 'published' THEN
      PERFORM add_published_article_stat(NEW.source_id, NEW.published_at);
    END IF;
    RETURN NEW;
  END IF;

  IF TG_OP = 'DELETE' THEN
    IF OLD.public_status = 'published' THEN
      PERFORM remove_published_article_stat(OLD.source_id, OLD.published_at);
    END IF;
    RETURN OLD;
  END IF;

  IF OLD.public_status = 'published' THEN
    PERFORM remove_published_article_stat(OLD.source_id, OLD.published_at);
  END IF;
  IF NEW.public_status = 'published' THEN
    PERFORM add_published_article_stat(NEW.source_id, NEW.published_at);
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER articles_maintain_source_article_statistics
AFTER INSERT OR DELETE OR UPDATE OF source_id, public_status, published_at ON articles
FOR EACH ROW EXECUTE FUNCTION maintain_source_article_statistics();

COMMIT;
