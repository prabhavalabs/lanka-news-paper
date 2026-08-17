CREATE TABLE political_parties (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  short_name text NOT NULL,
  name_en text NOT NULL,
  name_si text NOT NULL,
  aliases text[] NOT NULL DEFAULT '{}',
  economic_position numeric NOT NULL,
  confidence numeric NOT NULL,
  rationale text NOT NULL,
  evidence_urls jsonb NOT NULL DEFAULT '[]'::jsonb,
  active boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT political_parties_position_valid CHECK (economic_position BETWEEN -1 AND 1),
  CONSTRAINT political_parties_confidence_valid CHECK (confidence BETWEEN 0 AND 1)
);

CREATE TABLE article_political_analysis (
  article_id uuid PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  model text NOT NULL,
  economic_frame numeric NOT NULL,
  confidence numeric NOT NULL,
  mentions jsonb NOT NULL DEFAULT '[]'::jsonb,
  analyzed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CONSTRAINT article_political_frame_valid CHECK (economic_frame BETWEEN -1 AND 1),
  CONSTRAINT article_political_confidence_valid CHECK (confidence BETWEEN 0 AND 1)
);

CREATE INDEX article_political_analysis_frame
  ON article_political_analysis (economic_frame)
  WHERE confidence >= 0.45;

INSERT INTO political_parties (
  slug, short_name, name_en, name_si, aliases, economic_position, confidence, rationale, evidence_urls
) VALUES
  (
    'fsp', 'FSP', 'Frontline Socialist Party', 'පෙරටුගාමී සමාජවාදී පක්ෂය',
    ARRAY['fsp', 'frontline socialist party', 'පෙරටුගාමී සමාජවාදී පක්ෂය', 'පෙසප'],
    -0.95, 0.82, 'Marxist party placed at the far-left end of the economic-policy axis.',
    '["https://elections.gov.lk/en/political_party/political_party_list_E.html","https://carnegieendowment.org/research/2025/08/sri-lanka-aragalaya-protest-movement-oust-wickremesinghe-rajapaksa"]'
  ),
  (
    'jvp', 'JVP', 'Janatha Vimukthi Peramuna', 'ජනතා විමුක්ති පෙරමුණ',
    ARRAY['jvp', 'janatha vimukthi peramuna', 'ජනතා විමුක්ති පෙරමුණ', 'ජවිපෙ'],
    -0.90, 0.92, 'Marxist-Leninist roots and an explicit socialist programme place the JVP on the far left.',
    '["https://www.jvpsrilanka.com/english/","https://eprints.lse.ac.uk/41306/"]'
  ),
  (
    'npp', 'NPP', 'National People''s Power', 'ජාතික ජන බලවේගය',
    ARRAY['npp', 'national people''s power', 'ජාතික ජන බලවේගය', 'ජාජබ', 'anura kumara dissanayake', 'anura dissanayake', 'අනුර කුමාර දිසානායක', 'අනුර දිසානායක'],
    -0.55, 0.78, 'A left coalition with equity, economic democracy, and social-protection commitments, moderated by support for a mixed and market-participating economy.',
    '["https://www.npp.lk/en","https://www.npp.lk/up/policies/en/npppolicystatement.pdf"]'
  ),
  (
    'slfp', 'SLFP', 'Sri Lanka Freedom Party', 'ශ්‍රී ලංකා නිදහස් පක්ෂය',
    ARRAY['slfp', 'sri lanka freedom party', 'ශ්‍රී ලංකා නිදහස් පක්ෂය', 'ශ්‍රීලනිප'],
    -0.25, 0.70, 'Historically state-oriented and social-democratic, while later governments also accepted liberal-market policies.',
    '["https://www.veriteresearch.org/publication/mapping-sri-lankas-political-parties/"]'
  ),
  (
    'slpp', 'SLPP', 'Sri Lanka Podujana Peramuna', 'ශ්‍රී ලංකා පොදුජන පෙරමුණ',
    ARRAY['slpp', 'sri lanka podujana peramuna', 'ශ්‍රී ලංකා පොදුජන පෙරමුණ', 'පොහොට්ටුව', 'mahinda rajapaksa', 'namal rajapaksa', 'මහින්ද රාජපක්ෂ', 'නාමල් රාජපක්ෂ'],
    0.05, 0.55, 'Economically mixed and populist; kept near the centre because the left-right economic evidence is weaker than its clearer nationalist positioning.',
    '["https://www.veriteresearch.org/publication/mapping-sri-lankas-political-parties/"]'
  ),
  (
    'sjb', 'SJB', 'Samagi Jana Balawegaya', 'සමගි ජන බලවේගය',
    ARRAY['sjb', 'samagi jana balawegaya', 'සමගි ජන බලවේගය', 'සජබ', 'sajith premadasa', 'සජිත් ප්‍රේමදාස'],
    0.15, 0.62, 'A social-market and welfare-oriented party with market-friendly economic policy, placed slightly right of centre on this economic axis.',
    '["https://www.ft.lk/top-story/SJB-unveils-economic-blueprint-V3-with-a-view-for-Presidency/26-766363"]'
  ),
  (
    'unp', 'UNP', 'United National Party', 'එක්සත් ජාතික පක්ෂය',
    ARRAY['unp', 'united national party', 'එක්සත් ජාතික පක්ෂය', 'එජාප', 'ranil wickremesinghe', 'රනිල් වික්‍රමසිංහ'],
    0.55, 0.82, 'Historically the strongest market-liberal and pro-private-enterprise major party in Sri Lanka.',
    '["https://www.veriteresearch.org/publication/mapping-sri-lankas-political-parties/"]'
  );
