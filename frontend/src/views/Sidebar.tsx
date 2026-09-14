import type { ReactNode } from 'react'

import { Sparkline } from '@/components/Sparkline'
import { LayersIcon, LinkIcon, PlusIcon } from '@/components/icons'
import { Badge, Button, StatusDot, cn } from '@/components/ui'
import { instanceLabel, instanceTone } from '@/lib/status'
import type { Instance, ReplicaSet, Sample } from '@/lib/types'
import { metricsStore } from '@/state/AppState'
import { useSeries } from '@/state/ringStore'

export type Selection =
  | { kind: 'instance'; id: string }
  | { kind: 'replicaSet'; name: string }
  | { kind: 'settings' }

interface SidebarProps {
  instances: Instance[]
  replicaSets: ReplicaSet[]
  selection: Selection
  onSelect: (selection: Selection) => void
  checked: Set<string>
  onToggleChecked: (id: string) => void
  onCreate: () => void
  onGroup: () => void
}

export function Sidebar({
  instances,
  replicaSets,
  selection,
  onSelect,
  checked,
  onToggleChecked,
  onCreate,
  onGroup,
}: SidebarProps) {
  const standalone = instances.filter((instance) => !instance.replicaSet)
  const grouped = new Map<string, Instance[]>()
  for (const instance of instances) {
    if (!instance.replicaSet) continue
    const members = grouped.get(instance.replicaSet) ?? []
    members.push(instance)
    grouped.set(instance.replicaSet, members)
  }

  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-line bg-surface">
      <div className="flex-1 overflow-y-auto px-2 py-3">
        <SidebarSection
          title="Instances"
          count={standalone.length}
          action={
            <button
              type="button"
              onClick={onCreate}
              className="rounded p-1 text-subtle transition-colors hover:bg-sunken hover:text-ink"
              aria-label="New instance"
              title="New instance"
            >
              <PlusIcon size={14} />
            </button>
          }
        >
          {standalone.length === 0 ? (
            <p className="px-2 py-3 text-xs text-subtle">No standalone instances yet.</p>
          ) : (
            standalone.map((instance) => (
              <InstanceRow
                key={instance.id}
                instance={instance}
                selected={selection.kind === 'instance' && selection.id === instance.id}
                checked={checked.has(instance.id)}
                onSelect={() => onSelect({ kind: 'instance', id: instance.id })}
                onToggleChecked={() => onToggleChecked(instance.id)}
              />
            ))
          )}
        </SidebarSection>

        {replicaSets.length > 0 && (
          <SidebarSection title="Replica sets" count={replicaSets.length}>
            {replicaSets.map((replicaSet) => {
              const members = grouped.get(replicaSet.name) ?? []
              return (
                <div key={replicaSet.name} className="mb-1">
                  <button
                    type="button"
                    onClick={() => onSelect({ kind: 'replicaSet', name: replicaSet.name })}
                    className={cn(
                      'flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors',
                      selection.kind === 'replicaSet' && selection.name === replicaSet.name
                        ? 'bg-accent-soft text-ink'
                        : 'text-muted hover:bg-sunken hover:text-ink',
                    )}
                  >
                    <LayersIcon size={14} />
                    <span className="flex-1 truncate text-xs font-medium">{replicaSet.name}</span>
                    <Badge tone={replicaSet.status ? 'success' : 'neutral'}>{members.length}</Badge>
                  </button>

                  <div className="ml-3 border-l border-line pl-1">
                    {members.map((instance) => (
                      <InstanceRow
                        key={instance.id}
                        instance={instance}
                        compact
                        selected={selection.kind === 'instance' && selection.id === instance.id}
                        checked={checked.has(instance.id)}
                        onSelect={() => onSelect({ kind: 'instance', id: instance.id })}
                        onToggleChecked={() => onToggleChecked(instance.id)}
                      />
                    ))}
                  </div>
                </div>
              )
            })}
          </SidebarSection>
        )}
      </div>

      <div className="border-t border-line p-2">
        {checked.size >= 2 ? (
          <Button variant="primary" className="w-full" icon={<LinkIcon size={14} />} onClick={onGroup}>
            Group {checked.size} into a replica set
          </Button>
        ) : (
          <Button className="w-full" icon={<PlusIcon size={14} />} onClick={onCreate}>
            New instance
          </Button>
        )}
        <p className="mt-2 px-1 text-[11px] leading-relaxed text-subtle">
          {checked.size === 1
            ? 'Select at least one more instance to form a replica set.'
            : checked.size === 0
              ? 'Tick instances to combine them into a replica set.'
              : 'Members restart when the set is created.'}
        </p>
      </div>
    </aside>
  )
}

function SidebarSection({
  title,
  count,
  action,
  children,
}: {
  title: string
  count: number
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="mb-4">
      <header className="flex items-center justify-between px-2 py-1.5">
        <h2 className="text-[11px] font-semibold uppercase tracking-wider text-subtle">
          {title}
          <span className="ml-1.5 font-normal normal-case tracking-normal">{count}</span>
        </h2>
        {action}
      </header>
      {children}
    </section>
  )
}

function InstanceRow({
  instance,
  selected,
  checked,
  compact = false,
  onSelect,
  onToggleChecked,
}: {
  instance: Instance
  selected: boolean
  checked: boolean
  compact?: boolean
  onSelect: () => void
  onToggleChecked: () => void
}) {
  const samples = useSeries(metricsStore, instance.id)
  const running = instance.state === 'running'

  return (
    <div
      className={cn(
        'group flex items-center gap-2 rounded-lg px-2 py-1.5 transition-colors',
        selected ? 'bg-accent-soft' : 'hover:bg-sunken',
      )}
    >
      <button
        type="button"
        onClick={onToggleChecked}
        aria-label={checked ? `Deselect ${instance.name}` : `Select ${instance.name}`}
        className={cn(
          'flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border transition-colors',
          // Always visible: grouping instances is a headline feature, so the
          // control that starts it must not be hidden behind a hover.
          checked ? 'border-accent bg-accent' : 'border-line-strong hover:border-accent',
        )}
      >
        {checked && (
          <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="var(--accent-contrast)" strokeWidth="4">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        )}
      </button>

      <button type="button" onClick={onSelect} className="flex min-w-0 flex-1 items-center gap-2 text-left">
        <StatusDot tone={instanceTone(instance.state)} pulse={instance.state === 'starting'} />
        <span className="min-w-0 flex-1">
          <span className={cn('block truncate text-xs font-medium', selected ? 'text-ink' : 'text-ink/90')}>
            {instance.name}
          </span>
          {!compact && (
            <span className="block text-[11px] text-subtle tabular">
              :{instance.port} · {instanceLabel(instance.state)}
            </span>
          )}
        </span>

        {running && !compact && (
          <Sparkline values={operationRates(samples)} className="shrink-0 text-accent" />
        )}
      </button>
    </div>
  )
}

/** Total operations per second, the single number worth a sidebar sparkline. */
function operationRates(samples: readonly Sample[]): number[] {
  return samples
    .slice(-40)
    .map((sample) =>
      sample.insertsPerSec +
      sample.queriesPerSec +
      sample.updatesPerSec +
      sample.deletesPerSec +
      sample.commandsPerSec,
    )
}
