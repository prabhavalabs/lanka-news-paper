import { useState } from 'react'

import { cn } from '@snap/ui/utils'

/** An outlet's logo printed in black and white in a small frame; its initial when there is none. */
export function OutletLogo({ src, name, className }: { src: string | null | undefined; name: string; className?: string }) {
  const [failed, setFailed] = useState(false)
  const usable = src && /^(https?:\/\/|\/)/.test(src) && !failed
  return (
    <span
      aria-hidden="true"
      className={cn('grid size-8 shrink-0 place-items-center overflow-hidden border border-ink bg-paper-raised p-0.5', className)}
    >
      {usable ? (
        <img
          src={src}
          alt=""
          width={28}
          height={28}
          loading="lazy"
          decoding="async"
          referrerPolicy="no-referrer"
          className="size-full object-contain grayscale contrast-125"
          onError={() => setFailed(true)}
        />
      ) : (
        <span className="font-headline text-[1rem] leading-none text-ink">{[...name.trim()][0]?.toUpperCase() ?? '·'}</span>
      )}
    </span>
  )
}
