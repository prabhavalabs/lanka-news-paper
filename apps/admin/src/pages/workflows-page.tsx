import {
  createClient,
  type AgentFeedback,
  type AgentWorkflow,
  type AgentWorkflowInput,
  type LlmModel,
  type NewsletterSettings,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Bot,
  BrainCircuit,
  CheckCircle2,
  CircleAlert,
  Clock3,
  MessageSquareText,
  Save,
  Search,
  ShieldCheck,
  Sparkles,
  ThumbsDown,
  ThumbsUp,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, type BlockerFunction, useBeforeUnload, useBlocker, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

const client = createClient()

type View = 'configuration' | 'feedback'
type WorkflowDraft = AgentWorkflowInput
type SaveSnapshot = {
  task: string
  workflow: WorkflowDraft
  newsletter: NewsletterSettings | null
}

const feedbackCategories: Array<AgentFeedback['category']> = ['accuracy', 'tone', 'relevance', 'formatting', 'safety', 'other']

function workflowDraftFrom(workflow: AgentWorkflow): WorkflowDraft {
  return {
    custom_instructions: workflow.custom_instructions,
    personality: workflow.personality,
    tone: workflow.tone,
    response_language: workflow.response_language,
    audience: workflow.audience,
    enabled: workflow.enabled,
    provider_id: workflow.provider_id,
    model: workflow.model,
  }
}

function draftsMatch(left: unknown, right: unknown) {
  return JSON.stringify(left) === JSON.stringify(right)
}

export function WorkflowsPage() {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const workflows = useQuery({ queryKey: ['agent-workflows'], queryFn: () => client.agentWorkflows() })
  const feedback = useQuery({ queryKey: ['agent-feedback'], queryFn: () => client.agentFeedback() })
  const newsletter = useQuery({ queryKey: ['newsletter-settings'], queryFn: () => client.newsletterSettings() })
  const provider = useQuery({ queryKey: ['llm-provider'], queryFn: () => client.llmProvider(), staleTime: 60_000 })
  const models = useQuery({ queryKey: ['llm-models'], queryFn: () => client.llmModels(), staleTime: 60_000 })
  const [view, setView] = useState<View>('configuration')
  const requestedTask = searchParams.get('workflow')
  const selected = useMemo(() => {
    const items = workflows.data?.items ?? []
    return items.find((item) => item.task === requestedTask) ?? items[0] ?? null
  }, [requestedTask, workflows.data?.items])
  const [draft, setDraft] = useState<WorkflowDraft | null>(null)
  const [savedDraft, setSavedDraft] = useState<WorkflowDraft | null>(null)
  const [newsletterDraft, setNewsletterDraft] = useState<NewsletterSettings | null>(null)
  const [savedNewsletterDraft, setSavedNewsletterDraft] = useState<NewsletterSettings | null>(null)
  const [modelPickerOpen, setModelPickerOpen] = useState(false)
  const [modelSearch, setModelSearch] = useState('')

  useEffect(() => {
    if (!selected) return
    const nextDraft = workflowDraftFrom(selected)
    setDraft(nextDraft)
    setSavedDraft(nextDraft)
  }, [selected])

  useEffect(() => {
    if (!newsletter.data) return
    setNewsletterDraft(newsletter.data)
    setSavedNewsletterDraft(newsletter.data)
  }, [newsletter.data])

  const hasWorkflowChanges = Boolean(draft && savedDraft && !draftsMatch(draft, savedDraft))
  const hasNewsletterChanges = Boolean(
    selected?.task === 'newsletter_editorial'
    && newsletterDraft
    && savedNewsletterDraft
    && !draftsMatch(newsletterDraft, savedNewsletterDraft),
  )
  const hasUnsavedChanges = hasWorkflowChanges || hasNewsletterChanges

  const blocker = useBlocker(useCallback<BlockerFunction>(
    ({ currentLocation, nextLocation }) => hasUnsavedChanges
      && `${currentLocation.pathname}${currentLocation.search}${currentLocation.hash}`
        !== `${nextLocation.pathname}${nextLocation.search}${nextLocation.hash}`,
    [hasUnsavedChanges],
  ))

  useBeforeUnload(useCallback((event) => {
    if (!hasUnsavedChanges) return
    event.preventDefault()
    event.returnValue = ''
  }, [hasUnsavedChanges]), { capture: true })

  const save = useMutation({
    mutationFn: async (snapshot: SaveSnapshot) => {
      const operations: Array<Promise<unknown>> = [client.updateAgentWorkflow(snapshot.task, snapshot.workflow)]
      if (snapshot.task === 'newsletter_editorial' && snapshot.newsletter) {
        operations.push(client.updateNewsletterSettings(snapshot.newsletter))
      }
      return Promise.all(operations)
    },
    onSuccess: async (_, snapshot) => {
      setSavedDraft(snapshot.workflow)
      if (snapshot.newsletter) setSavedNewsletterDraft(snapshot.newsletter)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['agent-workflows'] }),
        queryClient.invalidateQueries({ queryKey: ['newsletter-settings'] }),
        queryClient.invalidateQueries({ queryKey: ['newsletter-subscribers'] }),
        queryClient.invalidateQueries({ queryKey: ['llm-profiles'] }),
      ])
      toast.success('Workflow configuration saved and versioned')
    },
    onError: () => toast.error('Could not save workflow configuration'),
  })

  const saveSnapshot = () => {
    if (!selected || !draft) return null
    return {
      task: selected.task,
      workflow: draft,
      newsletter: selected.task === 'newsletter_editorial' ? newsletterDraft : null,
    } satisfies SaveSnapshot
  }

  const saveCurrent = () => {
    const snapshot = saveSnapshot()
    if (snapshot) save.mutate(snapshot)
  }

  const keepEditing = () => {
    if (blocker.state === 'blocked') blocker.reset()
  }

  const discardAndContinue = () => {
    if (blocker.state !== 'blocked') return
    setDraft(savedDraft)
    setNewsletterDraft(savedNewsletterDraft)
    blocker.proceed()
  }

  const saveAndContinue = async () => {
    if (blocker.state !== 'blocked') return
    const snapshot = saveSnapshot()
    if (!snapshot) return
    try {
      await save.mutateAsync(snapshot)
      blocker.proceed()
    } catch {
      // The mutation displays the error and keeps this dialog open for retrying.
    }
  }

  const chooseWorkflow = (task: string) => {
    setSearchParams({ workflow: task })
    setView('configuration')
  }

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <Badge variant="outline" className="mb-2"><Sparkles /> Autonomous operations</Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Workflow Manager</h1>
          <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
            Control system behavior, editorial personality, newsletter delivery, model routing, and reviewed learning feedback.
          </p>
        </div>
      </div>

      <div className="grid min-w-0 gap-5 xl:grid-cols-[19rem_minmax(0,1fr)]">
        <Card className="h-fit gap-0 overflow-hidden py-0 shadow-sm">
          <CardHeader className="border-b py-5">
            <CardTitle>Agent workflows</CardTitle>
            <CardDescription>Select a workflow to configure.</CardDescription>
          </CardHeader>
          <CardContent className="p-2">
            {workflows.isPending ? <div className="space-y-2 p-2"><Skeleton className="h-20" /><Skeleton className="h-20" /><Skeleton className="h-20" /></div> : null}
            {workflows.isError ? <p className="p-4 text-sm text-destructive">Workflows are unavailable.</p> : null}
            {workflows.data?.items.map((workflow) => (
              <button
                type="button"
                key={workflow.task}
                onClick={() => chooseWorkflow(workflow.task)}
                className={cn(
                  'flex w-full items-start gap-3 rounded-xl p-3 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                  selected?.task === workflow.task && 'bg-primary/8 ring-1 ring-primary/20',
                )}
              >
                <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  {workflow.task === 'newsletter_editorial' ? <Bot className="size-4" /> : <BrainCircuit className="size-4" />}
                </span>
                <span className="min-w-0">
                  <span className="block font-medium">{workflow.name}</span>
                  <span className="mt-0.5 block text-xs text-muted-foreground">{workflow.category} · revision {workflow.revision}</span>
                </span>
              </button>
            ))}
          </CardContent>
        </Card>

        <div className="min-w-0 space-y-5">
          {selected && draft ? (
            <>
              <Card className="gap-0 py-0 shadow-sm">
                <CardHeader className="border-b py-5">
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <CardTitle>{selected.name}</CardTitle>
                        <Badge variant={draft.enabled ? 'secondary' : 'outline'}>{draft.enabled ? 'Admin behavior active' : 'Base behavior only'}</Badge>
                      </div>
                      <CardDescription className="mt-1 max-w-2xl">{selected.purpose}</CardDescription>
                    </div>
                    <Button
                      variant={draft.enabled ? 'outline' : 'default'}
                      onClick={() => setDraft({ ...draft, enabled: !draft.enabled })}
                    >
                      {draft.enabled ? 'Use base behavior only' : 'Enable admin behavior'}
                    </Button>
                  </div>
                  <div className="mt-4 flex gap-1 rounded-lg bg-muted p-1">
                    <Button size="sm" variant={view === 'configuration' ? 'default' : 'ghost'} onClick={() => setView('configuration')}>Configuration</Button>
                    <Button size="sm" variant={view === 'feedback' ? 'default' : 'ghost'} onClick={() => setView('feedback')}>
                      Feedback
                      {(feedback.data?.items.filter((item) => item.workflow_task === selected.task && item.status === 'new').length ?? 0) > 0 ? (
                        <Badge variant="secondary">{feedback.data?.items.filter((item) => item.workflow_task === selected.task && item.status === 'new').length}</Badge>
                      ) : null}
                    </Button>
                  </div>
                </CardHeader>
                {view === 'configuration' ? (
                  <CardContent className="space-y-7 p-6">
                    <div className="grid gap-5 md:grid-cols-3">
                      <ReadOnlyMetric label="AI provider" value={(provider.data?.name ?? selected.provider_id) || 'Not routed'} />
                      <ReadOnlyMetric label="Configuration revision" value={String(selected.revision)} />
                      <ReadOnlyMetric label="Last updated" value={new Date(selected.updated_at).toLocaleString()} />
                    </div>

                    <AIConfiguration
                      task={selected.task}
                      draft={draft}
                      onChange={setDraft}
                      provider={provider.data}
                      models={models.data?.items ?? []}
                      loading={provider.isPending || models.isPending}
                      error={provider.isError || models.isError}
                      pickerOpen={modelPickerOpen}
                      onPickerOpen={setModelPickerOpen}
                      search={modelSearch}
                      onSearch={setModelSearch}
                    />

                    <FieldGroup>
                      <Field>
                        <FieldLabel htmlFor="workflow-instructions">System behavior and instructions</FieldLabel>
                        <Textarea
                          id="workflow-instructions"
                          className="min-h-40 resize-y"
                          value={draft.custom_instructions}
                          placeholder="Add task-specific editorial rules, priorities, exclusions, and output expectations…"
                          onChange={(event) => setDraft({ ...draft, custom_instructions: event.target.value })}
                        />
                        <p className="text-xs text-muted-foreground">Applied after the task’s tested base prompt and saved as a versioned administrative overlay.</p>
                      </Field>
                      <Field>
                        <FieldLabel htmlFor="workflow-personality">Personality</FieldLabel>
                        <Textarea
                          id="workflow-personality"
                          className="min-h-24 resize-y"
                          value={draft.personality}
                          onChange={(event) => setDraft({ ...draft, personality: event.target.value })}
                        />
                      </Field>
                      <div className="grid gap-4 md:grid-cols-3">
                        <Field><FieldLabel htmlFor="workflow-tone">Tone</FieldLabel><Input id="workflow-tone" value={draft.tone} onChange={(event) => setDraft({ ...draft, tone: event.target.value })} /></Field>
                        <Field><FieldLabel htmlFor="workflow-language">Response language</FieldLabel><Input id="workflow-language" value={draft.response_language} onChange={(event) => setDraft({ ...draft, response_language: event.target.value })} /></Field>
                        <Field><FieldLabel htmlFor="workflow-audience">Audience</FieldLabel><Input id="workflow-audience" value={draft.audience} onChange={(event) => setDraft({ ...draft, audience: event.target.value })} /></Field>
                      </div>
                    </FieldGroup>

                    {selected.task === 'newsletter_editorial' ? (
                      <NewsletterConfiguration value={newsletterDraft} onChange={setNewsletterDraft} loading={newsletter.isPending} />
                    ) : null}

                    {selected.learning_notes ? (
                      <div className="rounded-xl border bg-muted/30 p-4">
                        <p className="text-sm font-medium">Administrator-approved learning notes</p>
                        <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">{selected.learning_notes}</p>
                      </div>
                    ) : null}

                    <div className="flex items-start gap-3 rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-4">
                      <ShieldCheck className="mt-0.5 size-5 shrink-0 text-emerald-700 dark:text-emerald-400" />
                      <div><p className="text-sm font-medium">Locked safety layer</p><p className="mt-1 text-xs leading-relaxed text-muted-foreground">Source text is always treated as untrusted data. The agent cannot fabricate facts or remove schema, factuality, or prompt-injection protections from this screen.</p></div>
                    </div>
                  </CardContent>
                ) : (
                  <FeedbackPanel workflow={selected} items={feedback.data?.items ?? []} />
                )}
              </Card>

              {view === 'configuration' ? (
                <div className="flex flex-wrap items-center justify-end gap-3">
                  {hasUnsavedChanges ? <Badge variant="secondary"><CircleAlert /> Unsaved changes</Badge> : null}
                  <Button size="lg" disabled={save.isPending || newsletter.isError} onClick={saveCurrent}>
                    <Save /> {save.isPending ? 'Saving…' : 'Save and create revision'}
                  </Button>
                </div>
              ) : null}
            </>
          ) : workflows.isPending ? <Skeleton className="h-[640px] w-full" /> : null}
        </div>
      </div>

      <Dialog open={blocker.state === 'blocked'} onOpenChange={(open) => { if (!open && !save.isPending) keepEditing() }}>
        <DialogContent showCloseButton={!save.isPending}>
          <DialogHeader>
            <DialogTitle>You have unsaved changes</DialogTitle>
            <DialogDescription>
              Save your changes to {selected?.name ?? 'this workflow'} before leaving, or discard them and continue.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" disabled={save.isPending} onClick={keepEditing}>Keep editing</Button>
            <Button variant="destructive" disabled={save.isPending} onClick={discardAndContinue}>Discard and continue</Button>
            <Button disabled={save.isPending || newsletter.isError} onClick={saveAndContinue}>
              <Save /> {save.isPending ? 'Saving…' : 'Save and continue'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}

