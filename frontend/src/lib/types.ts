/**
 * Domain types mirroring the Go JSON payloads.
 *
 * The generated bindings type every time.Time as `any`; these interfaces pin
 * them to the ISO strings that actually arrive, so components get real types.
 */

export type InstanceState = 'stopped' | 'starting' | 'running' | 'stopping' | 'failed'

export type LogSeverity = 'fatal' | 'error' | 'warning' | 'info' | 'debug'

export type ServiceState =
  | 'running'
  | 'stopped'
  | 'start_pending'
  | 'stop_pending'
  | 'paused'
  | 'unknown'

export type ServiceStartType = 'automatic' | 'manual' | 'disabled' | 'unknown'

export interface Instance {
  id: string
  name: string
  port: number
  dataDir: string
  configPath: string
  replicaSet: string
  createdAt: string
  state: InstanceState
  pid: number
  startedAt: string
  error: string
  uncleanStop: boolean
}

export interface CreateInstanceRequest {
  name: string
  port: number
  dataDir: string
}

export interface Sample {
  timestamp: string
  connectionsCurrent: number
  connectionsActive: number
  connectionsAvailable: number
  memResidentMb: number
  memVirtualMb: number
  cacheUsedBytes: number
  cacheMaxBytes: number
  cacheFillPercent: number
  queueReaders: number
  queueWriters: number
  insertsPerSec: number
  queriesPerSec: number
  updatesPerSec: number
  deletesPerSec: number
  commandsPerSec: number
  getMoresPerSec: number
  bytesInPerSec: number
  bytesOutPerSec: number
  readLatencyMs: number
  writeLatencyMs: number
  commandLatencyMs: number
  gap: boolean
}

export interface LogLine {
  timestamp: string
  severity: LogSeverity
  component: string
  message: string
  raw: string
}

export interface DatabaseInfo {
  name: string
  sizeOnDisk: number
  empty: boolean
  collections: number
  objects: number
  dataSize: number
  storageSize: number
  indexSize: number
  avgObjSize: number
}

export interface ReplicaSetMember {
  id: number
  host: string
  stateStr: string
  health: number
  uptimeSecs: number
  optimeDate: string
  lagSeconds: number
  isPrimary: boolean
  self: boolean
}

export interface ReplicaSetStatus {
  name: string
  members: ReplicaSetMember[]
}

export interface ReplicaSet {
  name: string
  memberIds: string[]
  createdAt: string
  status?: ReplicaSetStatus
  statusError: string
}

export interface PreflightReport {
  seedName: string
  requiresRestart: string[]
  populatedNonSeed: string[]
  evenMemberCount: boolean
}

export interface GroupRequest {
  name: string
  instanceIds: string[]
  clearNonSeedData: boolean
}

export interface ClusterProgress {
  step: string
  message: string
  index: number
  total: number
  done: boolean
  error: string
}

export interface Settings {
  dataRoot: string
  mongodPath: string
  basePort: number
}

export interface MongodBinary {
  path: string
  version: string
  major: number
  minor: number
  patch: number
}

export interface ServiceInfo {
  name: string
  displayName: string
  state: ServiceState
  startType: ServiceStartType
  binaryPath: string
}

export interface SystemInfo {
  mongod: MongodBinary
  settings: Settings
  elevated: boolean
  service?: ServiceInfo
  serviceError: string
}

/** Payload of the `instance:status` event. */
export interface StatusEvent {
  instanceId: string
  state: InstanceState
  pid: number
  port: number
  startedAt: string
  error: string
  uncleanStop: boolean
}

/** Payload of the `instance:log` event. */
export interface LogEvent {
  instanceId: string
  line: LogLine
}

/** Payload of the `instance:metrics` event. */
export interface MetricsEvent {
  instanceId: string
  sample: Sample
}
