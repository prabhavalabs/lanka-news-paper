import { createClient } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const client = createClient()

export function QueuePage() {
  const queryClient = useQueryClient()
  const queue = useQuery({ queryKey: ['queue'], queryFn: () => client.queue() })
  const quarantine = useQuery({ queryKey: ['quarantine'], queryFn: () => client.quarantine() })
  const publish = useMutation({
    mutationFn: (id: string) => client.setArticleStatus(id, 'published', 'desk publish'),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['queue'] }),
    onError: () => toast.error('Could not publish'),
  })
  const hold = useMutation({
    mutationFn: (id: string) => client.setArticleStatus(id, 'unpublished', 'desk unpublish'),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['queue'] }),
  })

  return (
    <section className="flex flex-col gap-8">
      <h1 className="text-xl font-medium">Editorial queue</h1>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Headline</TableHead>
            <TableHead>Source</TableHead>
            <TableHead>Status</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {queue.data?.items.map((item) => (
            <TableRow key={item.id}>
              <TableCell className="max-w-md">{item.headline}</TableCell>
              <TableCell>{item.source}</TableCell>
              <TableCell>
                {item.public_status}
                {item.confidence != null ? ` · ${item.confidence.toFixed(2)}` : ''}
              </TableCell>
              <TableCell className="flex gap-2">
                <Button size="sm" onClick={() => publish.mutate(item.id)}>
                  Publish
                </Button>
                <Button size="sm" variant="outline" onClick={() => hold.mutate(item.id)}>
                  Unpublish
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <div>
        <h2 className="mb-3 text-sm font-medium">Quarantined payloads</h2>
        <ul className="flex flex-col gap-2 text-sm">
          {quarantine.data?.items.map((item) => (
            <li key={item.id} className="border border-border p-3">
              <p>{item.reason}</p>
              {item.sample ? <pre className="mt-2 overflow-auto text-xs">{item.sample}</pre> : null}
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
