import { createClient } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useParams } from 'react-router'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const client = createClient()
const selectClass =
  'h-8 w-full border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50'
const modes = ['discovery_only', 'licensed_excerpt', 'licensed_media', 'full_syndication', 'internal_verification', 'disabled']
const endpointTypes = ['rss', 'atom', 'json_feed', 'rest_api', 'webhook', 'youtube']

export function SourceDetailPage() {
  const { id } = useParams()
  const queryClient = useQueryClient()
  const source = useQuery({
    queryKey: ['source', id],
    queryFn: () => client.adminSource(id ?? ''),
    enabled: Boolean(id),
  })
  const endpoints = useQuery({
    queryKey: ['endpoints', id],
    queryFn: () => client.adminEndpoints(id ?? ''),
    enabled: Boolean(id),
  })
  const rights = useQuery({
    queryKey: ['rights', id],
    queryFn: () => client.adminRights(id ?? ''),
    enabled: Boolean(id),
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
        <p className="text-sm text-muted-foreground">{source.data?.source_type}</p>
        <h1 className="text-xl font-medium">{source.data?.name ?? 'Source'}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{source.data?.legal_name}</p>
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
      <div>
        <h2 className="mb-3 text-sm font-medium">Endpoints</h2>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>URL</TableHead>
              <TableHead>Health</TableHead>
              <TableHead>Paused</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {endpoints.data?.items.map((endpoint) => (
              <TableRow key={endpoint.id}>
                <TableCell className="max-w-md break-all">
                  {endpoint.url}
                  {endpoint.last_error ? <p className="mt-1 text-xs text-destructive">{endpoint.last_error}</p> : null}
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{endpoint.health_state}</Badge>
                </TableCell>
                <TableCell>{endpoint.paused ? 'yes' : 'no'}</TableCell>
                <TableCell className="flex flex-wrap gap-2">
                  <Button variant="outline" size="sm" onClick={() => pause.mutate({ endpointId: endpoint.id, paused: !endpoint.paused })}>
                    {endpoint.paused ? 'Resume' : 'Pause'}
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => test.mutate(endpoint.id)}>
                    Test
                  </Button>
                  <Button size="sm" onClick={() => run.mutate(endpoint.id)}>
                    Run now
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <form
          className="mt-4 max-w-md"
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
            <Button type="submit">Add endpoint</Button>
          </FieldGroup>
        </form>
      </div>
      <div>
        <h2 className="mb-3 text-sm font-medium">Rights profiles</h2>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Mode</TableHead>
              <TableHead>Attribution</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rights.data?.items.map((profile) => (
              <TableRow key={profile.id}>
                <TableCell>{profile.mode}</TableCell>
                <TableCell>{profile.attribution}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <form
          className="mt-4 max-w-md"
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
                {endpoints.data?.items.map((endpoint) => (
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
            <Button type="submit">Add rights profile</Button>
          </FieldGroup>
        </form>
      </div>
    </section>
  )
}
