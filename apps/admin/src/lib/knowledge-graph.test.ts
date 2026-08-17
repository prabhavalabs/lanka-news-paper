import assert from 'node:assert/strict'
import test from 'node:test'

import type { KnowledgeGraph } from '@snap/api-client'

import { layoutKnowledgeGraph } from './knowledge-graph.ts'

test('lays out category, event, and source relationships', () => {
  const data: KnowledgeGraph = {
    generated_at: '2026-08-17T12:00:00Z',
    days: 1,
    summary: { articles: 2, events: 1, multi_source_events: 1, sources: 2 },
    categories: [{ slug: 'politics', name_si: 'දේශපාලන', name_en: 'Politics', articles: 2, events: 1 }],
    events: [{
      id: 'event-1', title: 'Shared event', category: 'politics', category_name_si: 'දේශපාලන',
      confidence: 0.8, is_breaking: false, locked: false, algorithm_version: 'v2',
      first_seen_at: '2026-08-17T11:00:00Z', last_update_at: '2026-08-17T12:00:00Z',
      articles: [
        { id: 'a', headline: 'A', source_id: 's1', source: 'One', source_icon: '', original_url: 'https://one.test', published_at: '2026-08-17T11:00:00Z' },
        { id: 'b', headline: 'B', source_id: 's2', source: 'Two', source_icon: '', original_url: 'https://two.test', published_at: '2026-08-17T12:00:00Z' },
      ],
    }],
  }

  const graph = layoutKnowledgeGraph(data)
  assert.equal(graph.nodes.filter((node) => node.kind === 'category').length, 1)
  assert.equal(graph.nodes.filter((node) => node.kind === 'event').length, 1)
  assert.equal(graph.nodes.filter((node) => node.kind === 'source').length, 2)
  assert.equal(graph.edges.length, 3)
})
