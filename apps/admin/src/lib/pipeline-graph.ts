export const PIPELINE_NODE_WIDTH = 180
export const PIPELINE_NODE_HEIGHT = 144

const HORIZONTAL_GAP = 30
const VERTICAL_GAP = 32
const CANVAS_PADDING = 24

export type PipelineGraphEdge = {
  from: string
  to: string
}

export type PipelineGraphPoint = {
  x: number
  y: number
}

export type PipelineGraphLayout = {
  width: number
  height: number
  positions: Record<string, PipelineGraphPoint>
}

type DependencyStep = {
  name: string
  output: Record<string, unknown>
}

function dependencyNames(step: DependencyStep) {
  const value = step.output.depends_on ?? step.output.dependencies
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
    .map((item) => item === 'source_intake' ? 'source' : item)
}

export function buildPipelineEdges(nodeKeys: string[], steps: DependencyStep[]): PipelineGraphEdge[] {
  const available = new Set(nodeKeys)
  const stepByName = new Map(steps.map((step) => [step.name, step]))
  const edges: PipelineGraphEdge[] = []

  nodeKeys.slice(1).forEach((key, index) => {
    const explicit = dependencyNames(stepByName.get(key) ?? { name: key, output: {} })
      .filter((dependency) => available.has(dependency) && dependency !== key)
    const parents = explicit.length ? explicit : [nodeKeys[index]!]
    parents.forEach((parent) => edges.push({ from: parent, to: key }))
  })

  return edges.filter((edge, index) => edges.findIndex((candidate) => candidate.from === edge.from && candidate.to === edge.to) === index)
}

export function layoutPipelineGraph(nodeKeys: string[], edges: PipelineGraphEdge[], viewportWidth: number): PipelineGraphLayout {
  if (!nodeKeys.length) return { width: Math.max(viewportWidth, 320), height: 0, positions: {} }

  const order = new Map(nodeKeys.map((key, index) => [key, index]))
  const incoming = new Map(nodeKeys.map((key) => [key, 0]))
  const outgoing = new Map(nodeKeys.map((key) => [key, [] as string[]]))
  const levels = new Map(nodeKeys.map((key) => [key, 0]))

  edges.forEach(({ from, to }) => {
    if (!incoming.has(from) || !incoming.has(to)) return
    incoming.set(to, (incoming.get(to) ?? 0) + 1)
    outgoing.get(from)?.push(to)
  })

  const queue = nodeKeys.filter((key) => incoming.get(key) === 0)
  const visited = new Set<string>()
  while (queue.length) {
    const key = queue.shift()!
    visited.add(key)
    for (const target of outgoing.get(key) ?? []) {
      levels.set(target, Math.max(levels.get(target) ?? 0, (levels.get(key) ?? 0) + 1))
      incoming.set(target, (incoming.get(target) ?? 1) - 1)
      if (incoming.get(target) === 0) queue.push(target)
    }
  }

  // Keep malformed or cyclic future metadata usable by falling back to recorded order.
  nodeKeys.filter((key) => !visited.has(key)).forEach((key, index) => levels.set(key, index))

  const columns = new Map<number, string[]>()
  nodeKeys.forEach((key) => {
    const level = levels.get(key) ?? 0
    columns.set(level, [...(columns.get(level) ?? []), key])
  })
  const orderedColumns = [...columns.entries()].sort(([left], [right]) => left - right)
  orderedColumns.forEach(([, keys]) => keys.sort((left, right) => (order.get(left) ?? 0) - (order.get(right) ?? 0)))

  const columnCount = orderedColumns.length
  const rowCount = Math.max(...orderedColumns.map(([, keys]) => keys.length))
  const contentWidth = columnCount * PIPELINE_NODE_WIDTH + Math.max(0, columnCount - 1) * HORIZONTAL_GAP
  const contentHeight = rowCount * PIPELINE_NODE_HEIGHT + Math.max(0, rowCount - 1) * VERTICAL_GAP
  const width = Math.max(viewportWidth, contentWidth + CANVAS_PADDING * 2, 320)
  const height = contentHeight + CANVAS_PADDING * 2
  const startX = (width - contentWidth) / 2
  const positions: Record<string, PipelineGraphPoint> = {}

  orderedColumns.forEach(([, keys], columnIndex) => {
    const columnHeight = keys.length * PIPELINE_NODE_HEIGHT + Math.max(0, keys.length - 1) * VERTICAL_GAP
    const startY = (height - columnHeight) / 2
    keys.forEach((key, rowIndex) => {
      positions[key] = {
        x: startX + columnIndex * (PIPELINE_NODE_WIDTH + HORIZONTAL_GAP),
        y: startY + rowIndex * (PIPELINE_NODE_HEIGHT + VERTICAL_GAP),
      }
    })
  })

  return { width, height, positions }
}
