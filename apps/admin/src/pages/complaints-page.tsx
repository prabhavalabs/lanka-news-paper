import { createClient } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { MessageSquareWarning } from 'lucide-react'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useTableQuery } from '@/hooks/use-table-query'

const client = createClient()

export function ComplaintsPage() {
  const queryClient = useQueryClient()
  const table = useTableQuery()
  const status = table.filter('status')
  const complaints = useQuery({
    queryKey: ['complaints', table.page, table.perPage, table.search, status],
    queryFn: () =>
      client.complaints({
        page: table.page,
        per_page: table.perPage,
        search: table.search,
        status,
      }),
    placeholderData: keepPreviousData,
  })
  const resolve = useMutation({
    mutationFn: (id: string) => client.resolveComplaint(id, 'resolved', 'reviewed'),
    onSuccess: () => {
      toast.success('Complaint resolved')
      void queryClient.invalidateQueries({ queryKey: ['complaints'] })
    },
    onError: () => toast.error('Could not resolve complaint'),
  })
  const pagination = complaints.data?.pagination

  return (
    <section className="flex flex-col gap-6">
      <div>
        <Badge variant="outline" className="mb-2">
          Audience care
        </Badge>
        <h1 className="font-heading text-2xl font-semibold tracking-tight">Complaints</h1>
        <p className="mt-1 text-sm text-muted-foreground">Review correction requests and record their resolution.</p>
      </div>
      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Complaint register</CardTitle>
          <CardDescription>Search requests by reason, entity, or contact.</CardDescription>
          <CardAction>
            <Badge variant="secondary">{pagination?.total ?? 0} requests</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar
            search={table.search}
            searchPlaceholder="Search complaints…"
            onSearch={table.setSearch}
          >
            <Select
              value={status || 'all'}
              onValueChange={(value) => {
                if (value !== null) table.setFilter('status', value === 'all' ? '' : value)
              }}
            >
              <SelectTrigger size="sm" className="min-w-36" aria-label="Filter complaint status">
                <SelectValue>
                  {() => (status ? status.replaceAll('_', ' ') : 'All statuses')}
                </SelectValue>
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="open">Open</SelectItem>
                <SelectItem value="in_review">In review</SelectItem>
                <SelectItem value="resolved">Resolved</SelectItem>
                <SelectItem value="rejected">Rejected</SelectItem>
              </SelectContent>
            </Select>
          </DataTableToolbar>
          <Table className="min-w-[760px]">
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead>Request</TableHead>
                <TableHead>Entity</TableHead>
                <TableHead>Contact</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {complaints.isPending
                ? Array.from({ length: 4 }, (_, index) => (
                    <TableRow key={index}>
                      {Array.from({ length: 5 }, (_, cell) => (
                        <TableCell key={cell}>
                          <Skeleton className="h-5 w-full max-w-40" />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                : null}
              {complaints.isError ? (
                <TableRow>
                  <TableCell colSpan={5} className="h-40 text-center text-muted-foreground">
                    Complaints are temporarily unavailable.
                  </TableCell>
                </TableRow>
              ) : null}
              {!complaints.isPending && !complaints.isError && complaints.data.items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="h-48 text-center">
                    <MessageSquareWarning className="mx-auto mb-2 size-5 text-muted-foreground" />
                    <p className="font-medium">No complaints found</p>
                    <p className="mt-1 text-xs text-muted-foreground">Try another search or status.</p>
                  </TableCell>
                </TableRow>
              ) : null}
              {complaints.data?.items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="max-w-md whitespace-normal font-medium">{item.reason}</TableCell>
                  <TableCell>
                    <p className="capitalize">{item.entity_type}</p>
                    <p className="font-mono text-xs text-muted-foreground">{item.entity_id}</p>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{item.contact ?? '—'}</TableCell>
                  <TableCell>
                    <Badge variant={item.status === 'open' ? 'destructive' : 'outline'}>
                      {item.status.replaceAll('_', ' ')}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    {item.status === 'open' ? (
                      <Button size="sm" disabled={resolve.isPending} onClick={() => resolve.mutate(item.id)}>
                        Resolve
                      </Button>
                    ) : null}
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
