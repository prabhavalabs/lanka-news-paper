import type { ComponentProps } from 'react'

import { cn } from './utils'

function Table({ className, ...props }: ComponentProps<'table'>) {
  return (
    <div className="w-full overflow-x-auto">
      <table className={cn('w-full caption-bottom text-sm', className)} {...props} />
    </div>
  )
}

function TableHeader({ className, ...props }: ComponentProps<'thead'>) {
  return <thead className={cn('sticky top-0 bg-paper', className)} {...props} />
}

function TableBody({ className, ...props }: ComponentProps<'tbody'>) {
  return <tbody className={cn('[&_tr]:border-b [&_tr]:border-rule', className)} {...props} />
}

function TableRow({ className, ...props }: ComponentProps<'tr'>) {
  return <tr className={cn('hover:bg-tint', className)} {...props} />
}

function TableHead({ className, ...props }: ComponentProps<'th'>) {
  return (
    <th
      className={cn(
        'border-b-2 border-ink px-3 py-2 text-left font-medium tabular-nums',
        className,
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: ComponentProps<'td'>) {
  return <td className={cn('px-3 py-2 tabular-nums', className)} {...props} />
}

export { Table, TableBody, TableCell, TableHead, TableHeader, TableRow }
