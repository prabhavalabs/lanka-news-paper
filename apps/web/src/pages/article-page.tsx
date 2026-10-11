import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useParams } from 'react-router'

import { articleCoverage } from '@/components/newsprint/coverage-sidebar'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { StoryReader } from '@/components/newsprint/story-reader'

const client = createClient()

function setMeta(property: string, content: string) {
  let element = document.querySelector(`meta[property="${property}"]`)
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute('property', property)
    document.head.appendChild(element)
  }
  element.setAttribute('content', content)
}

export function ArticlePage() {
  const { id = '' } = useParams()
  const article = useQuery({ queryKey: ['article', id], queryFn: () => client.getNews(id), enabled: Boolean(id) })
  const eventId = article.data?.event_id ?? ''
  // The outlet's logo travels with the event's coverage data.
  const event = useQuery({
    queryKey: ['event', eventId],
    queryFn: () => client.getEvent(eventId),
    enabled: Boolean(eventId),
    retry: false,
  })

  useEffect(() => {
    if (!article.data) return
    setMeta('og:title', `${article.data.headline} — තුලාව`)
    setMeta('og:description', `${article.data.source.name} වාර්තාව. සම්පූර්ණ ලිපිය මුල් ප්‍රකාශකයා සතුය.`)
    setMeta('og:type', 'article')
  }, [article.data])

  if (article.isPending) return <GalleyPlaceholder lines={10} className="max-w-3xl" />
  if (article.isError || !article.data) return <Notice title="මෙම වාර්තාව සොයාගත නොහැක" retry={() => article.refetch()} />

  const item = article.data
  const icon = event.data?.analysis?.source_spectrum.find((entry) => entry.source_id === item.source.id)?.source_icon || null
  const frame = item.analysis
  return (
    <StoryReader
      key={item.id}
      kind="article"
      id={item.id}
      headline={item.headline}
      summary={frame?.summary ?? item.excerpt}
      category={item.category}
      outlets={[articleCoverage(item, icon)]}
      coverage={
        frame
          ? {
              left: frame.left_probability * 100,
              center: frame.center_probability * 100,
              right: frame.right_probability * 100,
              rated: frame.relevant,
            }
          : null
      }
      photo={item.media ? { url: item.media, credit: item.source.name } : undefined}
      updatedAt={item.published_at}
    />
  )
}
