import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { ArticleCard } from './article-card'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'

const client = createClient()

export function ArticlePage() {
  const { id } = useParams()
  const [reason, setReason] = useState('')
  const article = useQuery({
    queryKey: ['article', id],
    queryFn: () => client.getNews(id ?? ''),
    enabled: Boolean(id),
  })
  useEffect(() => {
    if (!article.data) return
    document.title = `${article.data.headline} | ලංකා පුවත්`
    const setMeta = (property: string, content: string) => {
      let el = document.querySelector(`meta[property="${property}"]`)
      if (!el) {
        el = document.createElement('meta')
        el.setAttribute('property', property)
        document.head.appendChild(el)
      }
      el.setAttribute('content', content)
    }
    setMeta('og:title', `${article.data.headline} — ලංකා පුවත් සොයාගැනීම`)
    setMeta('og:description', `${article.data.source.name} වෙතින් සොයාගත් ශීර්ෂපාඨයකි. සම්පූර්ණ ලිපිය මුල් ප්‍රකාශකයා සතුය.`)
    setMeta('og:type', 'article')
  }, [article.data])
  if (!article.data) {
    return <p>පූරණය වේ…</p>
  }
  const item = article.data
  return (
    <section>
      <ArticleCard item={item} lead />
      {item.analysis ? (
        <section className="mt-8 border-y border-rule py-6" aria-labelledby="article-summary-title">
          <p className="text-xs font-bold uppercase tracking-[0.14em] text-muted-foreground">Article analysis</p>
          <h2 id="article-summary-title" className="font-headline mt-1 text-2xl font-bold">Source summary</h2>
          <p className="mt-3 max-w-4xl text-base leading-8">{item.analysis.summary}</p>
          {item.analysis.relevant ? <div className="mt-6 max-w-2xl"><div className="flex h-9 overflow-hidden border border-rule text-xs font-bold" aria-label={`Left ${item.analysis.left_probability * 100}%, center ${item.analysis.center_probability * 100}%, right ${item.analysis.right_probability * 100}%`}>
            {item.analysis.left_probability > 0 ? <span className="grid place-items-center bg-[#b96d70] text-white" style={{ width: `${item.analysis.left_probability * 100}%` }}>L {Math.round(item.analysis.left_probability * 100)}%</span> : null}
            {item.analysis.center_probability > 0 ? <span className="grid place-items-center bg-white text-black" style={{ width: `${item.analysis.center_probability * 100}%` }}>C {Math.round(item.analysis.center_probability * 100)}%</span> : null}
            {item.analysis.right_probability > 0 ? <span className="grid place-items-center bg-[#587ead] text-white" style={{ width: `${item.analysis.right_probability * 100}%` }}>R {Math.round(item.analysis.right_probability * 100)}%</span> : null}
          </div><p className="mt-2 text-xs text-muted-foreground">This score describes this article’s reporting frame, not the publisher as a whole.</p></div> : <p className="mt-5 text-sm text-muted-foreground">No meaningful political framing was detected in this report.</p>}
        </section>
      ) : null}
      <div className="mt-6 flex gap-4">
        {item.event_id ? <Link to={`/e/${item.event_id}`}>සිදුවීමේ වෙනත් වාර්තා</Link> : null}
        <Dialog>
          <DialogTrigger render={<Button variant="outline" />}>ගැටලුවක් වාර්තා කරන්න</DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>ගැටලුවක් වාර්තා කරන්න</DialogTitle>
            </DialogHeader>
            <form
              className="flex flex-col gap-3"
              onSubmit={async (event) => {
                event.preventDefault()
                try {
                  await client.complain({ entity_type: 'article', entity_id: item.id, reason })
                  toast.success('ලැබුණි')
                } catch {
                  toast.error('අසමත් විය')
                }
              }}
            >
              <Textarea value={reason} onChange={(event) => setReason(event.target.value)} required />
              <Button type="submit">යවන්න</Button>
            </form>
          </DialogContent>
        </Dialog>
      </div>
    </section>
  )
}
