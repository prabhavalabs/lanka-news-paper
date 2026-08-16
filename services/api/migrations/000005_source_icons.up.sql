BEGIN;

ALTER TABLE sources
  ADD COLUMN icon_url text,
  ADD CONSTRAINT sources_icon_url_https CHECK (icon_url IS NULL OR icon_url LIKE 'https://%');

UPDATE sources
SET icon_url = CASE website
  WHEN 'https://www.bbc.com/sinhala' THEN 'https://www.bbc.com/favicon.ico'
  WHEN 'https://www.itnnews.lk' THEN 'https://www.itnnews.lk/wp-content/uploads/2024/03/cropped-Favicon-192x192.png'
  WHEN 'https://www.lankadeepa.lk' THEN 'https://www.lankadeepa.lk/asserts_new/images/icons/favicon.png'
  WHEN 'https://sinhala.adaderana.lk' THEN 'https://sinhala.adaderana.lk/favicon.ico'
  WHEN 'https://www.divaina.lk' THEN 'https://www.divaina.lk/wp-content/uploads/2025/05/cropped-divaina-favicon-192x192.png'
  WHEN 'https://www.vikalpa.org' THEN 'https://www.vikalpa.org/favicon.ico'
  WHEN 'https://siyathanews.lk' THEN 'https://siyathanews.lk/wp-content/uploads/2019/10/cropped-siyathanewslogo512512-192x192.png'
  WHEN 'https://sinhala.srilankamirror.com' THEN 'https://sinhala.srilankamirror.com/wp-content/uploads/2022/10/cropped-favicon-192x192.png'
  WHEN 'https://www.anidda.lk' THEN 'https://www.anidda.lk/wp-content/uploads/2026/07/cropped-Untitled-1-192x192.jpg'
  WHEN 'https://dasathalankanews.com' THEN 'https://dasathalankanews.com/wp-content/uploads/favicon.webp'
  WHEN 'https://lankasara.com/si' THEN 'https://lankasara.com/wp-content/uploads/2020/05/cropped-favicon-1-192x192.png'
END
WHERE website IN (
  'https://www.bbc.com/sinhala',
  'https://www.itnnews.lk',
  'https://www.lankadeepa.lk',
  'https://sinhala.adaderana.lk',
  'https://www.divaina.lk',
  'https://www.vikalpa.org',
  'https://siyathanews.lk',
  'https://sinhala.srilankamirror.com',
  'https://www.anidda.lk',
  'https://dasathalankanews.com',
  'https://lankasara.com/si'
);

COMMIT;
