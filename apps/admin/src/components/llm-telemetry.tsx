import { createClient, type LLMCall } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ChevronLeft, ChevronRight, Clock3, EllipsisVertical, Eye, RadioTower,
  Trash2, TriangleAlert,
} from 'lucide-react'
import { Fragment, useEffect, useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ScrollArea, ScrollBar } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { getPaginationPages } from '@/lib/pagination'

const client = createClient()
const telemetryDate = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })

export function LLMTelemetryCard({ articleId }: { articleId: string }) {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState(10)
  const [selected, setSelected] = useState<LLMCall | null>(null)
  const [deleting, setDeleting] = useState<LLMCall | null>(null)
  const calls = useQuery({
    queryKey: ['article-llm-calls', articleId, page, perPage],
    queryFn: () => client.articleLLMCalls(articleId, { page, per_page: perPage }),
    placeholderData: keepPreviousData,
  })
  const remove = useMutation({
    mutationFn: (call: LLMCall) => client.deleteArticleLLMCall(articleId, call.id),
    onSuccess: async () => {
      setDeleting(null)
      setSelected(null)
      toast.success('LLM telemetry record deleted')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['article-llm-calls', articleId] }),
        queryClient.invalidateQueries({ queryKey: ['article', articleId] }),
      ])
    },
    onError: (error) => toast.error(error.message || 'Could not delete telemetry record'),
  })
  const pagination = calls.data?.pagination
  const pageCount = Math.max(1, pagination?.total_pages ?? 1)
  const currentPage = Math.min(page, pageCount)
  const pages = getPaginationPages(currentPage, pageCount)
  const rows = calls.data?.items ?? []

  useEffect(() => {
    if (page > pageCount) setPage(pageCount)
  }, [page, pageCount])

  return (
    <>
      <Card className="gap-0 overflow-hidden py-0">
        <CardHeader className="border-b py-6">
          <CardTitle>LLM telemetry</CardTitle>
          <CardDescription>Inspect model-call traces linked to this article and its pipeline runs.</CardDescription>
        </CardHeader>
        <CardContent className="px-0">
          <ScrollArea className="w-full">
            <Table className="min-w-[940px]">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead>Task</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Result</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>Time</TableHead>
                  <TableHead className="w-14 text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {calls.isPending ? Array.from({ length: 4 }, (_, index) => (
                  <TableRow key={index}>{Array.from({ length: 7 }, (_, cell) => <TableCell key={cell}><Skeleton className="h-5 w-full max-w-32" /></TableCell>)}</TableRow>
                )) : null}
                {calls.isError ? <TableRow><TableCell colSpan={7} className="h-32 text-center text-muted-foreground">LLM telemetry is temporarily unavailable.</TableCell></TableRow> : null}
                {!calls.isPending && !calls.isError && rows.length === 0 ? <TableRow><TableCell colSpan={7} className="h-32 text-center text-muted-foreground">No LLM calls recorded for this article yet.</TableCell></TableRow> : null}
                {!calls.isPending && !calls.isError ? rows.map((call) => (
                  <TableRow key={call.id} className="cursor-pointer" onClick={() => setSelected(call)}>
                    <TableCell className="font-medium">{call.task.replaceAll('_', ' ')}</TableCell>
                    <TableCell>{call.provider_id}</TableCell>
                    <TableCell>{call.model}<p className="mt-1 text-xs text-muted-foreground">{formatTokens(call)}</p></TableCell>
                    <TableCell>
                      <Badge variant={call.outcome === 'ok' ? 'outline' : call.outcome === 'running' ? 'secondary' : 'destructive'}>{call.outcome}</Badge>
                      {call.error_detail ? <p className="mt-1 max-w-sm line-clamp-1 text-xs text-destructive">{call.error_detail}</p> : null}
                    </TableCell>
                    <TableCell className="tabular-nums">{call.latency_ms == null ? 'In progress' : formatDuration(call.latency_ms)}{call.first_token_ms != null ? <p className="mt-1 text-xs text-muted-foreground">first token {formatDuration(call.first_token_ms)}</p> : null}</TableCell>
                    <TableCell className="text-muted-foreground">{telemetryDate.format(new Date(call.created_at))}</TableCell>
                    <TableCell className="text-right" onClick={(event) => event.stopPropagation()}>
                      <TelemetryActions call={call} onInspect={setSelected} onDelete={setDeleting} />
                    </TableCell>
                  </TableRow>
                )) : null}
              </TableBody>
            </Table>
            <ScrollBar orientation="horizontal" />
          </ScrollArea>
          {pagination ? (
            <div className="grid gap-4 border-t px-5 py-4 text-sm text-muted-foreground lg:grid-cols-[1fr_auto_1fr] lg:items-center">
              <div className="flex flex-wrap items-center justify-center gap-3 lg:justify-start">
                <span className="whitespace-nowrap">Rows per page</span>
                <Select value={String(perPage)} onValueChange={(value) => { if (value) { setPerPage(Number(value)); setPage(1) } }}>
                  <SelectTrigger size="sm" className="w-20" aria-label="Telemetry rows per page"><SelectValue /></SelectTrigger>
                  <SelectContent align="end">{[10, 25, 50].map((size) => <SelectItem key={size} value={String(size)}>{size}</SelectItem>)}</SelectContent>
                </Select>
                <span className="tabular-nums">{pageRange(currentPage, perPage, pagination.total)}</span>
              </div>
              <div className="flex flex-wrap items-center justify-center gap-1">
                <Button variant="ghost" size="icon-sm" aria-label="Previous telemetry page" disabled={currentPage <= 1} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft /></Button>
                {pages.map((number, index) => (
                  <Fragment key={number}>
                    {index > 0 && pages[index - 1] !== number - 1 ? <span className="px-1">…</span> : null}
                    <Button variant={number === currentPage ? 'outline' : 'ghost'} size="icon-sm" aria-label={`Telemetry page ${number}`} aria-current={number === currentPage ? 'page' : undefined} onClick={() => setPage(number)}>{number}</Button>
                  </Fragment>
                ))}
                <Button variant="ghost" size="icon-sm" aria-label="Next telemetry page" disabled={currentPage >= pageCount} onClick={() => setPage((current) => Math.min(pageCount, current + 1))}><ChevronRight /></Button>
              </div>
              <p className="text-center tabular-nums lg:text-right">Page {currentPage} of {pageCount}</p>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <TraceSheet call={selected} onOpenChange={(open) => { if (!open) setSelected(null) }} />
      <DeleteTelemetryDialog call={deleting} deleting={remove.isPending} onOpenChange={(open) => { if (!open) setDeleting(null) }} onConfirm={() => { if (deleting) remove.mutate(deleting) }} />
    </>
  )
}

