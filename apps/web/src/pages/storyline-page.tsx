import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'

import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { sinhalaDateTime } from '@/lib/format'

const client = createClient()
export function StorylinePage() {
  const { slug = '' } = useParams()
  const query = useQuery({ queryKey: ['storyline', slug], queryFn: () => client.storyline(slug), enabled: Boolean(slug) })
  if (query.isPending) return <GalleyPlaceholder lines={10} className="max-w-3xl" />
  if (query.isError || !query.data) return <Notice title="මෙම පුවත් පෙළ සොයාගත නොහැක" retry={() => query.refetch()} />
  const story = query.data
  return <article className="mx-auto max-w-3xl">
    <header className="border-b border-ink pb-6">
      <p className="mb-3 font-headline text-sm text-ink-secondary">{story.trending ? 'දැන් කතාබහට ලක්වන' : 'අඛණ්ඩ පුවත් පෙළ'}</p>
      <h1 className="font-headline text-3xl font-bold leading-relaxed sm:text-4xl">{story.title_si}</h1>
      {story.summary_si && <p className="mt-4 font-body text-lg leading-relaxed">{story.summary_si}</p>}
      <p className="mt-4 text-sm text-ink-secondary">වාර්තා {story.article_count} · මාධ්‍ය ආයතන {story.source_count} · සිදුවීම් {story.event_count}</p>
    </header>
    <h2 className="mt-6 font-headline text-lg font-bold">පුවත දිගහැරුණු අයුරු</h2>
    <ol className="mt-4 border-s border-rule ps-5">{story.events?.map((event) => <li key={event.id} className="relative border-b border-rule py-5 first:pt-0">
      <span aria-hidden="true" className="absolute -start-[1.5rem] top-6 size-2 rounded-full bg-ink" />
      <time dateTime={event.time} className="text-sm text-ink-tertiary">{sinhalaDateTime(event.time)}</time>
      <h3 className="mt-2 font-headline text-xl font-bold leading-relaxed"><Link className="np-headline-link" to={`/e/${event.id}`}>{event.display_title}</Link></h3>
      {event.summary && <p className="mt-2 line-clamp-4 font-body text-base leading-relaxed">{event.summary}</p>}
      <p className="mt-3 text-sm text-ink-secondary">වාර්තා {event.article_count} · මාධ්‍ය ආයතන {event.source_count}</p>
    </li>)}</ol>
    {!story.events?.length && <p className="py-6 font-body">දැනට ප්‍රසිද්ධ වාර්තා නොමැත.</p>}
  </article>
}
