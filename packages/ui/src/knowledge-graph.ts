export type KnowledgeGraphLayoutInput = {
  categories: { slug: string; name_en: string; articles: number }[]
  events: {
    id: string
    title: string
    category: string
    last_update_at: string
    articles: { source_id: string; source: string }[]
  }[]
}

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

export function connectedGraphNodeIDs(edges: GraphEdge[], selectedID: string) {
  const connected = new Set([selectedID])
  for (const edge of edges) {
    if (edge.source === selectedID) connected.add(edge.target)
    if (edge.target === selectedID) connected.add(edge.source)
  }
  return connected
}

export function eventNodeSize(articleCount: number) {
  return articleCount <= 1 ? 10 : 18 + Math.min(articleCount, 8) * 2
}

export function eventNodeColor(articleCount: number) {
  return eventNodeColors[Math.min(Math.max(articleCount, 1), eventNodeColors.length) - 1]!
}

const eventNodeColors = ['#a3a3a3', '#bfdbfe', '#93c5fd', '#60a5fa', '#2563eb'] as const
const graphLabelSpacing = 48
const graphVerticalPadding = 46

export function layoutKnowledgeGraph(data: KnowledgeGraphLayoutInput, width = 1200, height = 560) {
  const nodes: GraphNode[] = []
  const edges: GraphEdge[] = []

  const sourceNames = new Map<string, string>()
  data.events.forEach((event) => {
    event.articles.forEach((article) => sourceNames.set(article.source_id, article.source))
  })
  const sources = [...sourceNames].sort((left, right) => left[1].localeCompare(right[1]))
  const labelRows = Math.max(data.categories.length, sources.length)
  const layoutHeight = Math.max(
    height,
    graphVerticalPadding * 2 + Math.max(0, labelRows - 1) * graphLabelSpacing,
  )

  if (!data.categories.length || !data.events.length) {
    return { nodes, edges, width, height: layoutHeight }
  }

  const categoryY = new Map<string, number>()
  data.categories.forEach((category, index) => {
    const y = distributedY(index, data.categories.length, layoutHeight)
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

  sources.forEach(([id, label], index) => {
    nodes.push({
      id: `source:${id}`,
      kind: 'source',
      label,
      x: width - 105,
      y: distributedY(index, sources.length, layoutHeight),
      radius: 5,
    })
  })

  const times = data.events.map((event) => Date.parse(event.last_update_at))
  const minimum = Math.min(...times)
  const maximum = Math.max(...times)
  data.events.forEach((event) => {
    const timestamp = Date.parse(event.last_update_at)
    const ratio = maximum === minimum ? hash(event.id) / 100 : (timestamp - minimum) / (maximum - minimum)
    const baseY = categoryY.get(event.category) ?? layoutHeight / 2
    const y = clamp(baseY + ((hash(event.id) % 7) - 3) * 8, 36, layoutHeight - 36)
    nodes.push({
      id: `event:${event.id}`,
      kind: 'event',
      label: event.title,
      x: 245 + ratio * (width - 490),
      y,
      radius: eventNodeSize(event.articles.length) / 2,
      eventId: event.id,
    })
    edges.push({ source: `category:${event.category}`, target: `event:${event.id}`, kind: 'category' })
    new Set(event.articles.map((article) => article.source_id)).forEach((sourceID) => {
      edges.push({ source: `event:${event.id}`, target: `source:${sourceID}`, kind: 'source' })
    })
  })
  separateEventNodes(nodes, width, layoutHeight)
  return { nodes, edges, width, height: layoutHeight }
}

function separateEventNodes(nodes: GraphNode[], width: number, height: number) {
  const events = nodes.filter((node) => node.kind === 'event')
  // ponytail: pairwise relaxation is sufficient for hundreds of nodes; use a spatial index if graphs reach thousands.
  for (let pass = 0; pass < 16; pass += 1) {
    let moved = false
    for (let left = 0; left < events.length; left += 1) {
      for (let right = left + 1; right < events.length; right += 1) {
        const first = events[left]!
        const second = events[right]!
        let dx = second.x - first.x
        let dy = second.y - first.y
        let distance = Math.hypot(dx, dy)
        const minimumDistance = first.radius + second.radius + 4
        if (distance >= minimumDistance) continue
        if (distance === 0) {
          const angle = (hash(`${first.id}:${second.id}`) / 101) * Math.PI * 2
          dx = Math.cos(angle)
          dy = Math.sin(angle)
          distance = 1
        }
        const shift = (minimumDistance - distance) / 2
        const unitX = dx / distance
        const unitY = dy / distance
        first.x = clamp(first.x - unitX * shift, 190 + first.radius, width - 190 - first.radius)
        first.y = clamp(first.y - unitY * shift, first.radius, height - first.radius)
        second.x = clamp(second.x + unitX * shift, 190 + second.radius, width - 190 - second.radius)
        second.y = clamp(second.y + unitY * shift, second.radius, height - second.radius)
        moved = true
      }
    }
    if (!moved) break
  }
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
