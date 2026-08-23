import assert from 'node:assert/strict'
import test from 'node:test'

import type { KnowledgeGraph } from '@snap/api-client'

import {
  connectedGraphNodeIDs,
  eventNodeColor,
  eventNodeSize,
  layoutKnowledgeGraph,
} from './knowledge-graph.ts'

test('makes a single-report event distinctly smaller than shared events', () => {
  assert.equal(eventNodeSize(1), 10)
  assert.equal(eventNodeSize(2), 22)
  assert.equal(eventNodeSize(5), 28)
  assert.equal(eventNodeSize(20), 34)
  assert.equal(eventNodeColor(1), '#a3a3a3')
  assert.equal(eventNodeColor(2), '#bfdbfe')
  assert.equal(eventNodeColor(5), '#2563eb')
  assert.equal(eventNodeColor(20), '#2563eb')
})

test('separates event dots in the initial layout', () => {
  const articles = Array.from({ length: 5 }, (_, index) => ({ source_id: `source-${index}`, source: `Source ${index}` }))
  const graph = layoutKnowledgeGraph({
    categories: [{ slug: 'politics', name_en: 'Politics', articles: 100 }],
    events: Array.from({ length: 20 }, (_, index) => ({
      id: `event-${index}`,
      title: `Event ${index}`,
      category: 'politics',
      last_update_at: `2026-08-21T12:00:${String(index).padStart(2, '0')}Z`,
      articles,
    })),
  })
  const events = graph.nodes.filter((node) => node.kind === 'event')
  for (let left = 0; left < events.length; left += 1) {
    for (let right = left + 1; right < events.length; right += 1) {
      const first = events[left]!
      const second = events[right]!
      assert.ok(Math.hypot(first.x - second.x, first.y - second.y) >= first.radius + second.radius)
    }
  }
})

test('lays out category, event, and source relationships', () => {
  const data: KnowledgeGraph = {
    generated_at: '2026-08-17T12:00:00Z',
    days: 1,
    summary: { articles: 2, events: 1, multi_source_events: 1, sources: 2 },
    political: { axis: 'Economic policy', model: 'rules-v1', minimum_sample: 5, parties: [], sources: [] },
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

test('keeps only a selected node and its direct relationships active', () => {
  assert.deepEqual(
    [...connectedGraphNodeIDs([
      { source: 'category:politics', target: 'event:1', kind: 'category' },
      { source: 'event:1', target: 'source:one', kind: 'source' },
      { source: 'event:2', target: 'source:two', kind: 'source' },
    ], 'event:1')],
    ['event:1', 'category:politics', 'source:one'],
  )
})
