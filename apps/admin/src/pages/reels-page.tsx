import { createClient, type Reel, type ReelAction, type ReelEditInput } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Clapperboard, Copy, Download, LoaderCircle, Music2, Pencil, RefreshCw, Sparkles, TrendingUp, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { formatDateTime } from '@/lib/date-time'

const client = createClient()

const statusLabel: Record<Reel['status'], string> = {
  queued: 'Queued', generating: 'Building…', ready: 'Ready for review', approved: 'Approved',
  rejected: 'Rejected', posted: 'Posted', failed: 'Failed',
}
const busy = (reel: Reel) => reel.status === 'queued' || reel.status === 'generating'

type ScreenEdit = { tag: string; headline: string; facts: string; question: string; yes_label: string; no_label: string }

function lines(value: string) {
  return value.split('\n').map((line) => line.trim()).filter(Boolean)
}

export function ReelsPage() {
  const queryClient = useQueryClient()
  const reels = useQuery({
    queryKey: ['reels'],
    queryFn: () => client.reels(),
    refetchInterval: (query) => (query.state.data?.items.some(busy) ? 5000 : 60000),
  })
  const storylines = useQuery({ queryKey: ['admin-storylines'], queryFn: () => client.adminStorylines() })
  const [picking, setPicking] = useState(false)
  const [editing, setEditing] = useState<{ reel: Reel; screen: ScreenEdit } | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['reels'] })
  const errorText = (error: unknown, fallback: string) => (error instanceof Error && error.message ? error.message : fallback)

  const create = useMutation({
    mutationFn: (storylineID?: string) => client.createReel(storylineID ? { storyline_id: storylineID } : {}),
    onSuccess: () => {
      void refresh()
      setPicking(false)
      toast.success('Building the reel. It takes a few minutes.')
    },
    onError: (error) => toast.error(errorText(error, 'Could not start the reel')),
  })
  const act = useMutation({
    mutationFn: ({ id, action }: { id: string; action: ReelAction }) => client.reelAction(id, action),
    onSuccess: (_reel, { action }) => {
      void refresh()
      toast.success({
        approve: 'Approved. Download it and post it on Facebook.', reject: 'Reel rejected', posted: 'Marked as posted',
        reopen: 'Back in review', rerender: 'Re-rendering the reel', regenerate: 'Building a fresh reel for this story',
      }[action])
    },
    onError: (error) => toast.error(errorText(error, 'Could not update the reel')),
  })
  const edit = useMutation({
    mutationFn: ({ id, body }: { id: string; body: ReelEditInput }) => client.editReel(id, body),
    onSuccess: (reel) => {
      void refresh()
      setEditing(null)
      toast.success(busy(reel) ? 'Saved. Re-rendering the video.' : 'Saved')
    },
    onError: (error) => toast.error(errorText(error, 'Could not save the reel')),
  })

  const trending = (storylines.data?.items ?? []).filter((story) => story.trending && !story.hidden)

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <Badge variant="outline" className="mb-2"><Clapperboard /> Facebook Reels</Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Reel drafts</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            Every six hours (Colombo 00:00, 06:00, 12:00, 18:00) the server picks a trending story, weighs what Sri Lankans are searching on Google, and renders a 14-second reel with a debate question. Nothing is posted until you approve it, download it and post it on the Page.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => setPicking(true)}><TrendingUp /> Choose a story</Button>
          <Button disabled={create.isPending} onClick={() => create.mutate(undefined)}>
            {create.isPending ? <LoaderCircle className="animate-spin" /> : <Sparkles />} Make a reel now
          </Button>
        </div>
      </div>

      {reels.isPending ? <Skeleton className="h-96 w-full" /> : reels.isError ? (
        <Card><CardContent className="space-y-3"><p role="alert">Could not load reel drafts.</p><Button variant="outline" onClick={() => { void reels.refetch() }}>Try again</Button></CardContent></Card>
      ) : reels.data.items.length === 0 ? (
        <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">No reels yet. The next one is built at the next six-hour mark, or make one now.</CardContent></Card>
      ) : (
        <div className="grid gap-4">
          {reels.data.items.map((reel) => (
            <ReelCard key={reel.id} reel={reel} acting={act.isPending}
              onAction={(action) => act.mutate({ id: reel.id, action })}
              onSaveCaption={(caption) => edit.mutate({ id: reel.id, body: { caption } })}
              onEdit={() => setEditing({ reel, screen: {
                tag: reel.script.tag, headline: reel.script.headline.join('\n'), facts: reel.script.facts.join('\n'),
                question: reel.script.question, yes_label: reel.script.yes_label, no_label: reel.script.no_label,
              } })} />
          ))}
        </div>
      )}

      <Dialog open={picking} onOpenChange={(open) => { if (!create.isPending) setPicking(open) }}>
        <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Make a reel for a trending story</DialogTitle>
            <DialogDescription>Pick the story yourself instead of letting the server choose.</DialogDescription>
          </DialogHeader>
          {storylines.isPending ? <Skeleton className="h-40 w-full" /> : trending.length === 0 ? (
            <p className="text-sm text-muted-foreground">No storyline is trending right now.</p>
          ) : (
            <ul className="divide-y rounded-lg border">
              {trending.map((story) => (
                <li key={story.id} className="flex items-center justify-between gap-3 p-3">
                  <div className="min-w-0">
                    <p lang="si" className="truncate font-medium">{story.title_si}</p>
                    <p className="text-xs text-muted-foreground">Trend score {story.trend_score} · {story.source_count} outlets · {story.article_count} reports</p>
                  </div>
                  <Button size="sm" disabled={create.isPending} onClick={() => create.mutate(story.id)}>Make reel</Button>
                </li>
              ))}
            </ul>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={!!editing} onOpenChange={(open) => { if (!open && !edit.isPending) setEditing(null) }}>
        <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Edit the on-screen text</DialogTitle>
            <DialogDescription>Saving re-renders the video with the same images and music (about two minutes).</DialogDescription>
          </DialogHeader>
          {editing ? (
            <form className="space-y-4" onSubmit={(event) => {
              event.preventDefault()
              const s = editing.screen
              edit.mutate({ id: editing.reel.id, body: {
                tag: s.tag.trim(), headline: lines(s.headline), facts: lines(s.facts),
                question: s.question.trim(), yes_label: s.yes_label.trim(), no_label: s.no_label.trim(),
              } })
            }}>
              {([
                ['tag', 'Label (top right)', 1],
                ['headline', 'Headline (one bar per line, up to 3 lines)', 3],
                ['facts', 'Key facts (one per line, up to 3)', 4],
                ['question', 'Debate question', 2],
              ] as const).map(([field, label, rows]) => (
                <div key={field}>
                  <Label htmlFor={`reel-${field}`}>{label}</Label>
                  <Textarea id={`reel-${field}`} lang="si" rows={rows} className="mt-2" value={editing.screen[field]}
                    onChange={(event) => setEditing({ ...editing, screen: { ...editing.screen, [field]: event.target.value } })} />
                </div>
              ))}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label htmlFor="reel-yes">❤️ Love answer</Label>
                  <Input id="reel-yes" lang="si" className="mt-2" value={editing.screen.yes_label}
                    onChange={(event) => setEditing({ ...editing, screen: { ...editing.screen, yes_label: event.target.value } })} />
                </div>
                <div>
                  <Label htmlFor="reel-no">👍 Like answer</Label>
                  <Input id="reel-no" lang="si" className="mt-2" value={editing.screen.no_label}
                    onChange={(event) => setEditing({ ...editing, screen: { ...editing.screen, no_label: event.target.value } })} />
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" disabled={edit.isPending} onClick={() => setEditing(null)}>Cancel</Button>
                <Button type="submit" disabled={edit.isPending}>{edit.isPending ? <LoaderCircle className="animate-spin" /> : null} Save and re-render</Button>
              </DialogFooter>
            </form>
          ) : null}
        </DialogContent>
      </Dialog>
    </section>
  )
}

