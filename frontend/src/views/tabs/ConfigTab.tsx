import { useCallback, useEffect, useState } from 'react'

import { FolderIcon } from '@/components/icons'
import { Button, Callout, Card, SectionTitle, Spinner, Textarea, useToast } from '@/components/ui'
import { errorMessage, instances as instanceApi, system as systemApi } from '@/lib/api'
import type { Instance } from '@/lib/types'
import { useAppState } from '@/state/AppState'

export function ConfigTab({ instance }: { instance: Instance }) {
  const toast = useToast()
  const { refreshInstances } = useAppState()

  const [original, setOriginal] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)
  const [moving, setMoving] = useState(false)

  const load = useCallback(async () => {
    try {
      const content = await instanceApi.readConfig(instance.id)
      setOriginal(content)
      setDraft(content)
    } catch (error) {
      toast.error(errorMessage(error))
      setOriginal('')
      setDraft('')
    }
  }, [instance.id, toast])

  useEffect(() => {
    void load()
  }, [load])

  const dirty = original !== null && draft !== original

  const save = async () => {
    setSaving(true)
    try {
      await instanceApi.writeConfig(instance.id, draft)
      setOriginal(draft)
      await refreshInstances()
      toast.success('Configuration saved. Restart the server to apply it.')
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  const moveDataDirectory = async () => {
    try {
      const target = await systemApi.chooseDirectory('Choose a new data directory')
      if (!target) return

      setMoving(true)
      await instanceApi.moveDataDir(instance.id, target)
      await Promise.all([refreshInstances(), load()])
      toast.success('Data directory moved')
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setMoving(false)
    }
  }

  if (original === null) {
    return (
      <div className="flex items-center justify-center gap-2 py-16 text-xs text-muted">
        <Spinner /> Reading configuration…
      </div>
    )
  }

  return (
    <div className="space-y-4 p-5">
      <Card className="px-4 py-3.5">
        <SectionTitle
          title="Data directory"
          description="Where this server stores its database files."
          action={
            <Button
              size="sm"
              icon={<FolderIcon size={12} />}
              onClick={() => void moveDataDirectory()}
              loading={moving}
              disabled={instance.state !== 'stopped'}
            >
              Move…
            </Button>
          }
        />
        <p className="mt-2 truncate font-mono text-[11px] text-muted" data-selectable title={instance.dataDir}>
          {instance.dataDir}
        </p>
        {instance.state !== 'stopped' && (
          <p className="mt-2 text-[11px] text-subtle">Stop the server to move its data files.</p>
        )}
      </Card>

      <Card className="flex flex-col px-4 py-3.5">
        <SectionTitle
          title="mongod.conf"
          description="The full server configuration. Anything mongod accepts can be set here."
          action={
            <div className="flex items-center gap-2">
              <Button size="sm" onClick={() => setDraft(original)} disabled={!dirty}>
                Revert
              </Button>
              <Button size="sm" variant="primary" onClick={() => void save()} loading={saving} disabled={!dirty}>
                Save
              </Button>
            </div>
          }
        />

        <Textarea
          className="mt-3 min-h-[320px] resize-y"
          spellCheck={false}
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
        />

        {dirty && (
          <div className="mt-3">
            <Callout tone="warning" title="Unsaved changes">
              Saving writes the file. mongod reads it only at startup, so restart the server for the
              change to take effect.
            </Callout>
          </div>
        )}
      </Card>
    </div>
  )
}
