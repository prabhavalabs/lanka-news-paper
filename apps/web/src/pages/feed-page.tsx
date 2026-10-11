import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useParams, useSearchParams } from 'react-router'

import { cn } from '@snap/ui/utils'
import { PageHead, useDocumentTitle } from '@/components/newsprint/page-head'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { sourceTypeLabel } from '@/lib/format'
import { ArticleCard } from './article-card'

const client = createClient()
const sourceTypes = ['private_media', 'state_owned', 'government', 'international', 'independent'] as const

export function FeedPage({ mode }: { mode: 'category' | 'source' | 'search' }) {
  const { slug, id } = useParams()
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const cursor = params.get('cursor') ?? ''
  const type = params.get('type') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''
  const categories = useQuery({
    queryKey: ['categories'],
    queryFn: () => client.categories(),
    enabled: mode === 'category',
    staleTime: 10 * 60_000,
  })
  const source = useQuery({
    queryKey: ['public-source', id],
    queryFn: () => client.getSource(id ?? ''),
    enabled: mode === 'source' && Boolean(id),
  })
  const query = useQuery({
    queryKey: ['feed', mode, slug, id, q, cursor, type, from, to],
    queryFn: () =>
      client.listNews({
        ...(mode === 'category' && slug ? { category: slug } : {}),
        ...(mode === 'source' && id ? { source: id } : {}),
        ...(mode === 'search' && q ? { q } : {}),
        ...(type ? { type } : {}),
        ...(from ? { from } : {}),
        ...(to ? { to } : {}),
        ...(cursor ? { cursor } : {}),
      }),
  })

  const categoryName = categories.data?.items.find((item) => item.slug === slug)?.name_si
  const title =
    mode === 'search'
      ? q
        ? `සෙවුම: “${q}”`
        : 'සියලු නවතම පුවත්'
      : mode === 'category'
        ? (categoryName ?? 'පුවත්')
        : (source.data?.name ?? 'මූලාශ්‍රය')
  const kicker = mode === 'category' ? 'අංශය' : mode === 'source' ? (source.data ? sourceTypeLabel(source.data.type) : 'මූලාශ්‍රය') : 'සෙවුම'
  useDocumentTitle(title)
  const filtersActive = Boolean(type || from || to)

  const filters = (
    <form
      role="search"
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault()
        const data = new FormData(event.currentTarget)
        const next = new URLSearchParams(params)
        next.delete('cursor')
        for (const key of ['type', 'from', 'to', 'q'] as const) {
          const value = String(data.get(key) ?? '').trim()
          if (value) next.set(key, value)
          else next.delete(key)
        }
        setParams(next)
      }}
    >
      {mode === 'search' ? (
        <label className="flex flex-col gap-1">
          <span className="np-kicker text-ink">සෙවුම් පදය</span>
          <input name="q" type="search" enterKeyHint="search" defaultValue={q} className="np-field" placeholder="උදා: ඉන්ධන මිල" />
        </label>
      ) : null}
      <label className="flex flex-col gap-1">
        <span className="np-kicker text-ink">මාධ්‍ය වර්ගය</span>
        <select name="type" defaultValue={type} className="np-field">
          <option value="">සියලු වර්ග</option>
          {sourceTypes.map((value) => (
            <option key={value} value={value}>
              {sourceTypeLabel(value)}
            </option>
          ))}
        </select>
      </label>
      <div className="grid grid-cols-2 gap-3">
        <label className="flex flex-col gap-1">
          <span className="np-kicker text-ink">සිට</span>
          <input name="from" type="date" defaultValue={from} className="np-field" />
        </label>
        <label className="flex flex-col gap-1">
          <span className="np-kicker text-ink">දක්වා</span>
          <input name="to" type="date" defaultValue={to} className="np-field" />
        </label>
      </div>
      <button type="submit" className="np-btn">
        {mode === 'search' ? 'සොයන්න' : 'පෙරහන් යොදන්න'}
      </button>
    </form>
  )

  return (
    <section>
      <PageHead
        title={title}
        kicker={kicker}
        deck={mode === 'source' && source.data?.description ? source.data.description : undefined}
      />
      {mode === 'source' && source.data?.website ? (
        <p className="mt-2 text-center">
          <a href={source.data.website} target="_blank" rel="noopener noreferrer" className="np-link np-meta inline-flex min-h-11 items-center">
            {source.data.name} වෙබ් අඩවිය ↗
          </a>
        </p>
      ) : null}

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-12 lg:gap-8">
        <aside className="lg:order-2 lg:col-span-4 lg:border-s lg:border-rule lg:ps-8">
          {mode === 'search' ? (
            filters
          ) : (
            <>
              <details className="border border-ink lg:hidden" open={filtersActive || undefined}>
                <summary className="flex min-h-11 cursor-pointer items-center px-3 font-headline font-bold">
                  පෙරහන්{filtersActive ? ' (යොදා ඇත)' : ''}
                </summary>
                <div className="p-3">{filters}</div>
              </details>
              <div className="hidden lg:block">{filters}</div>
            </>
          )}
        </aside>

        <div className="lg:order-1 lg:col-span-8">
          {query.isPending ? <GalleyPlaceholder lines={8} /> : null}
          {query.isError ? <Notice title="පුවත් පූරණය කළ නොහැකි විය" retry={() => query.refetch()} /> : null}
          {query.data && !query.data.items.length ? (
            <Notice title="ගැළපෙන පුවත් හමු නොවීය">වෙනත් වචනයක් හෝ දින පරාසයක් උත්සාහ කරන්න.</Notice>
          ) : null}
          <ol>
            {query.data?.items.map((item, index) => (
              <li key={item.id} className={cn(index > 0 && 'border-t border-rule')}>
                <ArticleCard item={item} />
              </li>
            ))}
          </ol>
          {query.data && (cursor || query.data.next_cursor) ? (
            <nav aria-label="පිටු" className="mt-4 flex flex-wrap justify-between gap-3 border-t-2 border-ink pt-4">
              {cursor ? (
                <button
                  type="button"
                  className="np-btn np-btn-ghost"
                  onClick={() => {
                    const next = new URLSearchParams(params)
                    next.delete('cursor')
                    setParams(next)
                  }}
                >
                  ← නවතම පුවත් වෙත
                </button>
              ) : (
                <span />
              )}
              {query.data.next_cursor ? (
                <button
                  type="button"
                  className="np-btn"
                  onClick={() => {
                    const next = new URLSearchParams(params)
                    next.set('cursor', query.data.next_cursor ?? '')
                    setParams(next)
                    window.scrollTo({ top: 0 })
                  }}
                >
                  පැරණි පුවත් →
                </button>
              ) : null}
            </nav>
          ) : null}
        </div>
      </div>
    </section>
  )
}
