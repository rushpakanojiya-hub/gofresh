import { useCallback, useEffect, useState } from 'react'
import { getPickingMonitor, reassignPickingTask } from '../api/warehouse'
import type { PickingMonitorData, PickingMonitorTask } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

function fmtMin(m: number): string {
  if (m < 60) return `${Math.round(m)} min`
  return `${Math.floor(m / 60)}h ${Math.round(m % 60)}m`
}

const STATE_TONE: Record<string, string> = {
  idle: 'text-emerald-300',
  busy: 'text-amber-300',
  offline: 'text-slate-500',
}

export default function PickingMonitor() {
  const [data, setData] = useState<PickingMonitorData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [target, setTarget] = useState<PickingMonitorTask | null>(null)
  const [newPicker, setNewPicker] = useState<number | ''>('')
  const [saving, setSaving] = useState(false)
  const [modalError, setModalError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setData(await getPickingMonitor())
      setError(null)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load picking monitor.'))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const interval = setInterval(load, 15000)
    return () => clearInterval(interval)
  }, [load])

  function openReassign(t: PickingMonitorTask) {
    setTarget(t)
    setNewPicker('')
    setModalError(null)
  }

  async function submit() {
    if (!target || newPicker === '') return
    setSaving(true)
    setModalError(null)
    try {
      await reassignPickingTask(target.task_id, newPicker)
      setTarget(null)
      setNewPicker('')
      await load()
    } catch (err) {
      setModalError(getErrorMessage(err, 'Failed to reassign task.'))
    } finally {
      setSaving(false)
    }
  }

  const pickers = data?.pickers ?? []
  const tasks = data?.tasks ?? []
  const candidates = target ? pickers.filter((p) => p.online && p.staff_id !== target.picker_id) : []

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">Live</p>
          <h1 className="font-display text-2xl font-semibold">Picking Monitor</h1>
        </div>
        <button
          onClick={load}
          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}
      {isLoading && <p className="text-sm text-slate-400">Loading...</p>}

      {data && (
        <>
          <p className="text-xs uppercase tracking-wide text-slate-500 mb-2">Pickers</p>
          {pickers.length === 0 ? (
            <p className="text-sm text-slate-500 mb-6">No pickers in this warehouse.</p>
          ) : (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
              {pickers.map((p) => (
                <div key={p.staff_id} className="border border-slate-800 bg-slate-900 rounded-xl p-3">
                  <p className="text-sm font-medium">{p.name}</p>
                  <p className={`text-xs uppercase mt-1 ${STATE_TONE[p.state] ?? 'text-slate-400'}`}>
                    {p.state}
                    {p.active_tasks > 0 ? ` · ${p.active_tasks} task(s)` : ''}
                  </p>
                </div>
              ))}
            </div>
          )}

          <p className="text-xs uppercase tracking-wide text-slate-500 mb-2">Open Picking Tasks</p>
          {tasks.length === 0 ? (
            <div className="border border-slate-800 rounded-xl bg-slate-900 p-8 text-center text-sm text-slate-500">
              No open picking tasks.
            </div>
          ) : (
            <div className="border border-slate-800 rounded-xl bg-slate-900 overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-slate-800/50 text-slate-400 text-xs uppercase">
                  <tr>
                    <th className="text-left px-4 py-2.5">Order</th>
                    <th className="text-left px-4 py-2.5">Picker</th>
                    <th className="text-left px-4 py-2.5">Status</th>
                    <th className="text-right px-4 py-2.5">Waiting</th>
                    <th className="text-right px-4 py-2.5">Progress</th>
                    <th className="text-left px-4 py-2.5">Flags</th>
                    <th className="text-right px-4 py-2.5">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {tasks.map((t) => (
                    <tr key={t.task_id} className="hover:bg-slate-800/30">
                      <td className="px-4 py-3 font-medium">#{t.order_id}</td>
                      <td className="px-4 py-3">
                        {t.picker_id ? (
                          <span className={t.picker_online ? '' : 'text-slate-500'}>
                            {t.picker_name || `#${t.picker_id}`}
                            {!t.picker_online && ' (offline)'}
                          </span>
                        ) : (
                          <span className="text-slate-500">Unassigned</span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-slate-400">{t.status.replace('_', ' ')}</td>
                      <td className="px-4 py-3 text-right text-slate-400">{fmtMin(t.waiting_minutes)}</td>
                      <td className="px-4 py-3 text-right text-slate-400">
                        {t.items_done}/{t.items_total}
                      </td>
                      <td className="px-4 py-3 text-xs space-x-2">
                        {t.delayed && <span className="text-rose-300">Delayed</span>}
                        {t.stale && <span className="text-amber-300">Stale</span>}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => openReassign(t)}
                          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
                        >
                          Reassign
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {target && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-sm p-6">
            <h2 className="text-base font-semibold mb-1">Reassign order #{target.order_id}</h2>
            <p className="text-xs text-slate-500 mb-4">
              Only online pickers are listed. An in-progress task can be moved only if its current picker is offline.
            </p>
            {candidates.length === 0 ? (
              <p className="text-sm text-slate-400 mb-4">No other picker is online.</p>
            ) : (
              <select
                value={newPicker}
                onChange={(e) => setNewPicker(e.target.value === '' ? '' : Number(e.target.value))}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-4"
              >
                <option value="">Select picker...</option>
                {candidates.map((p) => (
                  <option key={p.staff_id} value={p.staff_id}>
                    {p.name} ({p.state})
                  </option>
                ))}
              </select>
            )}
            {modalError && (
              <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-3 py-2 mb-4">
                {modalError}
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setTarget(null)}
                className="text-sm px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={submit}
                disabled={saving || newPicker === ''}
                className="text-sm px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 disabled:opacity-40 transition-colors"
              >
                {saving ? 'Saving...' : 'Reassign'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}