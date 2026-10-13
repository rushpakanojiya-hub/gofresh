import { useEffect, useState } from 'react'
import {
  listMyStockTransfers,
  requestStockTransfer,
  receiveStockTransfer,
  approveStockTransfer,
  rejectStockTransfer,
  listProducts,
} from '../api/warehouse'
import { useAuth } from '../context/AuthContext'
import type { StockTransfer, Product } from '../types/warehouse'

export default function StockTransfers() {
  const { staff } = useAuth()
  const [transfers, setTransfers] = useState<StockTransfer[]>([])
  const [products, setProducts] = useState<Product[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actingId, setActingId] = useState<number | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ product_id: '', to_warehouse_id: '', quantity: '' })
  const [formError, setFormError] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const [transfersRes, productsRes] = await Promise.all([
        listMyStockTransfers(),
        listProducts({ limit: 100 }),
      ])
      setTransfers(transfersRes.stock_transfers ?? [])
      setProducts(productsRes.products ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load stock transfers.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function handleReceive(id: number) {
    setActingId(id)
    try {
      await receiveStockTransfer(id)
      setTransfers((prev) =>
        prev.map((t) => (t.id === id ? { ...t, status: 'received' } : t))
      )
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to receive transfer.')
    } finally {
      setActingId(null)
    }
  }

  async function handleApprove(id: number) {
    setActingId(id)
    try {
      await approveStockTransfer(id)
      setTransfers((prev) =>
        prev.map((t) => (t.id === id ? { ...t, status: 'in_transit' } : t))
      )
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to approve transfer.')
    } finally {
      setActingId(null)
    }
  }

  async function handleReject(id: number) {
    setActingId(id)
    try {
      await rejectStockTransfer(id)
      setTransfers((prev) =>
        prev.map((t) => (t.id === id ? { ...t, status: 'rejected' } : t))
      )
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to reject transfer.')
    } finally {
      setActingId(null)
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)

    const productId = parseInt(form.product_id, 10)
    const toWarehouseId = parseInt(form.to_warehouse_id, 10)
    const quantity = parseInt(form.quantity, 10)

    if (!productId || !toWarehouseId || !quantity || quantity <= 0) {
      setFormError('Please fill all fields with valid values.')
      return
    }

    setIsSaving(true)
    try {
      await requestStockTransfer({
        product_id: productId,
        to_warehouse_id: toWarehouseId,
        quantity,
      })
      setForm({ product_id: '', to_warehouse_id: '', quantity: '' })
      setShowCreate(false)
      load()
    } catch (err: any) {
      setFormError(err.response?.data?.error ?? 'Failed to request stock transfer.')
    } finally {
      setIsSaving(false)
    }
  }

  function statusColor(status: string) {
    if (status === 'received') return 'bg-emerald-500/15 text-emerald-400'
    if (status === 'rejected') return 'bg-red-500/15 text-red-400'
    if (status === 'in_transit') return 'bg-blue-500/15 text-blue-400'
    return 'bg-amber-500/15 text-amber-400'
  }

  return (
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Stock Transfers</h1>
            <p className="text-sm text-slate-400 mt-1">
              {transfers.length} transfer{transfers.length !== 1 ? 's' : ''} involving your warehouse
            </p>
          </div>
          <button
            onClick={() => setShowCreate((v) => !v)}
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            {showCreate ? 'Cancel' : '+ Request transfer'}
          </button>
        </div>

        {showCreate && (
          <div className="border border-slate-800 rounded-xl p-5 bg-slate-900 mb-6 max-w-md">
            <h2 className="text-sm font-medium mb-3">Request outgoing transfer</h2>
            <p className="text-xs text-slate-500 mb-3">
              Your warehouse ({staff?.warehouse?.name ?? `#${staff?.warehouse_id}`}) will be the source.
            </p>
            <form onSubmit={handleSubmit} className="space-y-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Product</label>
                <select
                  value={form.product_id}
                  onChange={(e) => setForm({ ...form, product_id: e.target.value })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="">Select a product</option>
                  {products.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Destination Warehouse ID</label>
                <input
                  type="number"
                  value={form.to_warehouse_id}
                  onChange={(e) => setForm({ ...form, to_warehouse_id: e.target.value })}
                  placeholder="e.g. 2"
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Quantity</label>
                <input
                  type="number"
                  value={form.quantity}
                  onChange={(e) => setForm({ ...form, quantity: e.target.value })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              {formError && <p className="text-red-400 text-xs">{formError}</p>}
              <button
                type="submit"
                disabled={isSaving}
                className="w-full py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
              >
                {isSaving ? 'Requesting...' : 'Request Transfer'}
              </button>
            </form>
          </div>
        )}

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && transfers.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No stock transfers yet.
          </div>
        )}

        {!isLoading && transfers.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Product</th>
                  <th className="px-4 py-3 font-medium">From</th>
                  <th className="px-4 py-3 font-medium">To</th>
                  <th className="px-4 py-3 font-medium">Quantity</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {transfers.map((t) => {
                  const isIncoming = t.to_warehouse_id === staff?.warehouse_id
                  const isSupplier = t.from_warehouse_id === staff?.warehouse_id
                  return (
                    <tr key={t.id} className="border-t border-slate-800">
                      <td className="px-4 py-3">{t.product?.name ?? `#${t.product_id}`}</td>
                      <td className="px-4 py-3 text-slate-400">
                        {t.from_warehouse?.name ?? `#${t.from_warehouse_id}`}
                      </td>
                      <td className="px-4 py-3 text-slate-400">
                        {t.to_warehouse?.name ?? `#${t.to_warehouse_id}`}
                      </td>
                      <td className="px-4 py-3">{t.quantity}</td>
                      <td className="px-4 py-3">
                        <span className={`text-xs px-2 py-1 rounded-full ${statusColor(t.status)}`}>
                          {({ pending: 'Truck requested', in_transit: 'Truck on the way', received: 'Truck arrived', rejected: 'Rejected', cancelled: 'Cancelled' } as Record<string, string>)[t.status] ?? t.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right space-x-3">
                        {isSupplier && t.status === 'pending' && (
                          <>
                            <button
                              onClick={() => handleApprove(t.id)}
                              disabled={actingId === t.id}
                              className="text-emerald-400 hover:text-emerald-300 text-xs disabled:opacity-50"
                            >
                              {actingId === t.id ? 'Dispatching...' : 'Load & dispatch truck'}
                            </button>
                            <button
                              onClick={() => handleReject(t.id)}
                              disabled={actingId === t.id}
                              className="text-red-400 hover:text-red-300 text-xs disabled:opacity-50"
                            >
                              {actingId === t.id ? 'Rejecting...' : 'Reject'}
                            </button>
                          </>
                        )}
                        {isIncoming && t.status === 'in_transit' && (
                          <button
                            onClick={() => handleReceive(t.id)}
                            disabled={actingId === t.id}
                            className="text-emerald-400 hover:text-emerald-300 text-xs disabled:opacity-50"
                          >
                            {actingId === t.id ? 'Receiving...' : 'Truck arrived'}
                          </button>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
  )
}
