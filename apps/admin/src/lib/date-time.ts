const dateTimeFormatter = new Intl.DateTimeFormat('en', {
  dateStyle: 'medium',
  timeStyle: 'short',
})

export function formatDateTime(value?: string | null) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return dateTimeFormatter.format(date)
}

export function formatCompactRelativeTime(value?: string | null, now = Date.now()) {
  if (!value) return 'Never'
  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) return 'Never'

  const differenceInSeconds = Math.trunc((timestamp - now) / 1000)
  const absoluteSeconds = Math.abs(differenceInSeconds)
  if (absoluteSeconds < 60) return 'now'

  let amount: number
  let unit: string
  if (absoluteSeconds < 60 * 60) {
    amount = Math.floor(absoluteSeconds / 60)
    unit = 'm'
  } else if (absoluteSeconds < 24 * 60 * 60) {
    amount = Math.floor(absoluteSeconds / (60 * 60))
    unit = 'h'
  } else if (absoluteSeconds < 7 * 24 * 60 * 60) {
    amount = Math.floor(absoluteSeconds / (24 * 60 * 60))
    unit = 'd'
  } else if (absoluteSeconds < 30 * 24 * 60 * 60) {
    amount = Math.floor(absoluteSeconds / (7 * 24 * 60 * 60))
    unit = 'w'
  } else if (absoluteSeconds < 365 * 24 * 60 * 60) {
    amount = Math.floor(absoluteSeconds / (30 * 24 * 60 * 60))
    unit = 'mo'
  } else {
    amount = Math.floor(absoluteSeconds / (365 * 24 * 60 * 60))
    unit = 'y'
  }

  return differenceInSeconds < 0 ? `${amount}${unit} ago` : `in ${amount}${unit}`
}
