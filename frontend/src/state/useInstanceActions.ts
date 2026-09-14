import { useCallback, useState } from 'react'

import { errorMessage, instances as instanceApi } from '@/lib/api'
import { useToast } from '@/components/ui'
import { useAppState } from './AppState'

type Action = 'start' | 'stop' | 'restart' | 'remove'

/**
 * Lifecycle actions with per-instance busy tracking, so a row can disable only
 * its own controls while a long start or stop is in flight.
 */
export function useInstanceActions() {
  const { refreshInstances, refreshClusters, hydrateInstance } = useAppState()
  const toast = useToast()
  const [busy, setBusy] = useState<Record<string, Action | undefined>>({})

  const run = useCallback(
    async (id: string, action: Action, operation: () => Promise<void>, successMessage: string) => {
      setBusy((current) => ({ ...current, [id]: action }))
      try {
        await operation()
        toast.success(successMessage)
        if (action === 'start' || action === 'restart') {
          await hydrateInstance(id)
        }
        await refreshInstances()
        if (action === 'remove') await refreshClusters()
      } catch (error) {
        toast.error(errorMessage(error))
        await refreshInstances()
      } finally {
        setBusy((current) => ({ ...current, [id]: undefined }))
      }
    },
    [toast, refreshInstances, refreshClusters, hydrateInstance],
  )

  const start = useCallback(
    (id: string, name: string) => run(id, 'start', () => instanceApi.start(id), `${name} is running`),
    [run],
  )

  const stop = useCallback(
    (id: string, name: string) => run(id, 'stop', () => instanceApi.stop(id), `${name} stopped`),
    [run],
  )

  const restart = useCallback(
    (id: string, name: string) => run(id, 'restart', () => instanceApi.restart(id), `${name} restarted`),
    [run],
  )

  const remove = useCallback(
    (id: string, name: string, deleteData: boolean) =>
      run(
        id,
        'remove',
        () => instanceApi.remove(id, deleteData),
        deleteData ? `${name} and its data were deleted` : `${name} was removed`,
      ),
    [run],
  )

  return { busy, start, stop, restart, remove }
}
