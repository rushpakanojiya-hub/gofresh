import { useEffect, useState, useCallback } from 'react'
import { getStaffPerformance } from '../api/warehouse'
import type { StaffPerformanceRow } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

type Period = 'today' | '7d' | '30d'

const PERIODS: { label: string; value: Period }[] = [
  { label: 'Today', value: 'today' },
  { label: '7 Days', value: '7d' },
  { label: '30 Days', value: '30d' },
]

function accuracyTone(rate: number): string {
  if (rate >= 90) return 'text-emerald-300'
  if (rate >= 70) return 'text-amber-300'
  return 'text-rose-300'
}

export default function Performance() {
  const [rows, setRows] = useState<StaffPerformanceRow[]>([])
  const [period, setPeriod] = useState<Period>('today')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      const data = await getStaffPerformance(period)
      setRows(data.staff_performance)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load staff performance.'))
    } finally {
      setIsLoading(false)
    }
  }, [period])

  useEffect(() => {
    setIsLoading(true)
    load()
  }, [load])

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-4">
        <h1 className="font-display text-2xl font-semibold">Staff Performance</h1>
        <button
          onClick={load}
          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      <div className="flex gap-1 border-b border-slate-800 mb-4">
        {PERIODS.map((p) => (
          <button
            key={p.value}
            onClick={() => setPeriod(p.value)}
            className={`px-3 py-2 text-sm border-b-2 transition-colors ${
              period === p.value
                ? 'border-red-400 text-red-300 font-medium'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            {p.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

      {isLoading && <p className="text-sm text-slate-400">Loading performance data...</p>}

      {!isLoading && rows.length === 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 p-8 text-center text-sm text-slate-500">
          No staff performance data available yet.
        </div>
      )}

      {!isLoading && rows.length > 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-800/50 text-slate-400 text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-2.5">Staff</th>
                <th className="text-right px-4 py-2.5">Orders Picked</th>
                <th className="text-right px-4 py-2.5">Orders Packed</th>
                <th className="text-right px-4 py-2.5">Avg Pick Time</th>
                <th className="text-right px-4 py-2.5">Avg Pack Time</th>
                <th className="text-right px-4 py-2.5">Items Picked</th>
                <th className="text-right px-4 py-2.5">Items / Hr</th>
                <th className="text-right px-4 py-2.5">Accuracy</th>
                <th className="text-right px-4 py-2.5">Exceptions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {rows.map((row) => (
                <tr key={row.staff_id} className="hover:bg-slate-800/30">
                  <td className="px-4 py-3 font-medium">
                    {row.staff_name}
                    {row.role && <span className="ml-2 text-xs text-slate-500">{row.role}</span>}
                  </td>
                  <td className="px-4 py-3 text-right">{row.orders_picked}</td>
                  <td className="px-4 py-3 text-right">{row.orders_packed}</td>
                  <td className="px-4 py-3 text-right text-slate-400">
                    {row.avg_picking_minutes.toFixed(1)} min
                  </td>
                  <td className="px-4 py-3 text-right text-slate-400">
                    {row.avg_packing_minutes.toFixed(1)} min
                  </td>
                  <td className="px-4 py-3 text-right text-slate-400">{row.total_items_picked}</td>
                  <td className="px-4 py-3 text-right text-slate-400">
                    {(row.items_per_hour ?? 0).toFixed(1)}
                  </td>
                  <td className={`px-4 py-3 text-right font-medium ${accuracyTone(row.accuracy_rate)}`}>
                    {row.accuracy_rate.toFixed(0)}%
                  </td>
                  <td className="px-4 py-3 text-right">
                    <span className={row.exceptions_caused > 0 ? 'text-amber-300' : 'text-slate-500'}>
                      {row.exceptions_caused}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}