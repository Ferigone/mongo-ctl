import { useEffect, useState } from 'react'

import { LayersIcon, RestartIcon } from '@/components/icons'
import { Badge, Button, Callout, Card, EmptyState, Modal, SectionTitle, useToast } from '@/components/ui'
import { clusters, errorMessage } from '@/lib/api'
import { formatDuration } from '@/lib/format'
import { memberTone } from '@/lib/status'
import type { ReplicaSet } from '@/lib/types'
import { useAppState } from '@/state/AppState'

const STATUS_REFRESH_MS = 5000

export function ReplicaSetDetail({ replicaSet }: { replicaSet: ReplicaSet }) {
  const toast = useToast()
  const { refreshClusters, refreshInstances } = useAppState()
  const [confirmDissolve, setConfirmDissolve] = useState(false)
  const [dissolving, setDissolving] = useState(false)

  // Election state and replication lag change without any action from the user,
  // so this view polls rather than waiting for an event.
  useEffect(() => {
    const timer = window.setInterval(() => void refreshClusters(), STATUS_REFRESH_MS)
    return () => window.clearInterval(timer)
  }, [refreshClusters])

  const dissolve = async () => {
    setDissolving(true)
    try {
      await clusters.dissolve(replicaSet.name)
      await Promise.all([refreshClusters(), refreshInstances()])
      toast.success(`${replicaSet.name} was dissolved; members run standalone again`)
      setConfirmDissolve(false)
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setDissolving(false)
    }
  }

  const members = replicaSet.status?.members ?? []

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <header className="border-b border-line px-5 pb-4 pt-4">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <LayersIcon size={16} />
              <h1 className="truncate text-base font-semibold text-ink">{replicaSet.name}</h1>
              <Badge tone={replicaSet.status ? 'success' : 'neutral'}>
                {replicaSet.status ? 'Online' : 'Offline'}
              </Badge>
            </div>
            <p className="mt-1 font-mono text-xs text-muted" data-selectable>
              mongodb://{members.map((member) => member.host).join(',') || '…'}/?replicaSet=
              {replicaSet.name}
            </p>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <Button size="sm" icon={<RestartIcon size={12} />} onClick={() => void refreshClusters()}>
              Refresh
            </Button>
            <Button size="sm" variant="danger" onClick={() => setConfirmDissolve(true)}>
              Dissolve
            </Button>
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto p-5">
        {replicaSet.statusError && (
          <div className="mb-4">
            <Callout tone="warning" title="Status unavailable">
              <span data-selectable>{replicaSet.statusError}</span>
            </Callout>
          </div>
        )}

        {members.length === 0 ? (
          <EmptyState
            icon={<LayersIcon size={26} />}
            title="No live member data"
            description="Start at least one member to read the replica set status."
          />
        ) : (
          <Card className="overflow-hidden">
            <div className="border-b border-line px-4 py-3">
              <SectionTitle title="Members" description="Roles are decided by election and can change." />
            </div>
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-line bg-sunken text-left text-subtle">
                  <th className="px-4 py-2 font-medium">Host</th>
                  <th className="px-4 py-2 font-medium">Role</th>
                  <th className="px-4 py-2 font-medium">Health</th>
                  <th className="px-4 py-2 text-right font-medium">Uptime</th>
                  <th className="px-4 py-2 text-right font-medium">Replication lag</th>
                </tr>
              </thead>
              <tbody>
                {members.map((member) => (
                  <tr key={member.id} className="border-b border-line last:border-0">
                    <td className="px-4 py-2.5 font-mono text-ink" data-selectable>
                      {member.host}
                    </td>
                    <td className="px-4 py-2.5">
                      <Badge tone={memberTone(member.stateStr)}>{member.stateStr}</Badge>
                    </td>
                    <td className="px-4 py-2.5">
                      <span className={member.health === 1 ? 'text-success' : 'text-danger'}>
                        {member.health === 1 ? 'Healthy' : 'Unreachable'}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-right text-muted tabular">
                      {formatDuration(member.uptimeSecs)}
                    </td>
                    <td className="px-4 py-2.5 text-right tabular">
                      {member.isPrimary ? (
                        <span className="text-subtle">—</span>
                      ) : (
                        <span className={member.lagSeconds > 10 ? 'text-warning' : 'text-muted'}>
                          {member.lagSeconds <= 0 ? 'in sync' : `${member.lagSeconds.toFixed(1)}s`}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </div>

      <Modal
        open={confirmDissolve}
        onClose={() => setConfirmDissolve(false)}
        title={`Dissolve ${replicaSet.name}?`}
        footer={
          <>
            <Button onClick={() => setConfirmDissolve(false)}>Cancel</Button>
            <Button variant="danger" onClick={() => void dissolve()} loading={dissolving}>
              Dissolve replica set
            </Button>
          </>
        }
      >
        <div className="space-y-3 text-xs text-muted">
          <p>
            Every member is stopped, switched back to standalone operation and started again. The
            data in each data directory is left untouched.
          </p>
          <Callout tone="warning">
            Transactions and change streams stop working once the set is gone, since those require a
            replica set.
          </Callout>
        </div>
      </Modal>
    </div>
  )
}
