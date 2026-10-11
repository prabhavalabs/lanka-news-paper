import { createClient, type ArticleNarrativeAnalysis } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link, useParams } from 'react-router'

import { fromArticles } from '@/components/newsprint/sources'
import { SpectrumBar } from '@/components/newsprint/spectrum'
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

  useEffect(() => {
    if (!article.data) return
    setMeta('og:title', `${article.data.headline} — තුලාව`)
    setMeta('og:description', `${article.data.source.name} වාර්තාව. සම්පූර්ණ ලිපිය මුල් ප්‍රකාශකයා සතුය.`)
    setMeta('og:type', 'article')
  }, [article.data])

  if (article.isPending) return <GalleyPlaceholder lines={10} className="mx-auto max-w-[40rem]" />
  if (article.isError || !article.data) return <Notice title="මෙම වාර්තාව සොයාගත නොහැක" retry={() => article.refetch()} />

  const item = article.data
  return (
    <StoryReader
      kind="article"
      id={item.id}
      headline={item.headline}
      summary={item.analysis?.summary ?? item.excerpt}
      category={item.category}
      sources={fromArticles([item])}
      photo={item.media ? { url: item.media, credit: item.source.name } : undefined}
      updatedAt={item.published_at}
      analysis={
        <>
          {item.analysis ? <FrameNote analysis={item.analysis} /> : null}
          {item.event_id ? (
            <p className="np-meta mt-3 text-[0.875rem]">
              <Link to={`/e/${item.event_id}`} className="np-link inline-flex min-h-11 items-center">
                මෙම පුවත අනෙක් මාධ්‍ය වාර්තා කළ ආකාරය →
              </Link>
            </p>
          ) : null}
        </>
      }
    />
  )
}

function FrameNote({ analysis }: { analysis: ArticleNarrativeAnalysis }) {
  return (
    <section aria-labelledby="frame-title" className="border-t border-ink pt-2">
      <h2 id="frame-title" className="np-kicker text-[0.875rem] text-ink">
        වාර්තාකරණ රාමුව
      </h2>
      {analysis.relevant ? (
        <SpectrumBar
          className="mt-2"
          left={analysis.left_probability * 100}
          center={analysis.center_probability * 100}
          right={analysis.right_probability * 100}
        />
      ) : (
        <p className="np-meta mt-1 text-[0.875rem]">කැපී පෙනෙන දේශපාලන රාමුවක් හමු නොවීය.</p>
      )}
      <p className="np-meta mt-1 text-[0.75rem]">
        ස්වයංක්‍රීය විශ්ලේෂණයකි; මෙම ලිපියේ වාර්තාකරණ රාමුව පමණක් විස්තර කරයි, ප්‍රකාශකයාට දෙන ස්ථිර ලේබලයක් නොවේ.
      </p>
    </section>
  )
}
