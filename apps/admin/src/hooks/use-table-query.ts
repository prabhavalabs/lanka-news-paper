import { useLocation, useNavigate, useSearchParams } from 'react-router'

const DEFAULT_PAGE_SIZE = 10

function positiveInteger(value: string | null, fallback: number) {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}

export function useTableQuery(prefix = '') {
  const location = useLocation()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const key = (name: string) => (prefix ? `${prefix}_${name}` : name)
  const pageKey = key('page')
  const pageSizeKey = key('per_page')
  const searchKey = key('search')
  const page = positiveInteger(params.get(pageKey), 1)
  const requestedPageSize = positiveInteger(params.get(pageSizeKey), DEFAULT_PAGE_SIZE)
  const perPage = requestedPageSize <= 100 ? requestedPageSize : DEFAULT_PAGE_SIZE
  const search = params.get(searchKey) ?? ''

  const href = (changes: Record<string, string | number | null>) => {
    const next = new URLSearchParams(params)
    for (const [name, value] of Object.entries(changes)) {
      const queryKey = key(name)
      if (value === null || value === '' || (name === 'page' && value === 1)) next.delete(queryKey)
      else next.set(queryKey, String(value))
    }
    const query = next.toString()
    return query ? `${location.pathname}?${query}` : location.pathname
  }

  const update = (changes: Record<string, string | number | null>, replace = false) => {
    void navigate(href(changes), { replace })
  }

  return {
    page,
    perPage,
    search,
    filter: (name: string) => params.get(key(name)) ?? '',
    pageHref: (nextPage: number) => href({ page: nextPage }),
    setSearch: (value: string) => update({ search: value.trim(), page: 1 }),
    setFilter: (name: string, value: string) => update({ [name]: value, page: 1 }),
    setPerPage: (value: number) => update({ per_page: value, page: 1 }),
  }
}
