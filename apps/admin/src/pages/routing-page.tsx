import { createClient } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Cpu } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
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

export function RoutingPage() {
  const queryClient = useQueryClient()
  const providerTable = useTableQuery('providers')
  const profileTable = useTableQuery('profiles')
  const providerState = providerTable.filter('state')
  const profileState = profileTable.filter('state')
  const providers = useQuery({
    queryKey: [
      'llm',
      providerTable.page,
      providerTable.perPage,
      providerTable.search,
      providerState,
    ],
    queryFn: () =>
      client.llmProviders({
        page: providerTable.page,
        per_page: providerTable.perPage,
        search: providerTable.search,
        state: providerState,
      }),
    placeholderData: keepPreviousData,
  })
  const profiles = useQuery({
    queryKey: [
      'llm-profiles',
      profileTable.page,
      profileTable.perPage,
      profileTable.search,
      profileState,
    ],
    queryFn: () =>
      client.llmProfiles({
        page: profileTable.page,
        per_page: profileTable.perPage,
        search: profileTable.search,
        state: profileState,
      }),
    placeholderData: keepPreviousData,
  })
  const [id, setId] = useState('openai')
  const [baseUrl, setBaseUrl] = useState('https://api.openai.com/v1')
  const [keyRef, setKeyRef] = useState('OPENAI_API_KEY')
  const save = useMutation({
    mutationFn: () =>
      client.upsertProvider({
        id,
        kind: 'openai_api',
        base_url: baseUrl,
        enabled: true,
        api_key_ref: keyRef,
      }),
    onSuccess: () => {
      toast.success('Provider saved')
      void queryClient.invalidateQueries({ queryKey: ['llm'] })
    },
    onError: () => toast.error('Could not save provider'),
  })

  return (
    <section className="flex flex-col gap-6">
      <div>
        <Badge variant="outline" className="mb-2">
          <Cpu />
          AI operations
        </Badge>
        <h1 className="font-heading text-2xl font-semibold tracking-tight">AI & routing</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Keys stay in environment variables. Keyword rules remain the fallback when no provider answers.
        </p>
      </div>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Providers</CardTitle>
          <CardDescription>Configured inference services and credential readiness.</CardDescription>
          <CardAction>
            <Badge variant="secondary">{providers.data?.pagination.total ?? 0} providers</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar
            search={providerTable.search}
            searchPlaceholder="Search providers…"
            onSearch={providerTable.setSearch}
          >
            <StateFilter value={providerState} onChange={(value) => providerTable.setFilter('state', value)} />
          </DataTableToolbar>
          <Table className="min-w-[680px]">
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead>Provider</TableHead>
                <TableHead>Kind</TableHead>
                <TableHead>Service status</TableHead>
                <TableHead>Configuration</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {providers.isPending ? <LoadingRows columns={4} /> : null}
              {providers.isError ? <TableMessage columns={4}>Providers are temporarily unavailable.</TableMessage> : null}
              {!providers.isPending && !providers.isError && providers.data.items.length === 0 ? (
                <TableMessage columns={4}>No providers match these filters.</TableMessage>
              ) : null}
              {providers.data?.items.map((provider) => (
                <TableRow key={provider.id}>
                  <TableCell>
                    <p className="font-medium">{provider.id}</p>
                    <p className="text-xs text-muted-foreground">{provider.base_url || 'Default endpoint'}</p>
                  </TableCell>
                  <TableCell>{provider.kind.replaceAll('_', ' ')}</TableCell>
                  <TableCell>
                    <Badge variant={provider.enabled ? 'outline' : 'secondary'}>
                      {provider.enabled ? provider.status : 'disabled'}
                    </Badge>
                  </TableCell>
                  <TableCell>{provider.key_set ? 'Key configured' : 'Key missing'}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {providers.data ? (
            <DataTablePagination
              pagination={providers.data.pagination}
              pageHref={providerTable.pageHref}
              onPerPageChange={providerTable.setPerPage}
            />
          ) : null}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Task profiles</CardTitle>
          <CardDescription>Model selection, priority, and runtime limits per newsroom task.</CardDescription>
          <CardAction>
            <Badge variant="secondary">{profiles.data?.pagination.total ?? 0} profiles</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar
            search={profileTable.search}
            searchPlaceholder="Search tasks, providers, or models…"
            onSearch={profileTable.setSearch}
          >
            <StateFilter value={profileState} onChange={(value) => profileTable.setFilter('state', value)} />
          </DataTableToolbar>
          <Table className="min-w-[680px]">
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead>Task</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Priority</TableHead>
                <TableHead>Timeout</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {profiles.isPending ? <LoadingRows columns={5} /> : null}
              {profiles.isError ? <TableMessage columns={5}>Profiles are temporarily unavailable.</TableMessage> : null}
              {!profiles.isPending && !profiles.isError && profiles.data.items.length === 0 ? (
                <TableMessage columns={5}>No task profiles match these filters.</TableMessage>
              ) : null}
              {profiles.data?.items.map((profile) => (
                <TableRow key={`${profile.task}-${profile.priority}`}>
                  <TableCell className="font-medium">{profile.task.replaceAll('_', ' ')}</TableCell>
                  <TableCell>{profile.provider_id}</TableCell>
                  <TableCell>{profile.model}</TableCell>
                  <TableCell className="tabular-nums">{profile.priority}</TableCell>
                  <TableCell className="tabular-nums">{profile.timeout_seconds}s</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {profiles.data ? (
            <DataTablePagination
              pagination={profiles.data.pagination}
              pageHref={profileTable.pageHref}
              onPerPageChange={profileTable.setPerPage}
            />
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Configure provider</CardTitle>
          <CardDescription>The API key value remains outside the application database.</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="max-w-xl"
            onSubmit={(event) => {
              event.preventDefault()
              save.mutate()
            }}
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="id">Provider id</FieldLabel>
                <Input id="id" value={id} onChange={(event) => setId(event.target.value)} required />
              </Field>
              <Field>
                <FieldLabel htmlFor="base">Base URL</FieldLabel>
                <Input id="base" type="url" value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} required />
              </Field>
              <Field>
                <FieldLabel htmlFor="ref">API key env name</FieldLabel>
                <Input id="ref" value={keyRef} onChange={(event) => setKeyRef(event.target.value)} required />
              </Field>
              <Button type="submit" disabled={save.isPending}>
                {save.isPending ? 'Saving…' : 'Save provider'}
              </Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </section>
  )
}

function StateFilter({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <Select
      value={value || 'all'}
      onValueChange={(nextValue) => {
        if (nextValue !== null) onChange(nextValue === 'all' ? '' : nextValue)
      }}
    >
      <SelectTrigger size="sm" className="min-w-32" aria-label="Filter enabled state">
        <SelectValue>{() => (value ? value : 'All states')}</SelectValue>
      </SelectTrigger>
      <SelectContent align="end">
        <SelectItem value="all">All states</SelectItem>
        <SelectItem value="enabled">Enabled</SelectItem>
        <SelectItem value="disabled">Disabled</SelectItem>
      </SelectContent>
    </Select>
  )
}

function LoadingRows({ columns }: { columns: number }) {
  return Array.from({ length: 3 }, (_, row) => (
    <TableRow key={row}>
      {Array.from({ length: columns }, (_, column) => (
        <TableCell key={column}>
          <Skeleton className="h-5 w-full max-w-40" />
        </TableCell>
      ))}
    </TableRow>
  ))
}

function TableMessage({ columns, children }: { columns: number; children: React.ReactNode }) {
  return (
    <TableRow>
      <TableCell colSpan={columns} className="h-40 text-center text-muted-foreground">
        {children}
      </TableCell>
    </TableRow>
  )
}
