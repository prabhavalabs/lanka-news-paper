import type { StoryImage } from '@snap/api-client'
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
    <figure className={cn('np-avoid-break', className)}>
      <img
        src={src}
        alt={alt}
        loading={priority ? 'eager' : 'lazy'}
        decoding="async"
        referrerPolicy="no-referrer"
        className="np-halftone aspect-[3/2] w-full border border-ink object-cover"
        onError={() => setFailed(true)}
      />
      {credit ? <figcaption className="np-meta mt-0.5 text-[0.75rem] leading-[1.5]">ඡායාරූපය: {credit}</figcaption> : null}
    </figure>
  )
}

export const ILLUSTRATION_NOTE = 'චිත්‍රය: AI නිර්මාණයකි — සැබෑ ඡායාරූපයක් නොවේ'

/** A generated woodcut-style illustration: printed in ink, always labelled as an illustration. */
export function Illustration({
  image,
  className,
  priority = false,
  large = false,
}: {
  image: StoryImage | null | undefined
  className?: string
  priority?: boolean
  large?: boolean
}) {
  const [failed, setFailed] = useState(false)
  if (!image || !isImageUrl(image.url) || failed) return null
  return (
    <figure className={cn('np-avoid-break', className)}>
      <img
        src={image.url}
        alt={image.alt}
        width={image.width || undefined}
        height={image.height || undefined}
        loading={priority ? 'eager' : 'lazy'}
        decoding="async"
        className="np-halftone h-auto w-full border border-ink"
        onError={() => setFailed(true)}
      />
      <figcaption className={cn('np-meta mt-0.5 leading-[1.5]', large ? 'text-[0.8125rem]' : 'text-[0.75rem]')}>
        {image.caption || ILLUSTRATION_NOTE}
      </figcaption>
    </figure>
  )
}
