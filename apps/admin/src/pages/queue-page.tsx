import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { ShieldAlert } from 'lucide-react'

import { DataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const client = createClient()

export function QueuePage() {
  const quarantine = useQuery({ queryKey: ['quarantine'], queryFn: () => client.quarantine() })

  return (
    <section className="flex flex-col gap-6">
      <div>
        <Badge variant="outline" className="mb-2">
          Editorial operations
        </Badge>
        <h1 className="font-heading text-2xl font-semibold tracking-tight">Review desk</h1>
        <p className="mt-1 text-sm text-muted-foreground">Publish, hold, and inspect items that need human judgment.</p>
      </div>
      <DataTable prefix="" editable />
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ShieldAlert className="size-4" />
            Quarantined payloads
          </CardTitle>
          <CardDescription>Raw ingestion payloads isolated for manual inspection.</CardDescription>
        </CardHeader>
        <CardContent>
        <ul className="flex flex-col gap-2 text-sm">
          {quarantine.data?.items.map((item) => (
            <li key={item.id} className="rounded-lg border border-border p-4">
              <p className="font-medium">{item.reason}</p>
              {item.sample ? <pre className="mt-2 overflow-auto text-xs">{item.sample}</pre> : null}
            </li>
          ))}
        </ul>
        </CardContent>
      </Card>
    </section>
  )
}
