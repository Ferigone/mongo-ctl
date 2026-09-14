import type { Tone } from '@/components/ui'
import type { InstanceState, ServiceState } from './types'

const instanceTones: Record<InstanceState, Tone> = {
  running: 'success',
  starting: 'info',
  stopping: 'warning',
  stopped: 'neutral',
  failed: 'danger',
}

const instanceLabels: Record<InstanceState, string> = {
  running: 'Running',
  starting: 'Starting',
  stopping: 'Stopping',
  stopped: 'Stopped',
  failed: 'Failed',
}

export function instanceTone(state: InstanceState): Tone {
  return instanceTones[state] ?? 'neutral'
}

export function instanceLabel(state: InstanceState): string {
  return instanceLabels[state] ?? state
}

/** True while the server is mid-transition and controls should stay disabled. */
export function isTransitional(state: InstanceState): boolean {
  return state === 'starting' || state === 'stopping'
}

const serviceTones: Record<ServiceState, Tone> = {
  running: 'success',
  stopped: 'neutral',
  start_pending: 'info',
  stop_pending: 'warning',
  paused: 'warning',
  unknown: 'neutral',
}

const serviceLabels: Record<ServiceState, string> = {
  running: 'Running',
  stopped: 'Stopped',
  start_pending: 'Starting',
  stop_pending: 'Stopping',
  paused: 'Paused',
  unknown: 'Unknown',
}

export function serviceTone(state: ServiceState): Tone {
  return serviceTones[state] ?? 'neutral'
}

export function serviceLabel(state: ServiceState): string {
  return serviceLabels[state] ?? state
}

/** Replica set member states worth colouring differently in the UI. */
export function memberTone(stateStr: string): Tone {
  switch (stateStr) {
    case 'PRIMARY':
      return 'accent'
    case 'SECONDARY':
      return 'success'
    case 'STARTUP':
    case 'STARTUP2':
    case 'RECOVERING':
      return 'info'
    case 'ARBITER':
      return 'neutral'
    default:
      return 'danger'
  }
}
