import { createClient, type SourceType } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ExternalLink, Plus, RadioTower } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
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
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
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

function label(value: string) {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}

export function SourcesPage() {
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
  const [active, setActive] = useState(false)
  const create = useMutation({
    mutationFn: () =>
      client.createSource({
        name,
        legal_name: legalName,
        source_type: sourceType,
        website,
        description: '',
        active,
      }),
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
    mutationFn: ({ id, active: nextActive }: { id: string; active: boolean }) =>
      client.setSourceActive(id, nextActive),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['admin-sources'] }),
    onError: () => toast.error('Could not update source'),
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
                      <SourceAvatar name={source.name} website={source.website} />
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
                    <div className="flex justify-end gap-2">
                      <Link
                        to={`/sources/${source.id}`}
                        className={buttonVariants({ variant: 'ghost', size: 'sm' })}
                      >
                        View
                      </Link>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={toggle.isPending}
                        onClick={() => toggle.mutate({ id: source.id, active: !source.active })}
                      >
                        {source.active ? 'Hold' : 'Activate'}
                      </Button>
                    </div>
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
    </section>
  )
}
