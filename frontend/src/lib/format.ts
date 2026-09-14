const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** Formats a byte count with a unit, e.g. 1536 -> "1.5 KB". */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'

  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), BYTE_UNITS.length - 1)
  const value = bytes / 1024 ** exponent
  const digits = value >= 100 || exponent === 0 ? 0 : 1

  return `${value.toFixed(digits)} ${BYTE_UNITS[exponent]}`
}

/** Formats a per-second rate, keeping small values readable. */
export function formatRate(value: number): string {
  if (!Number.isFinite(value) || value === 0) return '0'
  if (value < 1) return value.toFixed(2)
  if (value < 100) return value.toFixed(1)
  return Math.round(value).toLocaleString()
}

/** Formats a count with thousands separators. */
export function formatCount(value: number): string {
  if (!Number.isFinite(value)) return '0'
  return Math.round(value).toLocaleString()
}

/** Formats milliseconds with a precision that suits the magnitude. */
export function formatMilliseconds(value: number): string {
  if (!Number.isFinite(value) || value === 0) return '0 ms'
  if (value < 1) return `${value.toFixed(2)} ms`
  if (value < 100) return `${value.toFixed(1)} ms`
  return `${Math.round(value)} ms`
}

/** Formats a duration in seconds as a compact uptime, e.g. "3d 4h". */
export function formatDuration(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds < 1) return '—'

  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = Math.floor(totalSeconds % 60)

  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${seconds}s`
  return `${seconds}s`
}

/** Seconds elapsed since an ISO timestamp, or 0 when it is unset. */
export function secondsSince(iso: string): number {
  const started = Date.parse(iso)
  if (!Number.isFinite(started) || started <= 0) return 0
  return Math.max(0, (Date.now() - started) / 1000)
}

/** Formats an ISO timestamp as a wall clock time with seconds. */
export function formatClock(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleTimeString(undefined, { hour12: false })
}

/** Shortens a long filesystem path for display in tight spaces. */
export function shortenPath(path: string, maxLength = 48): string {
  if (path.length <= maxLength) return path

  const segments = path.split(/[\\/]/)
  if (segments.length <= 2) return `…${path.slice(-maxLength + 1)}`

  const tail = segments.slice(-2).join('\\')
  const head = segments[0]
  return `${head}\\…\\${tail}`
}
