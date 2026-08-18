import type { Overview } from '@snap/api-client'
import { CircleAlert, ListChecks, Newspaper, RadioTower } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardAction,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

type SectionCardsProps = {
  data?: Overview
  isLoading: boolean
}

export function SectionCards({ data, isLoading }: SectionCardsProps) {
  const queueTotal = data ? data.held + data.quarantined : undefined
  const alertsTotal = data ? data.complaints + data.sick_feeds + data.stale_feeds : undefined
  const cards = [
    {
      title: 'Published articles',
      value: data?.published,
      icon: Newspaper,
      badge: 'Public corpus',
      detail: 'Available to readers right now',
      subdetail: 'Live publishing total',
    },
    {
      title: 'Editorial queue',
      value: queueTotal,
      icon: ListChecks,
      badge: 'Needs review',
      detail: `${data?.held ?? 0} held · ${data?.quarantined ?? 0} quarantined`,
      subdetail: 'Prioritized for desk review',
    },
    {
      title: 'Source network',
      value: data?.sources,
      icon: RadioTower,
      badge: 'Monitoring',
      detail: `${data?.sick_feeds ?? 0} failed · ${data?.stale_feeds ?? 0} stale feeds`,
      subdetail: 'Registered active sources',
    },
    {
      title: 'Operational alerts',
      value: alertsTotal,
      icon: CircleAlert,
      badge: alertsTotal ? 'Attention' : 'All clear',
      detail: `${data?.complaints ?? 0} open complaints`,
      subdetail: 'Complaints and feed health signals',
      alert: Boolean(alertsTotal),
    },
  ]

  return (
    <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-4">
      {cards.map((card) => {
        const Icon = card.icon
        return (
          <Card
            key={card.title}
            className="@container/card bg-linear-to-b from-card to-primary/[0.035] shadow-sm transition-shadow hover:shadow-md dark:to-primary/[0.06]"
          >
            <CardHeader>
              <CardDescription>{card.title}</CardDescription>
              <CardTitle className="text-3xl font-semibold tabular-nums">
                {isLoading ? <Skeleton className="h-9 w-20" /> : (card.value ?? '—').toLocaleString()}
              </CardTitle>
              <CardAction>
                <Badge variant={card.alert ? 'destructive' : 'outline'}>
                  <Icon />
                  {card.badge}
                </Badge>
              </CardAction>
            </CardHeader>
            <CardFooter className="flex-col items-start gap-1">
              <p className="font-medium">{card.detail}</p>
              <p className="text-muted-foreground">{card.subdetail}</p>
            </CardFooter>
          </Card>
        )
      })}
    </div>
  )
}
