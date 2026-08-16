import { createClient } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const client = createClient()

export function ComplaintsPage() {
  const queryClient = useQueryClient()
  const complaints = useQuery({ queryKey: ['complaints'], queryFn: () => client.complaints() })
  const resolve = useMutation({
    mutationFn: (id: string) => client.resolveComplaint(id, 'resolved', 'reviewed'),
    onSuccess: () => {
      toast.success('Resolved')
      void queryClient.invalidateQueries({ queryKey: ['complaints'] })
    },
  })

  return (
    <section className="flex flex-col gap-4">
      <h1 className="text-xl font-medium">Complaints</h1>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Reason</TableHead>
            <TableHead>Status</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {complaints.data?.items.map((item) => (
            <TableRow key={item.id}>
              <TableCell>{item.reason}</TableCell>
              <TableCell>{item.status}</TableCell>
              <TableCell>
                {item.status === 'open' ? (
                  <Button size="sm" onClick={() => resolve.mutate(item.id)}>
                    Resolve
                  </Button>
                ) : null}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  )
}
