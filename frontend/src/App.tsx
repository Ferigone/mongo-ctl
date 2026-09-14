import { useCallback, useEffect, useMemo, useState } from 'react'

import { CreateInstanceDialog } from '@/dialogs/CreateInstanceDialog'
import { GroupDialog } from '@/dialogs/GroupDialog'
import { DatabaseIcon, MoonIcon, ServerIcon, SettingsIcon, SunIcon } from '@/components/icons'
import { Badge, Button, EmptyState, IconButton, Spinner, ToastProvider, cn } from '@/components/ui'
import { events } from '@/lib/api'
import { AppStateProvider, useAppState } from '@/state/AppState'
import { useTheme } from '@/state/useTheme'
import { InstanceDetail } from '@/views/InstanceDetail'
import { ReplicaSetDetail } from '@/views/ReplicaSetDetail'
import { SettingsPanel } from '@/views/SettingsPanel'
import { Sidebar, type Selection } from '@/views/Sidebar'

export default function App() {
  return (
    <ToastProvider>
      <AppStateProvider>
        <Shell />
      </AppStateProvider>
    </ToastProvider>
  )
}

function Shell() {
  const { instances, replicaSets, system, ready } = useAppState()
  const { theme, toggle } = useTheme()

  const [selection, setSelection] = useState<Selection>({ kind: 'settings' })
  const [checked, setChecked] = useState<Set<string>>(new Set())
  const [creating, setCreating] = useState(false)
  const [grouping, setGrouping] = useState(false)
  const [autoSelected, setAutoSelected] = useState(false)
  const [shuttingDown, setShuttingDown] = useState(false)

  // Closing the window keeps it open until the servers are down, so the delay
  // needs an explanation rather than a window that refuses to go away.
  useEffect(() => events.onShutdown(() => setShuttingDown(true)), [])

  // Land on the first instance once data arrives, but never fight the user's
  // own navigation afterwards.
  useEffect(() => {
    if (!ready || autoSelected) return
    setAutoSelected(true)
    if (instances.length > 0) setSelection({ kind: 'instance', id: instances[0].id })
  }, [ready, autoSelected, instances])

  // Drop selections for instances that no longer exist.
  useEffect(() => {
    setChecked((current) => {
      const alive = new Set(instances.map((instance) => instance.id))
      const next = new Set([...current].filter((id) => alive.has(id)))
      return next.size === current.size ? current : next
    })
  }, [instances])

  const toggleChecked = useCallback((id: string) => {
    setChecked((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const checkedInstances = useMemo(
    () => instances.filter((instance) => checked.has(instance.id)),
    [instances, checked],
  )

  const selectedInstance =
    selection.kind === 'instance' ? instances.find((item) => item.id === selection.id) : undefined
  const selectedReplicaSet =
    selection.kind === 'replicaSet'
      ? replicaSets.find((item) => item.name === selection.name)
      : undefined

  if (!ready) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-xs text-muted">
        <Spinner /> Starting MongoCtl…
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-12 shrink-0 items-center gap-3 border-b border-line bg-surface px-4">
        <div className="flex items-center gap-2 text-accent">
          <DatabaseIcon size={17} />
          <span className="text-sm font-semibold text-ink">MongoCtl</span>
        </div>

        {system?.mongod.version ? (
          <Badge tone="neutral">mongod {system.mongod.version}</Badge>
        ) : (
          <Badge tone="danger">mongod not found</Badge>
        )}

        <div className="ml-auto flex items-center gap-1">
          <IconButton
            label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
            onClick={toggle}
          >
            {theme === 'dark' ? <SunIcon size={15} /> : <MoonIcon size={15} />}
          </IconButton>
          <IconButton
            label="Settings"
            onClick={() => setSelection({ kind: 'settings' })}
            className={cn(selection.kind === 'settings' && 'bg-sunken text-ink')}
          >
            <SettingsIcon size={15} />
          </IconButton>
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <Sidebar
          instances={instances}
          replicaSets={replicaSets}
          selection={selection}
          onSelect={setSelection}
          checked={checked}
          onToggleChecked={toggleChecked}
          onCreate={() => setCreating(true)}
          onGroup={() => setGrouping(true)}
        />

        <main className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg">
          {selection.kind === 'settings' && <SettingsPanel />}

          {selection.kind === 'instance' &&
            (selectedInstance ? (
              <InstanceDetail key={selectedInstance.id} instance={selectedInstance} />
            ) : (
              <EmptyState title="That instance is gone" description="Pick another one from the list." />
            ))}

          {selection.kind === 'replicaSet' &&
            (selectedReplicaSet ? (
              <ReplicaSetDetail key={selectedReplicaSet.name} replicaSet={selectedReplicaSet} />
            ) : (
              <EmptyState title="That replica set is gone" />
            ))}

          {instances.length === 0 && selection.kind !== 'settings' && (
            <EmptyState
              icon={<ServerIcon size={28} />}
              title="No instances yet"
              description="Create a local MongoDB server to start, stop and monitor it from here."
              action={
                <Button variant="primary" onClick={() => setCreating(true)}>
                  Create the first instance
                </Button>
              }
            />
          )}
        </main>
      </div>

      <CreateInstanceDialog
        open={creating}
        onClose={() => setCreating(false)}
        onCreated={(id) => setSelection({ kind: 'instance', id })}
      />

      <GroupDialog
        open={grouping}
        onClose={() => setGrouping(false)}
        selected={checkedInstances}
        onGrouped={(name) => {
          setChecked(new Set())
          setSelection({ kind: 'replicaSet', name })
        }}
      />

      {shuttingDown && (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-bg/90 backdrop-blur-sm">
          <div className="flex max-w-xs flex-col items-center gap-3 text-center text-accent">
            <Spinner size={22} />
            <div>
              <p className="text-sm font-medium text-ink">Shutting down</p>
              <p className="mt-1 text-xs text-muted">
                Stopping the MongoDB servers cleanly, so the next start does not have to run
                recovery.
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
