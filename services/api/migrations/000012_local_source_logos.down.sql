BEGIN;

ALTER TABLE sources DROP CONSTRAINT sources_icon_url_allowed;

UPDATE sources
SET icon_url = CASE website
  WHEN 'https://sinhala.adaderana.lk' THEN 'https://sinhala.adaderana.lk/favicon.ico'
  WHEN 'https://aithiya.lk' THEN 'https://aithiya.lk/wp-content/uploads/2020/03/cropped-512-192x192.jpg'
  WHEN 'https://www.anidda.lk' THEN 'https://www.anidda.lk/wp-content/uploads/2026/07/cropped-Untitled-1-192x192.jpg'
  WHEN 'https://www.bbc.com/sinhala' THEN 'https://www.bbc.com/favicon.ico'
  WHEN 'https://dasathalankanews.com' THEN 'https://dasathalankanews.com/wp-content/uploads/favicon.webp'
  WHEN 'https://www.divaina.lk' THEN 'https://www.divaina.lk/wp-content/uploads/2025/05/cropped-divaina-favicon-192x192.png'
  WHEN 'https://www.itnnews.lk' THEN 'https://www.itnnews.lk/wp-content/uploads/2024/03/cropped-Favicon-192x192.png'
  WHEN 'https://www.infosrilanka.lk' THEN 'https://www.infosrilanka.lk/wp-content/uploads/2025/03/cropped-infosrilanka_favicon-192x192.webp'
  WHEN 'https://sinhala.lankanewsweb.net' THEN NULL
  WHEN 'https://lankasara.com/si' THEN 'https://lankasara.com/wp-content/uploads/2020/05/cropped-favicon-1-192x192.png'
  WHEN 'https://www.lankadeepa.lk' THEN 'https://www.lankadeepa.lk/asserts_new/images/icons/favicon.png'
  WHEN 'https://medialk.com' THEN 'https://medialk.com/wp-content/uploads/2026/05/cropped-MediaLK.com-Logo-Portrait-2-1-192x192.png'
  WHEN 'https://www.meepura.com' THEN 'https://www.meepura.com/favicon.ico'
  WHEN 'https://www.news19.lk' THEN 'https://www.news19.lk/wp-content/uploads/2020/11/Attachment-01-300x300.png'
  WHEN 'https://sinhala.newsfirst.lk' THEN 'https://cdn.newsfirst.lk/assets/favicon.png'
  WHEN 'https://praja.lk' THEN 'https://praja.lk/wp-content/uploads/2025/03/cropped-logo-1.jpeg'
  WHEN 'https://siyathanews.lk' THEN 'https://siyathanews.lk/wp-content/uploads/2019/10/cropped-siyathanewslogo512512-192x192.png'
  WHEN 'https://sinhala.srilankamirror.com' THEN 'https://sinhala.srilankamirror.com/wp-content/uploads/2022/10/cropped-favicon-192x192.png'
  WHEN 'https://www.vikalpa.org' THEN 'https://www.vikalpa.org/favicon.ico'
  WHEN 'https://www.yukthiya.lk' THEN 'https://www.yukthiya.lk/wp-content/uploads/2024/04/yicon1.png'
END
WHERE icon_url LIKE '/source-logos/%';

ALTER TABLE sources
  ADD CONSTRAINT sources_icon_url_https CHECK (icon_url IS NULL OR icon_url LIKE 'https://%');

COMMIT;
