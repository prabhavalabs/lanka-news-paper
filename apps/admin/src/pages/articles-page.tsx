import { createClient } from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Newspaper } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import {
  ArticleActionsMenu,
  ArticleDeleteDialog,
  ArticleReviewDialog,
  type ArticleActionItem,
  type ArticleReviewChange,
} from '@/components/article-actions'
import { SourceAvatar } from '@/components/source-avatar'
import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { Badge } from '@/components/ui/badge'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Combobox, ComboboxContent, ComboboxEmpty, ComboboxInput, ComboboxItem, ComboboxList, ComboboxTrigger } from '@/components/ui/combobox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useTableQuery } from '@/hooks/use-table-query'

const client = createClient()
const date = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })
const statusOptions = ['published', 'held', 'unpublished', 'quarantined', 'removed'].map(option)
const pipelineOptions = ['queued', 'running', 'succeeded', 'failed', 'not_started'].map(option)
const timeOptions = [
  { value: '1', label: 'Last 24 hours' },
  { value: '7', label: 'Last 7 days' },
  { value: '30', label: 'Last 30 days' },
  { value: '90', label: 'Last 90 days' },
]

export function ArticlesPage() {
  const queryClient = useQueryClient()
  const table = useTableQuery('articles')
  const status = table.filter('status')
  const pipeline = table.filter('pipeline')
  const category = table.filter('category')
  const source = table.filter('source')
  const days = table.filter('days')
  const filterOptions = useQuery({
    queryKey: ['article-filter-options'],
    queryFn: async () => {
      const [categories, sources] = await Promise.all([client.categories(), client.sources()])
      return {
        categories: categories.items.map((item) => option(item.slug)),
        sources: sources.items.map((item) => ({ value: item.id, label: item.name })),
      }
    },
    staleTime: 5 * 60_000,
  })
  const articles = useQuery({
    queryKey: ['articles', table.page, table.perPage, table.search, status, pipeline, category, source, days],
    queryFn: () => client.adminArticles({
      page: table.page,
      per_page: table.perPage,
      search: table.search,
      status,
      pipeline,
      category,
      source,
      days,
    }),
    placeholderData: keepPreviousData,
    refetchInterval: 15_000,
  })
  const [editingArticle, setEditingArticle] = useState<ArticleActionItem | null>(null)
  const [deletingArticle, setDeletingArticle] = useState<ArticleActionItem | null>(null)
  const review = useMutation({
    mutationFn: ({ id, status: nextStatus, category: nextCategory, reason }: ArticleReviewChange) =>
      client.reviewArticle(id, { status: nextStatus, category: nextCategory, reason }),
    onSuccess: (_data, change) => {
      toast.success('Article updated')
      setEditingArticle(null)
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: ['queue'] }),
        queryClient.invalidateQueries({ queryKey: ['articles'] }),
        queryClient.invalidateQueries({ queryKey: ['article', change.id] }),
      ])
    },
    onError: (error) => toast.error(error.message || 'Could not update article'),
  })
  const remove = useMutation({
    mutationFn: (article: ArticleActionItem) => client.deleteArticle(article.id, 'Deleted from the article registry'),
    onSuccess: (_data, article) => {
      toast.success('Article deleted')
      setDeletingArticle(null)
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: ['queue'] }),
        queryClient.invalidateQueries({ queryKey: ['articles'] }),
        queryClient.invalidateQueries({ queryKey: ['article', article.id] }),
      ])
    },
    onError: (error) => toast.error(error.message || 'Could not delete article'),
  })

  return (
    <section className="flex flex-col gap-6">
      <div>
        <Badge variant="outline" className="mb-2"><Newspaper /> Captured corpus</Badge>
        <h1 className="font-heading text-2xl font-semibold tracking-tight">Articles</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Inspect every captured report, its intelligence results, and complete processing telemetry.
        </p>
      </div>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Article registry</CardTitle>
          <CardDescription>Server-filtered articles from all newsroom sources.</CardDescription>
          <CardAction><Badge variant="secondary">{articles.data?.pagination.total ?? 0} articles</Badge></CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar search={table.search} searchPlaceholder="Search headlines or sources…" onSearch={table.setSearch}>
            <SearchableArticleFilter label="All categories" searchLabel="Search categories…" emptyLabel="No categories found." value={category} options={filterOptions.data?.categories ?? []} loading={filterOptions.isPending} onChange={(value) => table.setFilter('category', value)} />
            <SearchableArticleFilter label="All sources" searchLabel="Search sources…" emptyLabel="No sources found." value={source} options={filterOptions.data?.sources ?? []} loading={filterOptions.isPending} onChange={(value) => table.setFilter('source', value)} />
            <ArticleFilter label="All time" value={days} options={timeOptions} onChange={(value) => table.setFilter('days', value)} />
            <ArticleFilter label="All statuses" value={status} options={statusOptions} onChange={(value) => table.setFilter('status', value)} />
            <ArticleFilter label="All pipelines" value={pipeline} options={pipelineOptions} onChange={(value) => table.setFilter('pipeline', value)} />
          </DataTableToolbar>
          <Table className="min-w-[920px]">
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead>Article</TableHead>
                <TableHead>Category</TableHead>
                <TableHead>Publication</TableHead>
                <TableHead>Pipeline</TableHead>
                <TableHead>Captured</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {articles.isPending ? Array.from({ length: 6 }, (_, index) => (
                <TableRow key={index}>{Array.from({ length: 6 }, (_, cell) => <TableCell key={cell}><Skeleton className="h-5 w-full max-w-40" /></TableCell>)}</TableRow>
              )) : null}
              {articles.isError ? <TableMessage>Articles are temporarily unavailable.</TableMessage> : null}
              {!articles.isPending && !articles.isError && !articles.data.items.length ? <TableMessage>No articles match these filters.</TableMessage> : null}
              {articles.data?.items.map((article) => (
                <TableRow key={article.id}>
                  <TableCell className="max-w-xl whitespace-normal">
                    <div className="flex items-start gap-3">
                      <SourceAvatar name={article.source} iconUrl={article.source_icon} className="mt-0.5 size-9 shrink-0" />
                      <div className="min-w-0">
                        <Link to={`/articles/${article.id}`} className="line-clamp-2 font-medium leading-snug hover:underline">{article.headline}</Link>
                        <p className="mt-1 text-xs text-muted-foreground">{article.source}</p>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="capitalize">{article.category?.replaceAll('-', ' ') ?? 'Unassigned'}</TableCell>
                  <TableCell><StatusBadge status={article.public_status} /></TableCell>
                  <TableCell>
                    <div className="flex flex-col items-start gap-1">
                      <StatusBadge status={article.pipeline_status ?? 'not_started'} />
                      {article.current_step ? <span className="text-xs text-muted-foreground">{article.current_step.replaceAll('_', ' ')}</span> : null}
                    </div>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{date.format(new Date(article.received_at))}</TableCell>
                  <TableCell className="text-right">
                    <ArticleActionsMenu
                      article={article}
                      busy={review.isPending || remove.isPending}
                      onEdit={setEditingArticle}
                      onDelete={setDeletingArticle}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {articles.data ? <DataTablePagination pagination={articles.data.pagination} pageHref={table.pageHref} onPerPageChange={table.setPerPage} /> : null}
        </CardContent>
        {editingArticle ? (
          <ArticleReviewDialog
            key={editingArticle.id}
            article={editingArticle}
            categories={filterOptions.data?.categories ?? []}
            categoriesLoading={filterOptions.isPending}
            open
            saving={review.isPending}
            defaultReason="Updated from the article registry"
            onOpenChange={(open) => {
              if (!open && !review.isPending) setEditingArticle(null)
            }}
            onSave={(change) => review.mutate(change)}
          />
        ) : null}
        {deletingArticle ? (
          <ArticleDeleteDialog
            key={deletingArticle.id}
            article={deletingArticle}
            open
            deleting={remove.isPending}
            onOpenChange={(open) => {
              if (!open && !remove.isPending) setDeletingArticle(null)
            }}
            onConfirm={(article) => remove.mutate(article)}
          />
        ) : null}
      </Card>
    </section>
  )
}

type FilterOption = { value: string; label: string }

function option(value: string): FilterOption {
  const label = value.replaceAll('_', ' ').replaceAll('-', ' ')
  return { value, label: label.replace(/\b\w/g, (character) => character.toUpperCase()) }
}

function ArticleFilter({ label, value, options, loading = false, onChange }: { label: string; value: string; options: FilterOption[]; loading?: boolean; onChange: (value: string) => void }) {
  const selectedLabel = options.find((item) => item.value === value)?.label
  return (
    <Select value={value || 'all'} disabled={loading} onValueChange={(next) => next && onChange(next === 'all' ? '' : next)}>
      <SelectTrigger size="sm" className="min-w-36 max-w-52" aria-label={label}><SelectValue>{() => selectedLabel ?? (value ? value.replaceAll('_', ' ') : label)}</SelectValue></SelectTrigger>
      <SelectContent align="end">
        <SelectItem value="all">{label}</SelectItem>
        {options.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}

function SearchableArticleFilter({ label, searchLabel, emptyLabel, value, options, loading = false, onChange }: { label: string; searchLabel: string; emptyLabel: string; value: string; options: FilterOption[]; loading?: boolean; onChange: (value: string) => void }) {
  const allOption = { value: '', label }
  const items = [allOption, ...options]
  const selected = items.find((item) => item.value === value) ?? allOption

  return (
    <Combobox
      items={items}
      value={selected}
      disabled={loading}
      autoHighlight
      isItemEqualToValue={(item, selectedItem) => item.value === selectedItem.value}
      onValueChange={(next) => onChange(next?.value ?? '')}
    >
      <ComboboxTrigger className="min-w-36 max-w-52" aria-label={label}>
        {selected.label}
      </ComboboxTrigger>
      <ComboboxContent align="end">
        <ComboboxInput placeholder={searchLabel} aria-label={searchLabel} />
        <ComboboxEmpty>{emptyLabel}</ComboboxEmpty>
        <ComboboxList>
          {(item) => <ComboboxItem key={item.value || 'all'} value={item}>{item.label}</ComboboxItem>}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}

function StatusBadge({ status }: { status: string }) {
  const variant = status === 'failed' || status === 'quarantined' ? 'destructive' : status === 'succeeded' || status === 'published' ? 'default' : 'outline'
  return <Badge variant={variant} className="capitalize">{status.replaceAll('_', ' ')}</Badge>
}

function TableMessage({ children }: { children: string }) {
  return <TableRow><TableCell colSpan={6} className="h-48 text-center text-muted-foreground">{children}</TableCell></TableRow>
}
