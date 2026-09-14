import { useEffect, useMemo, useRef, useState } from 'react'

import { TerminalIcon } from '@/components/icons'
import { Checkbox, EmptyState, Input, Select, cn } from '@/components/ui'
import { formatClock } from '@/lib/format'
import type { Instance, LogLine, LogSeverity } from '@/lib/types'
import { logStore } from '@/state/AppState'
import { useSeries } from '@/state/ringStore'

/** Rendering the whole retained buffer would cost more than it shows. */
const RENDER_LIMIT = 600

const severityOrder: LogSeverity[] = ['debug', 'info', 'warning', 'error', 'fatal']

const severityStyles: Record<LogSeverity, string> = {
  fatal: 'text-danger font-semibold',
  error: 'text-danger',
  warning: 'text-warning',
  info: 'text-muted',
  debug: 'text-subtle',
}

export function LogsTab({ instance }: { instance: Instance }) {
  const lines = useSeries(logStore, instance.id)
  const [minSeverity, setMinSeverity] = useState<LogSeverity>('info')
  const [query, setQuery] = useState('')
  const [follow, setFollow] = useState(true)

  const scrollRef = useRef<HTMLDivElement>(null)

  const filtered = useMemo(() => {
    const threshold = severityOrder.indexOf(minSeverity)
    const needle = query.trim().toLowerCase()

    const matches = lines.filter((line) => {
      if (severityOrder.indexOf(line.severity) < threshold) return false
      if (!needle) return true
      return (
        line.message.toLowerCase().includes(needle) ||
        line.component.toLowerCase().includes(needle)
      )
    })

    return matches.slice(-RENDER_LIMIT)
  }, [lines, minSeverity, query])

  useEffect(() => {
    if (!follow) return
    const container = scrollRef.current
    if (container) container.scrollTop = container.scrollHeight
  }, [filtered, follow])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-3 border-b border-line px-5 py-2.5">
        <Input
          className="h-7 max-w-xs text-xs"
          placeholder="Filter messages…"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <Select
          className="h-7 w-auto text-xs"
          value={minSeverity}
          onChange={(event) => setMinSeverity(event.target.value as LogSeverity)}
        >
          <option value="debug">All severities</option>
          <option value="info">Info and above</option>
          <option value="warning">Warnings and above</option>
          <option value="error">Errors only</option>
        </Select>
        <div className="ml-auto">
          <Checkbox checked={follow} onChange={setFollow} label="Follow" />
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          icon={<TerminalIcon size={26} />}
          title={lines.length === 0 ? 'No output yet' : 'Nothing matches the filter'}
          description={
            lines.length === 0
              ? 'Log lines stream here while the server runs.'
              : 'Try a different severity or search term.'
          }
        />
      ) : (
        <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto bg-sunken px-5 py-3">
          <div className="space-y-0.5 font-mono text-[11px] leading-relaxed" data-selectable>
            {filtered.map((line, index) => (
              <LogRow key={`${line.timestamp}-${index}`} line={line} />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function LogRow({ line }: { line: LogLine }) {
  return (
    <div className="flex gap-2.5">
      <span className="shrink-0 text-subtle tabular">{formatClock(line.timestamp)}</span>
      {line.component && (
        <span className="w-20 shrink-0 truncate text-subtle" title={line.component}>
          {line.component}
        </span>
      )}
      <span className={cn('min-w-0 flex-1 break-words', severityStyles[line.severity])}>
        {line.message}
      </span>
    </div>
  )
}
