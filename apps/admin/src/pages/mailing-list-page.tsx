import {
  createClient,
  type NewsletterSubscriber,
  type NewsletterSubscriberStatus,
  type NewsletterTestInput,
  type NewsletterTestResult,
  type NewsletterTestRun,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Bot,
  CheckCircle2,
  Clock3,
  EllipsisVertical,
  Eye,
  FlaskConical,
  History,
  Mail,
  PauseCircle,
  Pencil,
  PlayCircle,
  Plus,
  Trash2,
  ShieldCheck,
  Settings2,
  Send,
  UserRoundX,
  UsersRound,
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Combobox, ComboboxContent, ComboboxEmpty, ComboboxInput, ComboboxItem, ComboboxList } from '@/components/ui/combobox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useDebouncedValue } from '@/hooks/use-debounced-value'
import { formatDateTime } from '@/lib/date-time'
import {
  filterNewsletterRecipientSuggestions,
  isValidTestEmail,
  normalizeTestEmail,
  testEmailValidationMessage,
} from '@/lib/newsletter-recipient-search'

const client = createClient()

type RecipientDraft = {
  email: string
  name: string
  status: NewsletterSubscriberStatus
}

const emptyDraft: RecipientDraft = { email: '', name: '', status: 'active' }

export function MailingListPage() {
  const queryClient = useQueryClient()
  const recipients = useQuery({
    queryKey: ['newsletter-subscribers'],
    queryFn: () => client.newsletterSubscribers(),
  })
  const tests = useQuery({ queryKey: ['newsletter-tests'], queryFn: () => client.newsletterTests() })
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<NewsletterSubscriberStatus | 'all'>('all')
  const [addOpen, setAddOpen] = useState(false)
  const [draft, setDraft] = useState<RecipientDraft>(emptyDraft)
  const [consentConfirmed, setConsentConfirmed] = useState(false)
  const [editing, setEditing] = useState<NewsletterSubscriber | null>(null)
  const [removing, setRemoving] = useState<NewsletterSubscriber | null>(null)
  const [testWindow, setTestWindow] = useState<NewsletterTestInput['window_mode']>('latest_24h')
  const [testEmail, setTestEmail] = useState('')
  const [testName, setTestName] = useState('')
  const [preview, setPreview] = useState<NewsletterTestResult | null>(null)

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['newsletter-subscribers'] })
  const create = useMutation({
    mutationFn: () => client.createNewsletterSubscriber({ ...draft, consent_confirmed: consentConfirmed }),
    onSuccess: async () => {
      await refresh()
      setAddOpen(false)
      setDraft(emptyDraft)
      setConsentConfirmed(false)
      toast.success('Recipient added to the morning newsletter')
    },
    onError: () => toast.error('Could not add recipient. Check whether the email is already listed.'),
  })
  const update = useMutation({
    mutationFn: (subscriber: NewsletterSubscriber) => client.updateNewsletterSubscriber(subscriber.id, {
      email: subscriber.email,
      name: subscriber.name,
      status: subscriber.status,
    }),
    onSuccess: async (_, subscriber) => {
      await refresh()
      setEditing(null)
      toast.success(subscriber.status === 'active' ? 'Recipient is active' : subscriber.status === 'paused' ? 'Delivery paused' : 'Recipient unsubscribed')
    },
    onError: () => toast.error('Could not update recipient'),
  })
  const remove = useMutation({
    mutationFn: (subscriber: NewsletterSubscriber) => client.deleteNewsletterSubscriber(subscriber.id),
    onSuccess: async () => {
      await refresh()
      setRemoving(null)
      toast.success('Recipient permanently removed')
    },
    onError: () => toast.error('Could not remove recipient'),
  })
  const runTest = useMutation({
    mutationFn: (mode: NewsletterTestInput['mode']) => client.runNewsletterTest({
      mode,
      window_mode: testWindow,
      recipient_email: normalizeTestEmail(testEmail),
      recipient_name: testName,
    }),
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: ['newsletter-tests'] })
      if (result.mode === 'preview') setPreview(result)
      toast.success(result.mode === 'send' ? `Test email sent to ${result.recipient_email}` : 'Newsletter preview generated')
    },
    onError: async () => {
      await queryClient.invalidateQueries({ queryKey: ['newsletter-tests'] })
      toast.error('Newsletter test failed. Review the latest diagnostic result below.')
    },
  })

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase()
    return (recipients.data?.items ?? []).filter((recipient) => {
      const matchesStatus = status === 'all' || recipient.status === status
      const matchesSearch = !term || recipient.email.toLowerCase().includes(term) || recipient.name.toLowerCase().includes(term)
      return matchesStatus && matchesSearch
    })
  }, [recipients.data?.items, search, status])

  const settings = recipients.data?.settings
  const summary = recipients.data?.summary

  return (
    <section className="flex flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <Badge variant="outline" className="mb-2"><Mail /> Morning newsletter</Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Mailing list</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            Manage who receives the Sinhala briefing generated from the previous 24 hours of reporting.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" nativeButton={false} render={<Link to="/workflows?workflow=newsletter_editorial" />}><Settings2 /> Configure newsletter</Button>
          <Dialog open={addOpen} onOpenChange={setAddOpen}>
            <DialogTrigger render={<Button />}><Plus data-icon="inline-start" />Add recipient</DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add newsletter recipient</DialogTitle>
              <DialogDescription>Only add people who have asked to receive the daily briefing.</DialogDescription>
            </DialogHeader>
            <form onSubmit={(event) => { event.preventDefault(); create.mutate() }}>
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="recipient-name">Name</FieldLabel>
                  <Input id="recipient-name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} placeholder="Optional display name" maxLength={160} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="recipient-email">Email</FieldLabel>
                  <Input id="recipient-email" type="email" value={draft.email} onChange={(event) => setDraft({ ...draft, email: event.target.value })} placeholder="reader@example.com" required autoComplete="email" />
                </Field>
                <label className="flex items-start gap-3 rounded-2xl border p-4 text-sm leading-6">
                  <input className="mt-1 size-4" type="checkbox" checked={consentConfirmed} onChange={(event) => setConsentConfirmed(event.target.checked)} required />
                  <span>I confirm this recipient requested or agreed to receive the newsletter.</span>
                </label>
                <Button type="submit" disabled={create.isPending || !consentConfirmed}>{create.isPending ? 'Adding…' : 'Add recipient'}</Button>
              </FieldGroup>
            </form>
          </DialogContent>
          </Dialog>
        </div>
      </div>

      {!settings?.enabled ? (
        <div className="flex items-start gap-3 rounded-3xl border border-amber-500/30 bg-amber-500/10 px-5 py-4 text-sm">
          <PauseCircle className="mt-0.5 size-5 shrink-0 text-amber-700 dark:text-amber-300" />
          <div><p className="font-medium">Scheduled delivery is disabled</p><p className="mt-1 text-muted-foreground">Recipients are saved, but mail will not be sent until newsletter delivery is enabled.</p></div>
        </div>
      ) : null}

      <Card className="gap-0 overflow-hidden py-0 shadow-sm">
        <CardHeader className="border-b py-5">
          <CardTitle className="flex items-center gap-2"><Bot className="size-5" />Autonomous newsletter workflow</CardTitle>
          <CardDescription>The versioned Morning newsletter workflow controls its instructions, personality, templates, ranking, and delivery schedule.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-0 p-0 md:grid-cols-2">
          <WorkflowRule icon={Clock3} title="1. Select the exact window">Use eligible stories from the 24 hours ending at {String(settings?.send_hour ?? 8).padStart(2, '0')}:00 in {settings?.timezone ?? 'Asia/Colombo'}.</WorkflowRule>
          <WorkflowRule icon={ShieldCheck} title="2. Verify and rank">Exclude held or restricted content, group related coverage, then prioritize breaking and independently corroborated stories.</WorkflowRule>
          <WorkflowRule icon={Mail} title="3. Write readable Sinhala">The editorial agent orders up to {settings?.max_stories ?? 30} verified stories, leads with {settings?.lead_story_count ?? 5}, and writes the introduction using the configured personality and tone.</WorkflowRule>
          <WorkflowRule icon={CheckCircle2} title="4. Deliver safely">Render a mobile-friendly Sinhala email and send one copy per active recipient with idempotency and one-click unsubscribe.</WorkflowRule>
        </CardContent>
      </Card>

      <NewsletterTestLab
        tests={tests.data?.items ?? []}
        loading={tests.isPending}
        recipients={recipients.data?.items ?? []}
        recipientsLoading={recipients.isPending}
        recipientsError={recipients.isError}
        windowMode={testWindow}
        onWindowMode={setTestWindow}
        email={testEmail}
        onEmail={setTestEmail}
        name={testName}
        onName={setTestName}
        pending={runTest.isPending}
        onRun={(mode) => runTest.mutate(mode)}
        preview={preview}
        onPreview={setPreview}
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <SummaryCard icon={UsersRound} label="All recipients" value={summary?.total ?? 0} />
        <SummaryCard icon={CheckCircle2} label="Active delivery" value={summary?.active ?? 0} />
        <SummaryCard icon={PauseCircle} label="Paused or unsubscribed" value={(summary?.paused ?? 0) + (summary?.unsubscribed ?? 0)} />
      </div>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Recipients</CardTitle>
          <CardDescription>
            {settings ? `Scheduled for ${String(settings.send_hour).padStart(2, '0')}:00 in ${settings.timezone}.` : 'Loading delivery schedule…'}
          </CardDescription>
          <CardAction><Badge variant="secondary">{filtered.length} shown</Badge></CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <div className="flex flex-col gap-3 border-b px-4 py-4 sm:flex-row sm:items-center sm:justify-between lg:px-6">
            <Input className="sm:max-w-sm" type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search name or email…" aria-label="Search newsletter recipients" />
            <Select value={status} onValueChange={(value) => value && setStatus(value as typeof status)}>
              <SelectTrigger size="sm" className="min-w-40" aria-label="Filter recipient status"><SelectValue>{() => status === 'all' ? 'All statuses' : status}</SelectValue></SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="active">Active</SelectItem>
                <SelectItem value="paused">Paused</SelectItem>
                <SelectItem value="unsubscribed">Unsubscribed</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Table className="min-w-[820px]">
            <TableHeader className="bg-muted/30"><TableRow><TableHead>Recipient</TableHead><TableHead>Status</TableHead><TableHead>Consent</TableHead><TableHead>Added</TableHead><TableHead className="text-right">Actions</TableHead></TableRow></TableHeader>
            <TableBody>
              {recipients.isPending ? Array.from({ length: 4 }, (_, row) => <TableRow key={row}>{Array.from({ length: 5 }, (_, cell) => <TableCell key={cell}><Skeleton className="h-5 w-full max-w-40" /></TableCell>)}</TableRow>) : null}
              {recipients.isError ? <TableRow><TableCell colSpan={5} className="h-40 text-center text-muted-foreground">The mailing list is temporarily unavailable.</TableCell></TableRow> : null}
              {!recipients.isPending && !recipients.isError && filtered.length === 0 ? <TableRow><TableCell colSpan={5} className="h-48 text-center"><UserRoundX className="mx-auto mb-2 size-5 text-muted-foreground" /><p className="font-medium">No recipients found</p><p className="mt-1 text-xs text-muted-foreground">Add a consenting recipient or change the filters.</p></TableCell></TableRow> : null}
              {filtered.map((recipient) => (
                <TableRow key={recipient.id}>
                  <TableCell><p className="font-medium">{recipient.name || 'Unnamed recipient'}</p><p className="mt-0.5 text-sm text-muted-foreground">{recipient.email}</p></TableCell>
                  <TableCell><StatusBadge status={recipient.status} /></TableCell>
                  <TableCell><p className="capitalize">{recipient.consent_source.replaceAll('_', ' ')}</p><p className="text-xs text-muted-foreground">{formatDateTime(recipient.consented_at)}</p></TableCell>
                  <TableCell className="text-muted-foreground">{formatDateTime(recipient.created_at)}</TableCell>
                  <TableCell className="text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={`Actions for ${recipient.email}`} />}><EllipsisVertical /></DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setEditing(recipient)}><Pencil />Edit</DropdownMenuItem>
                        {recipient.status === 'active'
                          ? <DropdownMenuItem onClick={() => update.mutate({ ...recipient, status: 'paused' })}><PauseCircle />Pause delivery</DropdownMenuItem>
                          : recipient.status === 'paused'
                            ? <DropdownMenuItem onClick={() => update.mutate({ ...recipient, status: 'active' })}><PlayCircle />Resume delivery</DropdownMenuItem>
                            : null}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem variant="destructive" onClick={() => setRemoving(recipient)}><Trash2 />Remove permanently</DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <EditRecipientDialog subscriber={editing} onChange={setEditing} onSave={(subscriber) => update.mutate(subscriber)} pending={update.isPending} />
      <Dialog open={Boolean(removing)} onOpenChange={(open) => { if (!open) setRemoving(null) }}>
        <DialogContent>
          <DialogHeader><DialogTitle>Remove recipient permanently?</DialogTitle><DialogDescription>This deletes the recipient and their delivery history. You can add the address again later with renewed consent.</DialogDescription></DialogHeader>
          <DialogFooter><Button variant="outline" onClick={() => setRemoving(null)}>Cancel</Button><Button variant="destructive" disabled={remove.isPending} onClick={() => removing && remove.mutate(removing)}>{remove.isPending ? 'Removing…' : 'Remove recipient'}</Button></DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}

function NewsletterTestLab({ tests, loading, recipients, recipientsLoading, recipientsError, windowMode, onWindowMode, email, onEmail, name, onName, pending, onRun, preview, onPreview }: {
  tests: NewsletterTestRun[]
  loading: boolean
  recipients: NewsletterSubscriber[]
  recipientsLoading: boolean
  recipientsError: boolean
  windowMode: NewsletterTestInput['window_mode']
  onWindowMode: (value: NewsletterTestInput['window_mode']) => void
  email: string
  onEmail: (value: string) => void
  name: string
  onName: (value: string) => void
  pending: boolean
  onRun: (mode: NewsletterTestInput['mode']) => void
  preview: NewsletterTestResult | null
  onPreview: (value: NewsletterTestResult | null) => void
}) {
  const emailValidation = testEmailValidationMessage(recipients, email)
  const canSend = !recipientsLoading && !recipientsError && isValidTestEmail(email) && !emailValidation

  return (
    <Card id="newsletter-test-lab" className="gap-0 overflow-hidden py-0 shadow-sm">
      <CardHeader className="border-b py-6"><CardTitle className="flex items-center gap-2"><FlaskConical className="size-5" />Newsletter test lab</CardTitle><CardDescription>Run the saved workflow against real eligible coverage before the next scheduled delivery. Preview does not send email; test send targets only the address entered here.</CardDescription></CardHeader>
      <CardContent className="flex flex-col gap-6 p-6">
        <div className="grid gap-4 lg:grid-cols-[minmax(0,0.75fr)_minmax(0,1fr)_minmax(0,1fr)]">
          <Field><FieldLabel htmlFor="test-window">Coverage window</FieldLabel><Select value={windowMode} onValueChange={(value) => value && onWindowMode(value as NewsletterTestInput['window_mode'])}><SelectTrigger id="test-window"><SelectValue>{() => windowMode === 'latest_24h' ? 'Latest rolling 24 hours' : 'Most recent scheduled window'}</SelectValue></SelectTrigger><SelectContent><SelectItem value="latest_24h">Latest rolling 24 hours</SelectItem><SelectItem value="scheduled">Most recent scheduled window</SelectItem></SelectContent></Select></Field>
          <Field><FieldLabel htmlFor="test-name">Test greeting name</FieldLabel><Input id="test-name" value={name} maxLength={160} onChange={(event) => onName(event.target.value)} placeholder="Optional preview name" /></Field>
          <TestRecipientCombobox
            recipients={recipients}
            loading={recipientsLoading}
            error={recipientsError}
            email={email}
            onEmail={onEmail}
            name={name}
            onName={onName}
            validationMessage={emailValidation}
            disabled={pending}
          />
        </div>
        <div className="flex flex-wrap gap-2"><Button disabled={pending} onClick={() => onRun('preview')}><Eye data-icon="inline-start" />{pending ? 'Running workflow…' : 'Generate preview'}</Button><Button variant="outline" disabled={pending || !canSend} onClick={() => onRun('send')}><Send data-icon="inline-start" />Send one test email</Button></div>
        <div className="border-t pt-6">
          <div className="mb-3 flex items-center justify-between"><div><h3 className="flex items-center gap-2 font-heading font-semibold"><History className="size-4" />Recent test performance</h3><p className="mt-1 text-xs text-muted-foreground">Compare model, duration, coverage, and outcomes from the latest 50 tests.</p></div><Badge variant="secondary">{tests.length} runs</Badge></div>
          {loading ? <Skeleton className="h-28 w-full" /> : null}
          {!loading && tests.length === 0 ? <p className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">No newsletter tests have run yet.</p> : null}
          <div className="space-y-2">{tests.slice(0, 10).map((run) => <TestHistoryRow key={run.id} run={run} />)}</div>
        </div>
      </CardContent>
      <Dialog open={Boolean(preview)} onOpenChange={(open) => { if (!open) onPreview(null) }}>
        <DialogContent className="max-h-[92vh] overflow-hidden sm:max-w-5xl">
          <DialogHeader><DialogTitle>Newsletter preview</DialogTitle><DialogDescription>{preview?.subject} · {preview?.story_count} stories · {preview?.duration_ms.toLocaleString()} ms · {preview?.model || 'No AI call required'}</DialogDescription></DialogHeader>
          {preview ? <iframe title="Generated newsletter preview" sandbox="" srcDoc={preview.html} className="h-[70vh] w-full rounded-xl border bg-white" /> : null}
        </DialogContent>
      </Dialog>
    </Card>
  )
}

function TestRecipientCombobox({ recipients, loading, error, email, onEmail, name, onName, validationMessage, disabled }: {
  recipients: NewsletterSubscriber[]
  loading: boolean
  error: boolean
  email: string
  onEmail: (value: string) => void
  name: string
  onName: (value: string) => void
  validationMessage: string | null
  disabled: boolean
}) {
  const [selectedRecipientID, setSelectedRecipientID] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const debouncedEmail = useDebouncedValue(email, 200)
  const suggestions = useMemo(
    () => filterNewsletterRecipientSuggestions(recipients, debouncedEmail),
    [debouncedEmail, recipients],
  )
  const selectedRecipient = recipients.find((recipient) => recipient.id === selectedRecipientID) ?? null
  const showValidation = Boolean(validationMessage) && !open

  const updateInput = (value: string) => {
    if (selectedRecipient && normalizeTestEmail(value) !== normalizeTestEmail(selectedRecipient.email)) {
      setSelectedRecipientID(null)
      if (name === selectedRecipient.name) onName('')
    }
    onEmail(value)
  }

  return (
    <Field data-invalid={showValidation}>
      <FieldLabel htmlFor="test-email">One test address</FieldLabel>
      <Combobox
        open={open}
        onOpenChange={setOpen}
        items={recipients}
        filteredItems={suggestions}
        filter={null}
        value={selectedRecipient}
        inputValue={email}
        onInputValueChange={updateInput}
        onValueChange={(recipient) => {
          if (!recipient) return
          setSelectedRecipientID(recipient.id)
          onEmail(recipient.email)
          onName(recipient.name)
        }}
        itemToStringLabel={(recipient) => recipient.email}
        itemToStringValue={(recipient) => recipient.email}
        isItemEqualToValue={(recipient, value) => recipient.id === value.id}
        autoHighlight
        disabled={disabled}
      >
        <ComboboxInput
          id="test-email"
          type="email"
          autoComplete="email"
          placeholder="Search recipients or enter an email…"
          aria-invalid={showValidation}
          aria-describedby="test-email-description"
        />
        <ComboboxContent className="w-[min(32rem,var(--available-width))]">
          <ComboboxEmpty>
            {loading
              ? 'Loading mailing-list recipients…'
              : error
                ? 'Recipient lookup is unavailable. Try again before sending a test.'
                : isValidTestEmail(email)
                  ? 'No matching recipient. This address can still be used for a one-time test.'
                  : 'Search by name or email, or finish typing a new address.'}
          </ComboboxEmpty>
          <ComboboxList>
            {(recipient: NewsletterSubscriber) => (
              <ComboboxItem key={recipient.id} value={recipient} disabled={recipient.status !== 'active'}>
                <span className="flex min-w-0 flex-col">
                  <span className="truncate">{recipient.name || 'Unnamed recipient'}</span>
                  <span className="truncate text-xs font-normal text-muted-foreground">{recipient.email} · {recipient.status}</span>
                </span>
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
      <FieldDescription id="test-email-description">Choose an active recipient or enter a valid one-time address. This never changes the mailing list.</FieldDescription>
      {showValidation ? <FieldError>{validationMessage}</FieldError> : null}
    </Field>
  )
}

function TestHistoryRow({ run }: { run: NewsletterTestRun }) {
  return <div className="grid gap-3 rounded-xl border p-4 text-sm md:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_auto] md:items-center"><div><div className="flex flex-wrap items-center gap-2"><Badge variant={run.status === 'succeeded' ? 'secondary' : 'destructive'}>{run.status}</Badge><span className="font-medium">{run.mode === 'send' ? 'One-address send' : 'Preview'}</span></div><p className="mt-1 truncate text-xs text-muted-foreground">{run.subject || run.error_detail || 'No subject generated'}</p></div><div><p className="truncate text-xs font-medium">{run.model || 'No model response'}</p><p className="mt-1 text-xs text-muted-foreground">{run.story_count} stories · {run.article_count} articles · {run.source_count} sources</p></div><div className="text-left md:text-right"><p className="font-medium tabular-nums">{run.duration_ms.toLocaleString()} ms</p><p className="mt-1 text-xs text-muted-foreground">{formatDateTime(run.created_at)}</p></div></div>
}

function SummaryCard({ icon: Icon, label, value }: { icon: typeof UsersRound; label: string; value: number }) {
  return <Card size="sm"><CardContent className="flex items-center gap-4"><span className="grid size-10 place-items-center rounded-2xl bg-muted"><Icon className="size-5" /></span><div><p className="text-2xl font-semibold tabular-nums">{value}</p><p className="text-xs text-muted-foreground">{label}</p></div></CardContent></Card>
}

function WorkflowRule({ icon: Icon, title, children }: { icon: typeof UsersRound; title: string; children: ReactNode }) {
  return <div className="flex gap-3 border-b p-5 last:border-b-0 md:odd:border-r"><span className="grid size-9 shrink-0 place-items-center rounded-xl bg-muted"><Icon className="size-4" /></span><div><p className="font-medium">{title}</p><p className="mt-1 text-sm leading-6 text-muted-foreground">{children}</p></div></div>
}

function StatusBadge({ status }: { status: NewsletterSubscriberStatus }) {
  if (status === 'active') return <Badge><CheckCircle2 />Active</Badge>
  if (status === 'paused') return <Badge variant="secondary"><PauseCircle />Paused</Badge>
  return <Badge variant="outline"><UserRoundX />Unsubscribed</Badge>
}

function EditRecipientDialog({ subscriber, onChange, onSave, pending }: { subscriber: NewsletterSubscriber | null; onChange: (subscriber: NewsletterSubscriber | null) => void; onSave: (subscriber: NewsletterSubscriber) => void; pending: boolean }) {
  return <Dialog open={Boolean(subscriber)} onOpenChange={(open) => { if (!open) onChange(null) }}><DialogContent><DialogHeader><DialogTitle>Edit recipient</DialogTitle><DialogDescription>{subscriber?.status === 'unsubscribed' ? 'This address unsubscribed and cannot be reactivated without renewed consent. Remove it and add it again only after they agree.' : 'Update the display name, address, or delivery status.'}</DialogDescription></DialogHeader>{subscriber ? <form onSubmit={(event) => { event.preventDefault(); onSave(subscriber) }}><FieldGroup><Field><FieldLabel htmlFor="edit-recipient-name">Name</FieldLabel><Input id="edit-recipient-name" value={subscriber.name} onChange={(event) => onChange({ ...subscriber, name: event.target.value })} maxLength={160} /></Field><Field><FieldLabel htmlFor="edit-recipient-email">Email</FieldLabel><Input id="edit-recipient-email" type="email" value={subscriber.email} onChange={(event) => onChange({ ...subscriber, email: event.target.value })} required /></Field><Field><FieldLabel htmlFor="edit-recipient-status">Status</FieldLabel><Select value={subscriber.status} disabled={subscriber.status === 'unsubscribed'} onValueChange={(value) => value && onChange({ ...subscriber, status: value as NewsletterSubscriberStatus })}><SelectTrigger id="edit-recipient-status" className="w-full"><SelectValue>{() => subscriber.status}</SelectValue></SelectTrigger><SelectContent><SelectItem value="active">Active</SelectItem><SelectItem value="paused">Paused</SelectItem><SelectItem value="unsubscribed">Unsubscribed</SelectItem></SelectContent></Select></Field><Button type="submit" disabled={pending}>{pending ? 'Saving…' : 'Save recipient'}</Button></FieldGroup></form> : null}</DialogContent></Dialog>
}
