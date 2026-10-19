UPDATE social_posts SET caption = replace(caption, E'\n\nවාර්තා කළේ: ', E'\n\nමූලාශ්‍ර: ')
WHERE status IN ('pending_review', 'approved', 'failed', 'needs_attention') AND position(E'\n\nවාර්තා කළේ: ' IN caption) > 0;
