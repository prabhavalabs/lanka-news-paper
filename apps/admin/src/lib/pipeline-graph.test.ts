import assert from 'node:assert/strict'
import test from 'node:test'

import {
  PIPELINE_NODE_HEIGHT,
  buildPipelineEdges,
  layoutPipelineGraph,
} from './pipeline-graph.ts'

test('centers a sequential pipeline with equal-height nodes', () => {
  const keys = ['source', 'categorization', 'clustering', 'analysis']
  const steps = keys.slice(1).map((name) => ({ name, output: {} }))
  const edges = buildPipelineEdges(keys, steps)
  const layout = layoutPipelineGraph(keys, edges, 1200)

  assert.deepEqual(edges, [
    { from: 'source', to: 'categorization' },
    { from: 'categorization', to: 'clustering' },
    { from: 'clustering', to: 'analysis' },
  ])
  assert.equal(new Set(Object.values(layout.positions).map((point) => point.y)).size, 1)
  assert.equal(layout.height, PIPELINE_NODE_HEIGHT + 48)
  assert.ok(layout.positions.source!.x > 24)
})

test('lays explicit dependencies out as branches that can rejoin', () => {
  const keys = ['source', 'summary', 'category', 'stance', 'synthesis']
  const steps = [
    { name: 'summary', output: {} },
    { name: 'category', output: { depends_on: ['summary'] } },
    { name: 'stance', output: { depends_on: ['summary'] } },
    { name: 'synthesis', output: { depends_on: ['category', 'stance'] } },
  ]
  const edges = buildPipelineEdges(keys, steps)
  const layout = layoutPipelineGraph(keys, edges, 900)

  assert.deepEqual(edges.slice(-2), [
    { from: 'category', to: 'synthesis' },
    { from: 'stance', to: 'synthesis' },
  ])
  assert.equal(layout.positions.category!.x, layout.positions.stance!.x)
  assert.notEqual(layout.positions.category!.y, layout.positions.stance!.y)
  assert.ok(layout.positions.synthesis!.x > layout.positions.category!.x)
})
