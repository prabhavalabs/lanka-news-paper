import { createClient } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, Columns3, Inbox } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useTableQuery } from '@/hooks/use-table-query'

const client = createClient()
const dateFormatter = new Intl.DateTimeFormat('en', {
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
})

type VisibleColumns = {
  source: boolean
  category: boolean
  confidence: boolean
  received: boolean
}

const columnLabels: Record<keyof VisibleColumns, string> = {
  source: 'Source',
  category: 'Category',
  confidence: 'Confidence',
  received: 'Received',
}

type DataTableProps = {
  prefix?: string
  editable?: boolean
}

export function DataTable({ prefix = 'queue', editable = false }: DataTableProps) {
  const queryClient = useQueryClient()
  const table = useTableQuery(prefix)
  const status = table.filter('status')
  const queue = useQuery({
    queryKey: ['queue', prefix, table.page, table.perPage, table.search, status],
    queryFn: () =>
      client.queue({
        page: table.page,
        per_page: table.perPage,
        search: table.search,
        status,
      }),
    placeholderData: keepPreviousData,
  })
  const updateStatus = useMutation({
    mutationFn: ({ id, nextStatus }: { id: string; nextStatus: string }) =>
      client.setArticleStatus(id, nextStatus, 'desk review'),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['queue'] }),
    onError: () => toast.error('Could not update article'),
  })
  const [visible, setVisible] = useState<VisibleColumns>({
    source: true,
    category: true,
    confidence: true,
    received: true,
  })
  const rows = queue.data?.items ?? []
  const pagination = queue.data?.pagination
  const visibleColumnCount = Object.values(visible).filter(Boolean).length + 3

  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardHeader className="border-b py-6">
        <CardTitle>Editorial queue</CardTitle>
        <CardDescription>Review held, quarantined, and low-confidence articles.</CardDescription>
        <CardAction>
          <Badge variant="secondary">{pagination?.total ?? 0} items</Badge>
        </CardAction>
      </CardHeader>
      <CardContent className="px-0">
        <DataTableToolbar
          search={table.search}
          searchPlaceholder="Search headline or source…"
          onSearch={table.setSearch}
        >
          <Select
            value={status || 'all'}
            onValueChange={(value) => {
              if (value !== null) table.setFilter('status', value === 'all' ? '' : value)
            }}
          >
            <SelectTrigger className="min-w-40" size="sm" aria-label="Filter queue">
              <SelectValue>
                {() => (status ? status.replaceAll('_', ' ') : 'All items')}
              </SelectValue>
            </SelectTrigger>
            <SelectContent align="end">
              <SelectItem value="all">All items</SelectItem>
              <SelectItem value="held">Held</SelectItem>
              <SelectItem value="quarantined">Quarantined</SelectItem>
              <SelectItem value="low_confidence">Low confidence</SelectItem>
            </SelectContent>
          </Select>
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button variant="outline" size="sm" aria-label="Choose table columns" />}>
              <Columns3 data-icon="inline-start" />
              <span className="hidden sm:inline">Columns</span>
              <ChevronDown data-icon="inline-end" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-40">
              <DropdownMenuGroup>
                <DropdownMenuLabel>Show columns</DropdownMenuLabel>
                {(Object.keys(columnLabels) as (keyof VisibleColumns)[]).map((column) => (
                  <DropdownMenuCheckboxItem
                    key={column}
                    checked={visible[column]}
                    onCheckedChange={(checked) =>
                      setVisible((current) => ({ ...current, [column]: Boolean(checked) }))
                    }
                  >
                    {columnLabels[column]}
                  </DropdownMenuCheckboxItem>
                ))}
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </DataTableToolbar>

        <Table className="min-w-[820px]">
          <TableHeader className="bg-muted/30">
            <TableRow>
              <TableHead>Headline</TableHead>
              {visible.source ? <TableHead>Source</TableHead> : null}
              {visible.category ? <TableHead>Category</TableHead> : null}
              <TableHead>Status</TableHead>
              {visible.confidence ? <TableHead>Confidence</TableHead> : null}
              {visible.received ? <TableHead>Received</TableHead> : null}
              <TableHead className="text-right">Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {queue.isPending
              ? Array.from({ length: 4 }, (_, index) => (
                  <TableRow key={index}>
                    {Array.from({ length: visibleColumnCount }, (_, cell) => (
                      <TableCell key={cell}>
                        <Skeleton className="h-5 w-full max-w-36" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              : null}
            {queue.isError ? (
              <TableRow>
                <TableCell colSpan={visibleColumnCount} className="h-40 text-center text-muted-foreground">
                  The editorial queue is temporarily unavailable.
                </TableCell>
              </TableRow>
            ) : null}
            {!queue.isPending && !queue.isError && rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={visibleColumnCount} className="h-48 text-center">
                  <div className="flex flex-col items-center gap-2 text-muted-foreground">
                    <Inbox className="size-5" />
                    <p className="font-medium text-foreground">No queue items found</p>
                    <p className="text-xs">Try another filter or enjoy the quiet newsroom.</p>
                  </div>
                </TableCell>
              </TableRow>
            ) : null}
            {!queue.isPending && !queue.isError
              ? rows.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="max-w-96 whitespace-normal">
                      <p className="line-clamp-2 font-medium leading-snug">{item.headline}</p>
                      {item.model ? <p className="mt-1 text-xs text-muted-foreground">{item.model}</p> : null}
                    </TableCell>
                    {visible.source ? <TableCell>{item.source}</TableCell> : null}
                    {visible.category ? (
                      <TableCell className="capitalize">
                        {item.category?.replaceAll('-', ' ') ?? 'Unassigned'}
                      </TableCell>
                    ) : null}
                    <TableCell>
                      <Badge variant={item.public_status === 'quarantined' ? 'destructive' : 'outline'}>
                        {item.public_status}
                      </Badge>
                    </TableCell>
                    {visible.confidence ? (
                      <TableCell className="tabular-nums">
                        {item.confidence == null
                          ? '—'
                          : item.confidence.toLocaleString('en', {
                              style: 'percent',
                              maximumFractionDigits: 0,
                            })}
                      </TableCell>
                    ) : null}
                    {visible.received ? (
                      <TableCell className="text-muted-foreground">
                        {dateFormatter.format(new Date(item.received_at))}
                      </TableCell>
                    ) : null}
                    <TableCell className="text-right">
                      {editable ? (
                        <div className="flex justify-end gap-2">
                          <Button
                            size="sm"
                            disabled={updateStatus.isPending}
                            onClick={() => updateStatus.mutate({ id: item.id, nextStatus: 'published' })}
                          >
                            Publish
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={updateStatus.isPending}
                            onClick={() => updateStatus.mutate({ id: item.id, nextStatus: 'unpublished' })}
                          >
                            Unpublish
                          </Button>
                        </div>
                      ) : (
                        <Button variant="ghost" size="sm" nativeButton={false} render={<Link to={`/articles/${item.id}`} />}>
                          Inspect
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              : null}
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
  )
}
