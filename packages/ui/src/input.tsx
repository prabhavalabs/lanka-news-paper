import type { ComponentProps } from 'react'

import { cn } from './utils'

function Input({ className, ...props }: ComponentProps<'input'>) {
  return (
    <input
      className={cn(
        'h-11 w-full border-0 border-b border-ink bg-transparent px-0 py-2 text-sm outline-none focus-visible:border-b-2',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
