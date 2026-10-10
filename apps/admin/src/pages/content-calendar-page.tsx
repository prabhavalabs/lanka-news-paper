import { createClient, type SocialPost, type SocialPostStatus } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { CalendarDays, ChevronLeft, ChevronRight, ExternalLink, ListChecks } from 'lucide-react'
import { Fragment, useMemo, useState } from 'react'
import { Link } from 'react-router'

import { PlanCell, PublishPlanPanel, ReviewBadge, ReviewPanel, triggerLabel } from '@/components/social-review'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatDateTime } from '@/lib/date-time'
import { safeSocialLink } from '@/lib/social-publishing'

const client = createClient()
const weekdays = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const statusStyle: Record<SocialPostStatus, { label: string; dot: string }> = {
  published: { label: 'Published', dot: 'bg-emerald-500' },
  publishing: { label: 'Publishing', dot: 'bg-sky-500' },
  approved: { label: 'Scheduled', dot: 'bg-sky-500' },
  pending_review: { label: 'Waiting for review', dot: 'bg-amber-500' },
  needs_attention: { label: 'Needs attention', dot: 'bg-red-500' },
  failed: { label: 'Failed', dot: 'bg-red-500' },
  rejected: { label: 'Rejected', dot: 'bg-zinc-400' },
  skipped: { label: 'Not selected', dot: 'bg-zinc-300' },
}

// The calendar places each post on the day it published, else when it is scheduled, else when it was drafted.
function calendarTime(post: SocialPost) {
  return new Date(post.published_at ?? post.scheduled_for ?? post.created_at)
}
function dayKey(date: Date) {
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
}
function monthGrid(month: Date) {
  const first = new Date(month.getFullYear(), month.getMonth(), 1)
  const start = new Date(first)
  start.setDate(first.getDate() - ((first.getDay() + 6) % 7))
  return Array.from({ length: 42 }, (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i))
}

