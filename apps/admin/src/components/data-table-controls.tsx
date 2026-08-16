import type { PaginationMeta } from '@snap/api-client'
import { Search, X } from 'lucide-react'
import { Fragment, useId } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { getPaginationPages } from '@/lib/pagination'
import { cn } from '@/lib/utils'

type DataTableToolbarProps = {
  search: string
  searchPlaceholder: string
  onSearch: (value: string) => void
  children?: React.ReactNode
}

export function DataTableToolbar({
  search,
  searchPlaceholder,
  onSearch,
  children,
}: DataTableToolbarProps) {
  const inputId = useId()

  return (
    <div className="flex flex-col gap-3 border-b px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
      <form
        key={search}
        className="flex w-full items-center gap-2 lg:max-w-md"
        onSubmit={(event) => {
          event.preventDefault()
          onSearch(String(new FormData(event.currentTarget).get('search') ?? ''))
        }}
      >
        <label htmlFor={inputId} className="sr-only">
          {searchPlaceholder}
        </label>
        <div className="relative min-w-0 flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id={inputId}
            name="search"
            defaultValue={search}
            placeholder={searchPlaceholder}
            className="pl-9"
          />
        </div>
        <Button type="submit" variant="outline">
          Search
        </Button>
        {search ? (
          <Button type="button" variant="ghost" size="icon" aria-label="Clear search" onClick={() => onSearch('')}>
            <X />
          </Button>
        ) : null}
      </form>
      {children ? <div className="flex flex-wrap items-center gap-2">{children}</div> : null}
    </div>
  )
}

type DataTablePaginationProps = {
  pagination: PaginationMeta
  pageHref: (page: number) => string
  onPerPageChange: (perPage: number) => void
}

export function DataTablePagination({
  pagination,
  pageHref,
  onPerPageChange,
}: DataTablePaginationProps) {
  const { page, per_page: perPage, total, total_pages: totalPages } = pagination
  const start = total === 0 ? 0 : (page - 1) * perPage + 1
  const end = Math.min(page * perPage, total)
  const pageCount = Math.max(1, totalPages)
  const pages = getPaginationPages(page, pageCount)
  const previousDisabled = page <= 1
  const nextDisabled = page >= pageCount

  return (
    <div className="grid gap-4 border-t px-5 py-4 text-sm text-muted-foreground lg:grid-cols-[1fr_auto_1fr] lg:items-center">
      <div className="flex flex-wrap items-center justify-center gap-4 lg:justify-start">
        <div className="flex items-center gap-2">
          <span className="whitespace-nowrap">Rows per page</span>
          <Select
            value={String(perPage)}
            onValueChange={(value) => {
              if (value !== null) onPerPageChange(Number(value))
            }}
          >
            <SelectTrigger size="sm" className="w-20" aria-label="Rows per page">
              <SelectValue />
            </SelectTrigger>
            <SelectContent align="end">
              {[10, 25, 50].map((size) => (
                <SelectItem key={size} value={String(size)}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <span className="whitespace-nowrap tabular-nums">
          {start}–{end} of {total} results
        </span>
      </div>
      <Pagination className="w-auto">
        <PaginationContent className="flex-wrap justify-center">
          <PaginationItem>
            <PaginationPrevious
              to={pageHref(Math.max(1, page - 1))}
              aria-disabled={previousDisabled}
              tabIndex={previousDisabled ? -1 : undefined}
              className={cn(previousDisabled && 'pointer-events-none opacity-50')}
            />
          </PaginationItem>
          {pages.map((number, index) => (
            <Fragment key={number}>
              {index > 0 && pages[index - 1] !== number - 1 ? (
                <PaginationItem>
                  <PaginationEllipsis />
                </PaginationItem>
              ) : null}
              <PaginationItem>
                <PaginationLink to={pageHref(number)} isActive={number === page} aria-label={`Page ${number}`}>
                  {number}
                </PaginationLink>
              </PaginationItem>
            </Fragment>
          ))}
          <PaginationItem>
            <PaginationNext
              to={pageHref(Math.min(pageCount, page + 1))}
              aria-disabled={nextDisabled}
              tabIndex={nextDisabled ? -1 : undefined}
              className={cn(nextDisabled && 'pointer-events-none opacity-50')}
            />
          </PaginationItem>
        </PaginationContent>
      </Pagination>
      <p className="text-center whitespace-nowrap tabular-nums lg:text-right">
        Page {page} of {pageCount}
      </p>
    </div>
  )
}
