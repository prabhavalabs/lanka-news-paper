import { createClient, type LlmModel, type LlmProfile } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BrainCircuit,
  CheckCircle2,
  CircleAlert,
  Clock3,
  Cpu,
  KeyRound,
  RefreshCw,
  Route,
  Search,
  Shapes,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

const client = createClient()

export function RoutingPage() {
  const queryClient = useQueryClient()
  const [selectedTask, setSelectedTask] = useState<LlmProfile | null>(null)
  const [selectedModel, setSelectedModel] = useState('')
  const [search, setSearch] = useState('')
  const provider = useQuery({
    queryKey: ['llm-provider'],
    queryFn: () => client.llmProvider(),
    staleTime: 60_000,
  })
  const models = useQuery({
    queryKey: ['llm-models'],
    queryFn: () => client.llmModels(),
    staleTime: 60_000,
  })
  const profiles = useQuery({
    queryKey: ['llm-profiles'],
    queryFn: () => client.llmProfiles(),
  })
  const save = useMutation({
    mutationFn: () => client.updateLlmProfile({ task: selectedTask?.task ?? '', model: selectedModel }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['llm-profiles'] })
      toast.success('Model assignment updated')
      setSelectedTask(null)
    },
    onError: () => toast.error('Could not update the model assignment'),
  })
  const compatibleModels = useMemo(() => {
    if (!selectedTask) return []
    const query = search.trim().toLowerCase()
    return (models.data?.items ?? []).filter(
      (model) =>
        (model.compatible_tasks ?? []).includes(selectedTask.task) &&
        (!query || model.name.toLowerCase().includes(query) || model.id.toLowerCase().includes(query)),
    ).sort((left, right) => Number(right.id === selectedModel) - Number(left.id === selectedModel))
  }, [models.data?.items, search, selectedModel, selectedTask])
  const visibleModels = compatibleModels.slice(0, 50)
  const refreshing = provider.isFetching || models.isFetching || profiles.isFetching

  const refresh = () => {
    void Promise.all([provider.refetch(), models.refetch(), profiles.refetch()])
  }

  const chooseModel = (profile: LlmProfile) => {
    setSelectedTask(profile)
    setSelectedModel(profile.model)
    setSearch('')
  }

  return (
    <section className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <Badge variant="outline" className="mb-2">
            <Cpu />
            AI operations
          </Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">AI routing</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Monitor OpenRouter and assign one model to each AI task.
          </p>
        </div>
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span>{provider.data ? `Last checked ${checkedLabel(provider.data.checked_at)}` : 'Checking provider…'}</span>
          <Button variant="outline" size="icon" onClick={refresh} disabled={refreshing} aria-label="Refresh AI routing">
            <RefreshCw className={cn(refreshing && 'animate-spin')} />
          </Button>
        </div>
      </div>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Provider status</CardTitle>
          <CardDescription>OpenRouter is the only inference provider enabled for AI tasks.</CardDescription>
        </CardHeader>
        <CardContent className="p-6">
          {provider.isPending ? <ProviderSkeleton /> : null}
          {provider.isError ? <InlineError message="Provider status is temporarily unavailable." /> : null}
          {provider.data ? (
            <div className="flex flex-col gap-6 lg:flex-row lg:flex-wrap lg:items-center">
              <div className="flex items-center gap-4 lg:min-w-44">
                <span className="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                  <Route className="size-6" />
                </span>
                <div>
                  <p className="font-heading font-semibold">{provider.data.name}</p>
                  <HealthBadge available={provider.data.available} status={provider.data.status} />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-6 lg:flex lg:flex-1 lg:items-center">
                <ProviderMetric icon={KeyRound} label="API key" value={provider.data.key_set ? 'Configured' : 'Missing'} />
                <ProviderMetric icon={CheckCircle2} label="Availability" value={provider.data.available ? 'Available' : 'Unavailable'} />
                <ProviderMetric icon={Shapes} label="Catalog" value={models.data ? `${models.data.items.length} text models` : models.isError ? 'Unavailable' : 'Loading…'} />
                <ProviderMetric icon={Clock3} label="Latency" value={`${provider.data.latency_ms} ms`} />
                <ProviderMetric icon={BrainCircuit} label="Access" value={provider.data.free_tier ? 'Free tier' : 'Paid account'} />
              </div>
              {!provider.data.available ? <p className="w-full text-xs text-destructive">{provider.data.status_detail}</p> : null}
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Task assignments</CardTitle>
          <CardDescription>One active OpenRouter model per AI task.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {profiles.isPending ? <TaskSkeleton /> : null}
          {profiles.isError ? <div className="p-6"><InlineError message="Task assignments are temporarily unavailable." /></div> : null}
          {profiles.data?.items.map((profile) => {
            const model = models.data?.items.find((item) => item.id === profile.model)
            const modelAvailable = Boolean(model?.compatible_tasks?.includes(profile.task))
            return (
              <div key={profile.task} className="flex flex-col gap-5 border-b p-6 last:border-b-0 lg:flex-row lg:items-center">
                <div className="flex items-start gap-4 lg:flex-1">
                  <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                    {profile.task === 'classify' ? <Shapes /> : <BrainCircuit />}
                  </span>
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-heading font-semibold">{profile.name}</p>
                      <Badge variant={profile.enabled ? 'secondary' : 'outline'}>{profile.enabled ? 'Active' : 'Inactive'}</Badge>
                    </div>
                    <p className="mt-1 max-w-md text-sm text-muted-foreground">{profile.purpose}</p>
                  </div>
                </div>
                <div className="lg:w-32"><TaskValue label="Provider" value="OpenRouter" /></div>
                <div className="lg:flex-1">
                  <p className="text-xs font-medium text-muted-foreground">Assigned model</p>
                  <p className="mt-1 font-medium">{model?.name ?? profile.model}</p>
                  <p className="mt-0.5 break-all text-xs text-muted-foreground">{profile.model}</p>
                  <div className="mt-2">
                    {models.isPending ? <Badge variant="outline">Checking model…</Badge> : <HealthBadge available={modelAvailable} status={modelAvailable ? 'Model available' : 'Model unavailable'} />}
                  </div>
                </div>
                <Button variant="outline" onClick={() => chooseModel(profile)} disabled={!provider.data?.available || models.isError || models.isPending}>
                  Change model
                </Button>
              </div>
            )
          })}
        </CardContent>
      </Card>

      <Sheet open={selectedTask !== null} onOpenChange={(open) => { if (!open) setSelectedTask(null) }}>
        <SheetContent
          side="right"
          style={{ width: '100%', maxWidth: '36rem' }}
          className="routing-model-sheet"
        >
          <SheetHeader className="border-b">
            <SheetTitle>Choose model for {selectedTask?.name}</SheetTitle>
            <SheetDescription>Select a compatible model from the live OpenRouter catalog.</SheetDescription>
          </SheetHeader>
          <div className="border-b px-6 pb-4">
            <label htmlFor="model-search" className="sr-only">Search OpenRouter models</label>
            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input id="model-search" className="pl-9" placeholder="Search OpenRouter models…" value={search} onChange={(event) => setSearch(event.target.value)} />
            </div>
            <p className="mt-2 text-xs text-muted-foreground">
              Showing {Math.min(compatibleModels.length, 50)} of {compatibleModels.length} compatible models
            </p>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4" role="radiogroup" aria-label="OpenRouter models">
            <div className="flex flex-col gap-3">
              {visibleModels.map((model) => (
                <ModelOption key={model.id} model={model} selected={selectedModel === model.id} onSelect={() => setSelectedModel(model.id)} />
              ))}
              {compatibleModels.length === 0 ? <div className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">No compatible models match this search.</div> : null}
            </div>
          </div>
          <SheetFooter className="border-t sm:flex-row sm:justify-end">
            <Button variant="outline" onClick={() => setSelectedTask(null)}>Cancel</Button>
            <Button onClick={() => save.mutate()} disabled={!selectedModel || save.isPending}>{save.isPending ? 'Saving…' : 'Save assignment'}</Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </section>
  )
}

function ProviderMetric({ icon: Icon, label, value }: { icon: typeof Clock3; label: string; value: string }) {
  return (
    <div className="flex items-start gap-2 lg:flex-1">
      <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div><p className="text-xs text-muted-foreground">{label}</p><p className="mt-1 text-sm font-medium">{value}</p></div>
    </div>
  )
}

function TaskValue({ label, value }: { label: string; value: string }) {
  return <div><p className="text-xs font-medium text-muted-foreground">{label}</p><p className="mt-1 text-sm font-medium">{value}</p></div>
}

function HealthBadge({ available, status }: { available: boolean; status: string }) {
  return (
    <Badge variant="outline" className={cn('mt-1.5', available ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' : 'border-destructive/30 bg-destructive/10 text-destructive')}>
      {available ? <CheckCircle2 /> : <CircleAlert />}{status}
    </Badge>
  )
}

function ModelOption({ model, selected, onSelect }: { model: LlmModel; selected: boolean; onSelect: () => void }) {
  return (
    <button type="button" role="radio" aria-checked={selected} onClick={onSelect} className={cn('rounded-2xl border p-4 text-left transition-colors outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring', selected && 'border-primary bg-primary/5 ring-1 ring-primary')}>
      <div className="flex items-start gap-3">
        <span className={cn('mt-1 flex size-5 shrink-0 items-center justify-center rounded-full border', selected && 'border-primary')} aria-hidden="true">{selected ? <span className="size-2.5 rounded-full bg-primary" /> : null}</span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div><p className="font-heading font-semibold">{model.name}</p><p className="mt-0.5 break-all text-xs text-muted-foreground">{model.id}</p></div>
            <span className="flex items-center gap-1.5 text-xs text-emerald-700 dark:text-emerald-400"><CheckCircle2 className="size-3.5" /> Available</span>
          </div>
          <div className="mt-4 grid grid-cols-3 gap-3 border-t pt-3">
            <ModelMetric label="Input / 1M" value={formatPrice(model.input_price_per_million)} />
            <ModelMetric label="Output / 1M" value={formatPrice(model.output_price_per_million)} />
            <ModelMetric label="Context" value={formatContext(model.context_length)} />
          </div>
          <div className="mt-3 flex flex-wrap gap-1.5">{modelCapabilities(model).map((capability) => <Badge key={capability} variant="secondary">{capability}</Badge>)}</div>
        </div>
      </div>
    </button>
  )
}

function ModelMetric({ label, value }: { label: string; value: string }) {
  return <div><p className="text-[11px] text-muted-foreground">{label}</p><p className="mt-1 text-sm font-medium tabular-nums">{value}</p></div>
}

function modelCapabilities(model: LlmModel) {
  const capabilities = ['Text']
  if (model.supported_parameters?.includes('structured_outputs')) capabilities.push('Structured output')
  if (model.supported_parameters?.includes('reasoning')) capabilities.push('Reasoning')
  if (model.supported_parameters?.includes('tools')) capabilities.push('Tools')
  return capabilities.slice(0, 4)
}

function formatPrice(value: number) {
  if (value === 0) return 'Free'
  if (value < 0.01) return '<$0.01'
  return `$${value.toLocaleString(undefined, { maximumFractionDigits: 2 })}`
}

function formatContext(value: number) {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2).replace(/\.00$/, '')}M`
  if (value >= 1_000) return `${Math.round(value / 1_000)}K`
  return String(value)
}

function checkedLabel(value: string) {
  const elapsed = Date.now() - new Date(value).getTime()
  if (elapsed < 60_000) return 'just now'
  return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function ProviderSkeleton() {
  return <div className="grid gap-4 sm:grid-cols-3">{Array.from({ length: 6 }, (_, index) => <Skeleton key={index} className="h-12 w-full" />)}</div>
}

function TaskSkeleton() {
  return <div className="flex flex-col gap-4 p-6"><Skeleton className="h-28 w-full" /><Skeleton className="h-28 w-full" /></div>
}

function InlineError({ message }: { message: string }) {
  return <div className="flex items-center gap-2 rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive"><CircleAlert className="size-4" />{message}</div>
}
