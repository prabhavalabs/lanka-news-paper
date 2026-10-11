import { useEffect } from 'react'

import { cn } from '@snap/ui/utils'
import { Fleuron } from './ornament'

export function useDocumentTitle(title: string | undefined) {
  useEffect(() => {
    if (title) document.title = `${title} | තුලාව`
  }, [title])
}

/** Section-front heading: ornament-flanked title between rules, like an inside-page head. */
export function PageHead({
  title,
  kicker,
  deck,
  className,
}: {
  title: string
  kicker?: string
  deck?: React.ReactNode
  className?: string
}) {
  return (
    <header className={cn('text-center', className)}>
      <div className="np-rule-double" />
      {kicker ? <p className="np-kicker mt-3">{kicker}</p> : null}
      <h1 className="np-headline mt-1 flex items-center justify-center gap-3 text-[2.125rem] leading-[1.35] md:text-[2.75rem]">
        <Fleuron flip className="hidden text-[0.5em] text-ink-secondary sm:inline-block" />
        <span>{title}</span>
        <Fleuron className="hidden text-[0.5em] text-ink-secondary sm:inline-block" />
      </h1>
      {deck ? <div className="np-body mx-auto mt-2 max-w-[60ch] text-[1.0625rem] text-ink-secondary">{deck}</div> : null}
      <div className="np-rule mt-4" />
    </header>
  )
}