function AIConfiguration({ task, draft, onChange, provider, models, loading, error, pickerOpen, onPickerOpen, search, onSearch }: {
  task: string
  draft: WorkflowDraft
  onChange: (value: WorkflowDraft) => void
  provider: Awaited<ReturnType<typeof client.llmProvider>> | undefined
  models: LlmModel[]
  loading: boolean
  error: boolean
  pickerOpen: boolean
  onPickerOpen: (open: boolean) => void
  search: string
  onSearch: (value: string) => void
}) {
  const selectedModel = models.find((item) => item.id === draft.model)
  const term = search.trim().toLowerCase()
  const compatible = models.filter((model) => model.compatible_tasks.includes(task) && (!term || model.name.toLowerCase().includes(term) || model.id.toLowerCase().includes(term)))
  return (
    <div className="space-y-4 rounded-2xl border bg-muted/20 p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div><h3 className="font-heading font-semibold">AI provider and model</h3><p className="mt-1 text-xs text-muted-foreground">The assignment is versioned with this workflow and used by its next autonomous run.</p></div>
        {provider ? <Badge variant="outline" className={cn(provider.available ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' : 'border-destructive/30 bg-destructive/10 text-destructive')}>{provider.available ? <CheckCircle2 /> : <CircleAlert />}{provider.status}</Badge> : null}
      </div>
      {error ? <p className="text-sm text-destructive">The live model catalog is temporarily unavailable. The current assignment remains unchanged.</p> : null}
      <div className="grid gap-4 md:grid-cols-[minmax(0,0.8fr)_minmax(0,1.6fr)]">
        <Field>
          <FieldLabel htmlFor="workflow-provider">Provider</FieldLabel>
          <Select value={draft.provider_id} disabled={loading || error} onValueChange={(value) => value && onChange({ ...draft, provider_id: value })}>
            <SelectTrigger id="workflow-provider"><SelectValue>{() => provider?.name ?? 'OpenRouter'}</SelectValue></SelectTrigger>
            <SelectContent><SelectItem value="openrouter">OpenRouter</SelectItem></SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">{provider?.key_set ? 'API key configured' : 'API key missing'}</p>
        </Field>
        <Field>
          <FieldLabel>Model</FieldLabel>
          <div className="flex min-h-10 flex-col justify-center rounded-md border bg-background px-3 py-2"><p className="text-sm font-medium">{(selectedModel?.name ?? draft.model) || 'No model assigned'}</p>{draft.model ? <p className="break-all text-xs text-muted-foreground">{draft.model}</p> : null}</div>
          <Button type="button" variant="outline" className="w-fit" disabled={loading || error || !provider?.available} onClick={() => { onSearch(''); onPickerOpen(true) }}>Change model</Button>
        </Field>
      </div>
      <Sheet open={pickerOpen} onOpenChange={onPickerOpen}>
        <SheetContent side="right" style={{ width: '100%', maxWidth: '36rem' }}>
          <SheetHeader className="border-b"><SheetTitle>Choose a workflow model</SheetTitle><SheetDescription>Only models compatible with this autonomous task are shown.</SheetDescription></SheetHeader>
          <div className="border-b px-6 pb-4"><div className="relative"><Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" type="search" placeholder="Search models…" value={search} onChange={(event) => onSearch(event.target.value)} /></div><p className="mt-2 text-xs text-muted-foreground">{compatible.length} compatible models</p></div>
          <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-4" role="radiogroup" aria-label="Compatible AI models">
            {compatible.slice(0, 75).map((model) => <button key={model.id} type="button" role="radio" aria-checked={draft.model === model.id} onClick={() => { onChange({ ...draft, provider_id: 'openrouter', model: model.id }); onPickerOpen(false) }} className={cn('w-full rounded-xl border p-4 text-left outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring', draft.model === model.id && 'border-primary bg-primary/5 ring-1 ring-primary')}><p className="font-medium">{model.name}</p><p className="mt-1 break-all text-xs text-muted-foreground">{model.id}</p><p className="mt-3 text-xs text-muted-foreground">{model.context_length.toLocaleString()} context · ${model.input_price_per_million.toLocaleString()}/1M input</p></button>)}
            {compatible.length === 0 ? <p className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">No compatible model matches this search.</p> : null}
          </div>
          <SheetFooter className="border-t"><Button variant="outline" onClick={() => onPickerOpen(false)}>Cancel</Button></SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  )
}

function NewsletterConfiguration({ value, onChange, loading }: { value: NewsletterSettings | null; onChange: (value: NewsletterSettings) => void; loading: boolean }) {
  if (loading || !value) return <Skeleton className="h-80 w-full" />
  return (
    <div className="space-y-5 border-t pt-7">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div><h3 className="font-heading font-semibold">Email delivery and template</h3><p className="mt-1 text-xs text-muted-foreground">The hourly coordinator reads these settings and sends once the configured local delivery time is due.</p></div>
        <Button variant={value.enabled ? 'default' : 'outline'} onClick={() => onChange({ ...value, enabled: !value.enabled })}>{value.enabled ? <CheckCircle2 /> : <Clock3 />}{value.enabled ? 'Delivery enabled' : 'Delivery paused'}</Button>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Field><FieldLabel htmlFor="newsletter-timezone">Timezone</FieldLabel><Input id="newsletter-timezone" value={value.timezone} onChange={(event) => onChange({ ...value, timezone: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="newsletter-hour">Send hour (0–23)</FieldLabel><Input id="newsletter-hour" type="number" min={0} max={23} value={value.send_hour} onChange={(event) => onChange({ ...value, send_hour: Number(event.target.value) })} /></Field>
        <Field><FieldLabel htmlFor="newsletter-max">Maximum stories</FieldLabel><Input id="newsletter-max" type="number" min={1} max={50} value={value.max_stories} onChange={(event) => onChange({ ...value, max_stories: Number(event.target.value) })} /></Field>
        <Field><FieldLabel htmlFor="newsletter-leads">Lead stories</FieldLabel><Input id="newsletter-leads" type="number" min={1} max={10} value={value.lead_story_count} onChange={(event) => onChange({ ...value, lead_story_count: Number(event.target.value) })} /></Field>
      </div>
      <FieldGroup>
        <Field><FieldLabel htmlFor="newsletter-subject">Subject template</FieldLabel><Input id="newsletter-subject" value={value.subject_template} onChange={(event) => onChange({ ...value, subject_template: event.target.value })} /><TemplateTokens /></Field>
        <Field><FieldLabel htmlFor="newsletter-preheader">Preheader template</FieldLabel><Input id="newsletter-preheader" value={value.preheader_template} onChange={(event) => onChange({ ...value, preheader_template: event.target.value })} /><TemplateTokens /></Field>
        <Field><FieldLabel htmlFor="newsletter-intro">Fallback introduction</FieldLabel><Textarea id="newsletter-intro" value={value.intro_text} onChange={(event) => onChange({ ...value, intro_text: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="newsletter-footer">Footer disclosure</FieldLabel><Textarea id="newsletter-footer" value={value.footer_text} onChange={(event) => onChange({ ...value, footer_text: event.target.value })} /></Field>
      </FieldGroup>
      <div className="flex flex-col gap-3 rounded-xl border bg-muted/20 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div><p className="text-sm font-medium">Validate before the next delivery</p><p className="mt-1 text-xs text-muted-foreground">Generate a real preview with the saved workflow or send one isolated test email.</p></div>
        <Button variant="outline" nativeButton={false} render={<Link to="/mailing-list?test=1" />}>Open newsletter test lab</Button>
      </div>
    </div>
  )
}

function TemplateTokens() {
  return <p className="text-xs text-muted-foreground">Tokens: {'{{date}} {{articles}} {{events}} {{sources}}'}</p>
}

function FeedbackPanel({ workflow, items }: { workflow: AgentWorkflow; items: AgentFeedback[] }) {
  const queryClient = useQueryClient()
  const [rating, setRating] = useState<AgentFeedback['rating']>('needs_improvement')
  const [category, setCategory] = useState<AgentFeedback['category']>('tone')
  const [message, setMessage] = useState('')
  const relevant = items.filter((item) => item.workflow_task === workflow.task)
  const create = useMutation({
    mutationFn: () => client.createAgentFeedback({ workflow_task: workflow.task, rating, category, message }),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['agent-feedback'] }); setMessage(''); toast.success('Feedback added for review') },
    onError: () => toast.error('Could not save feedback'),
  })
  const review = useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'applied' | 'dismissed' }) => client.reviewAgentFeedback(id, status),
    onSuccess: async (_, variables) => {
      await Promise.all([queryClient.invalidateQueries({ queryKey: ['agent-feedback'] }), queryClient.invalidateQueries({ queryKey: ['agent-workflows'] })])
      toast.success(variables.status === 'applied' ? 'Feedback applied to versioned learning notes' : 'Feedback dismissed')
    },
    onError: () => toast.error('Could not review feedback'),
  })
  return (
    <CardContent className="space-y-6 p-6">
      <div className="rounded-xl border p-5">
        <div className="flex items-start gap-3"><MessageSquareText className="mt-0.5 size-5 text-primary" /><div><p className="font-medium">Add human feedback</p><p className="mt-1 text-xs text-muted-foreground">Raw feedback never changes behavior automatically. Apply it explicitly after review.</p></div></div>
        <div className="mt-5 grid gap-4 sm:grid-cols-2">
          <Field><FieldLabel>Assessment</FieldLabel><Select value={rating} onValueChange={(value) => setRating(value as AgentFeedback['rating'])}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="needs_improvement">Needs improvement</SelectItem><SelectItem value="helpful">Helpful result</SelectItem></SelectContent></Select></Field>
          <Field><FieldLabel>Category</FieldLabel><Select value={category} onValueChange={(value) => setCategory(value as AgentFeedback['category'])}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{feedbackCategories.map((item) => <SelectItem key={item} value={item}>{item.charAt(0).toUpperCase() + item.slice(1)}</SelectItem>)}</SelectContent></Select></Field>
        </div>
        <Field className="mt-4"><FieldLabel htmlFor="workflow-feedback">Feedback and desired correction</FieldLabel><Textarea id="workflow-feedback" className="min-h-28" value={message} onChange={(event) => setMessage(event.target.value)} placeholder="Describe what worked or what the agent should do differently…" /></Field>
        <div className="mt-4 flex justify-end"><Button disabled={message.trim().length < 3 || create.isPending} onClick={() => create.mutate()}>{create.isPending ? 'Adding…' : 'Add feedback'}</Button></div>
      </div>
      <div className="space-y-3">
        <h3 className="font-heading font-semibold">Feedback history</h3>
        {relevant.length === 0 ? <p className="rounded-xl border border-dashed p-6 text-center text-sm text-muted-foreground">No feedback has been recorded for this workflow.</p> : null}
        {relevant.map((item) => (
          <article key={item.id} className="rounded-xl border p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="flex items-center gap-2">{item.rating === 'helpful' ? <ThumbsUp className="size-4 text-emerald-600" /> : <ThumbsDown className="size-4 text-amber-600" />}<Badge variant="outline">{item.category}</Badge><Badge variant={item.status === 'new' ? 'secondary' : 'outline'}>{item.status}</Badge></div>
              <span className="text-xs text-muted-foreground">{new Date(item.created_at).toLocaleString()}</span>
            </div>
            <p className="mt-3 whitespace-pre-wrap text-sm leading-relaxed">{item.message}</p>
            {item.status === 'new' || item.status === 'reviewed' ? <div className="mt-4 flex justify-end gap-2"><Button size="sm" variant="outline" onClick={() => review.mutate({ id: item.id, status: 'dismissed' })}>Dismiss</Button><Button size="sm" onClick={() => review.mutate({ id: item.id, status: 'applied' })}>Apply to learning notes</Button></div> : null}
          </article>
        ))}
      </div>
    </CardContent>
  )
}

function ReadOnlyMetric({ label, value }: { label: string; value: string }) {
  return <div className="rounded-xl border p-4"><p className="text-xs text-muted-foreground">{label}</p><p className="mt-1 break-all text-sm font-medium">{value}</p></div>
}
