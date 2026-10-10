import { createClient, type SocialPost, type SocialPostStatus, type SocialSettings, type SocialStatus } from '@snap/api-client'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Share2, ImagePlus, Loader2, RefreshCw, ShieldCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { CreateSocialPost } from '@/components/create-social-post'
import { ReviewBadge, ReviewPanel } from '@/components/social-review'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { formatDateTime } from '@/lib/date-time'
import { facebookAuthorizeURL, hashtagError, parseSocialHashtags, runeCount, runeLimitError, safeSocialLink, socialTextLimits } from '@/lib/social-publishing'

const client = createClient()
const tabs: { label: string; statuses: SocialPostStatus[] }[] = [
  { label: 'Pending review', statuses: ['pending_review'] },
  { label: 'Approved / Scheduled', statuses: ['approved', 'publishing'] },
  { label: 'Published', statuses: ['published'] },
  { label: 'Needs attention / Failed', statuses: ['needs_attention', 'failed'] },
  { label: 'Skipped / Rejected', statuses: ['skipped', 'rejected'] },
]
const refreshInterval = () => document.visibilityState === 'visible' ? 30_000 : false

export function SocialPage() {
  const queryClient = useQueryClient()
  const me = useQuery({ queryKey: ['me'], queryFn: () => client.me() })
  const administrator = me.data?.role === 'administrator'
  const status = useQuery({ queryKey: ['social', 'status'], queryFn: () => client.socialStatus(), enabled: administrator, refetchInterval: refreshInterval })
  const [tab, setTab] = useState(0)
  const [stateID, setStateID] = useState('')
  const [disconnectOpen, setDisconnectOpen] = useState(false)
  // Stays true after the API answers, until the browser leaves for Facebook, so the button cannot be clicked twice.
  const [redirecting, setRedirecting] = useState(false)
  useEffect(() => {
    // Returning with the browser Back button restores this page from cache; unlock the button again.
    const onPageShow = (event: PageTransitionEvent) => { if (event.persisted) setRedirecting(false) }
    window.addEventListener('pageshow', onPageShow)
    return () => window.removeEventListener('pageshow', onPageShow)
  }, [])
  const handledCallback = useRef(false)
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['social'] })
  useEffect(() => {
    const onVisible = () => { if (document.visibilityState === 'visible') void queryClient.invalidateQueries({ queryKey: ['social'] }) }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [queryClient])
  useEffect(() => {
    if (!administrator || handledCallback.current) return
    handledCallback.current = true
    const url = new URL(window.location.href)
    const outcome = url.searchParams.get('facebook')
    const error = url.searchParams.get('error')
    if (outcome === 'connected') { toast.success('Facebook Page connected'); void refresh() }
    if (outcome === 'choose_page') {
      const id = url.searchParams.get('state_id')
      if (id) setStateID(id)
      else toast.error('Page selection expired. Connect Facebook again.')
    }
    if (error) toast.error(({ invalid_state: 'Connection expired. Connect Facebook again.', not_configured: 'Facebook publishing is not configured.', authorization_failed: 'Facebook authorization failed.', administrator_required: 'An administrator must connect the Page.' } as Record<string, string>)[error] ?? 'Could not connect Facebook. Please try again.')
    if (outcome || error) {
      for (const key of ['facebook', 'state_id', 'error']) url.searchParams.delete(key)
      window.history.replaceState(window.history.state, '', url.pathname + url.search + url.hash)
    }
  }, [administrator]) // Callback is consumed once, including under StrictMode.
  const pages = useQuery({ queryKey: ['social', 'pending-pages', stateID], queryFn: () => client.pendingFacebookPages(stateID), enabled: !!stateID && administrator, retry: false })
  const connect = useMutation({ mutationFn: () => client.connectFacebook(), onSuccess: ({ authorize_url }) => {
    const safe = facebookAuthorizeURL(authorize_url)
    if (safe) { setRedirecting(true); window.location.assign(safe) }
    else toast.error('The server returned an invalid Facebook authorization URL.')
  }, onError: () => toast.error('Could not start Facebook connection.') })
  const select = useMutation({ mutationFn: (page_id: string) => client.selectFacebookPage({ state_id: stateID, page_id }), onSuccess: () => { setStateID(''); void refresh(); toast.success('Facebook Page connected') }, onError: () => toast.error('Could not select this Page. Try connecting again if the selection expired.') })
  const disconnect = useMutation({ mutationFn: () => client.disconnectFacebook(), onSuccess: () => { setDisconnectOpen(false); void refresh(); toast.success('Facebook Page disconnected') }, onError: () => toast.error('Could not disconnect Facebook.') })
  if (me.isPending) return <Skeleton className="h-48 w-full" />
  if (!administrator) return <p className="p-6 text-sm text-muted-foreground">Social publishing is available to administrators only.</p>
  return <section className="flex min-w-0 flex-col gap-6">
    <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end"><div><Badge variant="outline" className="mb-2"><Share2 />Facebook Page</Badge><h1 className="font-heading text-2xl font-semibold tracking-tight">Social publishing</h1><p className="mt-1 max-w-2xl text-sm text-muted-foreground">Prepare original Sinhala stories, review every image and caption, then publish to your Page.</p></div><CreateSocialPost onCreated={() => setTab(0)} /></div>
    {status.isPending ? <Skeleton className="h-48 w-full" /> : status.isError ? <Card><CardContent className="space-y-3"><p role="alert">Could not load social publishing.</p><Button variant="outline" onClick={() => void status.refetch()}>Try again</Button></CardContent></Card> : <>
      <Connection status={status.data} phase={redirecting ? 'redirecting' : connect.isPending ? 'starting' : 'idle'} onConnect={() => { if (!connect.isPending && !redirecting) connect.mutate() }} onDisconnect={() => setDisconnectOpen(true)} />
      <Settings settings={status.data.settings} next={status.data.next_publish_eligible_at} />
      <Card><CardHeader><CardTitle>Publishing queue</CardTitle><CardDescription>Review the source evidence and keep source credits and the site link in every caption. Updates refresh every 30 seconds while this page is visible.</CardDescription></CardHeader><CardContent className="space-y-5">
        <div role="tablist" aria-label="Post status" className="flex flex-wrap gap-2">{tabs.map((item, i) => <Button key={item.label} role="tab" id={`social-tab-${i}`} aria-controls="social-panel" aria-selected={tab === i} tabIndex={tab === i ? 0 : -1} onKeyDown={(event) => {
          const next = event.key === 'ArrowRight' ? (i + 1) % tabs.length : event.key === 'ArrowLeft' ? (i + tabs.length - 1) % tabs.length : event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : null
          if (next !== null) { event.preventDefault(); setTab(next); document.getElementById(`social-tab-${next}`)?.focus() }
        }} variant={tab === i ? 'default' : 'outline'} size="sm" onClick={() => setTab(i)}>{item.label}<Badge variant="secondary">{item.statuses.reduce((n, key) => n + (status.data.counts[key] ?? 0), 0)}</Badge></Button>)}</div>
        <div id="social-panel" role="tabpanel" aria-labelledby={`social-tab-${tab}`} className="space-y-4">{tabs[tab]!.statuses.map((key) => <Queue key={key} status={key} />)}</div>
      </CardContent></Card>
    </>}
    <Dialog open={!!stateID} onOpenChange={(open) => { if (!open && !select.isPending) setStateID('') }}><DialogContent><DialogHeader><DialogTitle>Choose a Facebook Page</DialogTitle><DialogDescription>Select the Page to publish to. This selection expires after ten minutes.</DialogDescription></DialogHeader>{pages.isPending ? <Skeleton className="h-24" /> : pages.isError ? <p role="alert" className="text-sm">Could not load Pages. Close this dialog and connect Facebook again.</p> : <div className="space-y-2">{(pages.data?.items ?? []).map((page) => <Button key={page.id} variant="outline" className="h-auto w-full justify-start py-3" disabled={select.isPending} aria-busy={select.isPending && select.variables === page.id} onClick={() => select.mutate(page.id)}>{page.picture ? <img src={page.picture} alt="" className="size-10 rounded-full" /> : <Share2 />}<span className="min-w-0 truncate">{page.name}</span>{select.isPending && select.variables === page.id ? <><Loader2 className="ms-auto animate-spin" /><span className="text-xs text-muted-foreground">Connecting…</span></> : null}</Button>)}{!pages.data?.items?.length ? <p>No eligible Pages found. Connect again with Page content permissions.</p> : null}</div>}</DialogContent></Dialog>
    <Dialog open={disconnectOpen} onOpenChange={setDisconnectOpen}><DialogContent><DialogHeader><DialogTitle>Disconnect Facebook Page?</DialogTitle><DialogDescription>This removes the saved Page token and pending connections. Queued posts remain, but publishing requires reconnecting.</DialogDescription></DialogHeader><DialogFooter><Button variant="outline" onClick={() => setDisconnectOpen(false)}>Cancel</Button><Button variant="destructive" disabled={disconnect.isPending} onClick={() => disconnect.mutate()}>{disconnect.isPending ? 'Disconnecting…' : 'Disconnect'}</Button></DialogFooter></DialogContent></Dialog>
  </section>
}

