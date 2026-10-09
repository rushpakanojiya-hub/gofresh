import { useCallback, useEffect, useState } from 'react'
import { assignPutawayTask, getPutawayTasks } from '../api/warehouse'
import type { PutawayBinRef, PutawayData, PutawayTaskRow } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

type Tab = 'open' | 'pending' | 'in_progress' | 'completed'

const TABS: { label: string; value: Tab }[] = [
  { label: 'Open', value: 'open' },
  { label: 'Pending', value: 'pending' },
  { label: 'In Progress', value: 'in_progress' },
  { label: 'Completed (7d)', value: 'completed' },
]

function binLabel(b?: PutawayBinRef | null, id?: number | null): string {
  if (!b) return id ? `Bin #${id}` : '\u2014'
  const parts = [b.rack?.zone?.name, b.rack?.name, b.name].filter(Boolean)
  return parts.length ? parts.join(' / ') : `Bin #${b.id}`
}

function fmtDate(s?: string | null): string {
  if (!s) return '\u2014'
  return new Date(s).toLocaleString()
}

const STATUS_TONE: Record<string, string> = {
  pending: 'text-amber-300',
  in_progress: 'text-sky-300',
  completed: 'text-emerald-300',
}

export default function Putaway() {
  const [tab, setTab] = useState<Tab>('open')
  const [data, setData] = useState<PutawayData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [target, setTarget] = useState<PutawayTaskRow | null>(null)
  const [putterId, setPutterId] = useState<number | ''>('')
  const [saving, setSaving] = useState(false)
  const [modalError, setModalError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setData(await getPutawayTasks(tab === 'open' ? undefined : tab))
      setError(null)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load putaway tasks.'))
    } finally {
      setIsLoading(false)
    }
  }, [tab])

  useEffect(() => {
    setIsLoading(true)
    load()
    const interval = setInterval(load, 15000)
    return () => clearInterval(interval)
  }, [load])

  function openAssign(t: PutawayTaskRow) {
    setTarget(t)
    setPutterId(t.putter_id ?? '')
    setModalError(null)
  }

  async function submit() {
    if (!target || putterId === '') return
    setSaving(true)
    setModalError(null)
    try {
      await assignPutawayTask(target.id, putterId)
      setTarget(null)
      setPutterId('')
      await load()
    } catch (err) {
      setModalError(getErrorMessage(err, 'Failed to assign task.'))
    } finally {
      setSaving(false)
    }
  }

  const tasks = data?.tasks ?? []
  const putters = data?.putters ?? []
  const s = data?.summary

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">Inbound</p>
          <h1 className="font-display text-2xl font-semibold">Putaway</h1>
        </div>
        <button
          onClick={load}
          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      {s && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-5">
          {[
            { label: 'Pending', value: s.pending },
            { label: 'In Progress', value: s.in_progress },
            { label: 'Completed Today', value: s.completed_today },
            { label: 'Wrong Location (7d)', value: s.wrong_location_7d },
          ].map((c) => (
            <div key={c.label} className="border border-slate-800 bg-slate-900 rounded-xl p-3">
              <p className="text-xs text-slate-500">{c.label}</p>
              <p className="text-xl font-semibold mt-1">{c.value}</p>
            </div>
          ))}
        </div>
      )}

      <div className="flex gap-1 border-b border-slate-800 mb-4">
        {TABS.map((t) => (
          <button
            key={t.value}
            onClick={() => setTab(t.value)}
            className={`px-3 py-2 text-sm border-b-2 transition-colors ${
              tab === t.value
                ? 'border-red-400 text-red-300 font-medium'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}
      {isLoading && <p className="text-sm text-slate-400">Loading...</p>}

      {!isLoading && tasks.length === 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 p-8 text-center text-sm text-slate-500">
          No putaway tasks here.
        </div>
      )}

      {tasks.length > 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-800/50 text-slate-400 text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-2.5">Product</th>
                <th className="text-right px-4 py-2.5">Qty</th>
                <th className="text-left px-4 py-2.5">Suggested Bin</th>
                <th className="text-left px-4 py-2.5">Putter</th>
                <th className="text-left px-4 py-2.5">Status</th>
                <th className="text-left px-4 py-2.5">{tab === 'completed' ? 'Put in' : 'Created'}</th>
                <th className="text-right px-4 py-2.5">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {tasks.map((t) => (
                <tr key={t.id} className="hover:bg-slate-800/30">
                  <td className="px-4 py-3">
                    <p className="font-medium">{t.product?.name ?? `Product #${t.product_id}`}</p>
                    <p className="text-xs text-slate-500">
                      Receiving #{t.receiving_id}
                      {t.receiving?.supplier_name ? ` \u00b7 ${t.receiving.supplier_name}` : ''}
                    </p>
                  </td>
                  <td className="px-4 py-3 text-right">{t.quantity}</td>
                  <td className="px-4 py-3 text-slate-400 text-xs">
                    {binLabel(t.suggested_bin, t.suggested_bin_id)}
                  </td>
                  <td className="px-4 py-3">
                    {t.putter_id ? (
                      t.putter_name || `#${t.putter_id}`
                    ) : (
                      <span className="text-slate-500">Unassigned</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`text-xs uppercase ${STATUS_TONE[t.status] ?? ''}`}>
                      {t.status.replace('_', ' ')}
                    </span>
                    {t.wrong_location && <span className="ml-2 text-xs text-rose-300">Wrong location</span>}
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-400">
                    {t.status === 'completed' ? (
                      <>
                        {binLabel(t.actual_bin, t.actual_bin_id)}
                        <br />
                        {fmtDate(t.completed_at)}
                      </>
                    ) : (
                      fmtDate(t.created_at)
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {t.status !== 'completed' && (
                      <button
                        onClick={() => openAssign(t)}
                        className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
                      >
                        {t.putter_id ? 'Reassign' : 'Assign'}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {target && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-sm p-6">
            <h2 className="text-base font-semibold mb-1">
              {target.putter_id ? 'Reassign' : 'Assign'} putaway #{target.id}
            </h2>
            <p className="text-xs text-slate-500 mb-4">
              {target.product?.name ?? `Product #${target.product_id}`} &times; {target.quantity}
            </p>
            {putters.length === 0 ? (
              <p className="text-sm text-slate-400 mb-4">
                No active putter in this warehouse. Create a staff member with the putter role first.
              </p>
            ) : (
              <select
                value={putterId}
                onChange={(e) => setPutterId(e.target.value === '' ? '' : Number(e.target.value))}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-4"
              >
                <option value="">Select putter...</option>
                {putters.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
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
                disabled={saving || putterId === ''}
                className="text-sm px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 disabled:opacity-40 transition-colors"
              >
                {saving ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}