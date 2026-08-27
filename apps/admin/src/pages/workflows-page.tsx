import {
  createClient,
  type AgentFeedback,
  type AgentWorkflow,
  type AgentWorkflowInput,
  type NewsletterSettings,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Bot,
  BrainCircuit,
  CheckCircle2,
  Clock3,
  MessageSquareText,
  Route,
  Save,
  ShieldCheck,
  Sparkles,
  ThumbsDown,
  ThumbsUp,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

const client = createClient()

type View = 'configuration' | 'feedback'
type WorkflowDraft = AgentWorkflowInput

const feedbackCategories: Array<AgentFeedback['category']> = ['accuracy', 'tone', 'relevance', 'formatting', 'safety', 'other']

export function WorkflowsPage() {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const workflows = useQuery({ queryKey: ['agent-workflows'], queryFn: () => client.agentWorkflows() })
  const feedback = useQuery({ queryKey: ['agent-feedback'], queryFn: () => client.agentFeedback() })
  const newsletter = useQuery({ queryKey: ['newsletter-settings'], queryFn: () => client.newsletterSettings() })
  const [view, setView] = useState<View>('configuration')
  const requestedTask = searchParams.get('workflow')
  const selected = useMemo(() => {
    const items = workflows.data?.items ?? []
    return items.find((item) => item.task === requestedTask) ?? items[0] ?? null
  }, [requestedTask, workflows.data?.items])
  const [draft, setDraft] = useState<WorkflowDraft | null>(null)
  const [newsletterDraft, setNewsletterDraft] = useState<NewsletterSettings | null>(null)

  useEffect(() => {
    if (!selected) return
    setDraft({
      custom_instructions: selected.custom_instructions,
      personality: selected.personality,
      tone: selected.tone,
      response_language: selected.response_language,
      audience: selected.audience,
      enabled: selected.enabled,
    })
  }, [selected])

  useEffect(() => {
    if (newsletter.data) setNewsletterDraft(newsletter.data)
  }, [newsletter.data])

  const save = useMutation({
    mutationFn: async () => {
      if (!selected || !draft) throw new Error('No workflow selected')
      const operations: Array<Promise<unknown>> = [client.updateAgentWorkflow(selected.task, draft)]
      if (selected.task === 'newsletter_editorial' && newsletterDraft) {
        operations.push(client.updateNewsletterSettings(newsletterDraft))
      }
      return Promise.all(operations)
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['agent-workflows'] }),
        queryClient.invalidateQueries({ queryKey: ['newsletter-settings'] }),
        queryClient.invalidateQueries({ queryKey: ['newsletter-subscribers'] }),
      ])
      toast.success('Workflow configuration saved and versioned')
    },
    onError: () => toast.error('Could not save workflow configuration'),
  })

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
        <Button variant="outline" render={<Link to="/routing" />}><Route /> Models &amp; routing</Button>
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
                      <ReadOnlyMetric label="Current model" value={selected.model || 'Deterministic / not routed'} />
                      <ReadOnlyMetric label="Configuration revision" value={String(selected.revision)} />
                      <ReadOnlyMetric label="Last updated" value={new Date(selected.updated_at).toLocaleString()} />
                    </div>

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
                <div className="flex justify-end">
                  <Button size="lg" disabled={save.isPending || newsletter.isError} onClick={() => save.mutate()}>
                    <Save /> {save.isPending ? 'Saving…' : 'Save and create revision'}
                  </Button>
                </div>
              ) : null}
            </>
          ) : workflows.isPending ? <Skeleton className="h-[640px] w-full" /> : null}
        </div>
      </div>
    </section>
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