function Connection({ status, phase, onConnect, onDisconnect }: { status: SocialStatus; phase: 'idle' | 'starting' | 'redirecting'; onConnect: () => void; onDisconnect: () => void }) {
  const busy = phase !== 'idle'
  const account = status.account
  return <Card><CardHeader><CardTitle className="flex items-center gap-2"><Share2 className="size-5" />Page connection</CardTitle><CardDescription>Connect the owner's Page through Facebook Login for Business.</CardDescription></CardHeader><CardContent className="space-y-4">
    <div className="flex flex-wrap items-center gap-3">{account?.picture ? <img src={account.picture} className="size-12 rounded-full" alt="Facebook Page" /> : null}<div><p className="font-medium">{account?.page_name || 'No Page connected'}</p>{account ? <p className="text-xs text-muted-foreground">Connected {formatDateTime(account.connected_at)}</p> : null}</div><Badge variant={status.connected ? 'default' : 'secondary'}>{status.connected ? 'Connected' : account?.status === 'needs_reconnect' ? 'Needs reconnect' : 'Disconnected'}</Badge><Badge variant="outline">{status.configured ? 'Configured' : 'Not configured'}</Badge></div>
    {account?.last_error ? <p role="alert" className="break-words rounded-xl border border-destructive/30 bg-destructive/5 p-3 text-sm">{account.last_error}</p> : null}
    {!status.configured ? <div className="space-y-2 rounded-xl border bg-muted/40 p-4 text-sm"><p className="font-medium">Set up Facebook publishing on the server</p><p className="text-muted-foreground">Required environment variables:</p><div className="flex flex-wrap gap-2">{['SNAP_META_APP_ID', 'SNAP_META_APP_SECRET', 'SNAP_SOCIAL_TOKEN_KEY'].map((name) => <code key={name} className="break-all rounded-md border bg-background px-2 py-1 text-xs">{name}</code>)}</div><p className="text-muted-foreground">Callback configuration: <code className="break-all">SNAP_META_REDIRECT_URL</code></p><a href="https://github.com/prabhavalabs/lanka-news-paper/blob/feature/facebook-page-publishing/docs/social-publishing.md" target="_blank" rel="noreferrer" className="inline-block underline underline-offset-4">Read docs/social-publishing.md</a></div> : null}
    <div className="flex flex-wrap items-center gap-2"><Button onClick={onConnect} disabled={!status.configured || busy} aria-busy={busy}>{busy ? <Loader2 className="animate-spin" /> : <Share2 />}{phase === 'starting' ? 'Starting connection…' : phase === 'redirecting' ? 'Opening Facebook…' : account ? 'Reconnect Facebook Page' : 'Connect Facebook Page'}</Button>{account ? <Button variant="outline" onClick={onDisconnect} disabled={busy}>Disconnect</Button> : null}{busy ? <p role="status" className="text-sm text-muted-foreground">{phase === 'starting' ? 'Preparing a secure Facebook login…' : 'Taking you to Facebook. Keep this tab open.'}</p> : null}</div>
  </CardContent></Card>
}

