import { useEffect, useState } from 'react'

import {
  ChartIcon,
  DatabaseIcon,
  FileIcon,
  PlayIcon,
  RestartIcon,
  StopIcon,
  TerminalIcon,
  TrashIcon,
} from '@/components/icons'
import { Badge, Button, Callout, IconButton, Modal, Tabs, Checkbox } from '@/components/ui'
import { formatDuration } from '@/lib/format'
import { instanceLabel, instanceTone, isTransitional } from '@/lib/status'
import type { Instance } from '@/lib/types'
import { useAppState } from '@/state/AppState'
import { useInstanceActions } from '@/state/useInstanceActions'
import { useNow } from '@/state/useNow'
import { ConfigTab } from './tabs/ConfigTab'
import { DatabasesTab } from './tabs/DatabasesTab'
import { LogsTab } from './tabs/LogsTab'
import { OverviewTab } from './tabs/OverviewTab'

const TABS = [
  { id: 'overview', label: 'Overview', icon: <ChartIcon size={13} /> },
  { id: 'databases', label: 'Databases', icon: <DatabaseIcon size={13} /> },
  { id: 'config', label: 'Configuration', icon: <FileIcon size={13} /> },
  { id: 'logs', label: 'Logs', icon: <TerminalIcon size={13} /> },
]

export function InstanceDetail({ instance }: { instance: Instance }) {
  const { hydrateInstance } = useAppState()
  const actions = useInstanceActions()
  const [tab, setTab] = useState('overview')
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    void hydrateInstance(instance.id)
  }, [instance.id, hydrateInstance])

  const busy = actions.busy[instance.id]
  const transitional = isTransitional(instance.state)
  const running = instance.state === 'running'

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <Header
        instance={instance}
        running={running}
        transitional={transitional}
        busy={Boolean(busy)}
        onStart={() => actions.start(instance.id, instance.name)}
        onStop={() => actions.stop(instance.id, instance.name)}
        onRestart={() => actions.restart(instance.id, instance.name)}
        onDelete={() => setConfirmDelete(true)}
      />

      <Tabs tabs={TABS} active={tab} onChange={setTab} />

      <div className="min-h-0 flex-1 overflow-y-auto">
        {tab === 'overview' && <OverviewTab instance={instance} />}
        {tab === 'databases' && <DatabasesTab instance={instance} />}
        {tab === 'config' && <ConfigTab instance={instance} />}
        {tab === 'logs' && <LogsTab instance={instance} />}
      </div>

      <DeleteDialog
        instance={instance}
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        onConfirm={(deleteData) => {
          setConfirmDelete(false)
          void actions.remove(instance.id, instance.name, deleteData)
        }}
      />
    </div>
  )
}

function Header({
  instance,
  running,
  transitional,
  busy,
  onStart,
  onStop,
  onRestart,
  onDelete,
}: {
  instance: Instance
  running: boolean
  transitional: boolean
  busy: boolean
  onStart: () => void
  onStop: () => void
  onRestart: () => void
  onDelete: () => void
}) {
  const now = useNow()
  const startedAt = Date.parse(instance.startedAt)
  const uptime =
    running && Number.isFinite(startedAt) ? formatDuration((now - startedAt) / 1000) : '—'

  return (
    <header className="border-b border-line px-5 pb-3 pt-4">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h1 className="truncate text-base font-semibold text-ink">{instance.name}</h1>
            <Badge tone={instanceTone(instance.state)}>{instanceLabel(instance.state)}</Badge>
            {instance.replicaSet && <Badge tone="accent">{instance.replicaSet}</Badge>}
          </div>
          <p className="mt-1 truncate font-mono text-xs text-muted" data-selectable>
            mongodb://127.0.0.1:{instance.port}
          </p>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {running ? (
            <Button icon={<StopIcon size={13} />} onClick={onStop} loading={busy} disabled={transitional}>
              Stop
            </Button>
          ) : (
            <Button
              variant="primary"
              icon={<PlayIcon size={13} />}
              onClick={onStart}
              loading={busy}
              disabled={transitional}
            >
              Start
            </Button>
          )}
          <Button
            icon={<RestartIcon size={13} />}
            onClick={onRestart}
            loading={busy}
            disabled={transitional || !running}
          >
            Restart
          </Button>
          <IconButton label="Delete instance" onClick={onDelete}>
            <TrashIcon size={15} />
          </IconButton>
        </div>
      </div>

      <dl className="mt-3 flex flex-wrap items-center gap-x-6 gap-y-1 text-xs">
        <Stat label="Uptime" value={uptime} />
        <Stat label="PID" value={instance.pid > 0 ? String(instance.pid) : '—'} />
        <Stat label="Data" value={instance.dataDir} mono title={instance.dataDir} />
      </dl>

      {instance.error && (
        <div className="mt-3">
          <Callout tone="danger" title="Last error">
            <span data-selectable>{instance.error}</span>
          </Callout>
        </div>
      )}

      {instance.uncleanStop && !instance.error && (
        <div className="mt-3">
          <Callout tone="warning" title="Stopped forcibly">
            This server had to be killed because it did not shut down in time. The next start runs
            WiredTiger recovery, which can take a moment.
          </Callout>
        </div>
      )}
    </header>
  )
}

function Stat({
  label,
  value,
  mono = false,
  title,
}: {
  label: string
  value: string
  mono?: boolean
  title?: string
}) {
  return (
    <div className="flex min-w-0 items-baseline gap-1.5">
      <dt className="text-subtle">{label}</dt>
      <dd
        className={`truncate text-ink ${mono ? 'font-mono text-[11px]' : 'tabular'}`}
        title={title}
        data-selectable
      >
        {value}
      </dd>
    </div>
  )
}

function DeleteDialog({
  instance,
  open,
  onClose,
  onConfirm,
}: {
  instance: Instance
  open: boolean
  onClose: () => void
  onConfirm: (deleteData: boolean) => void
}) {
  const [deleteData, setDeleteData] = useState(false)

  useEffect(() => {
    if (open) setDeleteData(false)
  }, [open])

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`Delete ${instance.name}?`}
      description="The server is stopped first."
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="danger" onClick={() => onConfirm(deleteData)}>
            {deleteData ? 'Delete instance and data' : 'Delete instance'}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Checkbox
          checked={deleteData}
          onChange={setDeleteData}
          label="Also delete the data directory"
          description={instance.dataDir}
        />
        {deleteData && (
          <Callout tone="danger" title="This cannot be undone">
            Every database stored in that folder is removed from disk.
          </Callout>
        )}
      </div>
    </Modal>
  )
}
