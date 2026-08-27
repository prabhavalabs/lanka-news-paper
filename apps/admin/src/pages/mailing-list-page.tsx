import {
  createClient,
  type NewsletterSubscriber,
  type NewsletterSubscriberStatus,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Bot,
  CheckCircle2,
  Clock3,
  EllipsisVertical,
  Mail,
  PauseCircle,
  Pencil,
  PlayCircle,
  Plus,
  Trash2,
  ShieldCheck,
  Settings2,
  UserRoundX,
  UsersRound,
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatDateTime } from '@/lib/date-time'

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
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<NewsletterSubscriberStatus | 'all'>('all')
  const [addOpen, setAddOpen] = useState(false)
  const [draft, setDraft] = useState<RecipientDraft>(emptyDraft)
  const [consentConfirmed, setConsentConfirmed] = useState(false)
  const [editing, setEditing] = useState<NewsletterSubscriber | null>(null)
  const [removing, setRemoving] = useState<NewsletterSubscriber | null>(null)

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
          <Button variant="outline" render={<Link to="/workflows?workflow=newsletter_editorial" />}><Settings2 /> Configure newsletter</Button>
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
