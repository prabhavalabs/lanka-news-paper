import { createClient, type SourceType } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const client = createClient()
const types: SourceType[] = ['private_media', 'state_owned', 'government', 'independent', 'international', 'other']
const selectClass =
  'h-8 w-full border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50'

export function SourcesPage() {
  const queryClient = useQueryClient()
  const sources = useQuery({ queryKey: ['admin-sources'], queryFn: () => client.adminSources() })
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [legalName, setLegalName] = useState('')
  const [sourceType, setSourceType] = useState<SourceType>('private_media')
  const [website, setWebsite] = useState('')
  const [active, setActive] = useState(false)
  const create = useMutation({
    mutationFn: () =>
      client.createSource({ name, legal_name: legalName, source_type: sourceType, website, description: '', active }),
    onSuccess: () => {
      toast.success('Source created')
      setOpen(false)
      setName('')
      setLegalName('')
      setWebsite('')
      void queryClient.invalidateQueries({ queryKey: ['admin-sources'] })
    },
    onError: () => toast.error('Could not create source'),
  })
  const toggle = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) => client.setSourceActive(id, active),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['admin-sources'] }),
  })

  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-medium">Sources</h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button />}>Add source</DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add source</DialogTitle>
            </DialogHeader>
            <form
              onSubmit={(event) => {
                event.preventDefault()
                create.mutate()
              }}
            >
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="name">Name</FieldLabel>
                  <Input id="name" value={name} onChange={(event) => setName(event.target.value)} required />
                </Field>
                <Field>
                  <FieldLabel htmlFor="legal">Legal name</FieldLabel>
                  <Input id="legal" value={legalName} onChange={(event) => setLegalName(event.target.value)} required />
                </Field>
                <Field>
                  <FieldLabel htmlFor="type">Type</FieldLabel>
                  <select id="type" className={selectClass} value={sourceType} onChange={(event) => setSourceType(event.target.value as SourceType)}>
                    {types.map((type) => (
                      <option key={type} value={type}>
                        {type}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field>
                  <FieldLabel htmlFor="site">Website</FieldLabel>
                  <Input id="site" type="url" value={website} onChange={(event) => setWebsite(event.target.value)} />
                </Field>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={active} onChange={(event) => setActive(event.target.checked)} />
                  Active (can publish when rights exist)
                </label>
                <Button type="submit">Create</Button>
              </FieldGroup>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      {!sources.data?.items.length ? (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>No publishers onboarded</EmptyTitle>
            <EmptyDescription>Add a source, then an HTTPS feed and a rights profile before polling.</EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Active</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {sources.data.items.map((source) => (
              <TableRow key={source.id}>
                <TableCell>
                  <Link to={`/sources/${source.id}`}>{source.name}</Link>
                </TableCell>
                <TableCell>{source.source_type}</TableCell>
                <TableCell>
                  <Badge variant="outline">{source.active ? 'active' : 'held'}</Badge>
                </TableCell>
                <TableCell>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => toggle.mutate({ id: source.id, active: !source.active })}
                  >
                    {source.active ? 'Hold' : 'Activate'}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  )
}
