import { DataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'

export function QueuePage() {
  return (
    <section className="flex flex-col gap-6">
      <div>
        <Badge variant="outline" className="mb-2">
          Editorial operations
        </Badge>
        <h1 className="font-heading text-2xl font-semibold tracking-tight">Review desk</h1>
        <p className="mt-1 text-sm text-muted-foreground">Review, categorize, and publish items that need human judgment.</p>
      </div>
      <DataTable prefix="" editable />
    </section>
  )
}
