import { cn } from '@snap/ui/utils'

/**
 * Printer's ornaments drawn as SVG so they print identically on every phone
 * (system fonts often lack ❧ and ⁂). Decorative only: hidden from assistive tech.
 */
export function Fleuron({ className, flip = false }: { className?: string; flip?: boolean }) {
  return (
    <svg
      viewBox="0 0 32 20"
      aria-hidden="true"
      className={cn('inline-block h-[0.9em] w-auto fill-current align-[-0.1em]', flip && '-scale-x-100', className)}
    >
      <path d="M17.5 3.2c3.9-2.6 9.6-2.1 12.2 1.6 2.4 3.4 1.2 8.2-2.6 10.5-3.5 2.1-8.2 1.6-11.6-.9 1.6 3.1 1.3 4.6-.4 5.6.5-2 .2-3.7-1.5-5.4-2.3 1.6-5.6 2.4-8.6 1.6C1.7 15.2-.4 12.4.1 9.7c.4-2.3 2.4-3.6 4.5-3.2-1.6.9-2.2 2.4-1.6 3.8.8 1.9 3.7 2.5 6.5 1.6 2-.7 3.7-2.1 4.7-3.9.9-1.7 2-3.3 3.3-4.8z" />
    </svg>
  )
}

export function Asterism({ className }: { className?: string }) {
  const star = (x: number, y: number) => (
    <g transform={`translate(${x} ${y})`}>
      <path d="M0-3.6V3.6M-3.1-1.8 3.1 1.8M-3.1 1.8 3.1-1.8" strokeWidth="1.3" strokeLinecap="round" />
    </g>
  )
  return (
    <svg viewBox="0 0 24 20" aria-hidden="true" className={cn('inline-block h-[1em] w-auto stroke-current', className)}>
      {star(12, 5)}
      {star(5.5, 15)}
      {star(18.5, 15)}
    </svg>
  )
}

/** A centred ornament flanked by hairlines: a section break. */
export function OrnamentRule({ className, kind = 'asterism' }: { className?: string; kind?: 'asterism' | 'fleuron' }) {
  return (
    <div className={cn('flex items-center gap-3 text-ink', className)} aria-hidden="true">
      <span className="h-px flex-1 bg-rule" />
      {kind === 'asterism' ? (
        <Asterism className="text-base" />
      ) : (
        <span className="flex items-center gap-1 text-sm">
          <Fleuron flip />
          <Fleuron />
        </span>
      )}
      <span className="h-px flex-1 bg-rule" />
    </div>
  )
}
