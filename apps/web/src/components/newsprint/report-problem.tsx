import { createClient } from '@snap/api-client'
import { useState } from 'react'
import { toast } from 'sonner'

import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from '@/components/ui/dialog'

const client = createClient()

export function ReportProblem({ entityType, entityId }: { entityType: 'article' | 'event'; entityId: string }) {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState('')
  const [sending, setSending] = useState(false)
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger className="np-btn np-btn-ghost">ගැටලුවක් වාර්තා කරන්න</DialogTrigger>
      <DialogContent className="border border-ink bg-paper-raised p-5 shadow-[4px_4px_0_color-mix(in_oklch,var(--ink),transparent_88%)] ring-0 sm:max-w-md">
        <DialogTitle className="np-headline text-[1.5rem] leading-[1.4] font-extrabold">ගැටලුවක් වාර්තා කරන්න</DialogTitle>
        <DialogDescription className="np-meta text-[0.9375rem]">
          වැරදි තොරතුරක්, වැරදි මූලාශ්‍රයක් හෝ ඉවත් කළ යුතු දෙයක් ඇත්නම් කෙටියෙන් සඳහන් කරන්න.
        </DialogDescription>
        <form
          className="flex flex-col gap-3"
          onSubmit={async (event) => {
            event.preventDefault()
            setSending(true)
            try {
              await client.complain({ entity_type: entityType, entity_id: entityId, reason })
              toast.success('ස්තූතියි. ඔබගේ වාර්තාව ලැබුණි.')
              setReason('')
              setOpen(false)
            } catch {
              toast.error('යැවීම අසාර්ථක විය. නැවත උත්සාහ කරන්න.')
            } finally {
              setSending(false)
            }
          }}
        >
          <label htmlFor="report-reason" className="sr-only">
            ගැටලුව
          </label>
          <textarea
            id="report-reason"
            className="np-field min-h-28"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            required
          />
          <button type="submit" className="np-btn" disabled={sending}>
            {sending ? 'යවමින්…' : 'යවන්න'}
          </button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
