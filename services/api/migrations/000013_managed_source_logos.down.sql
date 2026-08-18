UPDATE sources
SET icon_url = NULL
WHERE icon_url LIKE '/api/admin/media/source-logos/%';

ALTER TABLE sources
  DROP CONSTRAINT sources_icon_url_allowed,
  ADD CONSTRAINT sources_icon_url_allowed CHECK (
    icon_url IS NULL
    OR icon_url LIKE 'https://%'
    OR icon_url LIKE '/source-logos/%'
  );
