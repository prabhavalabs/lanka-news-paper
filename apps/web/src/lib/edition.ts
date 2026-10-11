import type { EditionBlock, EditionBlockSize, FrontPage, FrontPageStory } from '@snap/api-client'

import { paragraphs } from './format'

/** What a block prints: the desk's report when written, otherwise the story summary. */
export type BlockCopy = {
  headline: string
  standfirst: string
  paragraphs: string[]
}

export function blockCopy(block: EditionBlock): BlockCopy {
  const { report, story } = block
  if (report && report.paragraphs.length) {
    return { headline: report.headline || story.headline, standfirst: report.standfirst, paragraphs: report.paragraphs }
  }
  return { headline: story.headline, standfirst: '', paragraphs: paragraphs(story.summary) }
}

/**
 * Until the AI edition exists, lay out the curated/automatic slots as blocks:
 * lead → lead, first two top → feature, other top → standard, more → short then brief.
 */
export function editionBlocks(front: FrontPage): EditionBlock[] {
  if (front.blocks?.length) return front.blocks
  const block = (story: FrontPageStory, size: EditionBlockSize): EditionBlock => ({ size, story, report: null, image: null })
  return [
    ...(front.lead ? [block(front.lead, 'lead')] : []),
    ...front.top.map((story, index) => block(story, index < 2 ? 'feature' : 'standard')),
    ...front.more.map((story, index) => block(story, index < 6 ? 'short' : 'brief')),
  ]
}

/* ---------- Desktop composition: lanes of different widths that interlock ---------- */

export type Lane = { span: number; items: LaneItem[] }
export type LaneItem = { type: 'block'; block: EditionBlock } | { type: 'editorial' }
export type Section = { lanes: Lane[] }

const PAGE_WIDTH = 1184 // content width at 1280; only ratios matter for balancing
const BODY_LINE = 28
const SINHALA_ADVANCE = 5.6 // average advance per code point at 16px, incl. combining signs

const headlineSize: Record<EditionBlockSize, number> = { lead: 46, feature: 34, standard: 26, short: 22, brief: 19 }

export function textColumns(span: number) {
  return span >= 8 ? 3 : span >= 5 ? 2 : 1
}

/** Rough printed height of a block in a lane of `span` columns, used only to balance lanes. */
export function estimateHeight(block: EditionBlock, span: number) {
  const width = (PAGE_WIDTH * span) / 12 - 32
  const copy = blockCopy(block)
  const chars = copy.paragraphs.reduce((sum, text) => sum + text.length, 0)
  const body = Math.ceil((chars * SINHALA_ADVANCE) / width) * BODY_LINE
  const size = headlineSize[block.size]
  const headline = Math.ceil((copy.headline.length * size * 0.42) / width) * size * 1.35
  const standfirst = copy.standfirst ? Math.ceil((copy.standfirst.length * 6.5) / width) * 30 + 12 : 0
  const image = block.image ? (width * block.image.height) / Math.max(1, block.image.width) + 28 : 0
  return headline + standfirst + body + image + 96
}

const EDITORIAL_HEIGHT = 330
const EDITORIAL_KEY = 'editorial'

/**
 * Printed area (height × width, px²) of each block as measured in the browser. Text area is
 * roughly constant whatever the lane width, so it predicts the height in any other lane.
 */
export type MeasuredAreas = Map<string, number>

export function blockKey(block: EditionBlock) {
  return `${block.story.kind}:${block.story.id}`
}

export function itemKey(item: LaneItem) {
  return item.type === 'editorial' ? EDITORIAL_KEY : blockKey(item.block)
}

function laneWidth(span: number) {
  return (PAGE_WIDTH * span) / 12 - 32
}

/**
 * Builds the broadsheet: an opening spread (lead + a narrow lane with the editorial),
 * then feature spreads whose wide/narrow lanes alternate sides (the "bend"),
 * then the remaining copy poured into three or four lanes, rebalanced so lanes end level.
 */
