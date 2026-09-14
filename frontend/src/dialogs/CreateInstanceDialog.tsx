import { useEffect, useState } from 'react'

import { FolderIcon } from '@/components/icons'
import { Button, Field, Input, Modal, useToast } from '@/components/ui'
import { errorMessage, instances as instanceApi, system as systemApi } from '@/lib/api'
import { useAppState } from '@/state/AppState'

export function CreateInstanceDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (id: string) => void
}) {
  const toast = useToast()
  const { refreshInstances, system } = useAppState()

  const [name, setName] = useState('')
  const [port, setPort] = useState('')
  const [dataDir, setDataDir] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    if (!open) return

    setName('')
    setDataDir('')
    setPort('')

    void (async () => {
      try {
        setPort(String(await instanceApi.suggestPort()))
      } catch {
        // A blank field still works: the backend allocates a port when it is zero.
      }
    })()
  }, [open])

  const create = async () => {
    setCreating(true)
    try {
      const instance = await instanceApi.create({
        name: name.trim(),
        port: port ? Number(port) : 0,
        dataDir: dataDir.trim(),
      })
      await refreshInstances()
      toast.success(`${instance.name} created on port ${instance.port}`)
      onCreated(instance.id)
      onClose()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setCreating(false)
    }
  }

  const browse = async () => {
    try {
      const directory = await systemApi.chooseDirectory('Choose a data directory for this instance')
      if (directory) setDataDir(directory)
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  const defaultRoot = system?.settings.dataRoot ?? ''

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="New instance"
      description="A standalone mongod with its own data directory and port."
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            onClick={() => void create()}
            loading={creating}
            disabled={name.trim().length === 0}
          >
            Create instance
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label="Name">
          <Input
            autoFocus
            placeholder="orders-db"
            value={name}
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && name.trim()) void create()
            }}
          />
        </Field>

        <Field label="Port" hint="Suggested from the first free port above the configured base.">
          <Input
            type="number"
            min={1024}
            max={65535}
            className="max-w-36"
            value={port}
            onChange={(event) => setPort(event.target.value)}
          />
        </Field>

        <Field
          label="Data directory"
          hint={dataDir ? undefined : `Defaults to a folder named after the instance inside ${defaultRoot}`}
        >
          <div className="flex gap-2">
            <Input
              placeholder="Choose automatically"
              value={dataDir}
              onChange={(event) => setDataDir(event.target.value)}
            />
            <Button icon={<FolderIcon size={13} />} onClick={() => void browse()}>
              Browse
            </Button>
          </div>
        </Field>
      </div>
    </Modal>
  )
}
