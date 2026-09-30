import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { getPickerPerformance, listWarehouses } from '../api/admin'
import type { StaffPerformance, Warehouse } from '../types/admin'

export default function PickerPerformance() {
  const [rows, setRows] = useState<StaffPerformance[]>([])
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [warehouseFilter, setWarehouseFilter] = useState('')
  const [search, setSearch] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const [perfRes, whRes] = await Promise.all([
        getPickerPerformance(warehouseFilter ? parseInt(warehouseFilter, 10) : undefined),
        listWarehouses(),
      ])
      setRows(perfRes.staff_performance ?? [])
      setWarehouses(whRes.warehouses ?? whRes ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load picker performance.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [warehouseFilter])

  const filteredRows = rows.filter((r) =>
    r.staff_name.toLowerCase().includes(search.toLowerCase()),
  )

  const activeRows = filteredRows.filter((r) => r.orders_picked > 0 || r.orders_packed > 0)
  const topPerformers = [...activeRows].sort((a, b) => b.accuracy_rate - a.accuracy_rate).slice(0, 3)
  const slowPickers = [...activeRows]
    .filter((r) => r.avg_picking_minutes > 0)
    .sort((a, b) => b.avg_picking_minutes - a.avg_picking_minutes)
    .slice(0, 3)

  function workloadLabel(orders: number) {
    if (orders >= 10) return { label: 'High', color: 'bg-red-500/15 text-red-300' }
    if (orders >= 3) return { label: 'Medium', color: 'bg-amber-500/15 text-amber-300' }
    if (orders > 0) return { label: 'Low', color: 'bg-emerald-500/15 text-emerald-300' }
    return { label: 'Idle', color: 'bg-slate-700 text-slate-400' }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="mb-6">
          <h1 className="text-xl font-semibold">Picker Performance</h1>
          <p className="text-sm text-slate-400 mt-1">
            Picking/packing speed and accuracy across all warehouses.
          </p>
        </div>

        <div className="flex gap-3 mb-6">
          <select
            value={warehouseFilter}
            onChange={(e) => setWarehouseFilter(e.target.value)}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All Warehouses</option>
            {warehouses.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </select>
          <input
            type="text"
            placeholder="Search picker..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm flex-1 max-w-xs"
          />
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && (
          <>
            {topPerformers.length > 0 && (
              <div className="grid grid-cols-2 gap-4 mb-6">
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-3 font-semibold">TOP PERFORMERS (by accuracy)</p>
                  {topPerformers.map((p) => (
                    <div key={p.staff_id} className="flex justify-between text-sm py-1">
                      <span className="text-slate-300">{p.staff_name}</span>
                      <span className="text-emerald-300 font-medium">{p.accuracy_rate.toFixed(0)}%</span>
                    </div>
                  ))}
                </div>
                {slowPickers.length > 0 && (
                  <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                    <p className="text-xs text-slate-500 mb-3 font-semibold">SLOWEST AVG PICK TIME</p>
                    {slowPickers.map((p) => (
                      <div key={p.staff_id} className="flex justify-between text-sm py-1">
                        <span className="text-slate-300">{p.staff_name}</span>
                        <span className="text-amber-300 font-medium">{p.avg_picking_minutes.toFixed(1)} min</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            <div className="border border-slate-800 rounded-xl overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-slate-900 text-slate-400 text-left">
                    <th className="px-4 py-3 font-medium">Picker</th>
                    <th className="px-4 py-3 font-medium">Warehouse</th>
                    <th className="px-4 py-3 font-medium">Picked</th>
                    <th className="px-4 py-3 font-medium">Packed</th>
                    <th className="px-4 py-3 font-medium">Avg Pick Time</th>
                    <th className="px-4 py-3 font-medium">Avg Pack Time</th>
                    <th className="px-4 py-3 font-medium">Accuracy</th>
                    <th className="px-4 py-3 font-medium">Exceptions</th>
                    <th className="px-4 py-3 font-medium">Workload</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredRows.map((r) => {
                    const workload = workloadLabel(r.orders_picked)
                    return (
                      <tr key={r.staff_id} className="border-t border-slate-800">
                        <td className="px-4 py-3 text-slate-200">{r.staff_name}</td>
                        <td className="px-4 py-3 text-slate-400">{r.warehouse_name}</td>
                        <td className="px-4 py-3">{r.orders_picked}</td>
                        <td className="px-4 py-3">{r.orders_packed}</td>
                        <td className="px-4 py-3">{r.avg_picking_minutes > 0 ? `${r.avg_picking_minutes.toFixed(1)} min` : '-'}</td>
                        <td className="px-4 py-3">{r.avg_packing_minutes > 0 ? `${r.avg_packing_minutes.toFixed(1)} min` : '-'}</td>
                        <td className="px-4 py-3">
                          {r.orders_picked > 0 ? (
                            <span className={r.accuracy_rate >= 95 ? 'text-emerald-300' : r.accuracy_rate >= 80 ? 'text-amber-300' : 'text-red-300'}>
                              {r.accuracy_rate.toFixed(0)}%
                            </span>
                          ) : (
                            '-'
                          )}
                        </td>
                        <td className="px-4 py-3">{r.exceptions_caused}</td>
                        <td className="px-4 py-3">
                          <span className={`px-2 py-0.5 rounded-md text-xs font-medium ${workload.color}`}>
                            {workload.label}
                          </span>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </>
        )}
      </div>
    </Layout>
  )
}
