const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

export function formatDateTime(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : dateTime.format(d)
}

function plural(n, unit) {
  return `${n} ${unit}${n === 1 ? '' : 's'} ago`
}

/** "2 hours ago" for recent timestamps; older than two weeks falls back to a date. */
export function timeAgo(iso, now = Date.now()) {
  const seconds = Math.floor((now - new Date(iso).getTime()) / 1000)
  if (Number.isNaN(seconds)) return '—'
  if (seconds < 45) return 'just now'
  if (seconds < 3600) return plural(Math.max(1, Math.floor(seconds / 60)), 'minute')
  if (seconds < 86400) return plural(Math.floor(seconds / 3600), 'hour')
  if (seconds < 14 * 86400) return plural(Math.floor(seconds / 86400), 'day')
  return formatDateTime(iso)
}

export const STATUS_LABELS = { open: 'Open', in_progress: 'In progress', resolved: 'Resolved', closed: 'Closed' }
export const PRIORITY_LABELS = { low: 'Low', medium: 'Medium', high: 'High', urgent: 'Urgent' }
export const ROLE_LABELS = { requester: 'Requester', agent: 'Agent', admin: 'Admin' }

export const STATUSES = Object.keys(STATUS_LABELS)
export const PRIORITIES = Object.keys(PRIORITY_LABELS)
export const ROLES = Object.keys(ROLE_LABELS)

export function categoryLabel(value) {
  if (!value) return ''
  const text = value.replaceAll('_', ' ')
  return text.charAt(0).toUpperCase() + text.slice(1)
}

/** Button text for moving a ticket from one status to another. */
export function transitionLabel(from, to) {
  const key = `${from}>${to}`
  return (
    {
      'open>in_progress': 'Start progress',
      'in_progress>resolved': 'Mark as resolved',
      'resolved>closed': 'Close ticket',
      'resolved>open': 'Reopen ticket',
      'open>closed': 'Cancel ticket',
    }[key] ?? `Move to ${STATUS_LABELS[to] ?? to}`
  )
}

/** Visual weight of a transition button. */
export function transitionTone(from, to) {
  if (to === 'closed' && from === 'open') return 'danger'
  if (to === 'resolved' || (to === 'in_progress')) return 'primary'
  return 'secondary'
}
