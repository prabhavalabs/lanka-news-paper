import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MiniMap,
  NodeToolbar,
  Panel,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useNodesState,
  useReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from '@xyflow/react'
import { useEffect, useMemo } from 'react'

import '@xyflow/react/dist/style.css'

import {
  connectedGraphNodeIDs,
  eventNodeColor,
  eventNodeSize,
  layoutKnowledgeGraph,
  type GraphNode,
  type KnowledgeGraphLayoutInput,
} from './knowledge-graph'

type GraphVariant = 'admin' | 'public'

type KnowledgeNodeData = {
  kind: GraphNode['kind']
  label: string
  articleCount: number
  dimmed: boolean
  variant: GraphVariant
}

type KnowledgeNode = Node<KnowledgeNodeData, 'knowledge'>

export function KnowledgeGraphView({
  data,
  selectedID,
  onSelect,
  onReset,
  variant = 'admin',
  className = 'h-[560px]',
}: {
  data: KnowledgeGraphLayoutInput
  selectedID: string
  onSelect: (id: string) => void
  onReset: () => void
  variant?: GraphVariant
  className?: string
}) {
  const graph = useMemo(() => layoutKnowledgeGraph(data), [data])
  const graphKey = useMemo(() => graph.nodes.map((node) => node.id).join('|'), [graph.nodes])

  return (
    <div
      className={`relative overflow-hidden ${variant === 'admin' ? 'bg-background' : 'bg-tint'} ${className}`}
      role="application"
      aria-label="Interactive knowledge graph. Drag nodes or the canvas, scroll to zoom, and use Tab then Enter to select a node."
    >
      <ReactFlowProvider>
        <KnowledgeGraphCanvas
          key={graphKey}
          data={data}
          graph={graph}
          selectedID={selectedID}
          onSelect={onSelect}
          onReset={onReset}
          variant={variant}
        />
      </ReactFlowProvider>
    </div>
  )
}

