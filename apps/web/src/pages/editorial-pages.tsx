import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'

import { cn } from '@snap/ui/utils'
import { Fleuron, OrnamentRule } from '@/components/newsprint/ornament'
import { PageHead, useDocumentTitle } from '@/components/newsprint/page-head'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { absoluteTitle, dropCapSplit, paragraphs, sinhalaLongDate } from '@/lib/format'

const client = createClient()

function OpinionLabel() {
  return (
    <p className="np-meta inline-flex items-center gap-2 border border-ink px-3 py-1 text-[0.875rem] text-ink">
      <Fleuron className="text-ink-secondary" />
      කතුවැකිය — කතුවරයාගේ මතය; පුවත් වාර්තාවක් නොවේ
    </p>
  )
}

export function EditorialListPage() {
  useDocumentTitle('කතුවැකිය')
  const list = useQuery({ queryKey: ['editorials'], queryFn: () => client.editorials() })

  return (
    <section>
      <PageHead
        title="කතුවැකිය"
        kicker="කතු මණ්ඩල පිටුව"
        deck="ප්‍රධාන කතුවරයාගේ මතය. පුවත් පිටුවල ඇත්තේ මාධ්‍ය වාර්තා ය; මෙහි ඇත්තේ ඒවා ගැන කතුවරයා සිතන දෙයයි."
      />
      {list.isPending ? <GalleyPlaceholder className="mx-auto mt-8 max-w-3xl" /> : null}
      {list.isError ? (
        <Notice title="කතුවැකි දැනට පූරණය කළ නොහැක" retry={() => list.refetch()} className="mt-8" />
      ) : null}
      {list.data && !list.data.items.length ? (
        <Notice title="තවම කතුවැකියක් පළ කර නැත" className="mt-8">
          පළමු කතුවැකිය පළ වූ විට එය මෙහි දිස් වේ.
        </Notice>
      ) : null}
      {list.data?.items.length ? (
        <ol className="mx-auto mt-6 max-w-3xl">
          {list.data.items.map((item, index) => (
            <li key={item.id} className={cn('py-6', index > 0 && 'border-t border-rule')}>
              <article aria-labelledby={`ed-${item.id}`}>
                <p className="np-meta">
                  <time dateTime={item.published_at} title={absoluteTitle(item.published_at)}>
                    {sinhalaLongDate(item.published_at)}
                  </time>
                </p>
                <h2
                  id={`ed-${item.id}`}
                  className={cn('np-headline mt-1', index === 0 ? 'text-[2rem] leading-[1.35] md:text-[2.5rem]' : 'text-[1.5rem] leading-[1.45]')}
                >
                  <Link to={`/editorial/${item.id}`} className="np-headline-link">
                    {item.title}
                  </Link>
                </h2>
                {item.standfirst ? <p className="np-standfirst mt-2">{item.standfirst}</p> : null}
                <p className="mt-1 flex flex-wrap items-center justify-between gap-x-4">
                  <span className="np-meta">{item.author_name}</span>
                  <Link to={`/editorial/${item.id}`} className="np-link font-headline inline-flex min-h-11 items-center font-bold">
                    කියවන්න →
                  </Link>
                </p>
              </article>
            </li>
          ))}
        </ol>
      ) : null}
    </section>
  )
}

function Body({ text }: { text: string }) {
  const parts = paragraphs(text)
  if (!parts.length) return null
  const [first = '', ...rest] = parts
  const [cap, remainder] = dropCapSplit(first)
  return (
    <div className="np-body text-[1.125rem] md:text-[1.1875rem]">
      <p>
        <span className="np-dropcap">{cap}</span>
        {remainder}
      </p>
      {rest.map((paragraph, index) => (
        <p key={index} className="mt-3">
          {paragraph}
        </p>
      ))}
    </div>
  )
}

export function EditorialPage() {
  const { id = '' } = useParams()
  const editorial = useQuery({ queryKey: ['editorial', id], queryFn: () => client.editorial(id), enabled: Boolean(id) })
  useDocumentTitle(editorial.data ? `${editorial.data.title} — කතුවැකිය` : 'කතුවැකිය')

  if (editorial.isPending) return <GalleyPlaceholder className="mx-auto max-w-3xl" lines={12} />
  if (editorial.isError || !editorial.data) {
    return (
      <Notice title="මෙම කතුවැකිය සොයාගත නොහැක" retry={() => editorial.refetch()}>
        <Link to="/editorial" className="np-link">
          සියලු කතුවැකි බලන්න
        </Link>
      </Notice>
    )
  }

  const item = editorial.data
  return (
    <article className="mx-auto max-w-[44rem]" aria-labelledby="editorial-title">
      <div className="np-rule-double" />
      <div className="mt-4 text-center">
        <OpinionLabel />
      </div>
      <h1 id="editorial-title" className="np-headline mt-4 text-center text-[2.25rem] leading-[1.35] md:text-[3.25rem] md:leading-[1.25]">
        {item.title}
      </h1>
      {item.standfirst ? (
        <p className="np-standfirst mx-auto mt-4 max-w-[38rem] text-center md:text-[1.1875rem]">
          {item.standfirst}
        </p>
      ) : null}
      <OrnamentRule className="mx-auto mt-5 max-w-xs" kind="fleuron" />
      <p className="np-meta mt-3 text-center">
        <span className="font-headline font-bold text-ink">{item.author_name}</span>
        {' · '}
        <time dateTime={item.published_at} title={absoluteTitle(item.published_at)}>
          {sinhalaLongDate(item.published_at)}
        </time>
      </p>
      <div className="np-rule mt-4" />
      <div className="mt-6">
        <Body text={item.body} />
      </div>
      <OrnamentRule className="mt-8" />
      <div className="mt-4 flex flex-col gap-1 sm:flex-row sm:flex-wrap sm:justify-between">
        {item.event_id ? (
          <Link to={`/e/${item.event_id}`} className="np-link font-headline inline-flex min-h-11 items-center font-bold">
            මෙම කතුවැකියට අදාළ පුවත සහ මුල් වාර්තා →
          </Link>
        ) : null}
        <Link to="/editorial" className="np-link font-headline inline-flex min-h-11 items-center font-bold">
          සියලු කතුවැකි
        </Link>
      </div>
      <p className="np-meta mt-4 border-t border-rule pt-3 text-[0.875rem]">
        කතුවැකිය තුලාව ප්‍රධාන කතුවරයාගේ මතය පමණි. පුවත් පිටුවල පළ වන වාර්තා ඒවා ලියූ මාධ්‍ය ආයතනවලට අයත් ය.
      </p>
    </article>
  )
}
