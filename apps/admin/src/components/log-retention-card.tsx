import { createClient, type LogRetentionSettings, type LogRetentionSummaryKey } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, LoaderCircle, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/date-time'

const client = createClient()
const minDays = 1
const maxDays = 365

const summaryLabels: Record<LogRetentionSummaryKey, string> = {
  pipeline_runs: 'Pipeline runs',
  pipeline_logs: 'Pipeline log lines',
  model_calls: 'Model calls',
  ingestion_runs: 'Feed polls',
  crawl_attempts: 'Crawl attempts',
  log_files: 'Log files',
}

export function LogRetentionCard() {
  const queryClient = useQueryClient()
  const settings = useQuery({ queryKey: ['log-retention'], queryFn: () => client.logRetentionSettings() })
  const [draft, setDraft] = useState<string | null>(null)
  const current = settings.data?.retention_days
  const value = draft ?? (current === undefined ? '' : String(current))
  const days = Number(value)
  const valid = Number.isInteger(days) && days >= minDays && days <= maxDays

  const save = useMutation({
    mutationFn: () => client.updateLogRetentionSettings({ retention_days: days }),
    onSuccess: (next) => {
      queryClient.setQueryData(['log-retention'], next)
      setDraft(null)
      toast.success(`Logs are now kept for ${next.retention_days} ${next.retention_days === 1 ? 'day' : 'days'}`)
    },
    onError: () => toast.error('Could not save the retention period'),
  })
  const run = useMutation({
    mutationFn: () => client.runLogCleanup(),
    onSuccess: () => {
      toast.success('Cleanup queued. Refresh in a minute to see what it removed.')
      void queryClient.invalidateQueries({ queryKey: ['log-retention'] })
    },
    onError: () => toast.error('Could not queue the cleanup'),
  })

  return (
    <Card className="gap-0 overflow-hidden py-0 shadow-sm">
      <CardHeader className="border-b py-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex items-start gap-3">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"><Trash2 /></span>
            <div>
              <CardTitle>Log retention</CardTitle>
              <CardDescription className="mt-1 max-w-3xl">
                A daily cleanup deletes logs older than this: crawl attempts, feed polls, pipeline runs and their logs, model-call logs and Codex log files. Articles, stories, analyses and posts are never deleted.
              </CardDescription>
            </div>
          </div>
          {current ? <Badge variant="secondary">{current} {current === 1 ? 'day' : 'days'}</Badge> : null}
        </div>
      </CardHeader>
      <CardContent className="grid gap-5 p-5 sm:p-6 lg:grid-cols-2">
        {settings.isPending ? <Skeleton className="h-24 lg:col-span-2" /> : settings.isError ? (
          <div className="space-y-2 lg:col-span-2">
            <p role="alert" className="text-sm">Could not load log retention.</p>
            <Button variant="outline" size="sm" onClick={() => { void settings.refetch() }}>Try again</Button>
          </div>
        ) : (
          <>
            <form className="flex flex-col gap-3" onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate() }}>
              <div>
                <Label htmlFor="log-retention-days">Keep logs for (days)</Label>
                <Input id="log-retention-days" className="mt-2" type="number" inputMode="numeric" min={minDays} max={maxDays} step={1}
                  value={value} aria-invalid={!valid} aria-describedby="log-retention-help"
                  onChange={(event) => setDraft(event.target.value)} />
                <p id="log-retention-help" className={valid ? 'mt-1 text-xs text-muted-foreground' : 'mt-1 text-xs text-destructive'}>
                  {valid ? `Whole days from ${minDays} to ${maxDays}. Default 7.` : `Enter a whole number from ${minDays} to ${maxDays}.`}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button type="submit" disabled={!valid || days === current || save.isPending}>
                  {save.isPending ? <LoaderCircle className="animate-spin" /> : <CheckCircle2 />}
                  {save.isPending ? 'Saving…' : 'Save'}
                </Button>
                <Button type="button" variant="outline" disabled={run.isPending} onClick={() => run.mutate()}>
                  {run.isPending ? <LoaderCircle className="animate-spin" /> : <Trash2 />} Run cleanup now
                </Button>
              </div>
            </form>
            <LastRun settings={settings.data} />
          </>
        )}
      </CardContent>
    </Card>
  )
}

function LastRun({ settings }: { settings: LogRetentionSettings }) {
  if (!settings.last_run_at) {
    return <p className="rounded-xl border bg-muted/20 p-4 text-sm text-muted-foreground">The cleanup has not run yet.</p>
  }
  const entries = (Object.keys(summaryLabels) as LogRetentionSummaryKey[]).map((key) => [key, settings.last_run_summary[key] ?? 0] as const)
  return (
    <div className="rounded-xl border bg-muted/20 p-4 text-sm">
      <p className="font-medium">Last cleanup <span className="font-normal text-muted-foreground">· {formatDateTime(settings.last_run_at)}</span></p>
      {settings.last_run_error ? <p role="alert" className="mt-2 break-words text-destructive">{settings.last_run_error}</p> : null}
      <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1">
        {entries.map(([key, count]) => (
          <div key={key} className="flex justify-between gap-2"><dt className="text-muted-foreground">{summaryLabels[key]}</dt><dd className="tabular-nums">{count.toLocaleString('en')}</dd></div>
        ))}
      </dl>
    </div>
  )
}
