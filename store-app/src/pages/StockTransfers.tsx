import { useEffect, useRef, useState } from 'react'
import {
  listMyStockTransfers,
  requestStockTransfer,
  receiveStockTransfer,
  approveStockTransfer,
  rejectStockTransfer,
  listProducts,
  listTransferSuppliers,
  getSupplierStock,
} from '../api/warehouse'
import { useAuth } from '../context/AuthContext'
import type { StockTransfer, Product } from '../types/warehouse'

type Supplier = { id: number; name: string; city: string }
type Row = { key: number; product: Product | null; qty: string; error?: string }

const STATUS_LABEL: Record<string, string> = {
  pending: 'Waiting for warehouse',
  in_transit: 'Truck on the way',
  received: 'Received',
  rejected: 'Rejected',
  cancelled: 'Cancelled',
}

function statusColor(status: string) {
  if (status === 'received') return 'bg-emerald-500/15 text-emerald-400'
  if (status === 'rejected') return 'bg-red-500/15 text-red-400'
  if (status === 'in_transit') return 'bg-blue-500/15 text-blue-400'
  if (status === 'cancelled') return 'bg-slate-500/15 text-slate-400'
  return 'bg-amber-500/15 text-amber-400'
}

function ProductPicker({
  value,
  onPick,
  onClear,
}: {
  value: Product | null
  onPick: (p: Product) => void
  onClear: () => void
}) {
  const [q, setQ] = useState('')
  const [results, setResults] = useState<Product[]>([])
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const term = q.trim()
    if (term.length < 2) {
      setResults([])
      setLoading(false)
      return undefined
    }
    let alive = true
    setLoading(true)
    const t = setTimeout(async () => {
      try {
        const res = await listProducts({ search: term, limit: 8 })
        if (alive) setResults(res.products ?? [])
      } catch {
        if (alive) setResults([])
      } finally {
        if (alive) setLoading(false)
      }
    }, 300)
    return () => {
      alive = false
      clearTimeout(t)
    }
  }, [q])

  if (value) {
    return (
      <div className="flex items-center justify-between bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm">
        <span className="truncate">{value.name}</span>
        <button type="button" onClick={onClear} className="ml-2 text-slate-400 hover:text-slate-200 text-xs">
          Change
        </button>
      </div>
    )
  }

  return (
    <div className="relative">
      <input
        value={q}
        onChange={(e) => {
          setQ(e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        placeholder="Search product (type 2+ letters)"
        className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
      />
      {open && q.trim().length >= 2 && (
        <div className="absolute z-10 mt-1 w-full max-h-56 overflow-auto bg-slate-900 border border-slate-700 rounded-lg shadow-lg">
          {loading && <p className="px-3 py-2 text-xs text-slate-400">Searching...</p>}
          {!loading && results.length === 0 && <p className="px-3 py-2 text-xs text-slate-500">No products found</p>}
          {results.map((p) => (
            <button
              type="button"
              key={p.id}
              onClick={() => {
                onPick(p)
                setQ('')
                setOpen(false)
              }}
              className="block w-full text-left px-3 py-2 text-sm hover:bg-slate-800"
            >
              {p.name}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export default function StockTransfers() {
  const { staff } = useAuth()
  const myWarehouseId = staff?.warehouse_id
  const [transfers, setTransfers] = useState<StockTransfer[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actingId, setActingId] = useState<number | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [suppliers, setSuppliers] = useState<Supplier[]>([])
  const [supplierId, setSupplierId] = useState('')
  const keyRef = useRef(1)
  const [rows, setRows] = useState<Row[]>([{ key: 0, product: null, qty: '' }])
  const [stockMap, setStockMap] = useState<Record<string, number>>({})
  const [stockFor, setStockFor] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  async function load(silent = false) {
    if (!silent) setIsLoading(true)
    setError(null)
    try {
      const res = await listMyStockTransfers()
      setTransfers(res.stock_transfers ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load stock requests.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  useEffect(() => {
    listTransferSuppliers()
      .then((res) => {
        const list = res.suppliers ?? []
        setSuppliers(list)
        if (list.length === 1) setSupplierId(String(list[0].id))
      })
      .catch(() => {})
  }, [])

  const idsKey = rows
    .map((r) => r.product?.id)
    .filter((x): x is number => typeof x === 'number')
    .join(',')
  const stockKey = `${supplierId}|${idsKey}`

  useEffect(() => {
    if (!supplierId || !idsKey) return undefined
    let alive = true
    getSupplierStock(Number(supplierId), idsKey)
      .then((res) => {
        if (alive) {
          setStockMap(res.stock ?? {})
          setStockFor(stockKey)
        }
      })
      .catch(() => {
        if (alive) {
          setStockMap({})
          setStockFor('')
        }
      })
    return () => {
      alive = false
    }
  }, [supplierId, idsKey, stockKey])

  function stockLabel(r: Row) {
    if (!r.product || !supplierId) return ''
    if (stockFor !== stockKey) return 'checking...'
    return `${stockMap[r.product.id] ?? 0} at supplier`
  }

  function newRow(): Row {
    return { key: keyRef.current++, product: null, qty: '' }
  }
  function updateRow(key: number, patch: Partial<Row>) {
    setRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch, error: undefined } : r)))
  }
  function removeRow(key: number) {
    setRows((prev) => (prev.length === 1 ? [newRow()] : prev.filter((r) => r.key !== key)))
  }
  function toggleCreate() {
    if (!showCreate) {
      setRows([newRow()])
      setFormError(null)
    }
    setShowCreate((v) => !v)
  }

  async function act(id: number, fn: (id: number) => Promise<unknown>, failMsg: string) {
    setActingId(id)
    try {
      await fn(id)
      await load(true)
    } catch (err: any) {
      alert(err.response?.data?.error ?? failMsg)
    } finally {
      setActingId(null)
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)
    setNotice(null)

    const sid = parseInt(supplierId, 10)
    if (!sid) {
      setFormError('Choose a supplying warehouse.')
      return
    }
    if (sid === myWarehouseId) {
      setFormError('The supplying warehouse cannot be your own store.')
      return
    }
    if (!rows.some((r) => r.product)) {
      setFormError('Add at least one product.')
      return
    }

    const seen = new Set<number>()
    const checked = rows.map((r) => {
      if (!r.product) return r
      const q = parseInt(r.qty, 10)
      const available = stockMap[r.product.id] ?? 0
      let err: string | undefined
      if (!q || q <= 0) err = 'Enter a quantity'
      else if (seen.has(r.product.id)) err = 'Product added twice'
      else if (stockFor === stockKey && q > available) err = `Only ${available} available at supplier`
      seen.add(r.product.id)
      return { ...r, error: err }
    })
    if (checked.some((r) => r.error)) {
      setRows(checked)
      setFormError('Fix the highlighted rows.')
      return
    }

    setIsSaving(true)
    const failed: Row[] = []
    let ok = 0
    for (const r of checked.filter((x) => x.product)) {
      try {
        await requestStockTransfer({
          product_id: r.product!.id,
          to_warehouse_id: sid,
          quantity: parseInt(r.qty, 10),
        })
        ok++
      } catch (err: any) {
        failed.push({ ...r, error: err.response?.data?.error ?? 'Failed to send this request' })
      }
    }
    setIsSaving(false)
    await load(true)
    if (failed.length === 0) {
      setShowCreate(false)
      setRows([newRow()])
      setNotice(`${ok} stock request${ok !== 1 ? 's' : ''} sent.`)
    } else {
      setRows(failed)
      setFormError(`${ok} sent, ${failed.length} failed. The failed products are still in the list.`)
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold">Stock Requests</h1>
          <p className="text-sm text-slate-400 mt-1">
            Request stock from a supplying warehouse, then mark it received when it arrives.
          </p>
        </div>
        <button
          onClick={toggleCreate}
          className="px-4 py-2 rounded-lg bg-red-500 hover:bg-red-400 text-white text-sm font-medium transition-colors"
        >
          {showCreate ? 'Cancel' : '+ Request truck'}
        </button>
      </div>

      {notice && <p className="text-emerald-400 text-sm mb-4">{notice}</p>}

      {showCreate && (
        <div className="border border-slate-800 rounded-xl p-5 bg-slate-900 mb-6 max-w-2xl">
          <h2 className="text-sm font-medium mb-1">Request stock for your store</h2>
          <p className="text-xs text-slate-500 mb-4">
            Stock will be sent to your store ({staff?.warehouse?.name ?? `#${myWarehouseId}`}). The supplying
            warehouse has to approve each product first.
          </p>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="text-xs text-slate-400 block mb-1">Supplying warehouse</label>
              {suppliers.length === 0 ? (
                <p className="text-xs text-amber-400">
                  No supplying warehouse found. Ask the admin to add a Mother Warehouse.
                </p>
              ) : (
                <select
                  value={supplierId}
                  onChange={(e) => setSupplierId(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="">Select a warehouse</option>
                  {suppliers.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.city})
                    </option>
                  ))}
                </select>
              )}
            </div>

            <div className="space-y-3">
              <div className="grid grid-cols-[1fr_96px_120px_28px] gap-2 text-xs text-slate-400">
                <span>Product</span>
                <span>Quantity</span>
                <span>Supplier stock</span>
                <span></span>
              </div>
              {rows.map((r) => (
                <div key={r.key}>
                  <div className="grid grid-cols-[1fr_96px_120px_28px] gap-2 items-center">
                    <ProductPicker
                      value={r.product}
                      onPick={(p) => updateRow(r.key, { product: p })}
                      onClear={() => updateRow(r.key, { product: null })}
                    />
                    <input
                      type="number"
                      min={1}
                      value={r.qty}
                      onChange={(e) => updateRow(r.key, { qty: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                    />
                    <span className="text-xs text-slate-400">{stockLabel(r)}</span>
                    <button
                      type="button"
                      onClick={() => removeRow(r.key)}
                      className="text-slate-500 hover:text-red-400"
                      title="Remove"
                    >
                      &#10005;
                    </button>
                  </div>
                  {r.error && <p className="text-red-400 text-xs mt-1">{r.error}</p>}
                </div>
              ))}
              <button
                type="button"
                onClick={() => setRows((prev) => [...prev, newRow()])}
                className="text-xs text-red-400 hover:text-red-300"
              >
                + Add product
              </button>
            </div>

            {formError && <p className="text-red-400 text-xs">{formError}</p>}
            <button
              type="submit"
              disabled={isSaving || suppliers.length === 0}
              className="w-full py-2 rounded-lg bg-red-500 hover:bg-red-400 text-white text-sm font-medium transition-colors disabled:opacity-50"
            >
              {isSaving ? 'Sending...' : 'Request truck'}
            </button>
          </form>
        </div>
      )}

      {isLoading && <p className="text-slate-400">Loading...</p>}
      {error && <p className="text-red-400">{error}</p>}

      {!isLoading && !error && transfers.length === 0 && (
        <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
          No stock requests yet.
        </div>
      )}

      {!isLoading && transfers.length > 0 && (
        <div className="border border-slate-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-slate-900 text-slate-400 text-left">
                <th className="px-4 py-3 font-medium">Product</th>
                <th className="px-4 py-3 font-medium">Supplier</th>
                <th className="px-4 py-3 font-medium">Store</th>
                <th className="px-4 py-3 font-medium">Quantity</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {transfers.map((t) => {
                const iSupply = t.from_warehouse_id === myWarehouseId
                const iRequested = t.to_warehouse_id === myWarehouseId
                const busy = actingId === t.id
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
                        {STATUS_LABEL[t.status] ?? t.status}
                      </span>
                      <p className="text-[11px] text-slate-500 mt-1">
                        {iRequested ? 'You requested' : iSupply ? 'You supply' : ''}
                      </p>
                    </td>
                    <td className="px-4 py-3 text-right space-x-3">
                      {iSupply && t.status === 'pending' && (
                        <>
                          <button
                            onClick={() => act(t.id, approveStockTransfer, 'Failed to approve request.')}
                            disabled={busy}
                            className="text-emerald-400 hover:text-emerald-300 text-xs disabled:opacity-50"
                          >
                            {busy ? 'Working...' : 'Approve'}
                          </button>
                          <button
                            onClick={() => act(t.id, rejectStockTransfer, 'Failed to reject request.')}
                            disabled={busy}
                            className="text-red-400 hover:text-red-300 text-xs disabled:opacity-50"
                          >
                            Reject
                          </button>
                        </>
                      )}
                      {iRequested && t.status === 'in_transit' && (
                        <button
                          onClick={() => act(t.id, receiveStockTransfer, 'Failed to receive stock.')}
                          disabled={busy}
                          className="text-emerald-400 hover:text-emerald-300 text-xs disabled:opacity-50"
                        >
                          {busy ? 'Receiving...' : 'Truck arrived'}
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