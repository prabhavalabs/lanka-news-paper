import {
  createClient,
  type AdminFrontPage,
  type FrontPagePlacementInput,
  type FrontPageSlot,
  type FrontPageStory,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, ExternalLink, LayoutTemplate, LoaderCircle, RefreshCw, Save, Search, Undo2, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/date-time'
import { publicURL } from '@/lib/public-site'

const client = createClient()

const slots: { slot: FrontPageSlot; title: string; hint: string; limit: number }[] = [
  { slot: 'lead', title: 'Lead story', hint: 'The main story at the top of the page.', limit: 1 },
  { slot: 'top', title: 'Top stories', hint: 'Shown under the lead. Open places fill automatically up to six.', limit: 12 },
  { slot: 'more', title: 'More stories', hint: 'The shorter list. Open places fill automatically up to twelve.', limit: 24 },
]

type Draft = { slot: FrontPageSlot; story: FrontPageStory; note: string; expires_at: string }

const key = (story: FrontPageStory) => `${story.kind}:${story.id}`

function fromServer(data: AdminFrontPage): Draft[] {
  return data.placements.map((placement) => ({
    slot: placement.slot,
    story: placement.story,
    note: placement.note ?? '',
    expires_at: placement.expires_at ? toLocalInput(placement.expires_at) : '',
  }))
}

function toLocalInput(value: string) {
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

export function FrontPagePage() {
  const queryClient = useQueryClient()
  const me = useQuery({ queryKey: ['me'], queryFn: () => client.me() })
  const allowed = me.data?.role === 'administrator' || me.data?.role === 'chief_editor'
  const view = useQuery({ queryKey: ['front-page'], queryFn: () => client.adminFrontPage(), enabled: allowed })
  const [draft, setDraft] = useState<Draft[] | null>(null)
  const [search, setSearch] = useState('')

  const current = draft ?? (view.data ? fromServer(view.data) : [])
  const dirty = draft !== null
  const placed = useMemo(() => new Set(current.map((item) => key(item.story))), [current])

  const save = useMutation({
    mutationFn: () => {
      const placements: FrontPagePlacementInput[] = current.map((item) => ({
        slot: item.slot,
        kind: item.story.kind,
        id: item.story.id,
        note: item.note.trim() || null,
        expires_at: item.expires_at ? new Date(item.expires_at).toISOString() : null,
      }))
      return client.updateFrontPage({ placements })
    },
    onSuccess: (next) => {
      queryClient.setQueryData(['front-page'], next)
      setDraft(null)
      toast.success('Front page published. Readers see it within 30 seconds.')
    },
    onError: (error) => toast.error(error instanceof Error && error.message ? error.message : 'Could not save the front page'),
  })
  const rebuild = useMutation({
    mutationFn: () => client.rebuildFrontPage(),
    onSuccess: () => toast.success('Rebuilding the edition. The AI desk writes and illustrates stories, so this takes about 10–20 minutes.'),
    onError: () => toast.error('Could not queue a rebuild'),
  })
  const toggleSource = useMutation({
    mutationFn: ({ id, front_page }: { id: string; front_page: boolean }) => client.setFrontPageSource(id, { front_page }),
    onSuccess: (source) => {
      void queryClient.invalidateQueries({ queryKey: ['front-page'] })
      toast.success(source.front_page ? `${source.name} can appear on the front page` : `${source.name} is kept off automatic front-page slots`)
    },
    onError: () => toast.error('Could not update the source'),
  })

  const edit = (next: Draft[]) => setDraft(next)
  const place = (story: FrontPageStory, slot: FrontPageSlot) => {
    let next = current.filter((item) => key(item.story) !== key(story))
    if (slot === 'lead') next = next.map((item) => (item.slot === 'lead' ? { ...item, slot: 'top' as const } : item))
    const limit = slots.find((entry) => entry.slot === slot)?.limit ?? 0
    if (next.filter((item) => item.slot === slot).length >= limit) {
      toast.error(`The ${slot} section is full`)
      return
    }
    edit([...next, { slot, story, note: '', expires_at: '' }])
  }
  const move = (index: number, direction: -1 | 1) => {
    const item = current[index]
    if (!item) return
    const siblings = current.map((entry, position) => ({ entry, position })).filter(({ entry }) => entry.slot === item.slot)
    const at = siblings.findIndex(({ position }) => position === index)
    const target = siblings[at + direction]
    if (!target) return
    const next = [...current]
    next[index] = current[target.position] as Draft
    next[target.position] = item
    edit(next)
  }
  const update = (index: number, patch: Partial<Draft>) => edit(current.map((item, position) => (position === index ? { ...item, ...patch } : item)))
  const remove = (index: number) => edit(current.filter((_, position) => position !== index))

  const candidates = (view.data?.candidates ?? []).filter((story) => !placed.has(key(story)) && story.headline.toLowerCase().includes(search.trim().toLowerCase()))
  const autoSlot = new Map<string, FrontPageSlot>()
  const preview = view.data?.preview
  if (preview) for (const story of [preview.lead, ...preview.top, ...preview.more]) if (story && !story.curated) autoSlot.set(key(story), story.slot)

  if (me.isPending) return <Skeleton className="h-48 w-full" />
  if (!allowed) return <p className="p-6 text-sm text-muted-foreground">The front page is curated by the chief editor and administrators.</p>

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <Badge variant="outline" className="mb-2"><LayoutTemplate /> Chief editor</Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Front page</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            The AI layout editor builds a new edition every 30 minutes (05:00–24:00): it chooses the stories and block sizes, writes each story to fit, and draws a few illustrations. Stories you place here are always included in the place you choose, and saving rebuilds the edition.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => rebuild.mutate()} disabled={rebuild.isPending}>{rebuild.isPending ? <LoaderCircle className="animate-spin" /> : <RefreshCw />} Rebuild edition now</Button>
          {dirty ? <Button variant="outline" onClick={() => setDraft(null)} disabled={save.isPending}><Undo2 /> Discard changes</Button> : null}
          <Button onClick={() => save.mutate()} disabled={!dirty || save.isPending}>
            {save.isPending ? <LoaderCircle className="animate-spin" /> : <Save />} {save.isPending ? 'Publishing…' : 'Publish front page'}
          </Button>
        </div>
      </div>

      {view.isPending ? <Skeleton className="h-96 w-full" /> : view.isError ? (
        <Card><CardContent className="space-y-3"><p role="alert">Could not load the front page.</p><Button variant="outline" onClick={() => { void view.refetch() }}>Try again</Button></CardContent></Card>
      ) : (
        <div className="grid min-w-0 gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
          <div className="flex min-w-0 flex-col gap-6">
            {slots.map(({ slot, title, hint, limit }) => {
              const items = current.map((item, index) => ({ item, index })).filter(({ item }) => item.slot === slot)
              return (
                <Card key={slot} className="gap-0 py-0">
                  <CardHeader className="border-b py-4">
                    <CardTitle className="flex items-center justify-between gap-2">{title}<Badge variant="secondary">{items.length}/{limit} chosen</Badge></CardTitle>
                    <CardDescription>{hint}</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3 p-4">
                    {items.length === 0 ? <p className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">Nothing chosen. {slot === 'lead' ? 'The highest-ranked story leads automatically.' : 'Filled automatically.'}</p> : null}
                    {items.map(({ item, index }, position) => (
                      <div key={key(item.story)} className="space-y-3 rounded-xl border p-3">
                        <StoryLine story={item.story} />
                        <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_13rem]">
                          <Input aria-label="Note for readers (optional)" placeholder="Note for readers (optional, shown with the story)" maxLength={280} lang="si"
                            value={item.note} onChange={(event) => update(index, { note: event.target.value })} />
                          <Input aria-label="Keep until (optional)" type="datetime-local" value={item.expires_at} onChange={(event) => update(index, { expires_at: event.target.value })} />
                        </div>
                        <div className="flex flex-wrap gap-1.5">
                          <Button size="sm" variant="outline" disabled={position === 0} onClick={() => move(index, -1)} aria-label="Move up"><ArrowUp /></Button>
                          <Button size="sm" variant="outline" disabled={position === items.length - 1} onClick={() => move(index, 1)} aria-label="Move down"><ArrowDown /></Button>
                          {slots.filter((entry) => entry.slot !== slot).map((entry) => (
                            <Button key={entry.slot} size="sm" variant="outline" onClick={() => place(item.story, entry.slot)}>Move to {entry.title.toLowerCase()}</Button>
                          ))}
                          <Button size="sm" variant="ghost" onClick={() => remove(index)}><X /> Remove</Button>
                        </div>
                      </div>
                    ))}
                  </CardContent>
                </Card>
              )
            })}
          </div>

          <div className="flex min-w-0 flex-col gap-6">
            <Card className="gap-0 py-0">
              <CardHeader className="border-b py-4">
                <CardTitle>Recent stories</CardTitle>
                <CardDescription>Ranked by coverage, section and freshness (last 36 hours). "Auto" shows where a story appears if you leave places open.</CardDescription>
                <div className="relative mt-2">
                  <Search className="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input aria-label="Search stories" placeholder="Search headlines" lang="si" className="ps-9" value={search} onChange={(event) => setSearch(event.target.value)} />
                </div>
              </CardHeader>
              <CardContent className="max-h-[70vh] space-y-3 overflow-y-auto p-4">
                {candidates.length === 0 ? <p className="text-sm text-muted-foreground">No stories match.</p> : null}
                {candidates.map((story) => (
                  <div key={key(story)} className="space-y-2 rounded-xl border p-3">
                    <StoryLine story={story} auto={autoSlot.get(key(story))} />
                    <div className="flex flex-wrap gap-1.5">
                      {slots.map((entry) => <Button key={entry.slot} size="sm" variant={entry.slot === 'lead' ? 'default' : 'outline'} onClick={() => place(story, entry.slot)}>{entry.slot === 'lead' ? 'Make lead' : `Add to ${entry.title.toLowerCase()}`}</Button>)}
                    </div>
                  </div>
                ))}
              </CardContent>
            </Card>

            <Card className="gap-0 py-0">
              <CardHeader className="border-b py-4">
                <CardTitle>Sources on the front page</CardTitle>
                <CardDescription>Excluded sources never fill open places automatically. They still appear in sections, search and story pages, and you can still place their stories by hand.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-2 p-4 sm:grid-cols-2">
                {view.data.sources.map((source) => (
                  <label key={source.id} className="flex items-center gap-3 rounded-xl border p-3 text-sm">
                    <input type="checkbox" className="size-4 shrink-0 accent-primary" checked={source.front_page} disabled={toggleSource.isPending}
                      onChange={(event) => toggleSource.mutate({ id: source.id, front_page: event.target.checked })} />
                    <span className="min-w-0 truncate">{source.name}</span>
                  </label>
                ))}
              </CardContent>
            </Card>
          </div>
        </div>
      )}
    </section>
  )
}

function StoryLine({ story, auto }: { story: FrontPageStory; auto?: FrontPageSlot }) {
  const href = publicURL(story.kind === 'event' ? `/e/${story.id}` : `/a/${story.id}`)
  return (
    <div className="min-w-0">
      <div className="mb-1 flex flex-wrap items-center gap-1.5 text-xs">
        {story.breaking ? <Badge>Breaking</Badge> : null}
        {story.category ? <Badge variant="outline" lang="si">{story.category.name_si}</Badge> : null}
        <Badge variant="secondary">{story.source_count} {story.source_count === 1 ? 'source' : 'sources'}</Badge>
        {auto ? <Badge variant="outline">Auto: {auto}</Badge> : null}
        <span className="text-muted-foreground">Updated {formatDateTime(story.updated_at)}</span>
      </div>
      <a href={href} target="_blank" rel="noreferrer" lang="si" className="font-medium leading-relaxed underline-offset-4 hover:underline">
        {story.headline} <ExternalLink className="inline size-3.5 align-baseline text-muted-foreground" aria-label="Opens the public page" />
      </a>
      <p className="mt-1 truncate text-xs text-muted-foreground">{story.sources.map((source) => source.name).join(' · ')}</p>
    </div>
  )
}
