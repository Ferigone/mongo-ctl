import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'

import { clusters, events, instances as instanceApi, system as systemApi } from '@/lib/api'
import type { Instance, LogLine, ReplicaSet, Sample, SystemInfo } from '@/lib/types'
import { RingStore } from './ringStore'

/** Matches the backend's retention so a hydrated history is never truncated. */
export const metricsStore = new RingStore<Sample>(900)
export const logStore = new RingStore<LogLine>(2000)

interface AppStateValue {
  instances: Instance[]
  replicaSets: ReplicaSet[]
  system: SystemInfo | null
  ready: boolean
  refreshInstances: () => Promise<void>
  refreshClusters: () => Promise<void>
  refreshSystem: () => Promise<void>
  hydrateInstance: (id: string) => Promise<void>
}

const AppStateContext = createContext<AppStateValue | null>(null)

export function AppStateProvider({ children }: { children: ReactNode }) {
  const [instances, setInstances] = useState<Instance[]>([])
  const [replicaSets, setReplicaSets] = useState<ReplicaSet[]>([])
  const [system, setSystem] = useState<SystemInfo | null>(null)
  const [ready, setReady] = useState(false)

  const refreshInstances = useCallback(async () => {
    setInstances(await instanceApi.list())
  }, [])

  const refreshClusters = useCallback(async () => {
    setReplicaSets(await clusters.list())
  }, [])

  const refreshSystem = useCallback(async () => {
    setSystem(await systemApi.info())
  }, [])

  const hydrateInstance = useCallback(async (id: string) => {
    const [samples, lines] = await Promise.all([
      instanceApi.metricsHistory(id),
      instanceApi.logs(id),
    ])
    metricsStore.replace(id, samples)
    logStore.replace(id, lines)
  }, [])

  useEffect(() => {
    void (async () => {
      await Promise.all([refreshInstances(), refreshClusters(), refreshSystem()])
      setReady(true)
    })()
  }, [refreshInstances, refreshClusters, refreshSystem])

  useEffect(() => {
    const unsubscribeStatus = events.onStatus((event) => {
      setInstances((current) =>
        current.map((instance) =>
          instance.id === event.instanceId
            ? {
                ...instance,
                state: event.state,
                pid: event.pid,
                startedAt: event.startedAt,
                error: event.error,
                uncleanStop: event.uncleanStop,
              }
            : instance,
        ),
      )

      // A stopped server keeps no live series; dropping them stops charts from
      // showing a frozen tail that looks like current data.
      if (event.state === 'stopped' || event.state === 'failed') {
        metricsStore.clear(event.instanceId)
      }
    })

    const unsubscribeMetrics = events.onMetrics((event) => {
      metricsStore.append(event.instanceId, event.sample)
    })

    const unsubscribeLog = events.onLog((event) => {
      logStore.append(event.instanceId, event.line)
    })

    return () => {
      unsubscribeStatus()
      unsubscribeMetrics()
      unsubscribeLog()
    }
  }, [])

  const value = useMemo<AppStateValue>(
    () => ({
      instances,
      replicaSets,
      system,
      ready,
      refreshInstances,
      refreshClusters,
      refreshSystem,
      hydrateInstance,
    }),
    [
      instances,
      replicaSets,
      system,
      ready,
      refreshInstances,
      refreshClusters,
      refreshSystem,
      hydrateInstance,
    ],
  )

  return <AppStateContext.Provider value={value}>{children}</AppStateContext.Provider>
}

export function useAppState(): AppStateValue {
  const context = useContext(AppStateContext)
  if (!context) throw new Error('useAppState must be used inside an AppStateProvider')
  return context
}
