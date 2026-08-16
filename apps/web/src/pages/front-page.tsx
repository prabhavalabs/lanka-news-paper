import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

import { ArticleCard } from './article-card'
import { Skeleton } from '@/components/ui/skeleton'

const client = createClient()

function diversify<T extends { source: { id: string } }>(items: T[]) {
  const seen = new Set<string>()
  const first: T[] = []
  const rest: T[] = []
  for (const item of items) {
    if (!seen.has(item.source.id)) {
      seen.add(item.source.id)
      first.push(item)
    } else {
      rest.push(item)
    }
  }
  return [...first, ...rest]
}

export function FrontPage() {
  const news = useQuery({ queryKey: ['news'], queryFn: () => client.listNews({ limit: '24' }) })
  const events = useQuery({ queryKey: ['events'], queryFn: () => client.listEvents() })

  if (news.isPending) {
    return (
      <section className="flex flex-col gap-4">
        <Skeleton className="h-10 w-2/3" />
        <Skeleton className="h-40 w-full" />
      </section>
    )
  }
  if (news.isError) {
    return <p>පුවත් පූරණය කළ නොහැකි විය. API ධාවනය වේදැයි පරීක්ෂා කරන්න.</p>
  }

  const items = diversify(news.data?.items ?? [])
  const lead = items[0]
  const rest = items.slice(1, 9)

  return (
    <section className="grid grid-cols-1 gap-8 lg:grid-cols-12">
      <div className="lg:col-span-8 lg:border-e lg:border-rule lg:pe-8">
        {lead ? (
          <ArticleCard item={lead} lead />
        ) : (
          <article className="border-b border-rule pb-8">
            <p className="text-[0.8125rem] text-muted-foreground">නවතම</p>
            <h1 className="font-headline mt-2 text-[length:var(--text-h1)] font-bold">අදට තවම පුවත් නැත</h1>
          </article>
        )}
        <ol className="mt-2 lg:hidden">
          {rest.map((item) => (
            <li key={item.id}>
              <ArticleCard item={item} />
            </li>
          ))}
        </ol>
      </div>
      <aside className="hidden lg:col-span-4 lg:block">
        <h2 className="border-t-2 border-ink pt-3 text-[length:var(--text-h2)] font-bold">දෙවන තීරුව</h2>
        <ol>
          {rest.map((item) => (
            <li key={item.id}>
              <ArticleCard item={item} />
            </li>
          ))}
        </ol>
        {events.data?.items.length ? (
          <div className="mt-8">
            <h2 className="border-t-2 border-ink pt-3 text-[length:var(--text-h2)] font-bold">සිදුවීම්</h2>
            <ul className="mt-3 flex flex-col gap-2 text-sm">
              {events.data.items.slice(0, 5).map((event) => (
                <li key={event.id}>
                  <Link to={`/e/${event.id}`}>{event.title}</Link>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
        <p className="mt-6 text-[0.8125rem]">
          <Link to="/search">සියලු පුවත්</Link>
        </p>
      </aside>
    </section>
  )
}
