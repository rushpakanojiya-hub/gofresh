import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { getInventoryOverview, listWarehouses, listCategories, updateInventory } from '../api/admin'
import type { InventoryOverviewResponse } from '../types/admin'

export default function InventoryOverview() {
  const [data, setData] = useState<InventoryOverviewResponse | null>(null)
  const [warehouses, setWarehouses] = useState<any[]>([])
  const [categories, setCategories] = useState<any[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [warehouseFilter, setWarehouseFilter] = useState('')
  const [categoryFilter, setCategoryFilter] = useState('')
  const [stockStatusFilter, setStockStatusFilter] = useState('')
  const [page, setPage] = useState(1)

  async function loadFilters() {
    try {
      const [whRes, catRes] = await Promise.all([listWarehouses(), listCategories()])
      setWarehouses(whRes.warehouses ?? whRes ?? [])
      setCategories(catRes.categories ?? catRes ?? [])
    } catch {
      // non-fatal - filters just won't populate
    }
  }

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const params: Record<string, any> = { page, limit: 20 }
      if (warehouseFilter) params.warehouse_id = warehouseFilter
      if (categoryFilter) params.category_id = categoryFilter
      if (stockStatusFilter) params.stock_status = stockStatusFilter
      const res = await getInventoryOverview(params)
      setData(res)
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load inventory.')
    } finally {
      setIsLoading(false)
    }
  }

  async function handleStockEdit(productId: number, warehouseId: number, newStock: number) {
    if (Number.isNaN(newStock) || newStock < 0) return
    try {
      await updateInventory(productId, newStock, warehouseId)
      setData((prev) =>
        prev
          ? {
              ...prev,
              rows: prev.rows.map((r) =>
                r.product_id === productId && r.warehouse_id === warehouseId
                  ? { ...r, stock: newStock, available: newStock - r.reserved, in_stock: newStock > 0 }
                  : r
              ),
            }
          : prev
      )
    } catch {
      // silently ignore - row keeps its last known value on failure
    }
  }

  useEffect(() => {
    loadFilters()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, warehouseFilter, categoryFilter, stockStatusFilter])

  const summaryCards = data
    ? [
        { label: 'Total SKUs', value: data.total_skus, color: 'text-slate-100' },
        { label: 'Available Stock', value: data.total_available_stock, color: 'text-emerald-300' },
        { label: 'Reserved Stock', value: data.total_reserved_stock, color: 'text-amber-300' },
        { label: 'Low Stock', value: data.low_stock_count, color: 'text-orange-300' },
        { label: 'Out of Stock', value: data.out_of_stock_count, color: 'text-red-300' },
        { label: 'Damaged', value: data.damaged_stock, color: 'text-slate-400' },
        { label: 'Expired', value: data.expired_stock, color: 'text-slate-400' },
      ]
    : []

  return (
    <Layout>
      <div className="p-8">
        <div className="mb-6">
          <h1 className="text-xl font-semibold">Inventory Overview</h1>
          <p className="text-sm text-slate-400 mt-1">
            {data ? `${data.total} SKU-warehouse entries` : 'Loading...'}
          </p>
        </div>

        {data && (
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-7 gap-3 mb-6">
            {summaryCards.map((card) => (
              <div key={card.label} className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                <p className="text-xs text-slate-500 mb-1">{card.label}</p>
                <p className={`text-lg font-semibold ${card.color}`}>{card.value}</p>
              </div>
            ))}
          </div>
        )}

        <div className="flex items-center gap-3 mb-6">
          <select
            value={warehouseFilter}
            onChange={(e) => {
              setWarehouseFilter(e.target.value)
              setPage(1)
            }}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All warehouses</option>
            {warehouses.map((w: any) => (
              <option key={w.id} value={w.id}>{w.name}</option>
            ))}
          </select>
          <select
            value={categoryFilter}
            onChange={(e) => {
              setCategoryFilter(e.target.value)
              setPage(1)
            }}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All categories</option>
            {categories.map((c: any) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <select
            value={stockStatusFilter}
            onChange={(e) => {
              setStockStatusFilter(e.target.value)
              setPage(1)
            }}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All stock levels</option>
            <option value="low">Low stock</option>
            <option value="out">Out of stock</option>
          </select>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && data && data.rows.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No inventory entries found.
          </div>
        )}

        {!isLoading && data && data.rows.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Product</th>
                  <th className="px-4 py-3 font-medium">Category</th>
                  <th className="px-4 py-3 font-medium">Warehouse</th>
                  <th className="px-4 py-3 font-medium">Stock</th>
                  <th className="px-4 py-3 font-medium">Reserved</th>
                  <th className="px-4 py-3 font-medium">Available</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {data.rows.map((r) => (
                  <tr key={`${r.product_id}-${r.warehouse_id}`} className="border-t border-slate-800">
                    <td className="px-4 py-3">{r.product_name}</td>
                    <td className="px-4 py-3 text-slate-400">{r.category_name}</td>
                    <td className="px-4 py-3 text-slate-400">{r.warehouse_name}</td>
                    <td className="px-4 py-3">
                      <input
                        type="number"
                        defaultValue={r.stock}
                        onBlur={(e) => handleStockEdit(r.product_id, r.warehouse_id, parseInt(e.target.value, 10))}
                        className="w-20 bg-slate-800 border border-slate-700 rounded-md px-2 py-1 text-sm"
                      />
                    </td>
                    <td className="px-4 py-3 text-amber-300">{r.reserved}</td>
                    <td className="px-4 py-3 text-emerald-300">{r.available}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`px-2 py-1 rounded-md text-xs font-medium ${
                          r.stock <= 0
                            ? 'bg-red-500/15 text-red-300'
                            : r.stock < 10
                            ? 'bg-orange-500/15 text-orange-300'
                            : 'bg-emerald-500/15 text-emerald-300'
                        }`}
                      >
                        {r.stock <= 0 ? 'Out of stock' : r.stock < 10 ? 'Low stock' : 'In stock'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {!isLoading && data && data.total_pages > 1 && (
          <div className="flex items-center justify-center gap-2 mt-6">
            <button
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm disabled:opacity-40"
            >
              Prev
            </button>
            <span className="text-sm text-slate-400">
              Page {page} of {data.total_pages}
            </span>
            <button
              disabled={page >= data.total_pages}
              onClick={() => setPage((p) => p + 1)}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm disabled:opacity-40"
            >
              Next
            </button>
          </div>
        )}
      </div>
    </Layout>
  )
}