export function ContentCalendarPage() {
  const [month, setMonth] = useState(() => new Date(new Date().getFullYear(), new Date().getMonth(), 1))
  const [view, setView] = useState<'calendar' | 'log'>('calendar')
  const [channel, setChannel] = useState('all')
  const [status, setStatus] = useState<'all' | SocialPostStatus>('all')
  const [selected, setSelected] = useState<SocialPost | null>(null)
  const days = useMemo(() => monthGrid(month), [month])
  const from = days[0]!
  const to = new Date(days[41]!.getFullYear(), days[41]!.getMonth(), days[41]!.getDate() + 1)
  const channels = useQuery({ queryKey: ['social', 'channels'], queryFn: () => client.socialChannels() })
  const content = useQuery({
    queryKey: ['social', 'content', from.toISOString(), to.toISOString(), channel, status],
    queryFn: () => client.socialContent({ from: from.toISOString(), to: to.toISOString(), ...(channel !== 'all' ? { channel } : {}), ...(status !== 'all' ? { status } : {}) }),
    refetchInterval: () => (document.visibilityState === 'visible' ? 60_000 : false),
  })
  const items = useMemo(() => content.data?.items ?? [], [content.data])
  const byDay = useMemo(() => {
    const map = new Map<string, SocialPost[]>()
    for (const post of items) {
      const key = dayKey(calendarTime(post))
      map.set(key, [...(map.get(key) ?? []), post])
    }
    for (const list of map.values()) list.sort((a, b) => calendarTime(a).getTime() - calendarTime(b).getTime())
    return map
  }, [items])
  const inMonth = items.filter((post) => calendarTime(post).getMonth() === month.getMonth())
  const totals = (['published', 'approved', 'pending_review', 'failed', 'skipped'] as const).map((key) => ({
    key, label: key === 'failed' ? 'Failed / attention' : statusStyle[key].label,
    count: inMonth.filter((post) => post.status === key || (key === 'failed' && post.status === 'needs_attention') || (key === 'approved' && post.status === 'publishing')).length,
  }))
  const today = dayKey(new Date())
  const shift = (delta: number) => setMonth(new Date(month.getFullYear(), month.getMonth() + delta, 1))

  return <section className="flex min-w-0 flex-col gap-6">
    <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
      <div>
        <Badge variant="outline" className="mb-2"><CalendarDays />Content</Badge>
        <h1 className="font-heading text-2xl font-semibold tracking-tight">Content calendar</h1>
        <p className="mt-1 max-w-2xl text-sm text-muted-foreground">Everything prepared for social channels: what was published, what is scheduled, what is waiting for an editor and what was not selected.</p>
      </div>
      <Button variant="outline" nativeButton={false} render={<Link to="/social" />}><ListChecks />Review queue</Button>
    </div>

    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      {(channels.data?.items ?? []).map((item) => <Card key={item.key} size="sm"><CardContent className="flex items-center justify-between gap-3">
        <span className="text-sm font-medium">{item.name}</span>
        <Badge variant={item.status === 'connected' ? 'default' : 'outline'} className="capitalize">{item.status === 'planned' ? 'Coming later' : item.status.replaceAll('_', ' ')}</Badge>
      </CardContent></Card>)}
      {channels.isPending ? <Skeleton className="h-16 sm:col-span-2 xl:col-span-5" /> : null}
    </div>

    <Card>
      <CardHeader className="gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Button variant="outline" size="icon" aria-label="Previous month" onClick={() => shift(-1)}><ChevronLeft /></Button>
            <CardTitle className="min-w-40 text-center">{month.toLocaleDateString('en', { month: 'long', year: 'numeric' })}</CardTitle>
            <Button variant="outline" size="icon" aria-label="Next month" onClick={() => shift(1)}><ChevronRight /></Button>
            <Button variant="ghost" size="sm" onClick={() => setMonth(new Date(new Date().getFullYear(), new Date().getMonth(), 1))}>Today</Button>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div role="tablist" aria-label="View" className="flex gap-1 rounded-lg border p-1">
              {(['calendar', 'log'] as const).map((key) => <Button key={key} role="tab" aria-selected={view === key} size="sm" variant={view === key ? 'default' : 'ghost'} onClick={() => setView(key)}>{key === 'calendar' ? 'Calendar' : 'Content log'}</Button>)}
            </div>
            <Select value={channel} onValueChange={(value) => value && setChannel(value)}>
              <SelectTrigger aria-label="Channel" className="w-40"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="all">All channels</SelectItem>{(channels.data?.items ?? []).filter((item) => item.available).map((item) => <SelectItem key={item.key} value={item.key}>{item.name}</SelectItem>)}</SelectContent>
            </Select>
            <Select value={status} onValueChange={(value) => value && setStatus(value as typeof status)}>
              <SelectTrigger aria-label="Status" className="w-48"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="all">All statuses</SelectItem>{(Object.keys(statusStyle) as SocialPostStatus[]).map((key) => <SelectItem key={key} value={key}>{statusStyle[key].label}</SelectItem>)}</SelectContent>
            </Select>
          </div>
        </div>
        <CardDescription className="flex flex-wrap gap-x-5 gap-y-1">{totals.map((item) => <span key={item.key}>{item.label}: <span className="font-medium text-foreground">{item.count}</span></span>)}</CardDescription>
      </CardHeader>
      <CardContent>
        {content.isPending ? <Skeleton className="h-96" /> : content.isError ? <div className="space-y-2 rounded-xl border p-5"><p role="alert" className="text-sm">Could not load the content calendar.</p><Button variant="outline" size="sm" onClick={() => void content.refetch()}>Try again</Button></div> : view === 'calendar' ? <div className="overflow-x-auto">
          <div className="grid min-w-[720px] grid-cols-7 border-l border-t">
            {weekdays.map((day) => <div key={day} className="border-b border-r bg-muted/40 px-2 py-1.5 text-xs font-medium text-muted-foreground">{day}</div>)}
            {days.map((day) => {
              const posts = byDay.get(dayKey(day)) ?? []
              const outside = day.getMonth() !== month.getMonth()
              return <div key={day.toISOString()} className={`min-h-28 border-b border-r p-1.5 ${outside ? 'bg-muted/20 text-muted-foreground' : ''}`}>
                <div className={`mb-1 text-xs ${dayKey(day) === today ? 'inline-grid size-6 place-items-center rounded-full bg-primary text-primary-foreground' : ''}`}>{day.getDate()}</div>
                <div className="space-y-1">
                  {posts.slice(0, 3).map((post) => <button key={post.id} type="button" onClick={() => setSelected(post)} className="flex w-full items-center gap-1.5 rounded-md px-1 py-0.5 text-left text-xs hover:bg-muted" title={post.headline_si}>
                    <span className={`size-2 shrink-0 rounded-full ${statusStyle[post.status].dot}`} />
                    <span className="shrink-0 tabular-nums text-muted-foreground">{calendarTime(post).toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit', hour12: false })}</span>
                    <span className="truncate" lang="si">{post.headline_si || post.subject_title}</span>
                  </button>)}
                  {posts.length > 3 ? <button type="button" className="px-1 text-xs text-muted-foreground underline-offset-2 hover:underline" onClick={() => { setView('log') }}>+{posts.length - 3} more</button> : null}
                </div>
              </div>
            })}
          </div>
          <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">{(['published', 'approved', 'pending_review', 'failed', 'skipped'] as const).map((key) => <span key={key} className="flex items-center gap-1.5"><span className={`size-2 rounded-full ${statusStyle[key].dot}`} />{statusStyle[key].label}</span>)}</div>
        </div> : !items.length ? <p className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">No content in this period.</p> : <Table>
          <TableHeader><TableRow><TableHead>Date</TableHead><TableHead>Channel</TableHead><TableHead>Status</TableHead><TableHead title="When the post goes out, or why it will not">Publishes</TableHead><TableHead>Story</TableHead><TableHead>AI review</TableHead><TableHead title="How the story was picked">Found by</TableHead><TableHead>Sources</TableHead><TableHead /></TableRow></TableHeader>
          <TableBody>{items.map((post) => {
            const permalink = post.fb_permalink ? safeSocialLink(post.fb_permalink) : undefined
            return <TableRow key={post.id} className="cursor-pointer" onClick={() => setSelected(post)}>
              <TableCell className="whitespace-nowrap text-xs">{formatDateTime(calendarTime(post).toISOString())}</TableCell>
              <TableCell className="capitalize">{post.channel}</TableCell>
              <TableCell><span className="flex items-center gap-1.5 whitespace-nowrap text-xs"><span className={`size-2 rounded-full ${statusStyle[post.status].dot}`} />{statusStyle[post.status].label}</span></TableCell>
              <TableCell><PlanCell post={post} /></TableCell>
              <TableCell className="max-w-80"><p className="truncate" lang="si">{post.headline_si || post.subject_title}</p><p className="truncate text-xs text-muted-foreground">{post.category}</p></TableCell>
              <TableCell><ReviewBadge post={post} /></TableCell>
              <TableCell className="whitespace-nowrap text-xs">{triggerLabel[post.trigger] ?? post.trigger}</TableCell>
              <TableCell className="max-w-56"><p className="truncate text-xs">{(post.source_names ?? []).join(', ')}</p></TableCell>
              <TableCell>{permalink ? <Button size="sm" variant="ghost" nativeButton={false} render={<a href={permalink} target="_blank" rel="noreferrer" onClick={(event) => event.stopPropagation()} />}><ExternalLink />Open</Button> : null}</TableCell>
            </TableRow>
          })}</TableBody>
        </Table>}
      </CardContent>
    </Card>

    <Dialog open={!!selected} onOpenChange={(open) => { if (!open) setSelected(null) }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
        {selected ? <ContentDetail post={selected} /> : null}
      </DialogContent>
    </Dialog>
  </section>
}

function ContentDetail({ post }: { post: SocialPost }) {
  const image = post.image_url ? `${post.image_url}${post.image_url.includes('?') ? '&' : '?'}v=${encodeURIComponent(post.image_rendered_at ?? post.updated_at)}` : ''
  const permalink = post.fb_permalink ? safeSocialLink(post.fb_permalink) : undefined
  const timeline: [string, string | null][] = [['Drafted', post.created_at], ['Reviewed by AI', post.reviewed_at], ['Approved', post.approved_at], ['Earliest publish time', post.scheduled_for], ['Published', post.published_at], ['Rejected', post.rejected_at]]
  return <>
    <DialogHeader>
      <DialogTitle lang="si" className="leading-relaxed">{post.headline_si || post.subject_title}</DialogTitle>
      <DialogDescription className="flex flex-wrap items-center gap-2"><span className="capitalize">{post.channel}</span>·<span>{statusStyle[post.status].label}</span>·<span>{triggerLabel[post.trigger] ?? post.trigger}</span></DialogDescription>
    </DialogHeader>
    <div className="grid gap-4 sm:grid-cols-[180px_minmax(0,1fr)]">
      {image ? <a href={image} target="_blank" rel="noreferrer" className="self-start overflow-hidden rounded-xl border bg-muted"><img src={image} alt="" className="aspect-[4/5] w-full object-contain" /></a> : <div className="grid aspect-[4/5] place-items-center rounded-xl border bg-muted text-xs text-muted-foreground">No image</div>}
      <div className="min-w-0 space-y-3 text-sm">
        <PublishPlanPanel post={post} />
        <div className="flex flex-wrap gap-2"><ReviewBadge post={post} /><Badge variant="outline">Selection score {post.score}/100</Badge></div>
        <p className="whitespace-pre-wrap break-words rounded-xl bg-muted/50 p-3 leading-7" lang="si">{post.caption}</p>
        <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-1 text-xs">{timeline.filter(([, value]) => value).map(([label, value]) => <Fragment key={label}><dt className="text-muted-foreground">{label}</dt><dd>{formatDateTime(value)}</dd></Fragment>)}
          <dt className="text-muted-foreground">Sources</dt><dd>{(post.source_names ?? []).join(', ')}</dd>
        </dl>
        <ReviewPanel post={post} />
        {post.last_error ? <p role="alert" className="break-words rounded-xl border border-destructive/30 bg-destructive/5 p-3">{post.last_error}</p> : null}
        <div className="flex flex-wrap gap-2">{permalink ? <Button size="sm" variant="outline" nativeButton={false} render={<a href={permalink} target="_blank" rel="noreferrer" />}><ExternalLink />View on Facebook</Button> : null}<Button size="sm" variant="outline" nativeButton={false} render={<Link to="/social" />}>Open review queue</Button></div>
      </div>
    </div>
  </>
}
