import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { CircleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts'

import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useIsMobile } from '@/hooks/use-mobile'

const client = createClient()

const chartConfig = {
  published: {
    label: 'Published',
    color: 'var(--primary)',
  },
  received: {
    label: 'Received',
    color: 'var(--chart-2)',
  },
} satisfies ChartConfig

type TimeRange = '7' | '30' | '90'

export function ChartAreaInteractive() {
  const isMobile = useIsMobile()
  const [timeRange, setTimeRange] = useState<TimeRange>('90')
  const days = Number(timeRange) as 7 | 30 | 90
  const trends = useQuery({
    queryKey: ['overview', 'trends', days],
    queryFn: () => client.overviewTrends(days),
  })

  useEffect(() => {
    if (isMobile) setTimeRange('7')
  }, [isMobile])

  const rangeLabel = days === 90 ? 'Last 3 months' : `Last ${days} days`

  return (
    <Card className="@container/card shadow-sm">
      <CardHeader>
        <CardTitle>Publishing activity</CardTitle>
        <CardDescription>Articles received and published · {rangeLabel.toLowerCase()}</CardDescription>
        <CardAction>
          <ToggleGroup
            multiple={false}
            value={[timeRange]}
            onValueChange={(value) => setTimeRange((value[0] as TimeRange | undefined) ?? '90')}
            variant="outline"
            className="hidden *:data-[slot=toggle-group-item]:px-4! @[767px]/card:flex"
            aria-label="Publishing activity date range"
          >
            <ToggleGroupItem value="90">Last 3 months</ToggleGroupItem>
            <ToggleGroupItem value="30">Last 30 days</ToggleGroupItem>
            <ToggleGroupItem value="7">Last 7 days</ToggleGroupItem>
          </ToggleGroup>
          <Select
            value={timeRange}
            onValueChange={(value) => {
              if (value === '7' || value === '30' || value === '90') setTimeRange(value)
            }}
          >
            <SelectTrigger className="w-36 @[767px]/card:hidden" size="sm" aria-label="Date range">
              <SelectValue placeholder={rangeLabel} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="90">Last 3 months</SelectItem>
              <SelectItem value="30">Last 30 days</SelectItem>
              <SelectItem value="7">Last 7 days</SelectItem>
            </SelectContent>
          </Select>
        </CardAction>
      </CardHeader>
      <CardContent className="px-2 pt-2 sm:px-6">
        {trends.isPending ? (
          <Skeleton className="h-[280px] w-full" />
        ) : trends.isError ? (
          <div className="flex h-[280px] flex-col items-center justify-center gap-2 text-center text-muted-foreground">
            <CircleAlert className="size-5" />
            <p className="text-sm">Publishing activity is temporarily unavailable.</p>
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            className="h-[280px] w-full"
            role="img"
            aria-label={`Publishing activity for the ${rangeLabel.toLowerCase()}`}
          >
            <AreaChart data={trends.data.items} accessibilityLayer>
              <defs>
                <linearGradient id="fillPublished" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-published)" stopOpacity={0.55} />
                  <stop offset="95%" stopColor="var(--color-published)" stopOpacity={0.02} />
                </linearGradient>
                <linearGradient id="fillReceived" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-received)" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="var(--color-received)" stopOpacity={0.02} />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="date"
                tickLine={false}
                axisLine={false}
                tickMargin={10}
                minTickGap={32}
                tickFormatter={(value: string) =>
                  new Date(`${value}T00:00:00`).toLocaleDateString('en', {
                    month: 'short',
                    day: 'numeric',
                  })
                }
              />
              <YAxis hide domain={[0, 'auto']} />
              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    indicator="dot"
                    labelFormatter={(value) =>
                      new Date(`${String(value)}T00:00:00`).toLocaleDateString('en', {
                        month: 'long',
                        day: 'numeric',
                        year: 'numeric',
                      })
                    }
                  />
                }
              />
              <Area
                dataKey="received"
                type="monotone"
                fill="url(#fillReceived)"
                stroke="var(--color-received)"
                strokeWidth={2}
              />
              <Area
                dataKey="published"
                type="monotone"
                fill="url(#fillPublished)"
                stroke="var(--color-published)"
                strokeWidth={2}
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