function TelemetryActions({ call, onInspect, onDelete }: { call: LLMCall; onInspect: (call: LLMCall) => void; onDelete: (call: LLMCall) => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`Actions for ${call.task.replaceAll('_', ' ')} call`} />}><EllipsisVertical /></DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuItem onClick={() => onInspect(call)}><Eye />Inspect trace</DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onClick={() => onDelete(call)}><Trash2 />Delete record</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function TraceSheet({ call, onOpenChange }: { call: LLMCall | null; onOpenChange: (open: boolean) => void }) {
  return (
    <Sheet open={Boolean(call)} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-xl" side="right">
        <SheetHeader className="border-b pr-14">
          <div className="mb-2 flex size-10 items-center justify-center rounded-2xl bg-primary/10 text-primary"><RadioTower className="size-5" /></div>
          <SheetTitle>{call?.task.replaceAll('_', ' ') ?? 'LLM call'} trace</SheetTitle>
          <SheetDescription>Correlation, lifecycle, provider response, and failure details for this model call.</SheetDescription>
        </SheetHeader>
        {call ? <ScrollArea className="min-h-0 flex-1"><div className="space-y-6 p-6">
          <div className="flex items-center justify-between gap-3 rounded-2xl border bg-muted/20 p-4">
            <div><p className="text-xs uppercase tracking-wide text-muted-foreground">Result</p><p className="mt-1 font-medium capitalize">{call.outcome}</p></div>
            <Badge variant={call.outcome === 'ok' ? 'outline' : call.outcome === 'running' ? 'secondary' : 'destructive'}>{call.outcome}</Badge>
          </div>
          <section><h3 className="text-sm font-medium">Correlation</h3><dl className="mt-3 grid gap-3 sm:grid-cols-2"><TraceDetail label="Call ID" value={String(call.id)} mono /><TraceDetail label="Pipeline run" value={call.pipeline_run_id || 'Not linked'} mono /><TraceDetail label="Pipeline step" value={call.pipeline_step_id || 'Not linked'} mono /></dl></section>
          <section><h3 className="text-sm font-medium">Execution</h3><dl className="mt-3 grid gap-3 sm:grid-cols-2"><TraceDetail label="Provider" value={call.provider_id} /><TraceDetail label="Model" value={call.model} /><TraceDetail label="Transport" value={call.streamed ? 'Streaming' : 'Buffered'} /><TraceDetail label="Finish reason" value={call.finish_reason || 'Not reported'} /><TraceDetail label="Tokens" value={formatTokens(call)} /><TraceDetail label="First token" value={call.first_token_ms == null ? 'Not reported' : formatDuration(call.first_token_ms)} /></dl></section>
          <section><h3 className="flex items-center gap-2 text-sm font-medium"><Clock3 className="size-4" />Lifecycle</h3><div className="mt-3 grid gap-3 sm:grid-cols-2"><TraceDetail label="Started" value={telemetryDate.format(new Date(call.created_at))} /><TraceDetail label="Completed" value={call.completed_at ? telemetryDate.format(new Date(call.completed_at)) : 'Still running'} /><TraceDetail label="Total latency" value={call.latency_ms == null ? 'In progress' : formatDuration(call.latency_ms)} /></div></section>
          {call.error_detail ? <TracePayload title="Provider error" value={call.error_detail} error /> : null}
          {call.response_text ? <TracePayload title="Raw provider response" value={call.response_text} /> : <div className="rounded-2xl border border-dashed p-4 text-sm text-muted-foreground">No response body was recorded for this call.</div>}
        </div></ScrollArea> : null}
      </SheetContent>
    </Sheet>
  )
}

