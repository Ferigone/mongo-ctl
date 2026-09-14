import type { ReactNode } from 'react'

import { MetricChart, type ChartSeries } from '@/components/MetricChart'
import { Card, EmptyState } from '@/components/ui'
import { ChartIcon } from '@/components/icons'
import { formatBytes, formatCount, formatMilliseconds, formatRate } from '@/lib/format'
import type { Instance, Sample } from '@/lib/types'
import { metricsStore } from '@/state/AppState'
import { useSeries } from '@/state/ringStore'

const operationSeries: ChartSeries[] = [
  { key: 'queriesPerSec', label: 'Queries', color: 'var(--info)' },
  { key: 'insertsPerSec', label: 'Inserts', color: 'var(--accent)' },
  { key: 'updatesPerSec', label: 'Updates', color: 'var(--warning)' },
  { key: 'deletesPerSec', label: 'Deletes', color: 'var(--danger)' },
]

const networkSeries: ChartSeries[] = [
  { key: 'bytesInPerSec', label: 'In', color: 'var(--info)' },
  { key: 'bytesOutPerSec', label: 'Out', color: 'var(--accent)' },
]

const latencySeries: ChartSeries[] = [
  { key: 'readLatencyMs', label: 'Reads', color: 'var(--info)' },
  { key: 'writeLatencyMs', label: 'Writes', color: 'var(--warning)' },
  { key: 'commandLatencyMs', label: 'Commands', color: 'var(--accent)' },
]

const connectionSeries: ChartSeries[] = [
  { key: 'connectionsCurrent', label: 'Open', color: 'var(--accent)' },
  { key: 'connectionsActive', label: 'Active', color: 'var(--info)' },
]

export function OverviewTab({ instance }: { instance: Instance }) {
  const samples = useSeries(metricsStore, instance.id)
  const latest = samples.at(-1)

  if (instance.state !== 'running') {
    return (
      <EmptyState
        icon={<ChartIcon size={26} />}
        title="Metrics appear while the server runs"
        description="Start this instance to begin sampling its performance counters once per second."
      />
    )
  }

  return (
    <div className="space-y-4 p-5">
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatTile
          label="Operations / sec"
          value={latest ? formatRate(totalOperations(latest)) : '—'}
        />
        <StatTile
          label="Connections"
          value={latest ? formatCount(latest.connectionsCurrent) : '—'}
          detail={latest ? `${formatCount(latest.connectionsActive)} active` : undefined}
        />
        <StatTile
          label="Resident memory"
          value={latest ? `${formatCount(latest.memResidentMb)} MB` : '—'}
          detail={latest ? `${formatCount(latest.memVirtualMb)} MB virtual` : undefined}
        />
        <StatTile
          label="Cache used"
          value={latest ? formatBytes(latest.cacheUsedBytes) : '—'}
          detail={
            latest && latest.cacheMaxBytes > 0
              ? `${latest.cacheFillPercent.toFixed(0)}% of ${formatBytes(latest.cacheMaxBytes)}`
              : undefined
          }
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <ChartCard title="Operations per second">
          <MetricChart samples={samples} series={operationSeries} format={formatRate} />
        </ChartCard>

        <ChartCard title="Network throughput">
          <MetricChart
            samples={samples}
            series={networkSeries}
            format={(value) => `${formatBytes(value)}/s`}
          />
        </ChartCard>

        <ChartCard title="Average latency" subtitle="Mean per operation between samples">
          <MetricChart samples={samples} series={latencySeries} format={formatMilliseconds} />
        </ChartCard>

        <ChartCard title="Connections">
          <MetricChart
            samples={samples}
            series={connectionSeries}
            format={formatCount}
            breakOnGap={false}
          />
        </ChartCard>
      </div>
    </div>
  )
}

function totalOperations(sample: Sample): number {
  return (
    sample.insertsPerSec +
    sample.queriesPerSec +
    sample.updatesPerSec +
    sample.deletesPerSec +
    sample.commandsPerSec
  )
}

function StatTile({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return (
    <Card className="px-4 py-3">
      <p className="text-[11px] font-medium uppercase tracking-wide text-subtle">{label}</p>
      <p className="mt-1 text-xl font-semibold text-ink tabular">{value}</p>
      <p className="mt-0.5 h-4 text-[11px] text-muted tabular">{detail ?? ''}</p>
    </Card>
  )
}

function ChartCard({
  title,
  subtitle,
  children,
}: {
  title: string
  subtitle?: string
  children: ReactNode
}) {
  return (
    <Card className="px-4 py-3.5">
      <div className="mb-2">
        <h3 className="text-xs font-semibold text-ink">{title}</h3>
        {subtitle && <p className="text-[11px] text-subtle">{subtitle}</p>}
      </div>
      {children}
    </Card>
  )
}
