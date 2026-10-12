BEGIN;
-- Groups one incident reported in different words or figures by several outlets into one event.
INSERT INTO llm_task_profiles (task, provider_id, model, reasoning_effort, timeout_seconds, enabled)
VALUES ('event_matching', 'codex_cli', 'gpt-6-luna', 'medium', 180, true)
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflows (task, name, purpose, category, personality, tone, response_language, audience)
VALUES ('event_matching', 'Event matching', 'Decides whether a new report covers the same news event as an earlier one, so every outlet''s coverage is grouped and synthesised together.',
        'Editorial pipeline', 'Careful news desk editor who groups reports of one incident and keeps separate incidents apart.',
        'precise', 'en', 'newsroom editors')
ON CONFLICT (task) DO NOTHING;
INSERT INTO agent_workflow_versions (task, revision, snapshot)
SELECT task, revision, jsonb_build_object(
  'custom_instructions', custom_instructions, 'personality', personality,
  'learning_notes', learning_notes, 'tone', tone, 'response_language', response_language,
  'audience', audience, 'enabled', enabled
) FROM agent_workflows WHERE task = 'event_matching'
ON CONFLICT DO NOTHING;
COMMIT;
