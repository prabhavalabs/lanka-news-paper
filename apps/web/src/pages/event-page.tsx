import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router'

import { ArticleCard } from './article-card'

const client = createClient()

export function EventPage() {
  const { id } = useParams()
  const event = useQuery({
    queryKey: ['event', id],
    queryFn: () => client.getEvent(id ?? ''),
    enabled: Boolean(id),
  })
  if (!event.data) {
    return <p>පූරණය වේ…</p>
  }
  return (
    <section>
      <p className="text-[0.8125rem] text-muted-foreground">සිදුවීම</p>
      <h1 className="font-headline text-[length:var(--text-h1)] font-bold">{event.data.title}</h1>
      {event.data.is_breaking ? (
        <p className="mt-4 border border-rule bg-tint px-3 py-2 text-sm">විශේෂ පුවත් — මූලාශ්‍ර කිහිපයක් වාර්තා කර ඇත.</p>
      ) : null}
      <ol className="mt-6">
        {event.data.articles.map((item) => (
          <li key={item.id}>
            <ArticleCard item={item} />
          </li>
        ))}
      </ol>
    </section>
  )
}
