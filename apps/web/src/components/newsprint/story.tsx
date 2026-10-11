import type { FrontPageStory } from '@snap/api-client'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { absoluteTitle, lede, outletsReported, relativeTime } from '@/lib/format'
import { HalftoneFigure } from './figure'
import { Fleuron } from './ornament'
import { SourceLedger, SourceLine, fromStorySources } from './sources'

export function storyHref(story: Pick<FrontPageStory, 'kind' | 'id'>) {
  return story.kind === 'event' ? `/e/${story.id}` : `/a/${story.id}`
}

function Kicker({ story }: { story: FrontPageStory }) {
  return (
    <p className="np-kicker flex flex-wrap items-center gap-x-2">
      {story.breaking ? (
        <span className="bg-ink px-1.5 text-paper">විශේෂ පුවත</span>
      ) : null}
      {story.category ? (
        <Link to={`/c/${story.category.slug}`} className="np-headline-link">
          {story.category.name_si}
        </Link>
      ) : null}
      {story.curated ? (
        <span className="np-meta inline-flex items-center gap-1 text-[0.8125rem]" title="කතුවරයා මෙම පුවත මුල් පිටුවට තෝරා ගත්තේය">
          <Fleuron className="text-ink-secondary" />
          කතුවරයා තෝරාගත්
        </span>
      ) : null}
    </p>
  )
}

function EditorNote({ note }: { note: string | null }) {
  if (!note) return null
  return (
    <p className="mt-2 border-s-2 border-ink ps-3 font-headline text-[0.9375rem] leading-[1.7] font-medium text-ink-secondary">
      <span className="sr-only">කතුවරයාගේ සටහන: </span>
      {note}
    </p>
  )
}

function Dateline({ story }: { story: FrontPageStory }) {
  const value = story.updated_at || story.published_at
  return (
    <p className="np-meta">
      <span>{outletsReported(story.source_count || story.sources.length)}</span>
      {' · '}
      <time dateTime={value} title={absoluteTitle(value)}>
        {relativeTime(value)}
      </time>
    </p>
  )
}

/** The lead: set large across the broadsheet, with the full ledger of outlets. */
export function LeadStory({ story }: { story: FrontPageStory }) {
  const sources = fromStorySources(story.sources)
  const first = lede(story.summary, 440)
  const summary = first ? [first] : []
  return (
    <article aria-labelledby={`story-${story.id}`}>
      <Kicker story={story} />
      <h2 id={`story-${story.id}`} className="np-headline mt-1 text-[2rem] leading-[1.32] md:text-[2.5rem] lg:text-[3rem] lg:leading-[1.24]">
        <Link to={storyHref(story)} className="np-headline-link">
          {story.headline}
        </Link>
      </h2>
      <EditorNote note={story.editor_note} />
      <div className="mt-3">
        <Dateline story={story} />
      </div>
      <HalftoneFigure src={story.media} alt="" credit={sources[0]?.name} className="mt-4" priority />
      {summary.length ? (
        <div className="np-body mt-4 text-[1.125rem] text-ink-secondary md:columns-2 md:gap-8 md:[column-rule:1px_solid_var(--rule)]">
          {summary.slice(0, 2).map((text, index) => (
            <p key={index} className={cn(index > 0 && 'mt-3')}>
              {text}
            </p>
          ))}
          <p className="np-meta mt-3 text-[0.8125rem]">සාරාංශය: පහත මාධ්‍ය වාර්තා ඇසුරින් සකස් කළකි.</p>
        </div>
      ) : null}
      <p className="mt-3">
        <Link to={storyHref(story)} className="np-link font-headline inline-flex min-h-11 items-center font-bold">
          {story.kind === 'event' ? 'සියලු වාර්තා සසඳන්න →' : 'වාර්තාව විවෘත කරන්න →'}
        </Link>
      </p>
      <SourceLedger sources={sources} title="මෙම පුවත වාර්තා කළ මාධ්‍ය" showHeadlines={false} headingLevel={3} className="mt-2" />
    </article>
  )
}

/** A top story in a ruled column. */
export function TopStory({ story }: { story: FrontPageStory }) {
  const sources = fromStorySources(story.sources)
  const text = lede(story.summary, 260)
  return (
    <article aria-labelledby={`story-${story.id}`}>
      <Kicker story={story} />
      <h3 id={`story-${story.id}`} className="np-headline mt-1 text-[1.4375rem] leading-[1.42] lg:text-[1.5rem]">
        <Link to={storyHref(story)} className="np-headline-link">
          {story.headline}
        </Link>
      </h3>
      <EditorNote note={story.editor_note} />
      <HalftoneFigure src={story.media} alt="" credit={sources[0]?.name} className="mt-3" />
      {text ? <p className="np-body mt-2 text-[1.0625rem] text-ink-secondary">{text}</p> : null}
      <div className="mt-1">
        <Dateline story={story} />
      </div>
      <SourceLine sources={sources} />
    </article>
  )
}

/** A compact entry for the "more stories" column list. */
export function BriefStory({ story }: { story: FrontPageStory }) {
  const sources = fromStorySources(story.sources)
  return (
    <article aria-labelledby={`story-${story.id}`} className="break-inside-avoid">
      <Kicker story={story} />
      <h3 id={`story-${story.id}`} className="font-headline text-[1.1875rem] leading-[1.5] font-bold">
        <Link to={storyHref(story)} className="np-headline-link">
          {story.headline}
        </Link>
      </h3>
      <EditorNote note={story.editor_note} />
      <Dateline story={story} />
      <SourceLine sources={sources} />
    </article>
  )
}
