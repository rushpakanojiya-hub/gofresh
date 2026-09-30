import { useEffect, useState } from 'react'
import { getAuditLogs } from '../api/admin'
import type { AuditLog, AuditLogsResponse } from '../types/admin'

export default function AuditLogs() {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [action, setAction] = useState('')
  const [entityType, setEntityType] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  function load() {
    setIsLoading(true)
    setError(null)
    getAuditLogs({
      page,
      limit: 50,
      action: action || undefined,
      entity_type: entityType || undefined,
    })
      .then((res: AuditLogsResponse) => {
        setLogs(res.logs ?? [])
        setTotalPages(res.total_pages || 1)
        setTotal(res.total || 0)
      })
      .catch((err) => setError(err.response?.data?.error ?? 'Could not load audit logs.'))
      .finally(() => setIsLoading(false))
  }

  useEffect(() => {
    load()
  }, [page, action, entityType])

  function handleFilterChange(setter: (v: string) => void, value: string) {
    setter(value)
    setPage(1)
  }

  return (
    <div className="p-8">
      <div className="mb-6">
        <h1 className="text-lg font-semibold">Audit Logs</h1>
        <p className="text-sm text-slate-500">Every sensitive admin action, in order, with who did it and when.</p>
      </div>

      <div className="flex items-center gap-3 mb-4 text-sm">
        <label className="flex items-center gap-2">
          <span className="text-slate-500">Action</span>
          <input
            value={action}
            onChange={(e) => handleFilterChange(setAction, e.target.value)}
            placeholder="e.g. update_order_status"
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 w-56"
          />
        </label>
        <label className="flex items-center gap-2">
          <span className="text-slate-500">Entity type</span>
          <input
            value={entityType}
            onChange={(e) => handleFilterChange(setEntityType, e.target.value)}
            placeholder="e.g. order, product"
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 w-48"
          />
        </label>
        {(action || entityType) && (
          <button
            onClick={() => {
              setAction('')
              setEntityType('')
              setPage(1)
            }}
            className="text-xs text-slate-500 hover:text-slate-300"
          >
            Clear filters
          </button>
        )}
      </div>

      {isLoading && <p className="text-sm text-slate-500">Loading audit logs...</p>}
      {!isLoading && error && (
        <div className="border border-slate-800 rounded-xl p-6 text-sm text-slate-500">{error}</div>
      )}

      {!isLoading && !error && (
        <>
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-2 font-medium">Time</th>
                  <th className="px-4 py-2 font-medium">Admin</th>
                  <th className="px-4 py-2 font-medium">Action</th>
                  <th className="px-4 py-2 font-medium">Entity</th>
                  <th className="px-4 py-2 font-medium">Details</th>
                </tr>
              </thead>
              <tbody>
                {logs.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-4 py-6 text-center text-slate-500">
                      No audit log entries match this filter.
                    </td>
                  </tr>
                )}
                {logs.map((log) => (
                  <tr key={log.id} className="border-t border-slate-800 align-top">
                    <td className="px-4 py-2 text-slate-400 whitespace-nowrap">
                      {log.created_at.slice(0, 19).replace('T', ' ')}
                    </td>
                    <td className="px-4 py-2 text-slate-400">{log.admin_phone}</td>
                    <td className="px-4 py-2 font-mono text-xs">{log.action}</td>
                    <td className="px-4 py-2 text-slate-400">
                      {log.entity_type}
                      {log.entity_id && <span className="text-slate-600"> #{log.entity_id}</span>}
                    </td>
                    <td className="px-4 py-2 text-slate-400">{log.details || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between mt-4 text-sm text-slate-500">
            <span>
              {total} total entries · page {page} of {totalPages}
            </span>
            <div className="flex gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
                className="px-3 py-1.5 border border-slate-800 rounded-lg disabled:opacity-40"
              >
                Previous
              </button>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
                className="px-3 py-1.5 border border-slate-800 rounded-lg disabled:opacity-40"
              >
                Next
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
