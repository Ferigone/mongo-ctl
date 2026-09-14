import { useEffect, useState } from 'react'

import { FolderIcon, PlayIcon, RestartIcon, ShieldIcon, StopIcon } from '@/components/icons'
import {
  Badge,
  Button,
  Callout,
  Card,
  Field,
  Input,
  Modal,
  SectionTitle,
  Select,
  Textarea,
  useToast,
} from '@/components/ui'
import { BrowserOpenURL } from '@wails/runtime/runtime'

import { errorMessage, system as systemApi } from '@/lib/api'
import { serviceLabel, serviceTone } from '@/lib/status'
import type { MongodBinary, Settings } from '@/lib/types'
import { useAppState } from '@/state/AppState'

export function SettingsPanel() {
  const { system, refreshSystem } = useAppState()
  const toast = useToast()

  const [draft, setDraft] = useState<Settings | null>(null)
  const [binaries, setBinaries] = useState<MongodBinary[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (system) setDraft(system.settings)
  }, [system])

  useEffect(() => {
    void (async () => {
      try {
        setBinaries(await systemApi.availableBinaries())
      } catch {
        setBinaries([])
      }
    })()
  }, [])

  if (!system || !draft) return null

  const dirty = JSON.stringify(draft) !== JSON.stringify(system.settings)

  const save = async () => {
    setSaving(true)
    try {
      await systemApi.updateSettings(draft)
      await refreshSystem()
      toast.success('Settings saved')
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  const pickDataRoot = async () => {
    try {
      const directory = await systemApi.chooseDirectory('Choose the default data directory')
      if (directory) setDraft({ ...draft, dataRoot: directory })
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto max-w-3xl space-y-4 p-6">
        <header>
          <h1 className="text-base font-semibold text-ink">Settings</h1>
          <p className="mt-0.5 text-xs text-muted">
            Defaults for new instances and control over the MongoDB service installed on this
            machine.
          </p>
        </header>

        {!system.mongod.path && (
          <Callout tone="danger" title="No MongoDB server found">
            <p>
              MongoCtl starts and monitors MongoDB servers but does not ship one. Install MongoDB
              Community Server and it is picked up automatically — no configuration needed.
            </p>
            <Button
              size="sm"
              className="mt-2"
              onClick={() => BrowserOpenURL('https://www.mongodb.com/try/download/community')}
            >
              Open the download page
            </Button>
          </Callout>
        )}

        <Card className="space-y-4 px-4 py-4">
          <SectionTitle
            title="New instance defaults"
            description="Applied when an instance is created without an explicit location or port."
          />

          <Field label="Data directory" hint="Each instance gets its own folder inside this one.">
            <div className="flex gap-2">
              <Input
                value={draft.dataRoot}
                onChange={(event) => setDraft({ ...draft, dataRoot: event.target.value })}
              />
              <Button icon={<FolderIcon size={13} />} onClick={() => void pickDataRoot()}>
                Browse
              </Button>
            </div>
          </Field>

          <Field
            label="First port"
            hint="Ports are allocated upward from here, skipping any already in use."
          >
            <Input
              type="number"
              min={1024}
              max={65535}
              className="max-w-36"
              value={draft.basePort}
              onChange={(event) => setDraft({ ...draft, basePort: Number(event.target.value) })}
            />
          </Field>

          <Field
            label="mongod executable"
            hint={
              system.mongod.path
                ? `In use: MongoDB ${system.mongod.version}`
                : 'No mongod found; instances cannot start until one is selected.'
            }
          >
            <Select
              value={draft.mongodPath}
              onChange={(event) => setDraft({ ...draft, mongodPath: event.target.value })}
            >
              <option value="">Newest installation found automatically</option>
              {binaries.map((binary) => (
                <option key={binary.path} value={binary.path}>
                  MongoDB {binary.version} — {binary.path}
                </option>
              ))}
            </Select>
          </Field>

          <div className="flex justify-end gap-2 border-t border-line pt-3">
            <Button onClick={() => setDraft(system.settings)} disabled={!dirty}>
              Revert
            </Button>
            <Button
              variant="primary"
              onClick={() => void save()}
              loading={saving}
              disabled={!dirty}
            >
              Save settings
            </Button>
          </div>
        </Card>

        <WindowsServicePanel />
      </div>
    </div>
  )
}

function WindowsServicePanel() {
  const { system, refreshSystem } = useAppState()
  const toast = useToast()
  const [busy, setBusy] = useState<string | null>(null)
  const [editing, setEditing] = useState(false)

  if (!system) return null

  const service = system.service

  const run = async (action: string, operation: () => Promise<void>, message: string) => {
    setBusy(action)
    try {
      await operation()
      await refreshSystem()
      toast.success(message)
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(null)
    }
  }

  return (
    <Card className="space-y-4 px-4 py-4">
      <SectionTitle
        title="Windows MongoDB service"
        description="The server the MongoDB installer registered, separate from the instances this app creates."
        action={
          service && <Badge tone={serviceTone(service.state)}>{serviceLabel(service.state)}</Badge>
        }
      />

      {!service ? (
        <Callout tone="neutral">
          <span data-selectable>{system.serviceError || 'No MongoDB service is registered.'}</span>
        </Callout>
      ) : (
        <>
          <dl className="grid gap-2 text-xs">
            <div className="flex gap-2">
              <dt className="w-24 shrink-0 text-subtle">Display name</dt>
              <dd className="text-ink">{service.displayName}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="w-24 shrink-0 text-subtle">Command</dt>
              <dd className="min-w-0 break-all font-mono text-[11px] text-muted" data-selectable>
                {service.binaryPath}
              </dd>
            </div>
          </dl>

          {!system.elevated && (
            <Callout tone="info" title="Administrator rights required">
              Starting, stopping or reconfiguring a Windows service needs elevation, so each of these
              actions raises a UAC prompt. The instances this app creates run as ordinary processes
              and never prompt.
            </Callout>
          )}

          <div className="flex flex-wrap items-center gap-2 border-t border-line pt-3">
            <Button
              size="sm"
              variant="primary"
              icon={<PlayIcon size={12} />}
              disabled={service.state === 'running'}
              loading={busy === 'start'}
              onClick={() => void run('start', systemApi.serviceStart, 'Service started')}
            >
              Start
            </Button>
            <Button
              size="sm"
              icon={<StopIcon size={12} />}
              disabled={service.state !== 'running'}
              loading={busy === 'stop'}
              onClick={() => void run('stop', systemApi.serviceStop, 'Service stopped')}
            >
              Stop
            </Button>
            <Button
              size="sm"
              icon={<RestartIcon size={12} />}
              loading={busy === 'restart'}
              onClick={() => void run('restart', systemApi.serviceRestart, 'Service restarted')}
            >
              Restart
            </Button>

            <div className="ml-auto flex items-center gap-2">
              <span className="text-[11px] text-subtle">Start at boot</span>
              <Select
                className="h-7 w-auto text-xs"
                value={service.startType}
                disabled={busy !== null}
                onChange={(event) =>
                  void run(
                    'startType',
                    () => systemApi.serviceSetStartType(event.target.value),
                    'Start type updated',
                  )
                }
              >
                <option value="automatic">Automatic</option>
                <option value="manual">Manual</option>
                <option value="disabled">Disabled</option>
                {service.startType === 'unknown' && <option value="unknown">Unknown</option>}
              </Select>
            </div>
          </div>

          <div className="flex items-center justify-between border-t border-line pt-3">
            <p className="flex items-center gap-1.5 text-[11px] text-subtle">
              <ShieldIcon size={12} />
              Editing this configuration writes to Program Files and needs elevation.
            </p>
            <Button size="sm" onClick={() => setEditing(true)}>
              Edit configuration
            </Button>
          </div>
        </>
      )}

      <ServiceConfigDialog open={editing} onClose={() => setEditing(false)} />
    </Card>
  )
}

function ServiceConfigDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const toast = useToast()
  const [content, setContent] = useState('')
  const [original, setOriginal] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return

    void (async () => {
      try {
        const current = await systemApi.serviceReadConfig()
        setContent(current)
        setOriginal(current)
      } catch (error) {
        toast.error(errorMessage(error))
        onClose()
      }
    })()
  }, [open, toast, onClose])

  const save = async () => {
    setSaving(true)
    try {
      await systemApi.serviceWriteConfig(content)
      toast.success('Service configuration saved. Restart the service to apply it.')
      onClose()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Windows service configuration"
      description="A backup of the previous file is kept beside it."
      width="max-w-2xl"
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            onClick={() => void save()}
            loading={saving}
            disabled={content === original}
          >
            Save with elevation
          </Button>
        </>
      }
    >
      <Textarea
        className="min-h-[340px] resize-y"
        spellCheck={false}
        value={content}
        onChange={(event) => setContent(event.target.value)}
      />
    </Modal>
  )
}
