-- Facebook's website splits joined Sinhala letters (්‍ර) apart, so the caption's source credit
-- line no longer uses මූලාශ්‍ර. Posts not yet on the Page take the new label.
UPDATE social_posts SET caption = replace(caption, E'\n\nමූලාශ්‍ර: ', E'\n\nවාර්තා කළේ: ')
WHERE status IN ('pending_review', 'approved', 'failed', 'needs_attention') AND position(E'\n\nමූලාශ්‍ර: ' IN caption) > 0;
