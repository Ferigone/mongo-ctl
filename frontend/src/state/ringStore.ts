import { useCallback, useSyncExternalStore } from 'react'

type Listener = () => void

const EMPTY: readonly never[] = []

/**
 * A per-key bounded series with subscriptions.
 *
 * Metrics and logs arrive once per second per running instance. Holding them in
 * React state would re-render the whole tree on every tick, so they live here
 * and only the components that read a given key re-render.
 */
export class RingStore<T> {
  private readonly series = new Map<string, T[]>()
  private readonly listeners = new Map<string, Set<Listener>>()

  constructor(private readonly capacity: number) {}

  snapshot(key: string): readonly T[] {
    return this.series.get(key) ?? (EMPTY as readonly T[])
  }

  append(key: string, item: T): void {
    const current = this.series.get(key) ?? []
    const next = current.length >= this.capacity ? current.slice(current.length - this.capacity + 1) : current.slice()

    next.push(item)
    this.series.set(key, next)
    this.notify(key)
  }

  replace(key: string, items: T[]): void {
    const trimmed = items.length > this.capacity ? items.slice(items.length - this.capacity) : items
    this.series.set(key, trimmed)
    this.notify(key)
  }

  clear(key: string): void {
    if (!this.series.delete(key)) return
    this.notify(key)
  }

  subscribe(key: string, listener: Listener): () => void {
    let group = this.listeners.get(key)
    if (!group) {
      group = new Set()
      this.listeners.set(key, group)
    }
    group.add(listener)

    return () => {
      group.delete(listener)
      if (group.size === 0) this.listeners.delete(key)
    }
  }

  private notify(key: string): void {
    this.listeners.get(key)?.forEach((listener) => listener())
  }
}

/** Subscribes a component to one key of a RingStore. */
export function useSeries<T>(store: RingStore<T>, key: string | null): readonly T[] {
  const subscribe = useCallback(
    (listener: Listener) => (key ? store.subscribe(key, listener) : () => {}),
    [store, key],
  )
  const snapshot = useCallback(() => (key ? store.snapshot(key) : (EMPTY as readonly T[])), [store, key])

  return useSyncExternalStore(subscribe, snapshot)
}
