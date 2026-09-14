import { useCallback, useEffect, useState } from 'react'

import { DatabaseIcon, RestartIcon } from '@/components/icons'
import { Button, Card, EmptyState, Spinner, useToast } from '@/components/ui'
import { errorMessage, instances as instanceApi } from '@/lib/api'
import { formatBytes, formatCount } from '@/lib/format'
import type { DatabaseInfo, Instance } from '@/lib/types'

export function DatabasesTab({ instance }: { instance: Instance }) {
  const toast = useToast()
  const [databases, setDatabases] = useState<DatabaseInfo[] | null>(null)
  const [loading, setLoading] = useState(false)

  const running = instance.state === 'running'

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setDatabases(await instanceApi.databases(instance.id))
    } catch (error) {
      toast.error(errorMessage(error))
      setDatabases([])
    } finally {
      setLoading(false)
    }
  }, [instance.id, toast])

  useEffect(() => {
    if (running) void load()
    else setDatabases(null)
  }, [running, load])

  if (!running) {
    return (
      <EmptyState
        icon={<DatabaseIcon size={26} />}
        title="Databases are listed while the server runs"
        description="Start this instance to browse the databases it stores."
      />
    )
  }

  if (databases === null) {
    return (
      <div className="flex items-center justify-center gap-2 py-16 text-xs text-muted">
        <Spinner /> Reading databases…
      </div>
    )
  }

  return (
    <div className="space-y-3 p-5">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted">
          {databases.length} {databases.length === 1 ? 'database' : 'databases'}
        </p>
        <Button size="sm" icon={<RestartIcon size={12} />} onClick={() => void load()} loading={loading}>
          Refresh
        </Button>
      </div>

      {databases.length === 0 ? (
        <EmptyState title="No databases yet" description="They appear as soon as data is written." />
      ) : (
        <Card className="overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-line bg-sunken text-left text-subtle">
                <th className="px-4 py-2 font-medium">Database</th>
                <th className="px-4 py-2 text-right font-medium">Collections</th>
                <th className="px-4 py-2 text-right font-medium">Documents</th>
                <th className="px-4 py-2 text-right font-medium">Data</th>
                <th className="px-4 py-2 text-right font-medium">Indexes</th>
                <th className="px-4 py-2 text-right font-medium">On disk</th>
              </tr>
            </thead>
            <tbody>
              {databases.map((database) => (
                <tr key={database.name} className="border-b border-line last:border-0">
                  <td className="px-4 py-2 font-medium text-ink" data-selectable>
                    {database.name}
                  </td>
                  <td className="px-4 py-2 text-right text-muted tabular">
                    {formatCount(database.collections)}
                  </td>
                  <td className="px-4 py-2 text-right text-muted tabular">
                    {formatCount(database.objects)}
                  </td>
                  <td className="px-4 py-2 text-right text-muted tabular">
                    {formatBytes(database.dataSize)}
                  </td>
                  <td className="px-4 py-2 text-right text-muted tabular">
                    {formatBytes(database.indexSize)}
                  </td>
                  <td className="px-4 py-2 text-right text-ink tabular">
                    {formatBytes(database.sizeOnDisk)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  )
}
