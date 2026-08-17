import type { KnowledgeGraph } from '@snap/api-client'

export type GraphNode = {
  id: string
  kind: 'category' | 'event' | 'source'
  label: string
  x: number
  y: number
  radius: number
  eventId?: string
}

export type GraphEdge = {
  source: string
  target: string
  kind: 'category' | 'source'
}

export function layoutKnowledgeGraph(data: KnowledgeGraph, width = 1200, height = 560) {
  const nodes: GraphNode[] = []
  const edges: GraphEdge[] = []
  if (!data.categories.length || !data.events.length) return { nodes, edges }

  const categoryY = new Map<string, number>()
  data.categories.forEach((category, index) => {
    const y = distributedY(index, data.categories.length, height)
    categoryY.set(category.slug, y)
    nodes.push({
      id: `category:${category.slug}`,
      kind: 'category',
      label: category.name_en,
      x: 100,
      y,
      radius: 8 + Math.min(category.articles, 80) / 12,
    })
  })

  const sourceNames = new Map<string, string>()
  data.events.forEach((event) => {
    event.articles.forEach((article) => sourceNames.set(article.source_id, article.source))
  })
  const sources = [...sourceNames].sort((left, right) => left[1].localeCompare(right[1]))
  sources.forEach(([id, label], index) => {
    nodes.push({
      id: `source:${id}`,
      kind: 'source',
      label,
      x: width - 105,
      y: distributedY(index, sources.length, height),
      radius: 5,
    })
  })

  const times = data.events.map((event) => Date.parse(event.last_update_at))
  const minimum = Math.min(...times)
  const maximum = Math.max(...times)
  data.events.forEach((event) => {
    const timestamp = Date.parse(event.last_update_at)
    const ratio = maximum === minimum ? hash(event.id) / 100 : (timestamp - minimum) / (maximum - minimum)
    const baseY = categoryY.get(event.category) ?? height / 2
    const y = clamp(baseY + ((hash(event.id) % 7) - 3) * 8, 36, height - 36)
    nodes.push({
      id: `event:${event.id}`,
      kind: 'event',
      label: event.title,
      x: 245 + ratio * (width - 490),
      y,
      radius: 5 + Math.min(event.articles.length, 8) * 1.4,
      eventId: event.id,
    })
    edges.push({ source: `category:${event.category}`, target: `event:${event.id}`, kind: 'category' })
    new Set(event.articles.map((article) => article.source_id)).forEach((sourceId) => {
      edges.push({ source: `event:${event.id}`, target: `source:${sourceId}`, kind: 'source' })
    })
  })
  return { nodes, edges }
}

function distributedY(index: number, count: number, height: number) {
  return count === 1 ? height / 2 : 46 + (index * (height - 92)) / (count - 1)
}

function hash(value: string) {
  let result = 0
  for (let index = 0; index < value.length; index += 1) result = (result * 31 + value.charCodeAt(index)) % 101
  return result
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(maximum, Math.max(minimum, value))
}
