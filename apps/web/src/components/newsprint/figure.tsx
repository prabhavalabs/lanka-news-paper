import { useState } from 'react'

import { cn } from '@snap/ui/utils'
import { isImageUrl } from '@/lib/format'

/**
 * A photograph printed in newsprint black and white, always credited to its source.
 * Renders nothing when there is no usable image, so layouts never show empty boxes.
 */
export function HalftoneFigure({
  src,
  alt,
  credit,
  className,
  priority = false,
}: {
  src: string | null | undefined
  alt: string
  credit: string | undefined
  className?: string
  priority?: boolean
}) {
  const [failed, setFailed] = useState(false)
  if (!isImageUrl(src) || failed) return null
  return (
    <figure className={cn('border-y border-ink py-2', className)}>
      <div className="overflow-hidden bg-tint">
        <img
          src={src}
          alt={alt}
          loading={priority ? 'eager' : 'lazy'}
          decoding="async"
          referrerPolicy="no-referrer"
          className="np-halftone aspect-[3/2] w-full object-cover"
          onError={() => setFailed(true)}
        />
      </div>
      {credit ? (
        <figcaption className="np-meta mt-1 text-end text-[0.8125rem]">ඡායාරූපය: {credit}</figcaption>
      ) : null}
    </figure>
  )
}
