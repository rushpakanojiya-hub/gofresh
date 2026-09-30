import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { getReplenishmentSuggestions, listWarehouses, updateProductReorderLevel } from '../api/admin'
import type { ReplenishmentSuggestion, Warehouse } from '../types/admin'

export default function Replenishment() {
  const [suggestions, setSuggestions] = useState<ReplenishmentSuggestion[]>([])
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [warehouseFilter, setWarehouseFilter] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editingLevel, setEditingLevel] = useState<Record<number, string>>({})

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await getReplenishmentSuggestions(warehouseFilter ? parseInt(warehouseFilter, 10) : undefined)
      setSuggestions(res.suggestions ?? res ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load replenishment suggestions.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadWarehouses() {
    try {
      const res = await listWarehouses()
      setWarehouses(res.warehouses ?? res ?? [])
    } catch {
      // non-fatal
    }
  }

  useEffect(() => {
    loadWarehouses()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [warehouseFilter])

  async function handleSetReorderLevel(productId: number) {
    const value = editingLevel[productId]
    if (!value) return
    try {
      await updateProductReorderLevel(productId, parseInt(value, 10))
      setEditingLevel((prev) => {
        const next = { ...prev }
        delete next[productId]
        return next
      })
      load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update reorder level.')
    }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="mb-6">
          <h1 className="text-xl font-semibold">Low Stock &amp; Replenishment</h1>
          <p className="text-sm text-slate-400 mt-1">
            {suggestions.length} product{suggestions.length !== 1 ? 's' : ''} at or below reorder level
          </p>
        </div>

        <div className="flex items-center gap-3 mb-6">
          <select
            value={warehouseFilter}
            onChange={(e) => setWarehouseFilter(e.target.value)}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All warehouses</option>
            {warehouses.map((w) => (
              <option key={w.id} value={w.id}>{w.name}</option>
            ))}
          </select>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && suggestions.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            All products are above their reorder level. Nothing to replenish right now.
          </div>
        )}

        {!isLoading && suggestions.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Product</th>
                  <th className="px-4 py-3 font-medium">Warehouse</th>
                  <th className="px-4 py-3 font-medium">Current Stock</th>
                  <th className="px-4 py-3 font-medium">Reorder Level</th>
                  <th className="px-4 py-3 font-medium">Suggested Order Qty</th>
                  <th className="px-4 py-3 font-medium">Adjust reorder level</th>
                </tr>
              </thead>
              <tbody>
                {suggestions.map((s) => (
                  <tr key={`${s.product_id}-${s.warehouse_id}`} className="border-t border-slate-800">
                    <td className="px-4 py-3">{s.product_name}</td>
                    <td className="px-4 py-3 text-slate-400">{s.warehouse_name}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`px-2 py-1 rounded-md text-xs font-medium ${
                          s.current_stock <= 0
                            ? 'bg-red-500/15 text-red-300'
                            : 'bg-orange-500/15 text-orange-300'
                        }`}
                      >
                        {s.current_stock}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-400">{s.reorder_level}</td>
                    <td className="px-4 py-3 text-emerald-300 font-medium">{s.suggested_order_qty}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          placeholder={String(s.reorder_level)}
                          value={editingLevel[s.product_id] ?? ''}
                          onChange={(e) =>
                            setEditingLevel({ ...editingLevel, [s.product_id]: e.target.value })
                          }
                          className="w-20 bg-slate-800 border border-slate-700 rounded-lg px-2 py-1 text-sm"
                        />
                        <button
                          onClick={() => handleSetReorderLevel(s.product_id)}
                          className="text-indigo-400 hover:text-indigo-300 text-xs"
                        >
                          Save
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Layout>
  )
}
