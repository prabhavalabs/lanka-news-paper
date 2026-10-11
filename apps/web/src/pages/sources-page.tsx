import { createClient, type PublicSource } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

import { OutletName } from '@/components/newsprint/outlet'
import { PageHead, useDocumentTitle } from '@/components/newsprint/page-head'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { sourceTypeLabel } from '@/lib/format'

const client = createClient()
const order = ['state_owned', 'government', 'private_media', 'independent', 'international', 'other']
const rank = (type: string) => (order.includes(type) ? order.indexOf(type) : order.length)

export function SourcesPage() {
  useDocumentTitle('මූලාශ්‍ර මාධ්‍ය')
  const sources = useQuery({ queryKey: ['public-sources'], queryFn: () => client.sources() })
  const groups = new Map<string, PublicSource[]>()
  for (const source of sources.data?.items ?? []) {
    groups.set(source.type, [...(groups.get(source.type) ?? []), source])
  }
  const types = [...groups.keys()].sort((a, b) => rank(a) - rank(b))

  return (
    <section>
      <PageHead
        title="මූලාශ්‍ර මාධ්‍ය"
        kicker="අප පුවත් ලබා ගන්නා ආයතන"
        deck="තුලාවේ සෑම පුවතක්ම මෙම මාධ්‍ය ආයතනවලින් එකක් හෝ කිහිපයක් පළ කළ වාර්තාවකි. සෑම වාර්තාවකම මුල් ලිපියට සබැඳියක් ඇත."
      />
      {sources.isPending ? <GalleyPlaceholder className="mt-8" /> : null}
      {sources.isError ? <Notice title="මූලාශ්‍ර ලැයිස්තුව පූරණය කළ නොහැක" retry={() => sources.refetch()} className="mt-8" /> : null}
      <div className="np-columns mt-6 md:columns-2 lg:columns-3">
        {types.map((type) => (
          <section key={type} className="mb-6 break-inside-avoid" aria-labelledby={`type-${type}`}>
            <h2 id={`type-${type}`} className="np-section-head">
              {sourceTypeLabel(type)}
            </h2>
            <ul>
              {groups.get(type)?.map((source) => (
                <li key={source.id} className="border-b border-rule py-2">
                  <Link to={`/sources/${source.id}`} className="np-headline-link inline-flex min-h-11 items-center text-[1.0625rem]">
                    <OutletName name={source.name} />
                  </Link>
                  {source.description ? <p className="np-meta text-[0.875rem]">{source.description}</p> : null}
                  {source.website ? (
                    <a href={source.website} target="_blank" rel="noopener noreferrer" className="np-link np-meta inline-flex min-h-11 items-center text-[0.875rem]">
                      වෙබ් අඩවිය ↗<span className="sr-only"> (නව පටිත්තක)</span>
                    </a>
                  ) : null}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </section>
  )
}
