/**
 * Typed facade over the generated Wails bindings.
 *
 * Components import from here rather than from wailsjs directly, so the
 * generated `any` types stay contained to this one module.
 */

import * as InstanceBinding from '@wails/go/app/InstanceService'
import * as ClusterBinding from '@wails/go/app/ClusterService'
import * as SystemBinding from '@wails/go/app/SystemService'
import { EventsOn } from '@wails/runtime/runtime'

import type {
  ClusterProgress,
  CreateInstanceRequest,
  DatabaseInfo,
  GroupRequest,
  Instance,
  LogEvent,
  LogLine,
  MetricsEvent,
  MongodBinary,
  PreflightReport,
  ReplicaSet,
  Sample,
  Settings,
  StatusEvent,
  SystemInfo,
} from './types'

export const instances = {
  list: () => InstanceBinding.List() as Promise<Instance[]>,
  create: (request: CreateInstanceRequest) =>
    InstanceBinding.Create(request as never) as Promise<Instance>,
  remove: (id: string, deleteData: boolean) => InstanceBinding.Remove(id, deleteData),
  start: (id: string) => InstanceBinding.Start(id),
  stop: (id: string) => InstanceBinding.Stop(id),
  restart: (id: string) => InstanceBinding.Restart(id),
  logs: (id: string) => InstanceBinding.Logs(id) as Promise<LogLine[]>,
  metricsHistory: (id: string) => InstanceBinding.MetricsHistory(id) as Promise<Sample[]>,
  databases: (id: string) => InstanceBinding.Databases(id) as Promise<DatabaseInfo[]>,
  readConfig: (id: string) => InstanceBinding.ReadConfig(id),
  writeConfig: (id: string, content: string) => InstanceBinding.WriteConfig(id, content),
  moveDataDir: (id: string, target: string) => InstanceBinding.MoveDataDir(id, target),
  suggestPort: () => InstanceBinding.SuggestPort(),
}

export const clusters = {
  list: () => ClusterBinding.List() as Promise<ReplicaSet[]>,
  preflight: (instanceIds: string[]) =>
    ClusterBinding.Preflight(instanceIds) as Promise<PreflightReport>,
  group: (request: GroupRequest) => ClusterBinding.Group(request as never),
  dissolve: (name: string) => ClusterBinding.Dissolve(name),
}

export const system = {
  info: () => SystemBinding.Info() as Promise<SystemInfo>,
  availableBinaries: () => SystemBinding.AvailableBinaries() as Promise<MongodBinary[]>,
  updateSettings: (settings: Settings) =>
    SystemBinding.UpdateSettings(settings as never) as Promise<Settings>,
  chooseDirectory: (title: string) => SystemBinding.ChooseDirectory(title),
  serviceStart: () => SystemBinding.ServiceStart(),
  serviceStop: () => SystemBinding.ServiceStop(),
  serviceRestart: () => SystemBinding.ServiceRestart(),
  serviceSetStartType: (startType: string) => SystemBinding.ServiceSetStartType(startType),
  serviceReadConfig: () => SystemBinding.ServiceReadConfig(),
  serviceWriteConfig: (content: string) => SystemBinding.ServiceWriteConfig(content),
}

export const events = {
  onStatus: (handler: (event: StatusEvent) => void) =>
    EventsOn('instance:status', handler as (...data: unknown[]) => void),
  onLog: (handler: (event: LogEvent) => void) =>
    EventsOn('instance:log', handler as (...data: unknown[]) => void),
  onMetrics: (handler: (event: MetricsEvent) => void) =>
    EventsOn('instance:metrics', handler as (...data: unknown[]) => void),
  onClusterProgress: (handler: (event: ClusterProgress) => void) =>
    EventsOn('cluster:progress', handler as (...data: unknown[]) => void),
  onShutdown: (handler: () => void) =>
    EventsOn('app:shutdown', handler as (...data: unknown[]) => void),
}

/** Turns a Go error into the message the UI shows. */
export function errorMessage(error: unknown): string {
  if (typeof error === 'string') return error
  if (error instanceof Error) return error.message
  return String(error)
}