function ReelCard({ reel, acting, onAction, onSaveCaption, onEdit }: {
  reel: Reel; acting: boolean; onAction: (action: ReelAction) => void; onSaveCaption: (caption: string) => void; onEdit: () => void
}) {
  const [caption, setCaption] = useState(reel.caption)
  const [savedCaption, setSavedCaption] = useState(reel.caption)
  if (reel.caption !== savedCaption) {
    setSavedCaption(reel.caption)
    setCaption(reel.caption)
  }
  const chosen = reel.script.questions?.find((q) => q.question === reel.script.question)
  const video = reel.has_video ? `${client.reelFileURL(reel.id, 'video')}?v=${encodeURIComponent(reel.rendered_at ?? '')}` : null
  const working = busy(reel)

  return (
    <Card className="gap-0 py-0">
      <CardHeader className="py-4">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={reel.status === 'ready' ? 'default' : reel.status === 'failed' ? 'destructive' : 'secondary'}>
            {working ? <LoaderCircle className="animate-spin" /> : null}{statusLabel[reel.status]}
          </Badge>
          <span className="text-xs text-muted-foreground">
            {reel.trigger === 'schedule' ? 'Scheduled' : 'Requested'} {formatDateTime(reel.created_at)}
            {reel.rendered_at ? ` · Rendered ${formatDateTime(reel.rendered_at)}` : ''}
          </span>
        </div>
        <CardTitle lang="si" className="mt-2 leading-relaxed">{reel.storyline_title || reel.title || 'Choosing a trending story…'}</CardTitle>
        {reel.trend.reason ? (
          <CardDescription className="leading-relaxed">
            Trend score {reel.trend.score}{reel.trend.google_trends.length ? ` · Google searches: ${reel.trend.google_trends.join(', ')}` : ''} · {reel.trend.reason}
          </CardDescription>
        ) : null}
      </CardHeader>
      <CardContent className="grid gap-6 border-t p-4 lg:grid-cols-[minmax(0,320px)_1fr]">
        <div className="mx-auto w-full max-w-[320px]">
          {video ? (
            <video key={video} src={video} controls playsInline preload="metadata" className="aspect-[9/16] w-full rounded-xl bg-black"
              poster={`${client.reelFileURL(reel.id, 'cover')}?v=${encodeURIComponent(reel.rendered_at ?? '')}`} />
          ) : (
            <div className="flex aspect-[9/16] w-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed p-6 text-center text-sm text-muted-foreground">
              {working ? <><LoaderCircle className="size-6 animate-spin" />Writing the script, finding photos, drawing images and rendering. This takes a few minutes.</> : reel.error ?? 'No video'}
            </div>
          )}
        </div>
        <div className="flex min-w-0 flex-col gap-4">
          {reel.error && reel.status === 'failed' ? <p role="alert" className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">{reel.error}</p> : null}
          {reel.script.question ? (
            <div className="rounded-lg border p-3">
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Debate question</p>
              <p lang="si" className="mt-1 font-medium">{reel.script.question}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                ❤️ {reel.script.yes_label} · 👍 {reel.script.no_label}{chosen ? ` · model expects ${chosen.expected_yes_percent}% yes` : ''}
              </p>
            </div>
          ) : null}
          <div>
            <Label htmlFor={`caption-${reel.id}`}>Facebook description</Label>
            <Textarea id={`caption-${reel.id}`} lang="si" rows={10} className="mt-2 leading-6" value={caption}
              disabled={working || reel.status === 'posted'} onChange={(event) => setCaption(event.target.value)} />
            <div className="mt-2 flex flex-wrap gap-2">
              <Button size="sm" variant="outline" onClick={() => { void navigator.clipboard.writeText(caption).then(() => toast.success('Description copied')) }}><Copy /> Copy description</Button>
              {caption !== reel.caption ? <Button size="sm" variant="outline" onClick={() => onSaveCaption(caption)}>Save description</Button> : null}
            </div>
          </div>
          <div className="text-xs leading-5 text-muted-foreground">
            {reel.assets.music ? <p><Music2 className="mr-1 inline size-3" />{reel.assets.music.title}{reel.assets.music.generated ? '' : ` (from ${reel.assets.music.start}s)`}</p> : null}
            {reel.assets.photos.map((photo) => (
              <p key={photo.key}>{photo.kind === 'ai' ? 'AI image (labelled in the reel)' : photo.source_url ? <a className="underline" href={photo.source_url} target="_blank" rel="noreferrer">{photo.credit}</a> : photo.credit}</p>
            ))}
          </div>
          <div className="mt-auto flex flex-wrap gap-2 border-t pt-4">
            {reel.status === 'ready' || reel.status === 'rejected' ? <Button size="sm" disabled={acting} onClick={() => onAction('approve')}><Check /> Approve</Button> : null}
            {reel.has_video ? <Button size="sm" variant="outline" nativeButton={false} render={<a href={client.reelFileURL(reel.id, 'video', true)} />}><Download /> Download video</Button> : null}
            {reel.status === 'approved' ? <Button size="sm" disabled={acting} onClick={() => onAction('posted')}>Mark as posted</Button> : null}
            {!working && reel.status !== 'posted' && reel.script.question ? <Button size="sm" variant="outline" onClick={onEdit}><Pencil /> Edit text</Button> : null}
            {!working && reel.has_video ? <Button size="sm" variant="outline" disabled={acting} onClick={() => onAction('rerender')}><RefreshCw /> Re-render</Button> : null}
            {!working && reel.storyline_id ? <Button size="sm" variant="outline" disabled={acting} onClick={() => onAction('regenerate')}><Sparkles /> New version</Button> : null}
            {reel.status === 'ready' || reel.status === 'approved' || reel.status === 'failed' ? <Button size="sm" variant="ghost" disabled={acting} onClick={() => onAction('reject')}><X /> Reject</Button> : null}
            {reel.status === 'posted' || reel.status === 'approved' ? <Button size="sm" variant="ghost" disabled={acting} onClick={() => onAction('reopen')}>Back to review</Button> : null}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