export function composeEdition(blocks: EditionBlock[], hasEditorial: boolean, areas?: MeasuredAreas): Section[] {
  const heightOf = (item: LaneItem, span: number) => {
    const area = areas?.get(itemKey(item))
    if (area) return area / laneWidth(span)
    return item.type === 'editorial' ? EDITORIAL_HEIGHT : estimateHeight(item.block, span)
  }
  const laneHeight = (lane: Lane) => lane.items.reduce((sum, item) => sum + heightOf(item, lane.span), 0)
  const shortest = (lanes: Lane[]) => lanes.reduce((low, next) => (laneHeight(next) < laneHeight(low) ? next : low))
  const spread = (lanes: Lane[]) => {
    const heights = lanes.map(laneHeight)
    return Math.max(...heights) - Math.min(...heights)
  }
  const take = (pool: EditionBlock[]): LaneItem => ({ type: 'block', block: pool.shift() as EditionBlock })

  /** Move or swap stories between lanes while that brings the lane feet closer together. */
  const level = (lanes: Lane[]) => {
    for (let round = 0; round < 40; round += 1) {
      let best = spread(lanes)
      let apply: (() => void) | null = null
      for (const from of lanes) {
        for (const to of lanes) {
          if (from === to) continue
          from.items.forEach((item, i) => {
            if (item.type === 'editorial' || from.items.length < 2) return
            from.items.splice(i, 1)
            to.items.push(item)
            const score = spread(lanes)
            to.items.pop()
            from.items.splice(i, 0, item)
            if (score < best - 1) {
              best = score
              apply = () => {
                from.items.splice(i, 1)
                to.items.push(item)
              }
            }
            to.items.forEach((other, j) => {
              if (other.type === 'editorial') return
              from.items[i] = other
              to.items[j] = item
              const swapped = spread(lanes)
              from.items[i] = item
              to.items[j] = other
              if (swapped < best - 1) {
                best = swapped
                apply = () => {
                  from.items[i] = other
                  to.items[j] = item
                }
              }
            })
          })
        }
      }
      if (!apply) break
      ;(apply as () => void)()
    }
  }

  const leadIndex = blocks.findIndex((block) => block.size === 'lead')
  const lead = leadIndex >= 0 ? blocks[leadIndex] : undefined
  const rest = blocks.filter((_, index) => index !== leadIndex)
  const features = rest.filter((block) => block.size === 'feature')
  const standards = rest.filter((block) => block.size === 'standard')
  const small = rest.filter((block) => block.size === 'short' || block.size === 'brief')
  const filler = () => (standards.length ? standards : small)
  const sections: Section[] = []

  if (lead) {
    const main: Lane = { span: 8, items: [{ type: 'block', block: lead }] }
    const side: Lane = { span: 4, items: hasEditorial ? [{ type: 'editorial' }] : [] }
    // Fill the narrow lane beside the lead with short stories until both feet meet.
    while (small.length) {
      const next: LaneItem = { type: 'block', block: small[0] as EditionBlock }
      if (laneHeight(side) + heightOf(next, 4) / 2 > laneHeight(main)) break
      side.items.push(take(small))
    }
    sections.push({ lanes: [main, side] })
  } else if (hasEditorial) {
    sections.push({ lanes: [{ span: 12, items: [{ type: 'editorial' }] }] })
  }

  for (let index = 0; index < features.length; index += 2) {
    const pair = features.slice(index, index + 2)
    const flip = (index / 2) % 2 === 1
    const spans = pair.length === 2 ? (flip ? [5, 7] : [7, 5]) : flip ? [4, 8] : [8, 4]
    const lanes: Lane[] = spans.map((span) => ({ span, items: [] }))
    if (pair.length === 2) {
      pair.forEach((block, i) => lanes[i]?.items.push({ type: 'block', block }))
    } else {
      const wide = lanes.find((lane) => lane.span === 8) as Lane
      wide.items.push({ type: 'block', block: pair[0] as EditionBlock })
    }
    // Even the spread out with further stories under whichever side is shorter.
    while (filler().length) {
      const low = shortest(lanes)
      const high = lanes.find((lane) => lane !== low) as Lane
      const next: LaneItem = { type: 'block', block: filler()[0] as EditionBlock }
      const gap = laneHeight(high) - laneHeight(low)
      if (gap < heightOf(next, low.span) / 2) break
      low.items.push(take(filler()))
    }
    sections.push({ lanes })
  }

  const remaining = [...standards, ...small]
  if (remaining.length) {
    const count = remaining.length >= 8 ? 4 : remaining.length >= 3 ? 3 : remaining.length
    const span = 12 / count
    const lanes: Lane[] = Array.from({ length: count }, () => ({ span, items: [] }))
    while (remaining.length) shortest(lanes).items.push(take(remaining))
    level(lanes)
    sections.push({ lanes })
  }
  return sections
}

export function headlineClass(size: EditionBlockSize) {
  switch (size) {
    case 'lead':
      return 'text-[2.125rem] leading-[1.28] lg:text-[2.875rem] lg:leading-[1.22]'
    case 'feature':
      return 'text-[1.75rem] leading-[1.32] lg:text-[2.125rem] lg:leading-[1.28]'
    case 'standard':
      return 'text-[1.5rem] leading-[1.38] lg:text-[1.625rem]'
    case 'short':
      return 'text-[1.375rem] leading-[1.42] lg:text-[1.375rem]'
    default:
      return 'text-[1.25rem] leading-[1.45] lg:text-[1.1875rem]'
  }
}
