import { createClient } from '@snap/api-client'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ArrowLeft, ExternalLink, Eye, Newspaper, RadioTower } from 'lucide-react'
import { Link, useParams } from 'react-router'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from '@/components/ui/combobox'
import { ScrollArea, ScrollBar } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useTableQuery } from '@/hooks/use-table-query'
import { formatDateTime } from '@/lib/date-time'

const client = createClient()
const timeOptions = [
  { value: '1', label: 'Last 24 hours' },
  { value: '7', label: 'Last 7 days' },
  { value: '30', label: 'Last 30 days' },
  { value: '90', label: 'Last 90 days' },
]

type FilterOption = { value: string; label: string }

function option(value: string): FilterOption {
  const readable = value.replaceAll('_', ' ').replaceAll('-', ' ')
  return { value, label: readable.replace(/\b\w/g, (character) => character.toUpperCase()) }
}

function safeExternalURL(value?: string | null) {
  if (!value) return undefined
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.toString() : undefined
  } catch {
    return undefined
  }
}

function compactSnippet(value: string) {
  return value
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}(?:#{1,6}|>|[-*+] |\d+[.)] )\s*/gm, '')
    .replace(/<[^>]+>/g, ' ')
    .replace(/[`*_~]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

export function SourceFeedPage() {
  const { id = '' } = useParams()
  const table = useTableQuery('feed')
  const category = table.filter('category')
  const days = table.filter('days')
  const source = useQuery({
    queryKey: ['source', id],
    queryFn: () => client.adminSource(id),
    enabled: Boolean(id),
  })
  const categories = useQuery({
    queryKey: ['source-feed-categories'],
    queryFn: async () => {
      const result = await client.categories()
      return result.items.map((item) => option(item.slug))
    },
    staleTime: 5 * 60_000,
  })
  const articles = useQuery({
    queryKey: ['source-feed', id, table.page, table.perPage, table.search, category, days],
    queryFn: () => client.adminArticles({
      page: table.page,
      per_page: table.perPage,
      search: table.search,
      source: id,
      category,
      days,
    }),
    enabled: Boolean(id),
    placeholderData: keepPreviousData,
    refetchInterval: 15_000,
  })

  if (source.isError) {
    return (
      <section className="flex min-h-[50vh] flex-col items-center justify-center gap-4 text-center">
        <RadioTower className="size-8 text-muted-foreground" />
        <div>
          <h1 className="text-xl font-semibold">Source not found</h1>
          <p className="mt-1 text-sm text-muted-foreground">This source may have been removed.</p>
        </div>
        <Button variant="outline" nativeButton={false} render={<Link to="/sources" />}>
          <ArrowLeft />
          Back to sources
        </Button>
      </section>
    )
  }

  const sourceData = source.data
  const website = safeExternalURL(sourceData?.website)

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <Button className="-ml-2 w-fit" variant="ghost" size="sm" nativeButton={false} render={<Link to="/sources" />}>
        <ArrowLeft />
        Back to sources
      </Button>

      <div className="flex flex-col gap-5 border-b pb-6 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex min-w-0 items-start gap-4">
          {source.isPending ? (
            <Skeleton className="size-14 rounded-2xl" />
          ) : (
            <SourceAvatar
              className="size-14 rounded-2xl"
              name={sourceData?.name ?? 'Source'}
              website={sourceData?.website}
              iconUrl={sourceData?.icon_url}
            />
          )}
          <div className="min-w-0">
            <Badge variant="outline" className="mb-2"><Newspaper /> Source feed</Badge>
            {source.isPending ? <Skeleton className="h-8 w-52" /> : (
              <h1 className="truncate font-heading text-2xl font-semibold tracking-tight">{sourceData?.name}</h1>
            )}
            <p className="mt-1 text-sm text-muted-foreground">
              Browse every captured report from this publisher.
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" nativeButton={false} render={<Link to={`/sources/${id}`} />}>
            <RadioTower />
            View source
          </Button>
          {website ? (
            <Button variant="outline" nativeButton={false} render={<a href={website} target="_blank" rel="noreferrer" />}>
              <ExternalLink />
              Publisher website
            </Button>
          ) : null}
        </div>
      </div>

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <CardTitle>Captured articles</CardTitle>
          <CardDescription>Search, filter, and inspect articles captured from {sourceData?.name ?? 'this source'}.</CardDescription>
          <CardAction>
            <Badge variant="secondary">{articles.data?.pagination.total ?? 0} articles</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <DataTableToolbar
            search={table.search}
            searchPlaceholder="Search article headlines…"
            onSearch={table.setSearch}
          >
            <CategoryFilter
              value={category}
              options={categories.data ?? []}
              loading={categories.isPending}
              onChange={(value) => table.setFilter('category', value)}
            />
            <TimeFilter value={days} onChange={(value) => table.setFilter('days', value)} />
          </DataTableToolbar>

          <ScrollArea className="w-full" aria-label="Scroll through source articles">
            <Table className="min-w-[940px]" containerClassName="overflow-visible">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead>Article</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>Publication</TableHead>
                  <TableHead>Published</TableHead>
                  <TableHead>Captured</TableHead>
                  <TableHead className="text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {articles.isPending ? Array.from({ length: 6 }, (_, index) => (
                  <TableRow key={index}>
                    {Array.from({ length: 6 }, (_, cell) => <TableCell key={cell}><Skeleton className="h-5 w-full max-w-40" /></TableCell>)}
                  </TableRow>
                )) : null}
                {articles.isError ? <TableMessage>Source articles are temporarily unavailable.</TableMessage> : null}
                {!articles.isPending && !articles.isError && !articles.data?.items.length ? (
                  <TableMessage>No articles match these filters.</TableMessage>
                ) : null}
                {articles.data?.items.map((article) => {
                  const originalURL = safeExternalURL(article.original_url)
                  const snippet = compactSnippet(article.snippet)
                  return (
                    <TableRow key={article.id}>
                      <TableCell className="max-w-2xl whitespace-normal">
                        <Link to={`/articles/${article.id}`} className="group block py-0.5">
                          <span className="line-clamp-2 font-medium leading-snug group-hover:underline">{article.headline}</span>
                          <span className="mt-1.5 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
                            {snippet || 'No summary or description is available for this article.'}
                          </span>
                        </Link>
                      </TableCell>
                      <TableCell className="capitalize">{article.category?.replaceAll('-', ' ') ?? 'Unassigned'}</TableCell>
                      <TableCell><StatusBadge status={article.public_status} /></TableCell>
                      <TableCell className="text-muted-foreground">{formatDateTime(article.published_at)}</TableCell>
                      <TableCell className="text-muted-foreground">{formatDateTime(article.received_at)}</TableCell>
                      <TableCell className="text-right">
                        <div className="inline-flex items-center gap-1">
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={`View ${article.headline}`}
                            nativeButton={false}
                            render={<Link to={`/articles/${article.id}`} />}
                          >
                            <Eye />
                          </Button>
                          {originalURL ? (
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              aria-label={`Open original article: ${article.headline}`}
                              nativeButton={false}
                              render={<a href={originalURL} target="_blank" rel="noreferrer" />}
                            >
                              <ExternalLink />
                            </Button>
                          ) : null}
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
            <ScrollBar orientation="horizontal" />
          </ScrollArea>
          {articles.data ? (
            <DataTablePagination
              pagination={articles.data.pagination}
              pageHref={table.pageHref}
              onPerPageChange={table.setPerPage}
            />
          ) : null}
        </CardContent>
      </Card>
    </section>
  )
}

function CategoryFilter({
  value,
  options,
  loading,
  onChange,
}: {
  value: string
  options: FilterOption[]
  loading: boolean
  onChange: (value: string) => void
}) {
  const allOption = { value: '', label: 'All categories' }
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
      <ComboboxTrigger className="min-w-40 max-w-56" aria-label="Filter by category">
        {selected.label}
      </ComboboxTrigger>
      <ComboboxContent align="end">
        <ComboboxInput placeholder="Search categories…" aria-label="Search categories" />
        <ComboboxEmpty>No categories found.</ComboboxEmpty>
        <ComboboxList>
          {(item) => <ComboboxItem key={item.value || 'all'} value={item}>{item.label}</ComboboxItem>}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  )
}

function TimeFilter({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const selectedLabel = timeOptions.find((item) => item.value === value)?.label
  return (
    <Select value={value || 'all'} onValueChange={(next) => next && onChange(next === 'all' ? '' : next)}>
      <SelectTrigger size="sm" className="min-w-40" aria-label="Filter by timeframe">
        <SelectValue>{() => selectedLabel ?? 'All time'}</SelectValue>
      </SelectTrigger>
      <SelectContent align="end">
        <SelectItem value="all">All time</SelectItem>
        {timeOptions.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}

function StatusBadge({ status }: { status: string }) {
  const variant = status === 'quarantined'
    ? 'destructive'
    : status === 'published'
      ? 'default'
      : 'outline'
  return <Badge variant={variant} className="capitalize">{status.replaceAll('_', ' ')}</Badge>
}

function TableMessage({ children }: { children: string }) {
  return <TableRow><TableCell colSpan={6} className="h-48 text-center text-muted-foreground">{children}</TableCell></TableRow>
}
