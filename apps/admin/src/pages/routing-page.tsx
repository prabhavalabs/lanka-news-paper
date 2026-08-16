import { createClient } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const client = createClient()

export function RoutingPage() {
  const queryClient = useQueryClient()
  const providers = useQuery({ queryKey: ['llm'], queryFn: () => client.llmProviders() })
  const profiles = useQuery({ queryKey: ['llm-profiles'], queryFn: () => client.llmProfiles() })
  const [id, setId] = useState('openai')
  const [baseUrl, setBaseUrl] = useState('https://api.openai.com/v1')
  const [keyRef, setKeyRef] = useState('OPENAI_API_KEY')
  const save = useMutation({
    mutationFn: () => client.upsertProvider({ id, kind: 'openai_api', base_url: baseUrl, enabled: true, api_key_ref: keyRef }),
    onSuccess: () => {
      toast.success('Provider saved')
      void queryClient.invalidateQueries({ queryKey: ['llm'] })
    },
    onError: () => toast.error('Could not save provider'),
  })

  return (
    <section className="flex flex-col gap-8">
      <div>
        <h1 className="text-xl font-medium">AI & Routing</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Keys stay in environment variables. The pipeline falls back to keyword rules when no provider answers.
        </p>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>Kind</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Key</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {providers.data?.items.map((provider) => (
            <TableRow key={provider.id}>
              <TableCell>{provider.id}</TableCell>
              <TableCell>{provider.kind}</TableCell>
              <TableCell>{provider.status}</TableCell>
              <TableCell>{provider.key_set ? 'set' : 'unset'}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Task</TableHead>
            <TableHead>Provider</TableHead>
            <TableHead>Model</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {profiles.data?.items.map((profile) => (
            <TableRow key={`${profile.task}-${profile.priority}`}>
              <TableCell>{profile.task}</TableCell>
              <TableCell>{profile.provider_id}</TableCell>
              <TableCell>{profile.model}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <form
        className="max-w-md"
        onSubmit={(event) => {
          event.preventDefault()
          save.mutate()
        }}
      >
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="id">Provider id</FieldLabel>
            <Input id="id" value={id} onChange={(event) => setId(event.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="base">Base URL</FieldLabel>
            <Input id="base" value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="ref">API key env name</FieldLabel>
            <Input id="ref" value={keyRef} onChange={(event) => setKeyRef(event.target.value)} />
          </Field>
          <Button type="submit">Save provider</Button>
        </FieldGroup>
      </form>
    </section>
  )
}
