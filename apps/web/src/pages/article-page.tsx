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
