import { createClient, type AdminEditorial, type EditorialInput } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Archive, ExternalLink, Feather, LoaderCircle, Plus, Send, Trash2, Undo2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { formatDateTime } from '@/lib/date-time'
import { publicURL } from '@/lib/public-site'

const client = createClient()

const limits = { title: 200, standfirst: 400, body: 20000, author_name: 120 } as const
const emptyInput: EditorialInput = { title: '', standfirst: '', body: '', author_name: '', event_id: null }

const statusLabel: Record<AdminEditorial['status'], string> = { draft: 'Draft', published: 'Published', archived: 'Archived' }

function length(value: string) {
  return [...value.trim()].length
}

export function EditorialsPage() {
  const queryClient = useQueryClient()
  const me = useQuery({ queryKey: ['me'], queryFn: () => client.me() })
  const allowed = me.data?.role === 'administrator' || me.data?.role === 'chief_editor'
  const editorials = useQuery({ queryKey: ['editorials'], queryFn: () => client.adminEditorials(), enabled: allowed })
  const [editing, setEditing] = useState<{ id: string | null; input: EditorialInput } | null>(null)
  const [deleting, setDeleting] = useState<AdminEditorial | null>(null)

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['editorials'] })
  const save = useMutation({
    mutationFn: ({ id, input }: { id: string | null; input: EditorialInput }) => (id ? client.updateEditorial(id, input) : client.createEditorial(input)),
    onSuccess: (item) => {
      void refresh()
      setEditing(null)
      toast.success(item.status === 'published' ? 'Editorial updated on the site' : 'Draft saved')
    },
    onError: (error) => toast.error(error instanceof Error && error.message ? error.message : 'Could not save the editorial'),
  })
  const setStatus = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'publish' | 'unpublish' | 'archive' }) => client.setEditorialStatus(id, action),
    onSuccess: (item) => {
      void refresh()
      toast.success({ published: 'Editorial published', draft: 'Editorial moved back to drafts', archived: 'Editorial archived' }[item.status])
    },
    onError: () => toast.error('Could not change the editorial'),
  })
  const remove = useMutation({
    mutationFn: (id: string) => client.deleteEditorial(id),
    onSuccess: () => {
      void refresh()
      setDeleting(null)
      toast.success('Draft deleted')
    },
    onError: () => toast.error('Could not delete the draft'),
  })

  if (me.isPending) return <Skeleton className="h-48 w-full" />
  if (!allowed) return <p className="p-6 text-sm text-muted-foreground">Editorials are written by the chief editor and administrators.</p>

  const input = editing?.input
  const invalid = input ? {
    title: length(input.title) === 0 || length(input.title) > limits.title,
    standfirst: length(input.standfirst) > limits.standfirst,
    body: length(input.body) === 0 || length(input.body) > limits.body,
    author_name: length(input.author_name) === 0 || length(input.author_name) > limits.author_name,
  } : null
  const canSave = invalid ? !Object.values(invalid).some(Boolean) : false

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <Badge variant="outline" className="mb-2"><Feather /> Chief editor</Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Editorials</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            The editorial (කතුවැකිය) is the paper's own opinion, shown on the front page and the editorial page and labelled as opinion. The latest published editorial appears on the front page.
          </p>
        </div>
        <Button onClick={() => setEditing({ id: null, input: { ...emptyInput, author_name: me.data?.name ?? '' } })}><Plus /> New editorial</Button>
      </div>

      {editorials.isPending ? <Skeleton className="h-64 w-full" /> : editorials.isError ? (
        <Card><CardContent className="space-y-3"><p role="alert">Could not load editorials.</p><Button variant="outline" onClick={() => { void editorials.refetch() }}>Try again</Button></CardContent></Card>
      ) : editorials.data.items.length === 0 ? (
        <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">No editorials yet. Write the first one.</CardContent></Card>
      ) : (
        <div className="grid gap-4">
          {editorials.data.items.map((item) => (
            <Card key={item.id} className="gap-0 py-0">
              <CardHeader className="py-4">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={item.status === 'published' ? 'default' : 'secondary'}>{statusLabel[item.status]}</Badge>
                  <span className="text-xs text-muted-foreground">
                    {item.published_at ? `Published ${formatDateTime(item.published_at)} · ` : ''}Updated {formatDateTime(item.updated_at)} · {item.author_name}
                  </span>
                </div>
                <CardTitle lang="si" className="mt-2 leading-relaxed">{item.title}</CardTitle>
                {item.standfirst ? <CardDescription lang="si" className="leading-relaxed">{item.standfirst}</CardDescription> : null}
              </CardHeader>
              <CardContent className="flex flex-wrap gap-2 border-t p-4">
                <Button size="sm" variant="outline" onClick={() => setEditing({ id: item.id, input: { title: item.title, standfirst: item.standfirst, body: item.body, author_name: item.author_name, event_id: item.event_id } })}>Edit</Button>
                {item.status !== 'published' ? (
                  <Button size="sm" disabled={setStatus.isPending} onClick={() => setStatus.mutate({ id: item.id, action: 'publish' })}><Send /> Publish</Button>
                ) : (
                  <>
                    <Button size="sm" variant="outline" nativeButton={false} render={<a href={publicURL(`/editorial/${item.id}`)} target="_blank" rel="noreferrer" />}><ExternalLink /> View on site</Button>
                    <Button size="sm" variant="outline" disabled={setStatus.isPending} onClick={() => setStatus.mutate({ id: item.id, action: 'unpublish' })}><Undo2 /> Unpublish</Button>
                  </>
                )}
                {item.status !== 'archived' && item.published_at ? <Button size="sm" variant="outline" disabled={setStatus.isPending} onClick={() => setStatus.mutate({ id: item.id, action: 'archive' })}><Archive /> Archive</Button> : null}
                {item.status === 'draft' && !item.published_at ? <Button size="sm" variant="ghost" onClick={() => setDeleting(item)}><Trash2 /> Delete draft</Button> : null}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={!!editing} onOpenChange={(open) => { if (!open && !save.isPending) setEditing(null) }}>
        <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-3xl" showCloseButton={!save.isPending}>
          <DialogHeader>
            <DialogTitle>{editing?.id ? 'Edit editorial' : 'New editorial'}</DialogTitle>
            <DialogDescription>Saving keeps a draft as a draft; a published editorial updates on the site at once. Separate paragraphs with a blank line.</DialogDescription>
          </DialogHeader>
          {editing && input && invalid ? (
            <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); if (canSave) save.mutate(editing) }}>
              {([
                ['title', 'Title', false],
                ['standfirst', 'Standfirst (optional summary under the title)', true],
                ['author_name', 'Byline', false],
              ] as const).map(([field, label, multiline]) => (
                <div key={field}>
                  <Label htmlFor={`editorial-${field}`}>{label}</Label>
                  {multiline ? (
                    <Textarea id={`editorial-${field}`} lang="si" rows={2} className="mt-2" value={input[field]} aria-invalid={invalid[field]}
                      onChange={(event) => setEditing({ ...editing, input: { ...input, [field]: event.target.value } })} />
                  ) : (
                    <Input id={`editorial-${field}`} lang="si" className="mt-2" value={input[field]} aria-invalid={invalid[field]}
                      onChange={(event) => setEditing({ ...editing, input: { ...input, [field]: event.target.value } })} />
                  )}
                  <p className={invalid[field] ? 'mt-1 text-xs text-destructive' : 'mt-1 text-xs text-muted-foreground'}>{length(input[field])}/{limits[field]}</p>
                </div>
              ))}
              <div>
                <Label htmlFor="editorial-body">Editorial</Label>
                <Textarea id="editorial-body" lang="si" rows={14} className="mt-2 leading-7" value={input.body} aria-invalid={invalid.body}
                  onChange={(event) => setEditing({ ...editing, input: { ...input, body: event.target.value } })} />
                <p className={invalid.body ? 'mt-1 text-xs text-destructive' : 'mt-1 text-xs text-muted-foreground'}>{length(input.body).toLocaleString('en')}/{limits.body.toLocaleString('en')}</p>
              </div>
              <div>
                <Label htmlFor="editorial-event">Related story ID (optional)</Label>
                <Input id="editorial-event" className="mt-2 font-mono text-xs" placeholder="Story (event) ID from the front page or story URL" value={input.event_id ?? ''}
                  onChange={(event) => setEditing({ ...editing, input: { ...input, event_id: event.target.value.trim() || null } })} />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" disabled={save.isPending} onClick={() => setEditing(null)}>Cancel</Button>
                <Button type="submit" disabled={!canSave || save.isPending}>{save.isPending ? <LoaderCircle className="animate-spin" /> : null}{save.isPending ? 'Saving…' : 'Save'}</Button>
              </DialogFooter>
            </form>
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleting} onOpenChange={(open) => { if (!open && !remove.isPending) setDeleting(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete this draft?</DialogTitle>
            <DialogDescription lang="si">{deleting?.title}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" disabled={remove.isPending} onClick={() => setDeleting(null)}>Cancel</Button>
            <Button variant="destructive" disabled={remove.isPending} onClick={() => deleting && remove.mutate(deleting.id)}>{remove.isPending ? 'Deleting…' : 'Delete draft'}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
