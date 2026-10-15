import { createClient, type Storyline, type StorylinePatch } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { CreateSocialPost } from '@/components/create-social-post'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { formatDateTime } from '@/lib/date-time'

const client = createClient()
export function TrendingPage() {
  const cache = useQueryClient()
  const me = useQuery({ queryKey: ['me'], queryFn: () => client.me() })
  const administrator = me.data?.role === 'administrator'
  const query = useQuery({ queryKey: ['storylines'], queryFn: () => client.adminStorylines(), enabled: administrator, refetchInterval: () => document.visibilityState === 'visible' ? 30_000 : false })
  const refresh = useMutation({ mutationFn: () => client.refreshStorylines(), onSuccess: () => { toast.success('Storyline refresh queued'); void cache.invalidateQueries({ queryKey: ['storylines'] }) }, onError: () => toast.error('Could not queue refresh') })
  if (me.isPending) return <Skeleton className="h-48 w-full" />
  if (!administrator) return <p className="p-6 text-sm text-muted-foreground">Trending management is available to administrators only.</p>
  return <section className="flex min-w-0 flex-col gap-6">
    <div className="flex flex-wrap items-start justify-between gap-4"><div><h1 className="text-2xl font-semibold">Trending</h1><p className="mt-1 text-sm text-muted-foreground">Running stories across distinct news developments.</p></div><Button onClick={() => refresh.mutate()} disabled={refresh.isPending}><RefreshCw />{refresh.isPending ? 'Queuing…' : 'Refresh now'}</Button></div>
    {query.isPending ? <Skeleton className="h-48 w-full" /> : query.isError ? <div role="alert"><p>Could not load storylines.</p><Button variant="outline" onClick={() => void query.refetch()}>Try again</Button></div> : query.data?.items.length ? query.data.items.map((story) => <StorylineCard key={story.id} story={story} />) : <p className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">No running stories yet. Refresh now to group recent coverage.</p>}
  </section>
}

function StorylineCard({ story }: { story: Storyline }) {
  const cache = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [title, setTitle] = useState(story.title_si)
  const [summary, setSummary] = useState(story.summary_si)
  const [eventID, setEventID] = useState('')
  const invalidate = () => cache.invalidateQueries({ queryKey: ['storylines'] })
  const update = useMutation({ mutationFn: (patch: StorylinePatch) => client.updateStoryline(story.id, patch), onSuccess: () => { setEditing(false); void invalidate() }, onError: () => toast.error('Could not update storyline') })
  const membership = useMutation({ mutationFn: ({ id, remove }: { id: string; remove: boolean }) => remove ? client.removeStorylineEvent(story.id, id) : client.addStorylineEvent(story.id, id), onSuccess: () => { setEventID(''); void invalidate() }, onError: () => toast.error('Could not change membership. An event can belong to only one storyline.') })
  const m = story.metrics
  return <Card className="min-w-0">
    <CardHeader><div className="flex flex-wrap items-center gap-2"><Badge variant={story.trending ? 'default' : 'secondary'}>{story.trending ? 'Trending' : story.status}</Badge><Badge variant="outline">Score {story.trend_score}</Badge>{story.pinned && <Badge variant="secondary">Pinned</Badge>}{story.hidden && <Badge variant="secondary">Hidden</Badge>}</div><CardTitle className="mt-2 leading-relaxed">{story.title_si}</CardTitle><CardDescription className="leading-relaxed">{story.summary_si}</CardDescription></CardHeader>
    <CardContent className="space-y-5">
      <dl className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">{[['Reports / 3h', m.articles_3h], ['Reports / 6h', m.articles_6h], ['Reports / 24h', m.articles_24h], ['Outlets / 24h', m.sources_24h], ['Events / 24h', m.events_24h], ['Baseline / day', m.baseline_daily?.toFixed(1)], ['Acceleration', m.acceleration?.toFixed(2)], ['Last activity', formatDateTime(story.last_activity_at)]].map(([label, value]) => <div key={label}><dt className="text-muted-foreground">{label}</dt><dd className="mt-1 font-medium tabular-nums">{value ?? '—'}</dd></div>)}</dl>
      <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={update.isPending} onClick={() => update.mutate({ pinned: !story.pinned })}>{story.pinned ? 'Unpin' : 'Pin'}</Button><Button variant="outline" disabled={update.isPending} onClick={() => update.mutate({ hidden: !story.hidden })}>{story.hidden ? 'Unhide' : 'Hide'}</Button><Button variant="outline" onClick={() => { setTitle(story.title_si); setSummary(story.summary_si); setEditing(!editing) }}>Rename / edit</Button><Button variant="outline" disabled={update.isPending} onClick={() => update.mutate({ status: story.status === 'active' ? 'archived' : 'active' })}>{story.status === 'active' ? 'Archive' : 'Activate'}</Button>{story.latest_event && <CreateSocialPost subject={{ event_id: story.latest_event.id }} />}</div>
      {editing && <form className="space-y-3 rounded-lg border p-4" onSubmit={(e) => { e.preventDefault(); if (title.trim() && Array.from(title.trim()).length <= 120 && Array.from(summary).length <= 400) update.mutate({ title_si: title.trim(), summary_si: summary }) }}><Label htmlFor={`title-${story.id}`}>Sinhala title (renaming locks it against automatic changes)</Label><Input id={`title-${story.id}`} value={title} onChange={(e) => setTitle(e.target.value)} required /><Label htmlFor={`summary-${story.id}`}>Summary</Label><Textarea id={`summary-${story.id}`} value={summary} onChange={(e) => setSummary(e.target.value)} /><p className="text-xs text-muted-foreground">Title {Array.from(title).length}/120 · Summary {Array.from(summary).length}/400</p><div className="flex gap-2"><Button disabled={update.isPending || !title.trim() || Array.from(title).length > 120 || Array.from(summary).length > 400}>Save</Button><Button type="button" variant="outline" onClick={() => setEditing(false)}>Cancel</Button></div></form>}
      <details><summary className="cursor-pointer text-sm font-medium">Member events ({story.event_count})</summary><ol className="mt-3 space-y-3 border-l pl-4">{story.events?.map((event) => <li key={event.id} className="space-y-1"><div className="flex flex-wrap items-start justify-between gap-2"><div className="min-w-0 flex-1"><p className="text-sm font-medium leading-relaxed">{event.display_title}</p><p className="mt-1 text-xs text-muted-foreground">{formatDateTime(event.time)} · {event.source_count} outlets</p></div><Button size="sm" variant="ghost" disabled={membership.isPending} onClick={() => membership.mutate({ id: event.id, remove: true })}>Remove</Button></div>{event.summary && <p className="text-sm leading-relaxed text-muted-foreground">{event.summary}</p>}</li>)}</ol><form className="mt-4 flex flex-wrap gap-2" onSubmit={(e) => { e.preventDefault(); if (eventID.trim()) membership.mutate({ id: eventID.trim(), remove: false }) }}><Label htmlFor={`event-${story.id}`} className="w-full">Add event by ID</Label><Input id={`event-${story.id}`} value={eventID} onChange={(e) => setEventID(e.target.value)} className="min-w-0 flex-1" placeholder="Event UUID" required /><Button variant="outline" disabled={membership.isPending}>Add event</Button></form></details>
    </CardContent>
  </Card>
}
