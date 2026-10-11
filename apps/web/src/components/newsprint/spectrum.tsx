import { cn } from '@snap/ui/utils'

export type Lean = 'left' | 'center' | 'right' | 'unrated'

export const leanLabels: Record<Lean, string> = {
  left: 'වාම',
  center: 'සමබර',
  right: 'දක්ෂිණ',
  unrated: 'අවිනිශ්චිත',
}

const fills: Record<Lean, string> = {
  left: 'np-fill-left',
  center: 'np-fill-center',
  right: 'np-fill-right',
  unrated: 'bg-paper',
}

export function Swatch({ lean, className }: { lean: Lean; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn('inline-block size-3.5 shrink-0 border border-ink align-[-0.15em]', lean === 'unrated' && 'border-dashed', fills[lean], className)}
    />
  )
}

/** A small printed tag naming how one report leans. */
export function LeanTag({ lean, className }: { lean: Lean; className?: string }) {
  return (
    <span className={cn('np-meta inline-flex items-center gap-1.5 text-[0.75rem] text-ink-secondary', className)}>
      <Swatch lean={lean} className="size-3" />
      {leanLabels[lean]}
    </span>
  )
}

/**
 * Left/centre/right split printed in monochrome: hatching, stipple and solid ink
 * instead of colour, with a legend that carries the numbers for screen readers and print.
 */
export function SpectrumBar({
  left,
  center,
  right,
  counts,
  className,
}: {
  left: number
  center: number
  right: number
  /** Number of outlets in each lean, shown in the legend when given. */
  counts?: Partial<Record<Lean, number>>
  className?: string
}) {
  const values = [
    ['left', left],
    ['center', center],
    ['right', right],
  ] as const
  const label = values.map(([lean, value]) => `${leanLabels[lean]} ${Math.round(value)}%`).join(', ')
  return (
    <div className={className}>
      <div role="img" aria-label={label} className="flex h-5 overflow-hidden border border-ink">
        {values.map(([lean, value]) =>
          value > 0 ? <span key={lean} className={cn(fills[lean], 'h-full')} style={{ width: `${value}%` }} /> : null,
        )}
      </div>
      <ul className="np-meta mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[0.8125rem]" aria-hidden="true">
        {values.map(([lean, value]) => (
          <li key={lean} className="inline-flex items-center gap-1.5">
            <Swatch lean={lean} className="size-3" />
            {leanLabels[lean]}
            <span className="np-latin text-ink">{Math.round(value)}%</span>
            {counts?.[lean] !== undefined ? <span>({counts[lean]})</span> : null}
          </li>
        ))}
        {counts?.unrated ? (
          <li className="inline-flex items-center gap-1.5">
            <Swatch lean="unrated" className="size-3" />
            {leanLabels.unrated} ({counts.unrated})
          </li>
        ) : null}
      </ul>
    </div>
  )
}
