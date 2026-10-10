import type { SocialPost } from '@snap/api-client'

import { Badge } from '@/components/ui/badge'
import { formatDateTime } from '@/lib/date-time'

export function ReviewBadge({ post }: { post: SocialPost }) {
  if (post.review_status === 'passed') return <Badge className="bg-emerald-600 text-white hover:bg-emerald-600">AI review passed{post.review_score != null ? ` · ${post.review_score}` : ''}</Badge>
  if (post.review_status === 'failed') return <Badge variant="destructive">AI review failed{post.review_score != null ? ` · ${post.review_score}` : ''}</Badge>
  if (post.review_status === 'unavailable') return <Badge variant="outline">AI review unavailable</Badge>
  return <Badge variant="outline">Not reviewed</Badge>
}

export function ReviewPanel({ post }: { post: SocialPost }) {
  if (post.review_status === 'not_reviewed') return null
  const tone = post.review_status === 'passed' ? 'border-emerald-500/30 bg-emerald-500/5' : post.review_status === 'failed' ? 'border-destructive/30 bg-destructive/5' : 'bg-muted/40'
  return <div className={`space-y-2 rounded-xl border p-3 text-sm ${tone}`}>
    <p className="font-medium">AI quality review{post.reviewed_at ? <span className="font-normal text-muted-foreground"> · {formatDateTime(post.reviewed_at)}</span> : null}</p>
    {post.review_summary ? <p className="text-muted-foreground">{post.review_summary}</p> : null}
    {post.review_issues?.length ? <ul className="space-y-1">{post.review_issues.map((issue, i) => <li key={i} className="flex gap-2"><Badge variant={issue.severity === 'minor' ? 'secondary' : 'destructive'} className="shrink-0 capitalize">{issue.severity}</Badge><span><span className="text-muted-foreground">{issue.field}:</span> {issue.message}</span></li>)}</ul> : null}
  </div>
}
