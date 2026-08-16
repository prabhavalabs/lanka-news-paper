import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

const client = createClient()

export function DashboardPage() {
  const overview = useQuery({ queryKey: ['overview'], queryFn: () => client.overview() })
  const item = overview.data
  return (
    <section className="flex flex-col gap-6">
      <h1 className="text-xl font-medium">Desk</h1>
      <p className="text-sm text-muted-foreground">Live counts from the public corpus and ingest health.</p>
      <dl className="grid grid-cols-2 gap-4 text-sm md:grid-cols-4">
        <div className="border border-border p-4">
          <dt className="text-muted-foreground">Published</dt>
          <dd className="mt-1 text-2xl font-medium">{item?.published ?? '—'}</dd>
        </div>
        <div className="border border-border p-4">
          <dt className="text-muted-foreground">Held / queue</dt>
          <dd className="mt-1 text-2xl font-medium">{item?.held ?? '—'}</dd>
        </div>
        <div className="border border-border p-4">
          <dt className="text-muted-foreground">Complaints</dt>
          <dd className="mt-1 text-2xl font-medium">{item?.complaints ?? '—'}</dd>
        </div>
        <div className="border border-border p-4">
          <dt className="text-muted-foreground">Sick / stale feeds</dt>
          <dd className="mt-1 text-2xl font-medium">
            {item ? item.sick_feeds + item.stale_feeds : '—'}
          </dd>
        </div>
      </dl>
      <p className="text-sm">
        <Link to="/queue">Open editorial queue</Link>
        {' · '}
        <Link to="/sources">Manage sources</Link>
      </p>
    </section>
  )
}
