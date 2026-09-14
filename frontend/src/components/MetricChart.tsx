import { useId, useMemo } from 'react'
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import type { Sample } from '@/lib/types'

export interface ChartSeries {
  /** Key on Sample holding a numeric value. */
  key: keyof Sample
  label: string
  /** Any CSS colour; theme tokens keep the chart in step with the app theme. */
  color: string
}

interface MetricChartProps {
  samples: readonly Sample[]
  series: ChartSeries[]
  format: (value: number) => string
  height?: number
  /** Breaks the line where the server restarted instead of drawing a false drop. */
  breakOnGap?: boolean
}

interface ChartRow {
  time: number
  [key: string]: number | null
}

export function MetricChart({
  samples,
  series,
  format,
  height = 200,
  breakOnGap = true,
}: MetricChartProps) {
  const gradientId = useId()

  const data = useMemo<ChartRow[]>(
    () =>
      samples.map((sample) => {
        const row: ChartRow = { time: Date.parse(sample.timestamp) }
        for (const entry of series) {
          const value = sample[entry.key]
          row[entry.key as string] =
            breakOnGap && sample.gap ? null : typeof value === 'number' ? value : null
        }
        return row
      }),
    [samples, series, breakOnGap],
  )

  if (data.length === 0) {
    return (
      <div
        className="flex items-center justify-center rounded-lg border border-dashed border-line text-xs text-subtle"
        style={{ height }}
      >
        Waiting for samples…
      </div>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={height}>
      <AreaChart data={data} margin={{ top: 6, right: 6, bottom: 0, left: 0 }}>
        <defs>
          {series.map((entry) => (
            <linearGradient
              key={entry.key as string}
              id={`${gradientId}-${entry.key as string}`}
              x1="0"
              y1="0"
              x2="0"
              y2="1"
            >
              <stop offset="0%" stopColor={entry.color} stopOpacity={0.28} />
              <stop offset="100%" stopColor={entry.color} stopOpacity={0} />
            </linearGradient>
          ))}
        </defs>

        <CartesianGrid stroke="var(--border)" strokeDasharray="2 4" vertical={false} />

        <XAxis
          dataKey="time"
          type="number"
          domain={['dataMin', 'dataMax']}
          scale="time"
          tickFormatter={(value: number) =>
            new Date(value).toLocaleTimeString(undefined, {
              hour12: false,
              minute: '2-digit',
              second: '2-digit',
            })
          }
          stroke="var(--text-subtle)"
          tick={{ fontSize: 10 }}
          tickLine={false}
          axisLine={false}
          minTickGap={40}
        />

        <YAxis
          stroke="var(--text-subtle)"
          tick={{ fontSize: 10 }}
          tickLine={false}
          axisLine={false}
          width={54}
          tickFormatter={format}
        />

        <Tooltip
          contentStyle={{
            background: 'var(--surface-raised)',
            border: '1px solid var(--border)',
            borderRadius: 8,
            fontSize: 12,
            color: 'var(--text)',
          }}
          labelFormatter={(value) => new Date(Number(value)).toLocaleTimeString(undefined, { hour12: false })}
          formatter={(value, name) => [format(Number(value ?? 0)), String(name ?? '')]}
        />

        {series.length > 1 && (
          <Legend
            verticalAlign="top"
            align="right"
            height={22}
            iconType="plainline"
            iconSize={10}
            wrapperStyle={{ fontSize: 11, color: 'var(--text-muted)' }}
          />
        )}

        {series.map((entry) => (
          <Area
            key={entry.key as string}
            type="monotone"
            dataKey={entry.key as string}
            name={entry.label}
            stroke={entry.color}
            strokeWidth={1.75}
            fill={`url(#${gradientId}-${entry.key as string})`}
            connectNulls={false}
            dot={false}
            isAnimationActive={false}
          />
        ))}
      </AreaChart>
    </ResponsiveContainer>
  )
}
