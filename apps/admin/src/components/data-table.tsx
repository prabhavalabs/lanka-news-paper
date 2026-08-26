import { createClient } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, Columns3, Inbox } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import {
  ArticleActionsMenu,
  ArticleDeleteDialog,
  ArticleReviewDialog,
  articleCategoryLabel,
  type ArticleActionItem,
  type ArticleReviewChange,
} from '@/components/article-actions'
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

function statusLabel(status: string) {
  return status === 'published' ? 'Public' : status.replaceAll('_', ' ')
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
  const categories = useQuery({
    queryKey: ['categories'],
    queryFn: () => client.categories(),
  })
  const [editingItem, setEditingItem] = useState<ArticleActionItem | null>(null)
  const [deletingItem, setDeletingItem] = useState<ArticleActionItem | null>(null)
  const review = useMutation({
    mutationFn: ({ id, status: nextStatus, category, reason }: ArticleReviewChange) =>
      client.reviewArticle(id, { status: nextStatus, category, reason }),
    onSuccess: (_data, change) => {
      toast.success(change.quick ? 'Article is now public' : 'Editorial decision saved')
      setEditingItem(null)
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: ['queue'] }),
        queryClient.invalidateQueries({ queryKey: ['articles'] }),
        queryClient.invalidateQueries({ queryKey: ['article', change.id] }),
      ])
    },
    onError: (error) => toast.error(error.message || 'Could not save editorial decision'),
  })
  const remove = useMutation({
    mutationFn: (item: ArticleActionItem) => client.deleteArticle(item.id, 'Deleted from the editorial queue'),
    onSuccess: (_data, item) => {
      toast.success('Article deleted')
      setDeletingItem(null)
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: ['queue'] }),
        queryClient.invalidateQueries({ queryKey: ['articles'] }),
        queryClient.invalidateQueries({ queryKey: ['article', item.id] }),
      ])
    },
    onError: (error) => toast.error(error.message || 'Could not delete article'),
  })
  const [visible, setVisible] = useState<VisibleColumns>({
    source: true,
    category: true,
    confidence: true,
    received: true,
  })
  const rows = queue.data?.items ?? []
  const pagination = queue.data?.pagination
  const categoryOptions = (categories.data?.items ?? []).map((category) => ({
    value: category.slug,
    label: articleCategoryLabel(category.slug),
  }))
  const visibleColumnCount = Object.values(visible).filter(Boolean).length + 3

  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardHeader className="border-b py-6">
        <CardTitle>Editorial queue</CardTitle>
        <CardDescription>Review held and low-confidence articles that need an editorial decision.</CardDescription>
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
                      <Badge variant={item.public_status === 'published' ? 'default' : 'outline'} className="capitalize">
                        {statusLabel(item.public_status)}
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
                        <ArticleActionsMenu
                          article={item}
                          busy={review.isPending || remove.isPending}
                          quickPublish
                          onQuickPublish={(article) => review.mutate({
                            id: article.id,
                            status: 'published',
                            reason: 'Made public from the editorial queue',
                            quick: true,
                          })}
                          onEdit={setEditingItem}
                          onDelete={setDeletingItem}
                        />
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
      {editingItem ? (
        <ArticleReviewDialog
          key={editingItem.id}
          article={editingItem}
          categories={categoryOptions}
          categoriesLoading={categories.isPending}
          open
          saving={review.isPending}
          defaultReason="Updated from the editorial queue"
          onOpenChange={(open) => {
            if (!open && !review.isPending) setEditingItem(null)
          }}
          onSave={(change) => review.mutate(change)}
        />
      ) : null}
      {deletingItem ? (
        <ArticleDeleteDialog
          key={deletingItem.id}
          article={deletingItem}
          open
          deleting={remove.isPending}
          onOpenChange={(open) => {
            if (!open && !remove.isPending) setDeletingItem(null)
          }}
          onConfirm={(article) => remove.mutate(article)}
        />
      ) : null}
    </Card>
  )
}
