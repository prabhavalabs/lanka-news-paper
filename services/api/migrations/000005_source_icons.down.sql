BEGIN;

ALTER TABLE sources DROP CONSTRAINT sources_icon_url_https;
ALTER TABLE sources DROP COLUMN icon_url;

COMMIT;
