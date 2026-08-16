import { createClient } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useParams } from 'react-router'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useTableQuery } from '@/hooks/use-table-query'

const client = createClient()
const selectClass =
  'h-8 w-full border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50'
const modes = ['discovery_only', 'licensed_excerpt', 'licensed_media', 'full_syndication', 'internal_verification', 'disabled']
const endpointTypes = ['rss', 'atom', 'json_feed', 'rest_api', 'webhook', 'youtube']

export function SourceDetailPage() {
  const { id } = useParams()
  const queryClient = useQueryClient()
  const endpointTable = useTableQuery('endpoints')
  const rightsTable = useTableQuery('rights')
  const endpointHealth = endpointTable.filter('health')
  const endpointStatus = endpointTable.filter('status')
  const rightsMode = rightsTable.filter('mode')
  const source = useQuery({
    queryKey: ['source', id],
    queryFn: () => client.adminSource(id ?? ''),
    enabled: Boolean(id),
  })
  const endpoints = useQuery({
    queryKey: [
      'endpoints',
      id,
      endpointTable.page,
      endpointTable.perPage,
      endpointTable.search,
      endpointHealth,
      endpointStatus,
    ],
    queryFn: () =>
      client.adminEndpoints(id ?? '', {
        page: endpointTable.page,
        per_page: endpointTable.perPage,
        search: endpointTable.search,
        health: endpointHealth,
        status: endpointStatus,
      }),
    enabled: Boolean(id),
    placeholderData: keepPreviousData,
  })
  const endpointOptions = useQuery({
    queryKey: ['endpoint-options', id],
    queryFn: () => client.adminEndpoints(id ?? '', { per_page: 100 }),
    enabled: Boolean(id),
  })
  const rights = useQuery({
    queryKey: [
      'rights',
      id,
      rightsTable.page,
      rightsTable.perPage,
      rightsTable.search,
      rightsMode,
    ],
    queryFn: () =>
      client.adminRights(id ?? '', {
        page: rightsTable.page,
        per_page: rightsTable.perPage,
        search: rightsTable.search,
        mode: rightsMode,
      }),
    enabled: Boolean(id),
    placeholderData: keepPreviousData,
  })
  const [feedType, setFeedType] = useState('rss')
  const [feedUrl, setFeedUrl] = useState('')
  const [rightsEndpoint, setRightsEndpoint] = useState('')
  const [mode, setMode] = useState('discovery_only')
  const [attribution, setAttribution] = useState('')

  const addEndpoint = useMutation({
    mutationFn: () => client.createEndpoint(id ?? '', { endpoint_type: feedType, url: feedUrl }),
    onSuccess: () => {
      toast.success('Endpoint added (paused until you resume)')
      setFeedUrl('')
      void queryClient.invalidateQueries({ queryKey: ['endpoints', id] })
      void queryClient.invalidateQueries({ queryKey: ['endpoint-options', id] })
    },
    onError: () => toast.error('Could not add endpoint'),
  })
  const addRights = useMutation({
    mutationFn: () => client.createRights(id ?? '', { endpoint_id: rightsEndpoint, mode, attribution }),
    onSuccess: () => {
      toast.success('Rights profile saved')
      void queryClient.invalidateQueries({ queryKey: ['rights', id] })
    },
    onError: () => toast.error('Could not save rights'),
  })
  const run = useMutation({
    mutationFn: (endpointId: string) => client.runEndpoint(endpointId),
    onSuccess: () => {
      toast.success('Poll finished')
      void queryClient.invalidateQueries({ queryKey: ['endpoints', id] })
    },
    onError: () => toast.error('Poll failed'),
  })
  const test = useMutation({
    mutationFn: (endpointId: string) => client.testEndpoint(endpointId),
    onSuccess: (result) => toast.success(`HTTP ${result.status} · ${result.contentType || 'no content-type'}`),
    onError: () => toast.error('Test failed'),
  })
  const pause = useMutation({
    mutationFn: ({ endpointId, paused }: { endpointId: string; paused: boolean }) => client.pauseEndpoint(endpointId, paused),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['endpoints', id] }),
  })

  return (
    <section className="flex flex-col gap-8">
      <div>
        <div className="flex items-center gap-3">
          <SourceAvatar name={source.data?.name ?? 'Source'} website={source.data?.website} />
          <div>
            <p className="text-sm text-muted-foreground">{source.data?.source_type}</p>
            <h1 className="text-xl font-medium">{source.data?.name ?? 'Source'}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{source.data?.legal_name}</p>
          </div>
        </div>
        <Button
          className="mt-3"
          variant="outline"
          size="sm"
          onClick={() => {
            if (id && window.confirm('Archive this source?')) {
              void client.archiveSource(id)
            }
          }}
        >
          Archive source
        </Button>
      </div>
      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Endpoints</CardTitle>
          <CardDescription>Publisher feeds, polling state, and ingestion health.</CardDescription>
          <CardAction>
            <Badge variant="secondary">{endpoints.data?.pagination.total ?? 0} endpoints</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar
            search={endpointTable.search}
            searchPlaceholder="Search endpoint URLs or types…"
            onSearch={endpointTable.setSearch}
          >
            <Select
              value={endpointHealth || 'all'}
              onValueChange={(value) => {
                if (value !== null) endpointTable.setFilter('health', value === 'all' ? '' : value)
              }}
            >
              <SelectTrigger size="sm" className="min-w-32" aria-label="Filter endpoint health">
                <SelectValue>
                  {() => (endpointHealth ? endpointHealth.replaceAll('_', ' ') : 'All health')}
                </SelectValue>
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All health</SelectItem>
                {['unknown', 'healthy', 'stale', 'failed', 'auth_denied'].map((health) => (
                  <SelectItem key={health} value={health}>
                    {health.replaceAll('_', ' ')}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={endpointStatus || 'all'}
              onValueChange={(value) => {
                if (value !== null) endpointTable.setFilter('status', value === 'all' ? '' : value)
              }}
            >
              <SelectTrigger size="sm" className="min-w-28" aria-label="Filter endpoint state">
                <SelectValue>{() => (endpointStatus ? endpointStatus : 'All states')}</SelectValue>
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All states</SelectItem>
                <SelectItem value="active">Active</SelectItem>
                <SelectItem value="paused">Paused</SelectItem>
              </SelectContent>
            </Select>
          </DataTableToolbar>
          <Table className="min-w-[900px]">
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead>Endpoint</TableHead>
                <TableHead>Health</TableHead>
                <TableHead>State</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {endpoints.isPending
                ? Array.from({ length: 3 }, (_, row) => (
                    <TableRow key={row}>
                      {Array.from({ length: 4 }, (_, column) => (
                        <TableCell key={column}>
                          <Skeleton className="h-5 w-full max-w-40" />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                : null}
              {endpoints.isError ? (
                <TableRow>
                  <TableCell colSpan={4} className="h-40 text-center text-muted-foreground">
                    Endpoints are temporarily unavailable.
                  </TableCell>
                </TableRow>
              ) : null}
              {!endpoints.isPending && !endpoints.isError && endpoints.data.items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="h-40 text-center text-muted-foreground">
                    No endpoints match these filters.
                  </TableCell>
                </TableRow>
              ) : null}
              {endpoints.data?.items.map((endpoint) => (
                <TableRow key={endpoint.id}>
                  <TableCell className="max-w-md whitespace-normal break-all">
                    <p className="font-medium">{endpoint.url}</p>
                    <p className="mt-1 text-xs uppercase text-muted-foreground">{endpoint.endpoint_type}</p>
                    {endpoint.last_error ? <p className="mt-1 text-xs text-destructive">{endpoint.last_error}</p> : null}
                  </TableCell>
                  <TableCell>
                    <Badge variant={endpoint.health_state === 'failed' ? 'destructive' : 'outline'}>
                      {endpoint.health_state.replaceAll('_', ' ')}
                    </Badge>
                  </TableCell>
                  <TableCell>{endpoint.paused ? 'Paused' : 'Active'}</TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-2">
                      <Button variant="outline" size="sm" onClick={() => pause.mutate({ endpointId: endpoint.id, paused: !endpoint.paused })}>
                        {endpoint.paused ? 'Resume' : 'Pause'}
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => test.mutate(endpoint.id)}>
                        Test
                      </Button>
                      <Button size="sm" onClick={() => run.mutate(endpoint.id)}>
                        Run now
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {endpoints.data ? (
            <DataTablePagination
              pagination={endpoints.data.pagination}
              pageHref={endpointTable.pageHref}
              onPerPageChange={endpointTable.setPerPage}
            />
          ) : null}
          <form
            className="max-w-xl border-t p-6"
            onSubmit={(event) => {
              event.preventDefault()
              addEndpoint.mutate()
            }}
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="etype">Type</FieldLabel>
                <select id="etype" className={selectClass} value={feedType} onChange={(event) => setFeedType(event.target.value)}>
                  {endpointTypes.map((type) => (
                    <option key={type} value={type}>
                      {type}
                    </option>
                  ))}
                </select>
              </Field>
              <Field>
                <FieldLabel htmlFor="eurl">HTTPS URL</FieldLabel>
                <Input id="eurl" type="url" value={feedUrl} onChange={(event) => setFeedUrl(event.target.value)} required />
              </Field>
              <Button type="submit" disabled={addEndpoint.isPending}>Add endpoint</Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Rights profiles</CardTitle>
          <CardDescription>Usage permissions and required publisher attribution.</CardDescription>
          <CardAction>
            <Badge variant="secondary">{rights.data?.pagination.total ?? 0} profiles</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar
            search={rightsTable.search}
            searchPlaceholder="Search rights or attribution…"
            onSearch={rightsTable.setSearch}
          >
            <Select
              value={rightsMode || 'all'}
              onValueChange={(value) => {
                if (value !== null) rightsTable.setFilter('mode', value === 'all' ? '' : value)
              }}
            >
              <SelectTrigger size="sm" className="min-w-44" aria-label="Filter rights mode">
                <SelectValue>
                  {() => (rightsMode ? rightsMode.replaceAll('_', ' ') : 'All rights modes')}
                </SelectValue>
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All rights modes</SelectItem>
                {modes.map((item) => (
                  <SelectItem key={item} value={item}>
                    {item.replaceAll('_', ' ')}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </DataTableToolbar>
          <Table className="min-w-[620px]">
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead>Mode</TableHead>
                <TableHead>Attribution</TableHead>
                <TableHead>Endpoint</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rights.isPending
                ? Array.from({ length: 3 }, (_, row) => (
                    <TableRow key={row}>
                      {Array.from({ length: 3 }, (_, column) => (
                        <TableCell key={column}>
                          <Skeleton className="h-5 w-full max-w-40" />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                : null}
              {rights.isError ? (
                <TableRow>
                  <TableCell colSpan={3} className="h-40 text-center text-muted-foreground">
                    Rights profiles are temporarily unavailable.
                  </TableCell>
                </TableRow>
              ) : null}
              {!rights.isPending && !rights.isError && rights.data.items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} className="h-40 text-center text-muted-foreground">
                    No rights profiles match these filters.
                  </TableCell>
                </TableRow>
              ) : null}
              {rights.data?.items.map((profile) => (
                <TableRow key={profile.id}>
                  <TableCell>
                    <Badge variant="outline">{profile.mode.replaceAll('_', ' ')}</Badge>
                  </TableCell>
                  <TableCell className="max-w-md whitespace-normal">{profile.attribution}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{profile.endpoint_id}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {rights.data ? (
            <DataTablePagination
              pagination={rights.data.pagination}
              pageHref={rightsTable.pageHref}
              onPerPageChange={rightsTable.setPerPage}
            />
          ) : null}
          <form
            className="max-w-xl border-t p-6"
            onSubmit={(event) => {
              event.preventDefault()
              addRights.mutate()
            }}
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="rend">Endpoint</FieldLabel>
                <select id="rend" className={selectClass} value={rightsEndpoint} onChange={(event) => setRightsEndpoint(event.target.value)} required>
                  <option value="">Select endpoint</option>
                  {endpointOptions.data?.items.map((endpoint) => (
                    <option key={endpoint.id} value={endpoint.id}>
                      {endpoint.url}
                    </option>
                  ))}
                </select>
              </Field>
              <Field>
                <FieldLabel htmlFor="mode">Mode</FieldLabel>
                <select id="mode" className={selectClass} value={mode} onChange={(event) => setMode(event.target.value)}>
                  {modes.map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </Field>
              <Field>
                <FieldLabel htmlFor="attr">Attribution</FieldLabel>
                <Input id="attr" value={attribution} onChange={(event) => setAttribution(event.target.value)} required />
              </Field>
              <Button type="submit" disabled={addRights.isPending}>Add rights profile</Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </section>
  )
}
