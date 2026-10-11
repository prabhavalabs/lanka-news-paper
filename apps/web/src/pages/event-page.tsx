import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router'

import { eventCoverage } from '@/components/newsprint/coverage-sidebar'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { StoryReader } from '@/components/newsprint/story-reader'

const client = createClient()

export function EventPage() {
  const { id = '' } = useParams()
  const event = useQuery({ queryKey: ['event', id], queryFn: () => client.getEvent(id), enabled: Boolean(id) })

  if (event.isPending) return <GalleyPlaceholder lines={10} className="max-w-3xl" />
  if (event.isError || !event.data) return <Notice title="මෙම පුවත සොයාගත නොහැක" retry={() => event.refetch()} />

  const data = event.data
  const analysis = data.analysis
  const latest = data.articles.reduce<string>((max, article) => (article.published_at > max ? article.published_at : max), '')
  const withPhoto = data.articles.find((article) => article.media)
  return (
    <StoryReader
      key={data.id}
      kind="event"
      id={data.id}
      headline={data.title}
      summary={analysis?.summary}
      category={data.articles.find((article) => article.category)?.category}
      breaking={data.is_breaking}
      outlets={eventCoverage(data.articles, analysis)}
      coverage={
        analysis
          ? {
              left: analysis.left_percentage,
              center: analysis.center_percentage,
              right: analysis.right_percentage,
              rated: analysis.rated_source_count > 0,
            }
          : null
      }
      photo={withPhoto ? { url: withPhoto.media, credit: withPhoto.source.name } : undefined}
      updatedAt={latest || undefined}
    />
  )
}
