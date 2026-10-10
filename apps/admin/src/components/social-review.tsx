import type { SocialPost } from '@snap/api-client'

import { Badge } from '@/components/ui/badge'
import { formatDateTime } from '@/lib/date-time'

// How a post was found: the 15-minute selection job, a breaking story arriving, or an editor.
export const triggerLabel: Record<string, string> = { scheduled: 'Automatic check (every 15 min)', realtime: 'Breaking, on arrival', manual: 'Created by an editor' }

export function ReviewBadge({ post }: { post: SocialPost }) {
  if (post.review_status === 'passed') return <Badge className="bg-emerald-600 text-white hover:bg-emerald-600">AI review passed{post.review_score != null ? ` · ${post.review_score}` : ''}</Badge>
  if (post.review_status === 'failed') return <Badge variant="destructive">AI review failed{post.review_score != null ? ` · ${post.review_score}` : ''}</Badge>
  if (post.review_status === 'unavailable') return <Badge variant="outline">AI review unavailable</Badge>
  return <Badge variant="outline">Not reviewed</Badge>
}

const reviewMeaning: Record<string, string> = {
  passed: 'Meets the editorial standards (accuracy, attribution, neutrality, Sinhala and safety). It can publish automatically when auto-publish is on.',
  failed: 'The AI editor found a blocker or major problem, or scored it below the minimum. It is never published automatically. Fix the issues and run the review again, or approve it yourself if they are acceptable.',
  unavailable: 'The AI review could not run, so it is never published automatically. Run the review again or approve it yourself.',
}

export function ReviewPanel({ post }: { post: SocialPost }) {
  if (post.review_status === 'not_reviewed') return null
  const tone = post.review_status === 'passed' ? 'border-emerald-500/30 bg-emerald-500/5' : post.review_status === 'failed' ? 'border-destructive/30 bg-destructive/5' : 'bg-muted/40'
  return <div className={`space-y-2 rounded-xl border p-3 text-sm ${tone}`}>
    <p className="font-medium">AI quality review{post.reviewed_at ? <span className="font-normal text-muted-foreground"> · {formatDateTime(post.reviewed_at)}</span> : null}</p>
    {reviewMeaning[post.review_status] ? <p className="text-xs">{reviewMeaning[post.review_status]}</p> : null}
    {post.review_revisions > 0 ? <p className="text-xs text-muted-foreground">The first draft failed review, so the AI rewrote it to fix the issues{post.review_revisions > 1 ? ` (${post.review_revisions} times)` : ''}. This is the review of the rewritten post.</p> : null}
    {post.review_summary ? <p className="text-muted-foreground">{post.review_summary}</p> : null}
    {post.review_issues?.length ? <ul className="space-y-1">{post.review_issues.map((issue, i) => <li key={i} className="flex gap-2"><Badge variant={issue.severity === 'minor' ? 'secondary' : 'destructive'} className="shrink-0 capitalize">{issue.severity}</Badge><span><span className="text-muted-foreground">{issue.field}:</span> {issue.message}</span></li>)}</ul> : null}
  </div>
}

const planLabel: Record<string, string> = {
  published: 'Published', publishing: 'Publishing now', scheduled: 'Will publish', paused: 'Paused', needs_editor: 'Needs an editor', not_publishing: 'Will not publish',
}

/** Short "when does this go out" text for tables. */
export function PlanCell({ post }: { post: SocialPost }) {
  const plan = post.plan
  if (!plan) return null
  if (plan.at && (plan.state === 'scheduled' || plan.state === 'published')) return <span className="whitespace-nowrap text-xs" title={plan.note}>{plan.estimated ? '≈ ' : ''}{formatDateTime(plan.at)}</span>
  return <span className={`text-xs ${plan.state === 'needs_editor' ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'}`} title={plan.note}>{planLabel[plan.state]}</span>
}

/** Full explanation of whether and when a post goes out. */
export function PublishPlanPanel({ post }: { post: SocialPost }) {
  const plan = post.plan
  if (!plan) return null
  const tone = plan.state === 'needs_editor' ? 'border-amber-500/40 bg-amber-500/5' : plan.state === 'scheduled' || plan.state === 'publishing' ? 'border-sky-500/30 bg-sky-500/5' : plan.state === 'published' ? 'border-emerald-500/30 bg-emerald-500/5' : 'bg-muted/40'
  return <div className={`space-y-1 rounded-xl border p-3 text-sm ${tone}`}>
    <p className="font-medium">{planLabel[plan.state]}{plan.at ? <span className="font-normal"> · {plan.estimated ? 'about ' : ''}{formatDateTime(plan.at)}</span> : null}</p>
    {plan.note ? <p className="text-xs text-muted-foreground">{plan.note}</p> : null}
  </div>
}
