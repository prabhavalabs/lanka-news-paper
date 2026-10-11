import type { ArticleNarrativeAnalysis, EventNarrativeAnalysis, PublicArticle } from '@snap/api-client'

import { cn } from '@snap/ui/utils'
import { absoluteTitle, sinhalaDateTime, sourceTypeLabel } from '@/lib/format'
import { OutletLogo } from './outlet-logo'
import { OutletName } from './outlet'
import { ReportProblem } from './report-problem'
import { LeanTag, SpectrumBar, type Lean } from './spectrum'

export type OutletReport = { key: string; headline: string; url: string; publishedAt: string }

/** One outlet's coverage of a story: its logo, how its report leans, and its original article(s). */
export type OutletCoverage = {
  key: string
  name: string
  type: string
  icon: string | null
  lean: Lean
  reports: OutletReport[]
}

export type CoverageSummary = { left: number; center: number; right: number; rated: boolean }

function leanOf(analysis: Pick<ArticleNarrativeAnalysis, 'left_probability' | 'center_probability' | 'right_probability'>): Lean {
  const { left_probability: left, center_probability: center, right_probability: right } = analysis
  if (center >= left && center >= right) return 'center'
  return left > right ? 'left' : 'right'
}

/** Groups an event's reports by outlet (earliest first) and attaches each outlet's lean and logo. */
export function eventCoverage(articles: PublicArticle[], analysis: EventNarrativeAnalysis | undefined): OutletCoverage[] {
  const spectrum = analysis?.source_spectrum ?? []
  const byOutlet = new Map<string, OutletCoverage>()
  const ordered = [...articles].sort((a, b) => a.published_at.localeCompare(b.published_at))
  for (const article of ordered) {
    const rated = spectrum.find((item) => item.article_id === article.id) ?? spectrum.find((item) => item.source_id === article.source.id)
    const entry = byOutlet.get(article.source.id) ?? {
      key: article.source.id,
      name: article.source.name,
      type: article.source.type,
      icon: rated?.source_icon || null,
      lean: rated?.label ?? 'unrated',
      reports: [],
    }
    entry.reports.push({ key: article.id, headline: article.headline, url: article.original_url, publishedAt: article.published_at })
    byOutlet.set(article.source.id, entry)
  }
  return [...byOutlet.values()]
}

/** A single report as one outlet's coverage. */
export function articleCoverage(article: PublicArticle, icon: string | null): OutletCoverage {
  return {
    key: article.source.id,
    name: article.source.name,
    type: article.source.type,
    icon,
    lean: article.analysis?.relevant ? leanOf(article.analysis) : 'unrated',
    reports: [{ key: article.id, headline: article.headline, url: article.original_url, publishedAt: article.published_at }],
  }
}

/**
 * The reading page's sidebar: how the coverage leans overall, then every outlet that reported
 * the story with its logo (in ink), its lean, its own headline, time and original link.
 */
export function CoverageSidebar({
  outlets,
  summary,
  entity,
  className,
}: {
  outlets: OutletCoverage[]
  summary: CoverageSummary | null
  entity: { type: 'event' | 'article'; id: string }
  className?: string
}) {
  const counts = outlets.reduce<Record<Lean, number>>(
    (acc, outlet) => ({ ...acc, [outlet.lean]: acc[outlet.lean] + 1 }),
    { left: 0, center: 0, right: 0, unrated: 0 },
  )
  return (
    <aside aria-labelledby="coverage-title" className={className}>
      <div className="np-rule-double" />
      <h2 id="coverage-title" className="np-kicker mt-2 flex items-baseline justify-between gap-3 text-ink">
        <span className="text-[1.125rem]">මූලාශ්‍ර</span>
        <span className="np-meta text-[0.8125rem]">මාධ්‍ය ආයතන {outlets.length}</span>
      </h2>

      <section aria-label="වාර්තාකරණ නැඹුරුව" className="mt-2 border-t border-ink pt-2">
        <p className="np-meta text-[0.8125rem]">මෙම පුවත වාර්තා කළ ආකාරය</p>
        {summary?.rated ? (
          <SpectrumBar className="mt-1.5" left={summary.left} center={summary.center} right={summary.right} counts={counts} />
        ) : (
          <p className="np-meta mt-1 text-[0.8125rem] text-ink-secondary">දේශපාලන නැඹුරුවක් මැනිය හැකි වාර්තා නැත.</p>
        )}
      </section>

      <ul className="mt-3 border-t border-ink">
        {outlets.map((outlet) => (
          <li key={outlet.key} className="border-b border-rule py-2.5 last:border-b-0">
            <div className="flex items-center gap-2.5">
              <OutletLogo src={outlet.icon} name={outlet.name} />
              <div className="min-w-0 flex-1">
                <p className="leading-[1.3]">
                  <OutletName name={outlet.name} className="text-[0.9375rem]" />
                </p>
                <p className="np-meta text-[0.75rem] leading-[1.4]">{sourceTypeLabel(outlet.type)}</p>
              </div>
              <LeanTag lean={outlet.lean} />
            </div>
            <ul className="mt-1 ps-[2.625rem]">
              {outlet.reports.map((report) => (
                <li key={report.key}>
                  <a
                    href={report.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="group block py-1 max-lg:min-h-11 max-lg:py-2"
                  >
                    <span className="font-headline text-[0.9375rem] leading-[1.5] group-hover:underline">{report.headline}</span>
                    <span className="np-meta mt-0.5 flex flex-wrap gap-x-2 text-[0.75rem] leading-[1.4]">
                      <time dateTime={report.publishedAt} title={absoluteTitle(report.publishedAt)}>
                        {sinhalaDateTime(report.publishedAt)}
                      </time>
                      <span className="text-ink underline underline-offset-2">මුල් ලිපිය ↗</span>
                    </span>
                    <span className="sr-only"> — {outlet.name} මුල් ලිපිය (නව පටිත්තක)</span>
                  </a>
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ul>

      <p className={cn('np-meta mt-2 border-t border-rule pt-2 text-[0.75rem] leading-[1.6]')}>
        නැඹුරුව ස්වයංක්‍රීය විශ්ලේෂණයකි. එය එක් එක් වාර්තාවේ රාමුව පමණක් විස්තර කරයි; මාධ්‍ය ආයතනයකට දෙන ස්ථිර ලේබලයක් නොවේ.
      </p>
      <div className="mt-3">
        <ReportProblem entityType={entity.type} entityId={entity.id} />
      </div>
    </aside>
  )
}