const numericSettings = [
  ['max_posts_per_hour', 'Maximum posts per hour', 1, 100], ['max_posts_per_day', 'Maximum posts per day', 1, 1000],
  ['min_gap_minutes', 'Minimum gap (minutes)', 1, 1440], ['quiet_hours_start', 'Quiet hours start (hour)', 0, 23],
  ['quiet_hours_end', 'Quiet hours end (hour)', 0, 23], ['min_score', 'Minimum score', 0, 100],
  ['lookback_hours', 'Lookback (hours)', 1, 168], ['candidates_per_run', 'Candidates per run', 1, 30],
  ['min_review_score', 'Minimum AI review score', 0, 100],
] as const
function Settings({ settings, next }: { settings: SocialSettings; next: string | null }) {
  const [draft, setDraft] = useState(settings)
  const queryClient = useQueryClient()
  const save = useMutation({ mutationFn: () => client.updateSocialSettings(draft), onSuccess: (value) => { setDraft(value); void queryClient.invalidateQueries({ queryKey: ['social'] }); toast.success('Publishing settings saved') }, onError: () => toast.error('Could not save publishing settings.') })
  const run = useMutation({ mutationFn: () => client.runSocialSelection(), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['social'] }); toast.success('Selection queued. Review new posts when the run completes.') }, onError: () => toast.error('Could not queue selection.') })
  const valid = numericSettings.every(([key, , min, max]) => Number.isInteger(draft[key]) && draft[key] >= min && draft[key] <= max)
  return <Card><CardHeader><CardTitle className="flex items-center gap-2"><ShieldCheck className="size-5" />Selection and publishing settings</CardTitle><CardDescription>Enabling selection keeps human approval required unless auto-publish is also enabled.</CardDescription></CardHeader><CardContent><form className="space-y-5" onSubmit={(e) => { e.preventDefault(); if (valid) save.mutate() }}>
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{([['enabled', 'Enable selection and publishing'], ['auto_publish', 'Auto-publish posts that pass AI review'], ['realtime_enabled', 'Select as soon as news arrives'], ['include_single_articles', 'Include single articles']] as const).map(([key, label]) => <label key={key} className="flex items-center gap-3 rounded-xl border p-4 text-sm"><input type="checkbox" className="size-4 shrink-0 accent-primary" checked={draft[key]} onChange={(e) => setDraft({ ...draft, [key]: e.target.checked })} />{label}</label>)}</div>
    <p className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-900 dark:text-amber-200">With auto-publish on, a post goes out without a person only if the AI quality review passes it: score at or above the minimum, and no blocker or major issue. Anything else waits here for an editor. Caps and quiet hours still apply.</p>
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{numericSettings.map(([key, label, min, max]) => {
      const error = !Number.isInteger(draft[key]) || draft[key] < min || draft[key] > max
      return <Field key={key}><FieldLabel htmlFor={`social-${key}`}>{label}</FieldLabel><Input id={`social-${key}`} type="number" min={min} max={max} step={1} value={Number.isNaN(draft[key]) ? '' : draft[key]} aria-invalid={error} onChange={(e) => setDraft({ ...draft, [key]: e.target.value === '' ? NaN : Number(e.target.value) })} />{error ? <FieldError>Enter a whole number from {min} to {max}.</FieldError> : <p className="text-xs text-muted-foreground">{min}–{max}</p>}</Field>
    })}</div>
    <p className="text-sm text-muted-foreground">Quiet hours use {draft.timezone}. Equal start and end hours disable quiet hours. Scheduled times are earliest delivery times; caps and quiet hours can delay publishing.</p>
    {next ? <p className="text-sm">Next eligible publishing time: {formatDateTime(next)}</p> : null}
    <div className="flex flex-wrap gap-2"><Button type="submit" disabled={!valid || save.isPending}>{save.isPending ? 'Saving…' : 'Save settings'}</Button><Button type="button" variant="outline" disabled={!settings.enabled || run.isPending} onClick={() => run.mutate()}><RefreshCw />{run.isPending ? 'Queueing…' : 'Run selection now'}</Button></div>
    {!settings.enabled ? <p className="text-xs text-muted-foreground">Save with selection enabled to run selection. Manual posts can still be created while disabled.</p> : null}
  </form></CardContent></Card>
}

