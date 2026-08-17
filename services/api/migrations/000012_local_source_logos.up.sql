BEGIN;

ALTER TABLE sources
  DROP CONSTRAINT sources_icon_url_https,
  ADD CONSTRAINT sources_icon_url_allowed CHECK (
    icon_url IS NULL
    OR icon_url LIKE 'https://%'
    OR icon_url LIKE '/source-logos/%'
  );

UPDATE sources
SET icon_url = CASE website
  WHEN 'https://sinhala.adaderana.lk' THEN '/source-logos/ada-derana.png'
  WHEN 'https://aithiya.lk' THEN '/source-logos/aithiya.jpg'
  WHEN 'https://www.anidda.lk' THEN '/source-logos/anidda.jpg'
  WHEN 'https://www.bbc.com/sinhala' THEN '/source-logos/bbc-news-sinhala.svg'
  WHEN 'https://dasathalankanews.com' THEN '/source-logos/dasatha-lanka-news.webp'
  WHEN 'https://www.divaina.lk' THEN '/source-logos/divaina.png'
  WHEN 'https://www.itnnews.lk' THEN '/source-logos/itn-news.png'
  WHEN 'https://www.infosrilanka.lk' THEN '/source-logos/info-sri-lanka.webp'
  WHEN 'https://sinhala.lankanewsweb.net' THEN '/source-logos/lnw-sinhala.png'
  WHEN 'https://lankasara.com/si' THEN '/source-logos/lankasara.png'
  WHEN 'https://www.lankadeepa.lk' THEN '/source-logos/lankadeepa.jpg'
  WHEN 'https://medialk.com' THEN '/source-logos/medialk.png'
  WHEN 'https://www.meepura.com' THEN '/source-logos/meepura-news.gif'
  WHEN 'https://www.news19.lk' THEN '/source-logos/news-19.png'
  WHEN 'https://sinhala.newsfirst.lk' THEN '/source-logos/newsfirst-sinhala.webp'
  WHEN 'https://praja.lk' THEN '/source-logos/praja.jpeg'
  WHEN 'https://siyathanews.lk' THEN '/source-logos/siyatha-news.png'
  WHEN 'https://sinhala.srilankamirror.com' THEN '/source-logos/sri-lanka-mirror.png'
  WHEN 'https://www.vikalpa.org' THEN '/source-logos/vikalpa.svg'
  WHEN 'https://www.yukthiya.lk' THEN '/source-logos/yukthiya.png'
END
WHERE website IN (
  'https://sinhala.adaderana.lk',
  'https://aithiya.lk',
  'https://www.anidda.lk',
  'https://www.bbc.com/sinhala',
  'https://dasathalankanews.com',
  'https://www.divaina.lk',
  'https://www.itnnews.lk',
  'https://www.infosrilanka.lk',
  'https://sinhala.lankanewsweb.net',
  'https://lankasara.com/si',
  'https://www.lankadeepa.lk',
  'https://medialk.com',
  'https://www.meepura.com',
  'https://www.news19.lk',
  'https://sinhala.newsfirst.lk',
  'https://praja.lk',
  'https://siyathanews.lk',
  'https://sinhala.srilankamirror.com',
  'https://www.vikalpa.org',
  'https://www.yukthiya.lk'
);

COMMIT;
