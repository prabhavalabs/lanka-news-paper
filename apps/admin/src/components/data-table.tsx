import type { QueueItem } from '@snap/api-client'
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Columns3,
  Inbox,
  Search,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const PAGE_SIZE = 6
const dateFormatter = new Intl.DateTimeFormat('en', {
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
})

type QueueFilter = 'all' | 'held' | 'quarantined' | 'low-confidence'
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
  data: QueueItem[]
  isLoading: boolean
  isError: boolean
}

export function DataTable({ data, isLoading, isError }: DataTableProps) {
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState<QueueFilter>('all')
  const [page, setPage] = useState(0)
  const [visible, setVisible] = useState<VisibleColumns>({
    source: true,
    category: true,
    confidence: true,
    received: true,
  })

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase()
    return data.filter((item) => {
      const matchesSearch = !query || `${item.headline} ${item.source}`.toLowerCase().includes(query)
      const matchesFilter =
        filter === 'all' ||
        (filter === 'low-confidence'
          ? item.confidence != null && item.confidence < 0.45
          : item.public_status === filter)
      return matchesSearch && matchesFilter
    })
  }, [data, filter, search])

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const currentPage = Math.min(page, pageCount - 1)
  const rows = filtered.slice(currentPage * PAGE_SIZE, (currentPage + 1) * PAGE_SIZE)
  const visibleColumnCount = Object.values(visible).filter(Boolean).length + 3

  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardHeader className="border-b py-6">
        <CardTitle>Editorial queue</CardTitle>
        <CardDescription>Review held, quarantined, and low-confidence articles.</CardDescription>
        <CardAction>
          <Badge variant="secondary">{data.length} items</Badge>
        </CardAction>
      </CardHeader>
      <CardContent className="px-0">
        <div className="flex flex-col gap-3 border-b px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative w-full sm:max-w-sm">
            <label htmlFor="queue-search" className="sr-only">
              Search editorial queue
            </label>
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="queue-search"
              value={search}
              onChange={(event) => {
                setSearch(event.target.value)
                setPage(0)
              }}
              placeholder="Search headline or source…"
              className="pl-9"
            />
          </div>
          <div className="flex items-center gap-2">
            <Select
              value={filter}
              onValueChange={(value) => {
                if (
                  value === 'all' ||
                  value === 'held' ||
                  value === 'quarantined' ||
                  value === 'low-confidence'
                ) {
                  setFilter(value)
                  setPage(0)
                }
              }}
            >
              <SelectTrigger className="min-w-36" size="sm" aria-label="Filter queue">
                <SelectValue placeholder="All items" />
              </SelectTrigger>
              <SelectContent align="end">
                <SelectItem value="all">All items</SelectItem>
                <SelectItem value="held">Held</SelectItem>
                <SelectItem value="quarantined">Quarantined</SelectItem>
                <SelectItem value="low-confidence">Low confidence</SelectItem>
              </SelectContent>
            </Select>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<Button variant="outline" size="sm" aria-label="Choose table columns" />}
              >
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
          </div>
        </div>

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
            {isLoading
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
            {!isLoading && isError ? (
              <TableRow>
                <TableCell colSpan={visibleColumnCount} className="h-40 text-center text-muted-foreground">
                  The editorial queue is temporarily unavailable.
                </TableCell>
              </TableRow>
            ) : null}
            {!isLoading && !isError && rows.length === 0 ? (
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
            {!isLoading && !isError
              ? rows.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="max-w-96 whitespace-normal">
                      <p className="line-clamp-2 font-medium leading-snug">{item.headline}</p>
                      {item.model ? <p className="mt-1 text-xs text-muted-foreground">{item.model}</p> : null}
                    </TableCell>
                    {visible.source ? <TableCell>{item.source}</TableCell> : null}
                    {visible.category ? (
                      <TableCell className="capitalize">{item.category?.replaceAll('-', ' ') ?? 'Unassigned'}</TableCell>
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
                      <Button variant="ghost" size="sm" nativeButton={false} render={<Link to="/queue" />}>
                        Review
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              : null}
          </TableBody>
        </Table>
      </CardContent>
      <CardFooter className="justify-between gap-4 border-t py-4">
        <p className="text-sm text-muted-foreground">
          {filtered.length === 0
            ? '0 results'
            : `${currentPage * PAGE_SIZE + 1}–${Math.min((currentPage + 1) * PAGE_SIZE, filtered.length)} of ${filtered.length}`}
        </p>
        <div className="flex items-center gap-2">
          <span className="hidden text-sm text-muted-foreground sm:inline">
            Page {currentPage + 1} of {pageCount}
          </span>
          <Button
            variant="outline"
            size="icon-sm"
            aria-label="Previous page"
            disabled={currentPage === 0}
            onClick={() => setPage(currentPage - 1)}
          >
            <ChevronLeft />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            aria-label="Next page"
            disabled={currentPage >= pageCount - 1}
            onClick={() => setPage(currentPage + 1)}
          >
            <ChevronRight />
          </Button>
        </div>
      </CardFooter>
    </Card>
  )
}
