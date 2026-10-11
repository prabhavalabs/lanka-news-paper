import { cn } from '@snap/ui/utils'

export const leanLabels = { left: 'වම', center: 'මැද', right: 'දකුණ', unrated: 'නොමැනූ' } as const

const fills = { left: 'np-fill-left', center: 'np-fill-center', right: 'np-fill-right' } as const

export function Swatch({ lean, className }: { lean: keyof typeof fills; className?: string }) {
  return <span aria-hidden="true" className={cn('inline-block size-3.5 border border-ink align-[-0.15em]', fills[lean], className)} />
}

/**
 * Left/centre/right split printed in monochrome: hatching, stipple and solid ink
 * instead of colour, with a legend that carries the numbers for screen readers and print.
 */
export function SpectrumBar({ left, center, right, className }: { left: number; center: number; right: number; className?: string }) {
  const values = [
    ['left', left],
    ['center', center],
    ['right', right],
  ] as const
  const label = values.map(([lean, value]) => `${leanLabels[lean]} ${Math.round(value)}%`).join(', ')
  return (
    <div className={className}>
      <div role="img" aria-label={label} className="flex h-6 overflow-hidden border border-ink">
        {values.map(([lean, value]) =>
          value > 0 ? <span key={lean} className={cn(fills[lean], 'h-full')} style={{ width: `${value}%` }} /> : null,
        )}
      </div>
      <ul className="np-meta mt-2 flex flex-wrap gap-x-4 text-[0.875rem]" aria-hidden="true">
        {values.map(([lean, value]) => (
          <li key={lean} className="inline-flex items-center gap-1.5">
            <Swatch lean={lean} />
            {leanLabels[lean]} <span className="np-latin text-ink">{Math.round(value)}%</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