function KnowledgeGraphCanvas({
  data,
  graph,
  selectedID,
  onSelect,
  onReset,
  variant,
}: {
  data: KnowledgeGraphLayoutInput
  graph: ReturnType<typeof layoutKnowledgeGraph>
  selectedID: string
  onSelect: (id: string) => void
  onReset: () => void
  variant: GraphVariant
}) {
  const eventCounts = useMemo(
    () => new Map(data.events.map((event) => [`event:${event.id}`, event.articles.length])),
    [data.events],
  )
  const initialNodes = useMemo(
    () => graph.nodes.map((node) => toFlowNode(node, eventCounts.get(node.id), variant)),
    [eventCounts, graph.nodes, variant],
  )
  const [nodes, , onNodesChange] = useNodesState<KnowledgeNode>(initialNodes)
  const active = useMemo(
    () => selectedID ? connectedGraphNodeIDs(graph.edges, selectedID) : new Set<string>(),
    [graph.edges, selectedID],
  )
  const displayNodes = useMemo(() => nodes.map((node) => ({
    ...node,
    selected: node.id === selectedID,
    data: { ...node.data, dimmed: active.size > 0 && !active.has(node.id) },
  })), [active, nodes, selectedID])
  const edges = useMemo<Edge[]>(() => graph.edges.map((edge, index) => {
    const highlighted = edge.source === selectedID || edge.target === selectedID
    return {
      id: `${edge.source}-${edge.target}-${index}`,
      source: edge.source,
      target: edge.target,
      selectable: false,
      focusable: false,
      domAttributes: { 'aria-hidden': true },
      animated: highlighted,
      style: {
        stroke: variant === 'admin'
          ? edge.kind === 'category' ? 'var(--primary)' : 'var(--muted-foreground)'
          : edge.kind === 'category' ? 'var(--ink)' : 'var(--ink-tertiary)',
        strokeWidth: highlighted ? 2.25 : edge.kind === 'category' ? 1.4 : 1,
        opacity: highlighted ? 0.9 : active.size ? 0.04 : edge.kind === 'category' ? 0.18 : 0.08,
      },
    }
  }), [active.size, graph.edges, selectedID, variant])

  return (
    <ReactFlow<KnowledgeNode>
      nodes={displayNodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={onNodesChange}
      onNodeClick={(_, node) => onSelect(node.id)}
      fitView
      fitViewOptions={{ padding: 0.12, maxZoom: 1.1 }}
      minZoom={0.08}
      maxZoom={2.5}
      nodesConnectable={false}
      nodesDraggable
      nodesFocusable
      edgesFocusable={false}
      deleteKeyCode={null}
      panOnDrag
      zoomOnPinch
      zoomOnScroll
      autoPanOnNodeDrag
      onlyRenderVisibleElements
      proOptions={{ hideAttribution: true }}
      colorMode={variant === 'admin' ? 'system' : 'light'}
    >
      <ViewportFocus selectedID={selectedID} active={active} />
      <Background
        variant={BackgroundVariant.Dots}
        gap={22}
        size={1}
        color={variant === 'admin' ? 'var(--border)' : 'var(--rule)'}
      />
      <Controls
        position="top-right"
        showInteractive={false}
        className={variant === 'admin'
          ? 'overflow-hidden! rounded-xl! border! border-border! bg-background/95! shadow-sm!'
          : 'overflow-hidden! rounded-none! border! border-rule! bg-paper! shadow-none!'}
      />
      <MiniMap
        position="bottom-right"
        pannable
        zoomable
        ariaLabel="Knowledge graph overview and navigation map"
        nodeColor={(node) => miniMapColor(node as KnowledgeNode, variant)}
        maskColor={variant === 'admin' ? 'color-mix(in srgb, var(--background) 72%, transparent)' : 'rgba(245, 241, 232, 0.72)'}
        className={variant === 'admin'
          ? 'hidden rounded-xl! border! border-border! bg-background/95! shadow-sm! md:block'
          : 'hidden rounded-none! border! border-rule! bg-paper! shadow-none! md:block'}
      />
      {selectedID ? (
        <Panel position="top-left">
          <button
            type="button"
            onClick={onReset}
            className={variant === 'admin'
              ? 'rounded-xl border border-border bg-background/95 px-3 py-2 text-xs font-medium shadow-sm hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring'
              : 'border border-rule bg-paper px-3 py-2 text-xs font-bold hover:bg-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink'}
          >
            Reset complete view
          </button>
        </Panel>
      ) : null}
      <Panel position="bottom-left" className="pointer-events-none hidden md:block">
        <p className={variant === 'admin'
          ? 'rounded-xl border border-border bg-background/90 px-3 py-2 text-xs text-muted-foreground shadow-sm backdrop-blur'
          : 'border border-rule bg-paper px-3 py-2 text-xs text-muted-foreground'}>
          Drag nodes to rearrange · Drag canvas to move · Scroll to zoom
        </p>
      </Panel>
    </ReactFlow>
  )
}

function ViewportFocus({ selectedID, active }: { selectedID: string; active: Set<string> }) {
  const { fitView } = useReactFlow<KnowledgeNode>()
  useEffect(() => {
    const compact = window.matchMedia('(max-width: 767px)').matches
    void fitView({
      nodes: selectedID
        ? (compact ? [{ id: selectedID }] : [...active].map((id) => ({ id })))
        : undefined,
      padding: selectedID ? 0.45 : 0.12,
      maxZoom: selectedID ? 1.4 : 1.1,
      duration: 450,
    })
  }, [active, fitView, selectedID])
  return null
}

function KnowledgeNodeView({ data, selected }: NodeProps<KnowledgeNode>) {
  const publicView = data.variant === 'public'
  const opacity = data.dimmed ? 0.1 : 1

  if (data.kind === 'event') {
    const size = eventNodeSize(data.articleCount)
    return (
      <div className="relative" style={{ opacity }} title={data.label}>
        <Handle type="target" position={Position.Left} style={hiddenHandleStyle} />
        <div
          className="rounded-full border border-solid border-black"
          style={{ width: size, height: size, backgroundColor: eventNodeColor(data.articleCount) }}
        />
        <NodeToolbar isVisible={selected} position={Position.Bottom} offset={8}>
          <span className={`block max-w-64 px-2 py-1 text-xs font-medium shadow-lg ${publicView ? 'border border-rule bg-paper text-ink' : 'rounded-md border border-border bg-popover text-popover-foreground'}`}>
            {truncate(data.label, 58)} · {data.articleCount} report{data.articleCount === 1 ? '' : 's'}
          </span>
        </NodeToolbar>
        <Handle type="source" position={Position.Right} style={hiddenHandleStyle} />
      </div>
    )
  }

  const isCategory = data.kind === 'category'
  return (
    <div
      className={publicView
        ? `flex h-9 w-40 items-center gap-2 border bg-paper px-3 text-xs font-bold text-ink transition-opacity ${selected ? 'border-ink ring-2 ring-ink/20' : 'border-rule'}`
        : `flex h-9 w-40 items-center gap-2 rounded-xl border bg-background/95 px-3 text-xs font-medium shadow-sm transition-opacity ${selected ? 'border-primary ring-2 ring-primary/20' : 'border-border'}`}
      style={{ opacity }}
      title={data.label}
    >
      {isCategory ? null : <Handle type="target" position={Position.Left} style={hiddenHandleStyle} />}
      <span className={isCategory
        ? `size-2.5 shrink-0 rounded-full ${publicView ? 'bg-ink' : 'bg-primary'}`
        : `size-2.5 shrink-0 rounded-full border ${publicView ? 'border-ink bg-paper' : 'border-muted-foreground bg-background'}`}
      />
      <span className="truncate">{data.label}</span>
      {isCategory ? <Handle type="source" position={Position.Right} style={hiddenHandleStyle} /> : null}
    </div>
  )
}

const nodeTypes = { knowledge: KnowledgeNodeView }
const hiddenHandleStyle = { visibility: 'hidden' as const, pointerEvents: 'none' as const }

function toFlowNode(
  node: GraphNode,
  eventCount: number | undefined,
  variant: GraphVariant,
): KnowledgeNode {
  const eventSize = eventNodeSize(eventCount ?? 0)
  const width = node.kind === 'event' ? eventSize : 160
  const height = node.kind === 'event' ? eventSize : 36
  return {
    id: node.id,
    type: 'knowledge',
    position: { x: node.x - width / 2, y: node.y - height / 2 },
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    ariaLabel: `${node.kind}: ${node.label}${eventCount ? `, ${eventCount} reports` : ''}`,
    data: {
      kind: node.kind,
      label: node.label,
      articleCount: eventCount ?? 0,
      dimmed: false,
      variant,
    },
  }
}

function miniMapColor(node: KnowledgeNode, variant: GraphVariant) {
  if (node.data.kind === 'event') return eventNodeColor(node.data.articleCount)
  if (variant === 'public') return '#f5f1e8'
  if (node.data.kind === 'category') return '#2563eb'
  return '#a3a3a3'
}

function truncate(value: string, length: number) {
  return value.length > length ? `${value.slice(0, length - 1)}…` : value
}
