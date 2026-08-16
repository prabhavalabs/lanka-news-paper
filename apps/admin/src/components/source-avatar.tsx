import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'

type SourceAvatarProps = {
  name: string
  website?: string | null
}

export function SourceAvatar({ name, website }: SourceAvatarProps) {
  let favicon: string | undefined

  try {
    favicon = website ? new URL('/favicon.ico', website).toString() : undefined
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
    <Avatar aria-hidden="true" className="bg-background">
      {favicon ? <AvatarImage src={favicon} alt="" /> : null}
      <AvatarFallback className="font-medium">{initials || '—'}</AvatarFallback>
    </Avatar>
  )
}
