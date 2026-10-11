import type { EditionBlock, FrontPageStory } from '@snap/api-client'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { blockCopy, headlineClass, textColumns } from '@/lib/edition'
import { dropCapSplit, lede } from '@/lib/format'
import { Illustration } from './figure'
import { Fleuron } from './ornament'
import { SourceLine, fromStorySources } from './sources'

export function storyHref(story: Pick<FrontPageStory, 'kind' | 'id'>) {
  return story.kind === 'event' ? `/e/${story.id}` : `/a/${story.id}`
}

function Kicker({ story }: { story: FrontPageStory }) {
  if (!story.breaking && !story.category && !story.curated) return null
  return (
    <p className="np-kicker flex flex-wrap items-center gap-x-2 text-[0.8125rem] leading-[1.6]">
      {story.breaking ? <span className="bg-ink px-1 text-paper">විශේෂ</span> : null}
      {story.category ? <span>{story.category.name_si}</span> : null}
      {story.curated ? (
        <span className="inline-flex items-center gap-1 text-ink-tertiary" title="කතුවරයා මෙම පුවත මුල් පිටුවට තෝරා ගත්තේය">
          <Fleuron />
          <span className="sr-only">කතුවරයා තෝරාගත්</span>
        </span>
      ) : null}
    </p>
  )
}

function Paragraphs({ items, dropCap }: { items: string[]; dropCap: boolean }) {
  return (
    <>
      {items.map((text, index) => {
        if (index === 0 && dropCap) {
          const [cap, remainder] = dropCapSplit(text)
          return (
            <p key={index}>
              <span className="np-dropcap text-[2.9em]">{cap}</span>
              {remainder}
            </p>
          )
        }
        return <p key={index}>{text}</p>
      })}
    </>
  )
}

/**
 * A complete story printed inside its box: headline, standfirst, illustration and the whole text
 * flowing through balanced, justified columns, closed by one small source line.
 */
export function PrintBlock({ block, span, idPrefix = 'p' }: { block: EditionBlock; span: number; idPrefix?: string }) {
  const { story, size, image } = block
  const copy = blockCopy(block)
  const cols = textColumns(span)
  const isLead = size === 'lead'
  const titleId = `${idPrefix}-${story.kind}-${story.id}`
  const imageOnTop = image && (isLead || cols === 1)
  return (
    <article aria-labelledby={titleId} className="py-3">
      <Kicker story={story} />
      <h2 id={titleId} className={cn('np-headline', headlineClass(size))}>
        <Link to={storyHref(story)} className="np-headline-link">
          {copy.headline}
        </Link>
      </h2>
      {copy.standfirst && (isLead || size === 'feature') ? (
        <p className="np-standfirst mt-1">{copy.standfirst}</p>
      ) : null}
      {isLead && story.editor_note ? (
        <p className="np-meta mt-1 text-[0.8125rem]">
          <span className="sr-only">කතුවරයාගේ සටහන: </span>— {story.editor_note}
        </p>
      ) : null}
      {imageOnTop ? <Illustration image={image} className="mt-2" priority={isLead} /> : null}
      {copy.paragraphs.length ? (
        <div
          className={cn(
            'np-copy mt-2 lg:text-[1rem] lg:leading-[1.75]',
            cols === 3 && 'np-cols-3',
            cols === 2 && 'np-cols-2',
          )}
        >
          {image && !imageOnTop ? <Illustration image={image} className="mb-2" /> : null}
          <Paragraphs items={copy.paragraphs} dropCap={isLead} />
        </div>
      ) : null}
      <SourceLine sources={fromStorySources(story.sources)} updatedAt={story.updated_at || story.published_at} className="mt-1.5" />
    </article>
  )
}

/** Phone version: the lead prints in full; other stories give headline, opening and a way in. */
export function PhoneBlock({ block }: { block: EditionBlock }) {
  const { story, size, image } = block
  const copy = blockCopy(block)
  const isLead = size === 'lead'
  const titleId = `m-${story.kind}-${story.id}`
  const opening = copy.standfirst || lede(copy.paragraphs.join('\n\n'), 260)
  return (
    <article aria-labelledby={titleId} className="py-4">
      <Kicker story={story} />
      <h2 id={titleId} className={cn('np-headline', headlineClass(size))}>
        <Link to={storyHref(story)} className="np-headline-link">
          {copy.headline}
        </Link>
      </h2>
      {isLead ? (
        <>
          {copy.standfirst ? <p className="np-standfirst mt-1">{copy.standfirst}</p> : null}
          <Illustration image={image} className="mt-3" priority />
          <div className="np-copy mt-3">
            <Paragraphs items={copy.paragraphs} dropCap />
          </div>
        </>
      ) : (
        <>
          {image && (size === 'feature' || size === 'standard') ? <Illustration image={image} className="mt-2" /> : null}
          {opening ? <p className="np-copy mt-1.5">{opening}</p> : null}
        </>
      )}
      <SourceLine sources={fromStorySources(story.sources)} updatedAt={story.updated_at || story.published_at} linked={false} className="mt-1" />
      {!isLead ? (
        <Link to={storyHref(story)} className="np-link font-headline inline-flex min-h-11 items-center text-[0.9375rem]">
          සම්පූර්ණ පුවත →
        </Link>
      ) : null}
    </article>
  )
}
