import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useParams, useSearchParams } from 'react-router'

import { ArticleCard } from './article-card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const client = createClient()
const selectClass =
  'h-8 w-full border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50'

export function FeedPage({ mode }: { mode: 'category' | 'source' | 'search' }) {
  const { slug, id } = useParams()
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const cursor = params.get('cursor') ?? ''
  const type = params.get('type') ?? ''
  const from = params.get('from') ?? ''
  const to = params.get('to') ?? ''
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

  return (
    <section>
      <h1 className="font-headline text-[length:var(--text-h1)] font-bold">
        {mode === 'search' ? `සෙවුම: ${q || 'සියල්ල'}` : mode === 'category' ? slug : (source.data?.name ?? 'මූලාශ්‍රය')}
      </h1>
      {mode === 'source' && source.data?.description ? (
        <p className="mt-3 max-w-[65ch] text-[color:var(--ink-secondary)]">{source.data.description}</p>
      ) : null}
      <form
        className="mt-6 flex max-w-xl flex-col gap-3"
        onSubmit={(event) => {
          event.preventDefault()
          const data = new FormData(event.currentTarget)
          const next = new URLSearchParams(params)
          next.delete('cursor')
          for (const key of ['type', 'from', 'to', 'q'] as const) {
            const value = String(data.get(key) ?? '')
            if (value) next.set(key, value)
            else next.delete(key)
          }
          setParams(next)
        }}
      >
        {mode === 'search' ? <Input name="q" defaultValue={q} placeholder="සෙවුම" /> : null}
        <select name="type" defaultValue={type} className={selectClass} aria-label="මූලාශ්‍ර වර්ගය">
          <option value="">සියලු වර්ග</option>
          <option value="private_media">පෞද්ගලික</option>
          <option value="state_owned">රාජ්‍ය</option>
          <option value="government">රජය</option>
          <option value="international">ජාත්‍යන්තර</option>
          <option value="independent">ස්වාධීන</option>
        </select>
        <div className="flex gap-3">
          <Input name="from" type="date" defaultValue={from} aria-label="සිට" />
          <Input name="to" type="date" defaultValue={to} aria-label="දක්වා" />
        </div>
        <Button type="submit">පෙරහන්</Button>
      </form>
      <ol className="mt-6">
        {query.data?.items.map((item) => (
          <li key={item.id}>
            <ArticleCard item={item} />
          </li>
        ))}
      </ol>
      {query.data?.next_cursor ? (
        <Button
          variant="outline"
          className="mt-6"
          onClick={() => {
            params.set('cursor', query.data.next_cursor ?? '')
            setParams(params)
          }}
        >
          ඊළඟ පිටුව
        </Button>
      ) : null}
    </section>
  )
}