function Queue({ status }: { status: SocialPostStatus }) {
  const posts = useInfiniteQuery({ queryKey: ['social', 'posts', status], initialPageParam: '', queryFn: ({ pageParam }) => client.socialPosts({ status, limit: 10, cursor: pageParam }), getNextPageParam: (page) => page.next_cursor || undefined, refetchInterval: refreshInterval })
  const items = posts.data?.pages.flatMap((page) => page.items) ?? []
  return <div className="space-y-4"><h3 className="text-sm font-medium capitalize">{status.replaceAll('_', ' ')}</h3>{posts.isPending ? <Skeleton className="h-48" /> : posts.isError ? <div className="space-y-2 rounded-xl border p-5"><p role="alert" className="text-sm">Could not load these posts.</p><Button variant="outline" size="sm" onClick={() => void posts.refetch()}>Try again</Button></div> : !items.length ? <div className="rounded-xl border border-dashed p-8 text-center"><ImagePlus className="mx-auto mb-3 size-6 text-muted-foreground" /><p className="text-sm font-medium">No {status.replaceAll('_', ' ')} posts</p><p className="mt-1 text-xs text-muted-foreground">Create a post from a story or run selection to prepare the review queue.</p></div> : items.map((post) => <PostCard key={post.id} post={post} />)}{posts.hasNextPage ? <Button variant="outline" disabled={posts.isFetchingNextPage} onClick={() => void posts.fetchNextPage()}>{posts.isFetchingNextPage ? 'Loading…' : 'Load more'}</Button> : null}</div>
}

