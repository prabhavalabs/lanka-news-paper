import { createClient } from '@snap/api-client'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { FileSearch, Newspaper } from 'lucide-react'
import { Link } from 'react-router'

import { SourceAvatar } from '@/components/source-avatar'
import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useTableQuery } from '@/hooks/use-table-query'

const client = createClient()
const date = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })

export function ArticlesPage() {
  const table = useTableQuery('articles')
  const status = table.filter('status')
  const pipeline = table.filter('pipeline')
  const articles = useQuery({
    queryKey: ['articles', table.page, table.perPage, table.search, status, pipeline],
    queryFn: () => client.adminArticles({
      page: table.page,
      per_page: table.perPage,
      search: table.search,
      status,
      pipeline,
    }),
    placeholderData: keepPreviousData,
    refetchInterval: 15_000,
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
            <ArticleFilter label="All statuses" value={status} options={['published', 'held', 'unpublished', 'quarantined', 'removed']} onChange={(value) => table.setFilter('status', value)} />
            <ArticleFilter label="All pipelines" value={pipeline} options={['queued', 'running', 'succeeded', 'failed', 'not_started']} onChange={(value) => table.setFilter('pipeline', value)} />
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
                    <Button variant="ghost" size="sm" nativeButton={false} render={<Link to={`/articles/${article.id}`} />}>
                      <FileSearch /> Inspect
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {articles.data ? <DataTablePagination pagination={articles.data.pagination} pageHref={table.pageHref} onPerPageChange={table.setPerPage} /> : null}
        </CardContent>
      </Card>
    </section>
  )
}

function ArticleFilter({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (value: string) => void }) {
  return (
    <Select value={value || 'all'} onValueChange={(next) => next && onChange(next === 'all' ? '' : next)}>
      <SelectTrigger size="sm" className="min-w-36" aria-label={label}><SelectValue>{() => value ? value.replaceAll('_', ' ') : label}</SelectValue></SelectTrigger>
      <SelectContent align="end">
        <SelectItem value="all">{label}</SelectItem>
        {options.map((option) => <SelectItem key={option} value={option}>{option.replaceAll('_', ' ')}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}

function StatusBadge({ status }: { status: string }) {
  const variant = status === 'failed' || status === 'quarantined' ? 'destructive' : status === 'succeeded' || status === 'published' ? 'default' : 'outline'
  return <Badge variant={variant} className="capitalize">{status.replaceAll('_', ' ')}</Badge>
}

function TableMessage({ children }: { children: string }) {
  return <TableRow><TableCell colSpan={6} className="h-48 text-center text-muted-foreground">{children}</TableCell></TableRow>
}
