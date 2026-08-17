import { createClient, type AdminSource, type SourceType } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CirclePause,
  CirclePlay,
  EllipsisVertical,
  ExternalLink,
  Eye,
  Pencil,
  Plus,
  RadioTower,
  Trash2,
} from 'lucide-react'
import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { SourceAvatar } from '@/components/source-avatar'
import { SourceLogoUpload } from '@/components/source-logo-upload'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useTableQuery } from '@/hooks/use-table-query'
import { cn } from '@/lib/utils'

const client = createClient()
const types: SourceType[] = [
  'private_media',
  'state_owned',
  'government',
  'independent',
  'international',
  'other',
]

type ConfirmAction = {
  kind: 'status' | 'delete'
  source: AdminSource
}

function label(value: string) {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}

export function SourcesPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const table = useTableQuery()
  const typeFilter = table.filter('type')
  const statusFilter = table.filter('status')
  const sources = useQuery({
    queryKey: ['admin-sources', table.page, table.perPage, table.search, typeFilter, statusFilter],
    queryFn: () =>
      client.adminSources({
        page: table.page,
        per_page: table.perPage,
        search: table.search,
        type: typeFilter,
        status: statusFilter,
      }),
    placeholderData: keepPreviousData,
  })
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [legalName, setLegalName] = useState('')
  const [sourceType, setSourceType] = useState<SourceType>('private_media')
  const [website, setWebsite] = useState('')
  const [createLogo, setCreateLogo] = useState<File | null>(null)
  const [active, setActive] = useState(false)
  const [editingSource, setEditingSource] = useState<AdminSource | null>(null)
  const [editLogo, setEditLogo] = useState<File | null>(null)
  const [removeEditLogo, setRemoveEditLogo] = useState(false)
  const [confirmAction, setConfirmAction] = useState<ConfirmAction | null>(null)
  const create = useMutation({
    mutationFn: async () => {
      const source = await client.createSource({
        name,
        legal_name: legalName,
        source_type: sourceType,
        website,
        icon_url: '',
        description: '',
        active,
      })
      if (!createLogo) return { logoFailed: false }
      try {
        await client.uploadSourceLogo(source.id, createLogo)
        return { logoFailed: false }
      } catch {
        return { logoFailed: true }
      }
    },
    onSuccess: ({ logoFailed }) => {
      if (logoFailed) toast.warning('Source created, but its logo could not be uploaded')
      else toast.success('Source created')
      setOpen(false)
      setName('')
      setLegalName('')
      setWebsite('')
      setCreateLogo(null)
      void queryClient.invalidateQueries({ queryKey: ['admin-sources'] })
    },
    onError: () => toast.error('Could not create source'),
  })
  const toggle = useMutation({
    mutationFn: ({ id, active: nextActive }: { id: string; active: boolean }) =>
      client.setSourceActive(id, nextActive),
    onSuccess: (_, variables) => {
      toast.success(variables.active ? 'Source activated' : 'Source held')
      setConfirmAction(null)
      void queryClient.invalidateQueries({ queryKey: ['admin-sources'] })
    },
    onError: () => toast.error('Could not update source'),
  })
  const update = useMutation({
    mutationFn: async (source: AdminSource) => {
      const { id, ...body } = source
      await client.updateSource(id, body)
      if (removeEditLogo) await client.removeSourceLogo(id)
      else if (editLogo) await client.uploadSourceLogo(id, editLogo)
    },
    onSuccess: (_, source) => {
      toast.success('Source updated')
      setEditingSource(null)
      setEditLogo(null)
      setRemoveEditLogo(false)
      void queryClient.invalidateQueries({ queryKey: ['admin-sources'] })
      void queryClient.invalidateQueries({ queryKey: ['source', source.id] })
    },
    onError: () => toast.error('Could not update source'),
  })
  const archive = useMutation({
    mutationFn: (id: string) => client.archiveSource(id),
    onSuccess: () => {
      toast.success('Source deleted')
      setConfirmAction(null)
      void queryClient.invalidateQueries({ queryKey: ['admin-sources'] })
    },
    onError: () => toast.error('Could not delete source'),
  })
  const pagination = sources.data?.pagination

  return (
    <section className="flex flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <Badge variant="outline" className="mb-2">
            <RadioTower />
            Source registry
          </Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Publishers and feeds</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage publisher access, ownership type, and ingestion readiness.
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button />}>
            <Plus data-icon="inline-start" />
            Add source
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add publisher source</DialogTitle>
              <DialogDescription>
                Create the publisher first, then add its endpoints and rights profile.
              </DialogDescription>
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
                  <Input
                    id="legal"
                    value={legalName}
                    onChange={(event) => setLegalName(event.target.value)}
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="source-type">Type</FieldLabel>
                  <Select value={sourceType} onValueChange={(value) => setSourceType(value as SourceType)}>
                    <SelectTrigger id="source-type" className="w-full">
                      <SelectValue>{(value) => label(String(value))}</SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      {types.map((type) => (
                        <SelectItem key={type} value={type}>
                          {label(type)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
                <Field>
                  <FieldLabel htmlFor="site">Website</FieldLabel>
                  <Input
                    id="site"
                    type="url"
                    value={website}
                    onChange={(event) => setWebsite(event.target.value)}
                  />
                </Field>
                <SourceLogoUpload
                  file={createLogo}
                  name={name}
                  onFileChange={setCreateLogo}
                  onRemoveChange={() => undefined}
                  remove={false}
                  website={website}
                />
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={active}
                    onChange={(event) => setActive(event.target.checked)}
                  />
                  Active when publishing rights are ready
                </label>
                <Button type="submit" disabled={create.isPending}>
                  {create.isPending ? 'Creating…' : 'Create source'}
                </Button>
              </FieldGroup>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Publisher network</CardTitle>
          <CardDescription>Every source is searchable, filterable, and paginated by the server.</CardDescription>
          <CardAction>
            <Badge variant="secondary">{pagination?.total ?? 0} sources</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar
            search={table.search}
            searchPlaceholder="Search publishers, legal names, or websites…"
            onSearch={table.setSearch}
          >
            <Select
              value={typeFilter || 'all'}
              onValueChange={(value) => {
                if (value !== null) table.setFilter('type', value === 'all' ? '' : value)
              }}
            >
              <SelectTrigger size="sm" className="min-w-40" aria-label="Filter by source type">
                <SelectValue>{() => (typeFilter ? label(typeFilter) : 'All source types')}</SelectValue>
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All source types</SelectItem>
                {types.map((type) => (
                  <SelectItem key={type} value={type}>
                    {label(type)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={statusFilter || 'all'}
              onValueChange={(value) => {
                if (value !== null) table.setFilter('status', value === 'all' ? '' : value)
              }}
            >
              <SelectTrigger size="sm" className="min-w-32" aria-label="Filter by source status">
                <SelectValue>{() => (statusFilter ? label(statusFilter) : 'All statuses')}</SelectValue>
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="active">Active</SelectItem>
                <SelectItem value="held">Held</SelectItem>
              </SelectContent>
            </Select>
          </DataTableToolbar>

          <Table className="min-w-[860px]">
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead>Publisher</TableHead>
                <TableHead>Ownership</TableHead>
                <TableHead>Website</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sources.isPending
                ? Array.from({ length: 5 }, (_, index) => (
                    <TableRow key={index}>
                      {Array.from({ length: 5 }, (_, cell) => (
                        <TableCell key={cell}>
                          <Skeleton className="h-5 w-full max-w-40" />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                : null}
              {sources.isError ? (
                <TableRow>
                  <TableCell colSpan={5} className="h-40 text-center text-muted-foreground">
                    Sources are temporarily unavailable.
                  </TableCell>
                </TableRow>
              ) : null}
              {!sources.isPending && !sources.isError && sources.data.items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="h-48 text-center">
                    <div className="flex flex-col items-center gap-2">
                      <RadioTower className="size-5 text-muted-foreground" />
                      <p className="font-medium">No sources found</p>
                      <p className="text-xs text-muted-foreground">Try a different search or filter.</p>
                    </div>
                  </TableCell>
                </TableRow>
              ) : null}
              {sources.data?.items.map((source) => (
                <TableRow key={source.id}>
                  <TableCell className="whitespace-normal">
                    <div className="flex items-center gap-3">
                      <SourceAvatar
                        className="size-10"
                        name={source.name}
                        website={source.website}
                        iconUrl={source.icon_url}
                      />
                      <div className="min-w-0">
                        <Link to={`/sources/${source.id}`} className="font-medium hover:underline">
                          {source.name}
                        </Link>
                        <p className="mt-1 line-clamp-1 text-xs text-muted-foreground">{source.legal_name}</p>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{label(source.source_type)}</Badge>
                  </TableCell>
                  <TableCell>
                    {source.website ? (
                      <a
                        href={source.website}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex max-w-56 items-center gap-1 truncate text-muted-foreground hover:text-foreground"
                      >
                        <span className="truncate">{source.website.replace(/^https?:\/\//, '')}</span>
                        <ExternalLink className="size-3 shrink-0" />
                      </a>
                    ) : (
                      <span className="text-muted-foreground">Not set</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant={source.active ? 'outline' : 'secondary'} className="gap-1.5">
                      <span
                        className={cn('size-1.5 rounded-full', source.active ? 'bg-emerald-500' : 'bg-amber-500')}
                        aria-hidden="true"
                      />
                      {source.active ? 'Active' : 'Held'}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={`Open context menu for ${source.name}`}
                          />
                        }
                      >
                        <EllipsisVertical />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" className="w-44">
                        <DropdownMenuItem onClick={() => navigate(`/sources/${source.id}`)}>
                          <Eye />
                          View
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => {
                            setEditLogo(null)
                            setRemoveEditLogo(false)
                            setEditingSource(source)
                          }}
                        >
                          <Pencil />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => setConfirmAction({ kind: 'status', source })}>
                          {source.active ? <CirclePause /> : <CirclePlay />}
                          {source.active ? 'Hold' : 'Activate'}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => setConfirmAction({ kind: 'delete', source })}
                        >
                          <Trash2 />
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {pagination ? (
            <DataTablePagination
              pagination={pagination}
              pageHref={table.pageHref}
              onPerPageChange={table.setPerPage}
            />
          ) : null}
        </CardContent>
      </Card>

      <Dialog
        open={editingSource !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && !update.isPending) {
            setEditingSource(null)
            setEditLogo(null)
            setRemoveEditLogo(false)
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit publisher source</DialogTitle>
            <DialogDescription>Update the publisher details shown throughout the admin portal.</DialogDescription>
          </DialogHeader>
          {editingSource ? (
            <form
              onSubmit={(event) => {
                event.preventDefault()
                update.mutate(editingSource)
              }}
            >
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="edit-name">Name</FieldLabel>
                  <Input
                    id="edit-name"
                    value={editingSource.name}
                    onChange={(event) =>
                      setEditingSource((current) =>
                        current ? { ...current, name: event.target.value } : current,
                      )
                    }
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="edit-legal-name">Legal name</FieldLabel>
                  <Input
                    id="edit-legal-name"
                    value={editingSource.legal_name}
                    onChange={(event) =>
                      setEditingSource((current) =>
                        current ? { ...current, legal_name: event.target.value } : current,
                      )
                    }
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="edit-source-type">Type</FieldLabel>
                  <Select
                    value={editingSource.source_type}
                    onValueChange={(value) =>
                      setEditingSource((current) =>
                        current && value ? { ...current, source_type: value as SourceType } : current,
                      )
                    }
                  >
                    <SelectTrigger id="edit-source-type" className="w-full">
                      <SelectValue>{(value) => label(String(value))}</SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      {types.map((type) => (
                        <SelectItem key={type} value={type}>
                          {label(type)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
                <Field>
                  <FieldLabel htmlFor="edit-website">Website</FieldLabel>
                  <Input
                    id="edit-website"
                    type="url"
                    value={editingSource.website}
                    onChange={(event) =>
                      setEditingSource((current) =>
                        current ? { ...current, website: event.target.value } : current,
                      )
                    }
                  />
                </Field>
                <SourceLogoUpload
                  currentIconUrl={editingSource.icon_url}
                  disabled={update.isPending}
                  file={editLogo}
                  name={editingSource.name}
                  onFileChange={setEditLogo}
                  onRemoveChange={setRemoveEditLogo}
                  remove={removeEditLogo}
                  website={editingSource.website}
                />
                <Field>
                  <FieldLabel htmlFor="edit-description">Description</FieldLabel>
                  <Input
                    id="edit-description"
                    value={editingSource.description}
                    onChange={(event) =>
                      setEditingSource((current) =>
                        current ? { ...current, description: event.target.value } : current,
                      )
                    }
                  />
                </Field>
                <DialogFooter>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setEditingSource(null)
                      setEditLogo(null)
                      setRemoveEditLogo(false)
                    }}
                  >
                    Cancel
                  </Button>
                  <Button type="submit" disabled={update.isPending}>
                    {update.isPending ? 'Saving…' : 'Save changes'}
                  </Button>
                </DialogFooter>
              </FieldGroup>
            </form>
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog
        open={confirmAction !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && !toggle.isPending && !archive.isPending) setConfirmAction(null)
        }}
      >
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>
              {confirmAction?.kind === 'delete'
                ? `Delete ${confirmAction.source.name}?`
                : `${confirmAction?.source.active ? 'Hold' : 'Activate'} ${confirmAction?.source.name}?`}
            </DialogTitle>
            <DialogDescription>
              {confirmAction?.kind === 'delete'
                ? 'This removes the source from the registry and stops its endpoints. Existing articles are preserved.'
                : confirmAction?.source.active
                  ? 'Newly captured articles will be held from publication until this source is activated again.'
                  : 'Future captured articles can be published according to this source’s rights profile.'}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={toggle.isPending || archive.isPending}
              onClick={() => setConfirmAction(null)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant={
                confirmAction?.kind === 'delete' || confirmAction?.source.active
                  ? 'destructive'
                  : 'default'
              }
              disabled={toggle.isPending || archive.isPending}
              onClick={() => {
                if (!confirmAction) return
                if (confirmAction.kind === 'delete') {
                  archive.mutate(confirmAction.source.id)
                  return
                }
                toggle.mutate({ id: confirmAction.source.id, active: !confirmAction.source.active })
              }}
            >
              {archive.isPending || toggle.isPending
                ? 'Working…'
                : confirmAction?.kind === 'delete'
                  ? 'Delete source'
                  : confirmAction?.source.active
                    ? 'Hold source'
                    : 'Activate source'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
