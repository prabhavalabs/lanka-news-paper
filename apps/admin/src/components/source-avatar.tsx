import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { cn } from '@/lib/utils'

type SourceAvatarProps = {
  name: string
  website?: string | null
  iconUrl?: string | null
  className?: string
}

export function SourceAvatar({ name, website, iconUrl, className }: SourceAvatarProps) {
  let favicon: string | undefined

  try {
    favicon = iconUrl || (website ? new URL('/favicon.ico', website).toString() : undefined)
  } catch {
    favicon = undefined
  }

  const initials = name
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0])
    .join('')
    .toUpperCase()

  return (
    <Avatar aria-hidden="true" className={cn('bg-background', className)}>
      {favicon ? <AvatarImage src={favicon} alt="" /> : null}
      <AvatarFallback className="font-medium">{initials || '—'}</AvatarFallback>
    </Avatar>
  )
}