type PostAction = 'edit' | 'approve' | 'reject' | 'retry' | 'render' | 'review'
function PostCard({ post }: { post: SocialPost }) {
  const queryClient = useQueryClient()
  const [action, setAction] = useState<PostAction | null>(null)
  const [schedule, setSchedule] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const [draft, setDraft] = useState({ headline_si: post.headline_si, summary_si: post.summary_si, caption: post.caption, hashtags: (post.hashtags ?? []).map((t) => `#${t}`).join(' ') })
  const editable = ['pending_review', 'approved', 'failed'].includes(post.status)
  const tags = parseSocialHashtags(draft.hashtags)
  const tagError = hashtagError(tags)
  const textValid = Object.entries(socialTextLimits).every(([key, limit]) => !runeLimitError(draft[key as keyof typeof socialTextLimits], limit))
  const scheduleValid = !schedule || (!Number.isNaN(new Date(schedule).getTime()) && new Date(schedule).getTime() > Date.now())
  const mutation = useMutation({ mutationFn: async (selected: PostAction) => {
    if (selected === 'edit') return client.editSocialPost(post.id, { ...draft, hashtags: tags })
    if (selected === 'approve') return client.approveSocialPost(post.id, schedule ? { scheduled_for: new Date(schedule).toISOString() } : {})
    if (selected === 'reject') return client.rejectSocialPost(post.id)
    if (selected === 'retry') return client.retrySocialPost(post.id, confirmed)
    if (selected === 'review') return client.reviewSocialPost(post.id)
    return client.renderSocialPost(post.id)
  }, onSuccess: (updated, selected) => {
    setAction(null)
    void queryClient.invalidateQueries({ queryKey: ['social'] })
    if (updated.last_error && (selected === 'edit' || selected === 'render')) toast.error(`Image needs attention: ${updated.last_error}`)
    else if (selected === 'review') toast[updated.review_status === 'passed' ? 'success' : 'warning'](updated.review_status === 'passed' ? 'AI review passed' : updated.review_status === 'unavailable' ? 'AI review could not run' : 'AI review found issues')
    else toast.success(({ edit: 'Post saved and image rendered', approve: 'Post approved', reject: 'Post rejected', retry: 'Retry queued', render: 'Image rendered' })[selected])
  }, onError: () => { void queryClient.invalidateQueries({ queryKey: ['social'] }); toast.error('Could not update post. Its status may have changed; refresh and try again.') } })
  const open = (selected: PostAction) => {
    setDraft({ headline_si: post.headline_si, summary_si: post.summary_si, caption: post.caption, hashtags: (post.hashtags ?? []).map((t) => `#${t}`).join(' ') })
    setSchedule(''); setConfirmed(false); setAction(selected)
  }
  const image = post.image_url ? `${post.image_url}${post.image_url.includes('?') ? '&' : '?'}v=${encodeURIComponent(post.image_rendered_at ?? post.updated_at)}` : ''
  const subjectURL = safeSocialLink(post.link_url)
  const permalink = post.fb_permalink ? safeSocialLink(post.fb_permalink) : undefined
  return <Card className="overflow-hidden"><CardContent className="grid min-w-0 gap-5 md:grid-cols-[220px_minmax(0,1fr)]">
    {image ? <a href={image} target="_blank" rel="noreferrer" className="block self-start overflow-hidden rounded-xl border bg-muted"><img src={image} alt={`Social image: ${post.headline_si}`} className="aspect-[4/5] w-full object-contain" /></a> : <div className="grid aspect-[4/5] max-h-72 place-items-center rounded-xl border bg-muted p-4 text-center text-sm text-muted-foreground"><div><ImagePlus className="mx-auto mb-2 size-6" />Image not ready<br />Use Re-render image to try again.</div></div>}
    <div className="min-w-0 space-y-4"><div className="flex flex-wrap gap-2"><Badge variant={post.kind === 'breaking' ? 'default' : 'secondary'}>{post.kind === 'breaking' ? 'Breaking' : 'Top story'}</Badge><Badge variant="outline">{post.status.replaceAll('_', ' ')}</Badge><Badge variant="outline">Score {post.score}/100</Badge><Badge variant="outline">{triggerLabel[post.trigger] ?? post.trigger}</Badge><ReviewBadge post={post} /></div>
      <div><h3 className="font-medium leading-relaxed">{subjectURL ? <a href={subjectURL} target="_blank" rel="noreferrer" className="underline underline-offset-4">{post.subject_title || post.headline_si}</a> : post.subject_title || post.headline_si}</h3><p className="mt-2 text-sm text-muted-foreground">{post.selection_reason}</p><p className="mt-2 text-xs text-muted-foreground">Sources: {(post.source_names ?? []).join(' · ') || 'None recorded'}</p></div>
      <p className="whitespace-pre-wrap break-words rounded-xl bg-muted/50 p-4 text-sm leading-7" lang="si">{post.caption}</p>
      <div className="flex flex-wrap gap-2">{(post.hashtags ?? []).map((tag) => <Badge key={tag} variant="secondary">#{tag}</Badge>)}</div>
      <div className="flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground"><span>Created {formatDateTime(post.created_at)}</span><span>Updated {formatDateTime(post.updated_at)}</span>{post.image_rendered_at ? <span>Image {formatDateTime(post.image_rendered_at)}</span> : null}{post.scheduled_for ? <span>Scheduled {formatDateTime(post.scheduled_for)}</span> : null}{post.published_at ? <span>Published {formatDateTime(post.published_at)}</span> : null}</div>
      <ReviewPanel post={post} />
      {post.last_error ? <p role="alert" className="break-words rounded-xl border border-destructive/30 bg-destructive/5 p-3 text-sm">{post.last_error}</p> : null}
      <div className="flex flex-wrap gap-2">{['pending_review', 'approved'].includes(post.status) ? <Button size="sm" disabled={mutation.isPending} onClick={() => open('approve')}>{post.status === 'approved' ? 'Reschedule' : 'Approve'}</Button> : null}{editable ? <><Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => open('edit')}>Edit</Button><Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => mutation.mutate('render')}><RefreshCw />{mutation.isPending && mutation.variables === 'render' ? 'Rendering…' : 'Re-render image'}</Button><Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => mutation.mutate('review')}>{mutation.isPending && mutation.variables === 'review' ? <Loader2 className="animate-spin" /> : <ShieldCheck />}{mutation.isPending && mutation.variables === 'review' ? 'Reviewing…' : 'Run AI review'}</Button><Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => open('reject')}>Reject</Button></> : null}{['failed', 'needs_attention'].includes(post.status) ? <Button size="sm" variant="outline" onClick={() => open('retry')} disabled={mutation.isPending}>Retry</Button> : null}{permalink ? <Button size="sm" variant="outline" nativeButton={false} render={<a href={permalink} target="_blank" rel="noreferrer" />}>View on Facebook</Button> : null}</div>
    </div>
    <Dialog open={!!action} onOpenChange={(value) => { if (!value && !mutation.isPending) setAction(null) }}><DialogContent className="max-h-[90dvh] overflow-y-auto" showCloseButton={!mutation.isPending}><DialogHeader><DialogTitle>{action === 'edit' ? 'Edit social post' : action === 'approve' ? 'Approve social post' : action === 'retry' ? 'Retry social post?' : 'Reject social post?'}</DialogTitle><DialogDescription>{action === 'edit' ? 'Saving re-renders the image. Caption is the complete Facebook text: retain the source credits and any hashtags you want published. Saving clears the AI review; run it again before approving.' : action === 'approve' ? 'Publish as soon as eligible or choose an earliest delivery time. Connection, enabled settings, quiet hours and caps still apply.' : action === 'retry' ? 'Check the actual Facebook Page before retrying to avoid a duplicate post.' : 'This removes the post from the publishing queue and retains its review history.'}</DialogDescription></DialogHeader>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); if (action) mutation.mutate(action) }}>
        {action === 'edit' ? <>{Object.entries(socialTextLimits).map(([key, limit]) => {
          const field = key as keyof typeof socialTextLimits
          const error = runeLimitError(draft[field], limit)
          return <Field key={key}><FieldLabel htmlFor={`edit-${post.id}-${key}`}>{({ headline_si: 'Sinhala headline', summary_si: 'Sinhala summary', caption: 'Facebook caption' })[field]}</FieldLabel><Textarea id={`edit-${post.id}-${key}`} lang="si" rows={field === 'caption' ? 6 : 3} value={draft[field]} aria-invalid={!!error} onChange={(e) => setDraft({ ...draft, [field]: e.target.value })} /><p className="text-xs text-muted-foreground">{runeCount(draft[field].trim())}/{limit} characters (Unicode code points)</p>{error ? <FieldError>{error}</FieldError> : null}</Field>
        })}<Field><FieldLabel htmlFor={`tags-${post.id}`}>Hashtags</FieldLabel><Input id={`tags-${post.id}`} value={draft.hashtags} onChange={(e) => setDraft({ ...draft, hashtags: e.target.value })} /><p className="text-xs text-muted-foreground">Space-separated; up to 3 tags, 60 characters each. Update the caption too if you change tags.</p>{tagError ? <FieldError>{tagError}</FieldError> : null}</Field></> : null}
        {action === 'approve' ? <Field><FieldLabel htmlFor={`schedule-${post.id}`}>Schedule (optional, your local timezone)</FieldLabel><Input id={`schedule-${post.id}`} type="datetime-local" value={schedule} onChange={(e) => setSchedule(e.target.value)} />{!scheduleValid ? <FieldError>Choose a future date and time.</FieldError> : <p className="text-xs text-muted-foreground">Leave empty to publish as soon as eligible.</p>}</Field> : null}
        {action === 'retry' && post.status === 'needs_attention' ? <label className="flex items-start gap-3 rounded-xl border p-4 text-sm"><input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} className="mt-1 size-4 shrink-0" /><span>I confirmed this was not published on Facebook</span></label> : null}
        <DialogFooter><Button type="button" variant="outline" disabled={mutation.isPending} onClick={() => setAction(null)}>Cancel</Button><Button type="submit" variant={action === 'reject' ? 'destructive' : 'default'} disabled={mutation.isPending || (action === 'edit' && (!textValid || !!tagError)) || (action === 'approve' && !scheduleValid) || (action === 'retry' && post.status === 'needs_attention' && !confirmed)}>{mutation.isPending ? 'Working…' : action === 'edit' ? 'Save and re-render' : action === 'approve' ? schedule ? 'Approve and schedule' : 'Approve for ASAP' : action === 'retry' ? 'Confirm retry' : 'Reject post'}</Button></DialogFooter>
      </form>
    </DialogContent></Dialog>
  </CardContent></Card>
}

const triggerLabel: Record<string, string> = { scheduled: 'Scheduled run', realtime: 'Arrived live', manual: 'Manual' }

