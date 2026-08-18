import type { SourcePerformance } from '@snap/api-client'
import { Area, AreaChart, Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'

const activityConfig = {
  captured: {
    label: 'Captured',
    color: 'var(--primary)',
  },
  published: {
    label: 'Published',
    color: 'var(--chart-2)',
  },
} satisfies ChartConfig

const dailyConfig = {
  captured: {
    label: 'Captured',
    color: 'var(--primary)',
  },
} satisfies ChartConfig

function shortDate(value: string) {
  return new Date(`${value}T00:00:00`).toLocaleDateString('en', {
    month: 'short',
    day: 'numeric',
  })
}

export function SourcePerformanceCharts({ daily }: Pick<SourcePerformance, 'daily'>) {
  const lastWeek = daily.slice(-7)

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,0.8fr)]">
      <Card className="shadow-sm">
        <CardHeader>
          <CardTitle>Ingestion activity</CardTitle>
          <CardDescription>News items captured and published over the last 30 days.</CardDescription>
          <div className="flex items-center gap-4 pt-1 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <span className="size-2 rounded-full bg-primary" />
              Captured
            </span>
            <span className="inline-flex items-center gap-1.5">
              <span className="size-2 rounded-full bg-[var(--chart-2)]" />
              Published
            </span>
          </div>
        </CardHeader>
        <CardContent className="px-2 sm:px-6">
          <ChartContainer
            config={activityConfig}
            className="h-[270px] w-full"
            role="img"
            aria-label="Captured and published news items over the last 30 days"
          >
            <AreaChart data={daily} accessibilityLayer>
              <defs>
                <linearGradient id="sourceCaptured" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-captured)" stopOpacity={0.45} />
                  <stop offset="95%" stopColor="var(--color-captured)" stopOpacity={0.02} />
                </linearGradient>
                <linearGradient id="sourcePublished" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-published)" stopOpacity={0.28} />
                  <stop offset="95%" stopColor="var(--color-published)" stopOpacity={0.01} />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="date"
                tickLine={false}
                axisLine={false}
                tickMargin={10}
                minTickGap={34}
                tickFormatter={shortDate}
              />
              <YAxis hide domain={[0, 'auto']} />
              <ChartTooltip
                cursor={false}
                content={<ChartTooltipContent indicator="dot" labelFormatter={(value) => shortDate(String(value))} />}
              />
              <Area
                dataKey="captured"
                type="monotone"
                fill="url(#sourceCaptured)"
                stroke="var(--color-captured)"
                strokeWidth={2}
              />
              <Area
                dataKey="published"
                type="monotone"
                fill="url(#sourcePublished)"
                stroke="var(--color-published)"
                strokeWidth={2}
              />
            </AreaChart>
          </ChartContainer>
        </CardContent>
      </Card>

      <Card className="shadow-sm">
        <CardHeader>
          <CardTitle>Daily captured</CardTitle>
          <CardDescription>New items received each day · last 7 days.</CardDescription>
        </CardHeader>
        <CardContent className="px-2 sm:px-6">
          <ChartContainer
            config={dailyConfig}
            className="h-[270px] w-full"
            role="img"
            aria-label="News items captured each day over the last 7 days"
          >
            <BarChart data={lastWeek} accessibilityLayer>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="date"
                tickLine={false}
                axisLine={false}
                tickMargin={10}
                tickFormatter={(value: string) =>
                  new Date(`${value}T00:00:00`).toLocaleDateString('en', { weekday: 'short' })
                }
              />
              <YAxis hide domain={[0, 'auto']} />
              <ChartTooltip
                cursor={false}
                content={<ChartTooltipContent hideIndicator labelFormatter={(value) => shortDate(String(value))} />}
              />
              <Bar dataKey="captured" fill="var(--color-captured)" radius={[7, 7, 2, 2]} />
            </BarChart>
          </ChartContainer>
        </CardContent>
      </Card>
    </div>
  )
}
