import { createClient, type TrendTopic } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, LoaderCircle, Radar, RefreshCw, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/date-time'

const client = createClient()

const statusLabel: Record<TrendTopic['status'], string> = { ready: 'Ready for a reel', watch: 'Watch', skip: 'Not for a reel' }
const statusHint: Record<TrendTopic['status'], string> = {
  ready: 'Covered by at least two of our outlets.',
  watch: 'Hot, but our outlets do not cover it enough yet.',
  skip: 'Unsuitable for a reaction poll.',
}

function views(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${Math.round(n / 1_000)}k`
  return String(n)
}

/** The trend radar: what Sri Lanka is watching, searching and discussing, ranked for reels. */
export function TrendRadarPanel({ onMakeReel, making }: { onMakeReel: (storylineID: string) => void; making: boolean }) {
  const queryClient = useQueryClient()
  const [waitingSince, setWaitingSince] = useState<number | null>(null)
  const radar = useQuery({
    queryKey: ['trend-radar'],
    queryFn: () => client.trendRadar(),
    refetchInterval: (query) => {
      const run = query.state.data?.run
      if (run?.status === 'running') return 5000
      if (waitingSince && Date.now() - waitingSince < 15 * 60_000) return 10_000
      return 120_000
    },
  })
  const run = radar.data?.run ?? null
  if (waitingSince && run && run.status !== 'running' && new Date(run.created_at).getTime() >= waitingSince - 60_000) setWaitingSince(null)
  const refresh = useMutation({
    mutationFn: () => client.refreshTrendRadar(),
    onSuccess: () => {
      setWaitingSince(Date.now())
      void queryClient.invalidateQueries({ queryKey: ['trend-radar'] })
      toast.success('Refreshing the radar. Web research takes a few minutes.')
    },
    onError: () => toast.error('Could not start the radar'),
  })
  const busy = run?.status === 'running' || waitingSince !== null

  return (
    <Card className="gap-0 py-0">
      <CardHeader className="py-4">
        <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
          <div>
            <CardTitle className="flex items-center gap-2"><Radar className="size-4" /> Trend radar</CardTitle>
            <CardDescription className="mt-1 max-w-3xl leading-relaxed">
              Ranks what Sri Lanka is paying attention to from four signals: YouTube videos on Sri Lankan news channels running ahead of their channel's 30-day normal, rising Google searches, live web research, and how fast our outlets are covering each story. Reels are built from the top <em>ready</em> topic.
            </CardDescription>
            {run ? (
              <p className="mt-2 text-xs text-muted-foreground">
                {run.status === 'running' ? 'Running now…' : run.status === 'failed' ? `Last run failed ${formatDateTime(run.created_at)}: ${run.error ?? ''}` : `Updated ${formatDateTime(run.finished_at ?? run.created_at)}`}
                {run.signals ? ` · ${run.signals.youtube_videos ?? 0} YouTube videos · ${run.signals.google_searches ?? 0} Google searches · ${run.signals.web_topics ?? 0} web topics · ${run.signals.storylines ?? 0} storylines` : ''}
              </p>
            ) : null}
          </div>
          <Button variant="outline" disabled={busy || refresh.isPending} onClick={() => refresh.mutate()}>
            {busy ? <LoaderCircle className="animate-spin" /> : <RefreshCw />} Refresh radar
          </Button>
        </div>
      </CardHeader>
      <CardContent className="border-t p-0">
        {radar.isPending ? <Skeleton className="m-4 h-40" /> : !run || run.topics.length === 0 ? (
          <p className="p-6 text-sm text-muted-foreground">{busy ? 'Collecting signals…' : 'No radar yet. It runs with each reel draft, or refresh it now.'}</p>
        ) : (
          <ol className="divide-y">
            {run.topics.slice(0, 15).map((topic) => <TopicRow key={topic.id} topic={topic} making={making} onMakeReel={onMakeReel} />)}
          </ol>
        )}
      </CardContent>
    </Card>
  )
}

function TopicRow({ topic, making, onMakeReel }: { topic: TrendTopic; making: boolean; onMakeReel: (storylineID: string) => void }) {
  const [open, setOpen] = useState(false)
  const c = topic.components
  const parts = [
    c.youtube ? `YouTube ${Math.round(c.youtube)}` : null,
    c.search ? `Google ${Math.round(c.search)}` : null,
    c.web ? `Web ${Math.round(c.web)}` : null,
    c.coverage ? `Coverage ${Math.round(c.coverage)}` : null,
    c.agreement ? `Agreement +${Math.round(c.agreement)}` : null,
  ].filter(Boolean)
  return (
    <li className="p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start">
        <span className="w-8 shrink-0 font-heading text-lg font-semibold tabular-nums text-muted-foreground">{topic.rank}</span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={topic.status === 'ready' ? 'default' : topic.status === 'watch' ? 'secondary' : 'outline'} title={statusHint[topic.status]}>{statusLabel[topic.status]}</Badge>
            <Badge variant="outline">{topic.category}</Badge>
            <span className="text-xs tabular-nums text-muted-foreground">Score {topic.score.toFixed(1)} · {parts.join(' · ')}</span>
          </div>
          <p lang="si" className="mt-2 font-medium leading-relaxed">{topic.title_si}</p>
          <p className="text-sm text-muted-foreground">{topic.title_en}</p>
          <p className="mt-1 text-sm leading-relaxed">{topic.reason}</p>
          <button type="button" className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground" onClick={() => setOpen(!open)}>
            <ChevronDown className={`size-3 transition-transform ${open ? 'rotate-180' : ''}`} /> Evidence
          </button>
          {open ? (
            <div className="mt-2 space-y-2 rounded-lg bg-muted/40 p-3 text-xs leading-5">
              {topic.evidence.storyline ? (
                <p>Our coverage: <span lang="si">{topic.evidence.storyline.title}</span> · {topic.evidence.storyline.outlets} outlets · {topic.evidence.storyline.articles_24h} reports in 24h · {topic.evidence.storyline.acceleration_vs_30_day_baseline}× its 30-day pace</p>
              ) : null}
              {topic.evidence.youtube.map((video) => (
                <p key={video.id}><a className="underline" href={video.url} target="_blank" rel="noreferrer">{video.title}</a> — {video.channel}, {views(video.views)} views in {Math.round(video.age_hours)}h{video.outlier ? `, ${video.outlier.toFixed(1)}× the channel's usual` : ''}{video.chart_rank ? `, #${video.chart_rank} on YouTube Sri Lanka` : ''}</p>
              ))}
              {topic.evidence.google.map((g) => <p key={g.search}>Google search “{g.search}” · {g.approx_searches} searches</p>)}
              {topic.evidence.web.map((w) => (
                <div key={w.title_en}>
                  <p>Web ({w.momentum}): {w.why}</p>
                  {w.evidence.map((link) => <p key={link.url} className="truncate"><a className="underline" href={link.url} target="_blank" rel="noreferrer">{link.title || link.url}</a></p>)}
                </div>
              ))}
            </div>
          ) : null}
        </div>
        {topic.status === 'ready' && topic.storyline_id ? (
          <Button size="sm" disabled={making} onClick={() => onMakeReel(topic.storyline_id!)}><Sparkles /> Make reel</Button>
        ) : null}
      </div>
    </li>
  )
}
