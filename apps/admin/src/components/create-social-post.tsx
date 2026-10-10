import { createClient, type SocialCreateInput } from '@snap/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { parseSocialSubject } from '@/lib/social-publishing'

const client = createClient()
export function CreateSocialPost({ subject, onCreated }: { subject?: SocialCreateInput; onCreated?: () => void }) {
  const [open, setOpen] = useState(false)
  const [kind, setKind] = useState<'event' | 'article'>('event')
  const [value, setValue] = useState('')
  const [created, setCreated] = useState(false)
  const queryClient = useQueryClient()
  const input = subject ?? parseSocialSubject(value, kind)
  const create = useMutation({
    mutationFn: () => client.createSocialPost(input!),
    onSuccess: async (post) => {
      await queryClient.invalidateQueries({ queryKey: ['social'] })
      setCreated(true)
      onCreated?.()
      toast.success('Post created for review')
      if (post.last_error) toast.error(`Image needs attention: ${post.last_error}`)
    },
    onError: () => toast.error('Could not create post. It may already be queued, ineligible, or the editorial model may be unavailable.'),
  })
  return <Dialog open={open} onOpenChange={(next) => { if (!create.isPending) { setOpen(next); if (next) setCreated(false) } }}>
    <DialogTrigger render={<Button variant={subject ? 'outline' : 'default'} />}><Plus />{subject ? 'Create social post' : 'Create post from…'}</DialogTrigger>
    <DialogContent showCloseButton={!create.isPending}>
      <DialogHeader><DialogTitle>Create social post</DialogTitle><DialogDescription>Generate an original Sinhala caption and image for human review. This may take up to a few minutes while the editorial model runs.</DialogDescription></DialogHeader>
      {created ? <div className="space-y-4"><p className="text-sm">Your post is in Pending review. Inspect the content and image before approving.</p><Button nativeButton={false} render={<Link to="/social" onClick={() => setOpen(false)} />}>Open social queue</Button></div> : <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); if (input) create.mutate() }}>
        {!subject ? <><Field><FieldLabel htmlFor="social-subject-kind">Subject type</FieldLabel><Select value={kind} onValueChange={(v) => v && setKind(v as typeof kind)}><SelectTrigger id="social-subject-kind"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="event">Event</SelectItem><SelectItem value="article">Article</SelectItem></SelectContent></Select></Field><Field><FieldLabel htmlFor="social-subject">Event or article ID / public URL</FieldLabel><Input id="social-subject" value={value} onChange={(e) => setValue(e.target.value)} placeholder="Paste an ID or /e/… or /a/… URL" disabled={create.isPending} />{value && !input ? <FieldError>Enter a valid ID or Thulawa public URL.</FieldError> : null}</Field></> : <p className="text-sm text-muted-foreground">Create a review post from this {subject.event_id ? 'event' : 'article'}.</p>}
        {create.isPending ? <p role="status" className="text-sm text-muted-foreground">Preparing caption and image… Please keep this dialog open.</p> : null}
        <Button type="submit" disabled={!input || create.isPending}>{create.isPending ? 'Creating…' : 'Create review post'}</Button>
      </form>}
    </DialogContent>
  </Dialog>
}