function TraceDetail({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <div className="min-w-0 rounded-xl border bg-background p-3"><dt className="text-xs text-muted-foreground">{label}</dt><dd className={`mt-1 break-words text-sm ${mono ? 'font-mono text-xs' : 'font-medium'}`}>{value}</dd></div>
}

function TracePayload({ title, value, error = false }: { title: string; value: string; error?: boolean }) {
  return <section><h3 className={`text-sm font-medium ${error ? 'text-destructive' : ''}`}>{title}</h3><ScrollArea className={`mt-3 h-64 rounded-xl border ${error ? 'border-destructive/30 bg-destructive/5' : 'border-zinc-700 bg-zinc-950'}`}><pre className={`min-w-full whitespace-pre-wrap break-words p-4 text-[11px] leading-5 ${error ? 'text-destructive' : 'text-zinc-200'}`}>{value}</pre></ScrollArea></section>
}

function DeleteTelemetryDialog({ call, deleting, onOpenChange, onConfirm }: { call: LLMCall | null; deleting: boolean; onOpenChange: (open: boolean) => void; onConfirm: () => void }) {
  return (
    <Dialog open={Boolean(call)} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={!deleting}>
        <DialogHeader>
          <div className="mb-1 flex size-10 items-center justify-center rounded-full bg-destructive/10 text-destructive"><TriangleAlert className="size-5" /></div>
          <DialogTitle>Delete this telemetry record?</DialogTitle>
          <DialogDescription>This permanently removes the selected model-call trace. The article and pipeline result are not deleted.</DialogDescription>
        </DialogHeader>
        {call ? <div className="rounded-2xl border bg-muted/20 p-4"><p className="font-medium capitalize">{call.task.replaceAll('_', ' ')}</p><p className="mt-1 text-xs text-muted-foreground">Call #{call.id} · {call.provider_id} / {call.model}</p></div> : null}
        <DialogFooter><Button variant="outline" disabled={deleting} onClick={() => onOpenChange(false)}>Cancel</Button><Button variant="destructive" disabled={deleting} onClick={onConfirm}>{deleting ? 'Deleting…' : 'Delete record'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function formatTokens(call: Pick<LLMCall, 'input_tokens' | 'output_tokens'>) {
  if (call.input_tokens == null && call.output_tokens == null) return 'Tokens unavailable'
  return `${call.input_tokens ?? 0} in · ${call.output_tokens ?? 0} out`
}

function formatDuration(milliseconds: number) {
  if (milliseconds < 1_000) return `${milliseconds}ms`
  return `${(milliseconds / 1_000).toFixed(milliseconds >= 10_000 ? 1 : 2)}s`
}

function pageRange(page: number, perPage: number, total: number) {
  if (total === 0) return '0 results'
  return `${(page - 1) * perPage + 1}–${Math.min(page * perPage, total)} of ${total}`
}
