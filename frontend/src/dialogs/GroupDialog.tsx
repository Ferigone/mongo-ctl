import { useEffect, useState } from 'react'

import { CheckIcon } from '@/components/icons'
import {
  Button,
  Callout,
  Checkbox,
  Field,
  Input,
  Modal,
  Spinner,
  StatusDot,
  useToast,
  cn,
} from '@/components/ui'
import { clusters, errorMessage, events } from '@/lib/api'
import type { ClusterProgress, Instance, PreflightReport } from '@/lib/types'
import { useAppState } from '@/state/AppState'

export function GroupDialog({
  open,
  onClose,
  selected,
  onGrouped,
}: {
  open: boolean
  onClose: () => void
  selected: Instance[]
  onGrouped: (name: string) => void
}) {
  const toast = useToast()
  const { refreshClusters, refreshInstances } = useAppState()

  const [name, setName] = useState('')
  const [clearData, setClearData] = useState(true)
  const [report, setReport] = useState<PreflightReport | null>(null)
  const [progress, setProgress] = useState<ClusterProgress | null>(null)
  const [working, setWorking] = useState(false)

  useEffect(() => {
    if (!open) return

    setName('rs-local')
    setClearData(true)
    setProgress(null)
    setReport(null)

    void (async () => {
      try {
        setReport(await clusters.preflight(selected.map((instance) => instance.id)))
      } catch (error) {
        toast.error(errorMessage(error))
      }
    })()
  }, [open, selected, toast])

  useEffect(() => {
    if (!open) return
    return events.onClusterProgress(setProgress)
  }, [open])

  const group = async () => {
    setWorking(true)
    try {
      await clusters.group({
        name: name.trim(),
        instanceIds: selected.map((instance) => instance.id),
        clearNonSeedData: clearData,
      })
      await Promise.all([refreshClusters(), refreshInstances()])
      toast.success(`Replica set ${name.trim()} is ready`)
      onGrouped(name.trim())
      onClose()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setWorking(false)
    }
  }

  const seed = selected[0]

  return (
    <Modal
      open={open}
      onClose={working ? () => {} : onClose}
      title="Create a replica set"
      description={`${selected.length} instances will become members.`}
      footer={
        <>
          <Button onClick={onClose} disabled={working}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={() => void group()}
            loading={working}
            disabled={name.trim().length === 0}
          >
            Create replica set
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label="Replica set name">
          <Input
            autoFocus
            value={name}
            disabled={working}
            onChange={(event) => setName(event.target.value)}
          />
        </Field>

        <div>
          <p className="mb-1.5 text-xs font-medium text-ink">Members</p>
          <ul className="space-y-1 rounded-lg border border-line bg-sunken px-3 py-2">
            {selected.map((instance, index) => (
              <li key={instance.id} className="flex items-center gap-2 text-xs">
                <StatusDot tone={index === 0 ? 'accent' : 'neutral'} />
                <span className="text-ink">{instance.name}</span>
                <span className="text-subtle tabular">:{instance.port}</span>
                {index === 0 && <span className="ml-auto text-[11px] text-accent">seed</span>}
              </li>
            ))}
          </ul>
        </div>

        <Callout tone="warning" title="Every member restarts">
          mongod reads the replica set name only at startup, so each selected instance is stopped,
          reconfigured and started again. This is unavoidable, not a choice this app makes.
        </Callout>

        {report?.evenMemberCount && (
          <Callout tone="warning" title="Even number of members">
            With {selected.length} members a single failure leaves no majority, so no primary can be
            elected. An odd number — three is the usual choice — avoids that.
          </Callout>
        )}

        {report && report.populatedNonSeed.length > 0 && (
          <>
            <Callout tone="danger" title="Members already hold data">
              {report.populatedNonSeed.join(', ')} already contain databases. A new member must start
              empty so it can copy the data from {seed?.name ?? 'the seed'} during initial sync.
            </Callout>
            <Checkbox
              checked={clearData}
              onChange={setClearData}
              label="Erase the data of every member except the seed"
              description="Required for the set to form. Only the seed's data is kept."
            />
          </>
        )}

        {!report && (
          <p className="flex items-center gap-2 text-xs text-muted">
            <Spinner /> Checking the selected instances…
          </p>
        )}

        {progress && <ProgressSteps progress={progress} />}
      </div>
    </Modal>
  )
}

const STEPS = [
  { id: 'stop', label: 'Stop members' },
  { id: 'configure', label: 'Enable replication' },
  { id: 'start', label: 'Restart members' },
  { id: 'initiate', label: 'Initiate the set' },
  { id: 'elect', label: 'Elect a primary' },
]

function ProgressSteps({ progress }: { progress: ClusterProgress }) {
  const activeIndex = STEPS.findIndex((step) => step.id === progress.step)
  const failed = Boolean(progress.error)

  return (
    <div className="rounded-lg border border-line bg-sunken px-3 py-2.5">
      <ol className="space-y-1.5">
        {STEPS.map((step, index) => {
          const done = progress.done || index < activeIndex
          const active = index === activeIndex && !progress.done

          return (
            <li key={step.id} className="flex items-center gap-2 text-xs">
              <span
                className={cn(
                  'flex h-4 w-4 items-center justify-center rounded-full',
                  done && 'bg-accent text-accent-contrast',
                  active && !failed && 'bg-info-soft text-info',
                  active && failed && 'bg-danger-soft text-danger',
                  !done && !active && 'border border-line-strong',
                )}
              >
                {done ? <CheckIcon size={10} /> : active && !failed ? <Spinner size={10} /> : null}
              </span>
              <span className={done || active ? 'text-ink' : 'text-subtle'}>{step.label}</span>
            </li>
          )
        })}
      </ol>

      {failed && <p className="mt-2 text-[11px] text-danger" data-selectable>{progress.error}</p>}
    </div>
  )
}
