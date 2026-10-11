import { createClient, type EventNarrativeAnalysis } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router'

import { fromArticles } from '@/components/newsprint/sources'
import { SpectrumBar, leanLabels } from '@/components/newsprint/spectrum'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { StoryReader } from '@/components/newsprint/story-reader'

const client = createClient()

export function EventPage() {
  const { id = '' } = useParams()
  const event = useQuery({ queryKey: ['event', id], queryFn: () => client.getEvent(id), enabled: Boolean(id) })

  if (event.isPending) return <GalleyPlaceholder lines={10} className="mx-auto max-w-[40rem]" />
  if (event.isError || !event.data) return <Notice title="මෙම පුවත සොයාගත නොහැක" retry={() => event.refetch()} />

  const data = event.data
  const latest = data.articles.reduce<string>((max, article) => (article.published_at > max ? article.published_at : max), '')
  const withPhoto = data.articles.find((article) => article.media)
  return (
    <StoryReader
      kind="event"
      id={data.id}
      headline={data.title}
      summary={data.analysis?.summary}
      category={data.articles.find((article) => article.category)?.category}
      breaking={data.is_breaking}
      sources={fromArticles(data.articles)}
      photo={withPhoto ? { url: withPhoto.media, credit: withPhoto.source.name } : undefined}
      updatedAt={latest || undefined}
      analysis={data.analysis ? <CoverageNote analysis={data.analysis} /> : null}
    />
  )
}

/** Compact, monochrome note on how each outlet framed the story. */
function CoverageNote({ analysis }: { analysis: EventNarrativeAnalysis }) {
  const groups = (['left', 'center', 'right'] as const)
    .map((lean) => ({ lean, names: analysis.source_spectrum.filter((item) => item.label === lean).map((item) => item.source) }))
    .filter((group) => group.names.length)
  const unrated = analysis.source_spectrum.filter((item) => item.label === 'unrated').map((item) => item.source)
  return (
    <section aria-labelledby="coverage-title" className="border-t border-ink pt-2">
      <h2 id="coverage-title" className="np-kicker text-[0.875rem] text-ink">
        වාර්තාකරණ රාමු
      </h2>
      {analysis.rated_source_count > 0 ? (
        <SpectrumBar className="mt-2" left={analysis.left_percentage} center={analysis.center_percentage} right={analysis.right_percentage} />
      ) : (
        <p className="np-meta mt-1 text-[0.875rem]">දේශපාලන රාමුවක් මැනිය හැකි වාර්තා නැත.</p>
      )}
      <ul className="np-meta mt-2 text-[0.8125rem] leading-[1.7]">
        {groups.map((group) => (
          <li key={group.lean}>
            <span className="text-ink">{leanLabels[group.lean]}:</span> {[...new Set(group.names)].join(', ')}
          </li>
        ))}
        {unrated.length ? (
          <li>
            <span className="text-ink">{leanLabels.unrated}:</span> {[...new Set(unrated)].join(', ')}
          </li>
        ) : null}
      </ul>
      <p className="np-meta mt-1 text-[0.75rem]">
        ස්වයංක්‍රීය විශ්ලේෂණයකි; එක් එක් ලිපියේ වාර්තාකරණ රාමුව පමණක් විස්තර කරයි, ප්‍රකාශකයන්ට දෙන ස්ථිර ලේබල නොවේ.
      </p>
    </section>
  )
}
