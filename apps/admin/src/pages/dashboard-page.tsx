import { createClient } from '@snap/api-client'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, RefreshCw } from 'lucide-react'
import { Link } from 'react-router'

import { ChartAreaInteractive } from '@/components/chart-area-interactive'
import { DataTable } from '@/components/data-table'
import { SectionCards } from '@/components/section-cards'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const client = createClient()

export function DashboardPage() {
  const queryClient = useQueryClient()
  const overview = useQuery({ queryKey: ['overview'], queryFn: () => client.overview() })
  const isRefreshing = overview.isFetching

  return (
    <section className="flex flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div className="space-y-2">
          <Badge variant="outline" className="gap-1.5">
            <span className="size-1.5 rounded-full bg-emerald-500" aria-hidden="true" />
            Live newsroom
          </Badge>
          <div>
            <h1 className="font-heading text-2xl font-semibold tracking-tight">Newsroom performance</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Publishing, intake health, and editorial work in one place.
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" nativeButton={false} render={<Link to="/queue" />}>
            Review queue
            <ArrowRight data-icon="inline-end" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            aria-label="Refresh dashboard data"
            disabled={isRefreshing}
            onClick={() => {
              void Promise.all([
                queryClient.invalidateQueries({ queryKey: ['overview'] }),
                queryClient.invalidateQueries({ queryKey: ['queue'] }),
              ])
            }}
          >
            <RefreshCw className={isRefreshing ? 'animate-spin' : undefined} />
          </Button>
        </div>
      </div>

      <SectionCards data={overview.data} isLoading={overview.isPending} />
      <ChartAreaInteractive />
      <DataTable />
    </section>
  )
}
