import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import Modal from '../components/Modal'
import {
  listPurchaseOrders,
  createPurchaseOrder,
  updatePurchaseOrderStatus,
  receivePurchaseOrderItems,
  listSuppliers,
  listWarehouses,
  listProducts,
} from '../api/admin'
import type { PurchaseOrder, Supplier, Warehouse } from '../types/admin'

const STATUS_STYLES: Record<string, string> = {
  draft: 'bg-slate-700 text-slate-300',
  sent: 'bg-blue-500/15 text-blue-300',
  confirmed: 'bg-indigo-500/15 text-indigo-300',
  partially_received: 'bg-amber-500/15 text-amber-300',
  received: 'bg-emerald-500/15 text-emerald-400',
  cancelled: 'bg-red-500/15 text-red-300',
}

const NEXT_STATUS: Record<string, string[]> = {
  draft: ['sent', 'cancelled'],
  sent: ['confirmed', 'cancelled'],
  confirmed: ['cancelled'],
  partially_received: ['cancelled'],
  received: [],
  cancelled: [],
}

type ItemRow = { product_id: string; quantity_ordered: string; unit_price: string }

export default function PurchaseOrders() {
  const [orders, setOrders] = useState<PurchaseOrder[]>([])
  const [suppliers, setSuppliers] = useState<Supplier[]>([])
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [products, setProducts] = useState<any[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('')

  const [showCreate, setShowCreate] = useState(false)
  const [vendorId, setVendorId] = useState('')
  const [warehouseId, setWarehouseId] = useState('')
  const [expectedDate, setExpectedDate] = useState('')
  const [notes, setNotes] = useState('')
  const [items, setItems] = useState<ItemRow[]>([{ product_id: '', quantity_ordered: '', unit_price: '' }])
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const [receivingPO, setReceivingPO] = useState<PurchaseOrder | null>(null)
  const [receiveQtys, setReceiveQtys] = useState<Record<number, string>>({})

  async function loadOrders() {
    setIsLoading(true)
    setError(null)
    try {
      const params: Record<string, any> = {}
      if (statusFilter) params.status = statusFilter
      const res = await listPurchaseOrders(params)
      setOrders(res.purchase_orders ?? res ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load purchase orders.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadRefs() {
    try {
      const [supRes, whRes, prodRes] = await Promise.all([listSuppliers(), listWarehouses(), listProducts()])
      setSuppliers(supRes.vendors ?? supRes.suppliers ?? supRes ?? [])
      setWarehouses(whRes.warehouses ?? whRes ?? [])
      setProducts(prodRes.products ?? prodRes ?? [])
    } catch {
      // non-fatal
    }
  }

  useEffect(() => {
    loadRefs()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    loadOrders()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter])

  function openCreate() {
    setVendorId('')
    setWarehouseId('')
    setExpectedDate('')
    setNotes('')
    setItems([{ product_id: '', quantity_ordered: '', unit_price: '' }])
    setFormError(null)
    setShowCreate(true)
  }

  function addItemRow() {
    setItems([...items, { product_id: '', quantity_ordered: '', unit_price: '' }])
  }

  function removeItemRow(idx: number) {
    setItems(items.filter((_, i) => i !== idx))
  }

  function updateItemRow(idx: number, field: keyof ItemRow, value: string) {
    setItems(items.map((it, i) => (i === idx ? { ...it, [field]: value } : it)))
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)

    if (!vendorId || !warehouseId) {
      setFormError('Supplier and warehouse are required.')
      return
    }
    const validItems = items.filter((it) => it.product_id && it.quantity_ordered)
    if (validItems.length === 0) {
      setFormError('Add at least one product line.')
      return
    }

    setIsSaving(true)
    try {
      await createPurchaseOrder({
        vendor_id: parseInt(vendorId, 10),
        warehouse_id: parseInt(warehouseId, 10),
        expected_date: expectedDate || undefined,
        notes: notes || undefined,
        items: validItems.map((it) => ({
          product_id: parseInt(it.product_id, 10),
          quantity_ordered: parseInt(it.quantity_ordered, 10),
          unit_price: parseFloat(it.unit_price || '0'),
        })),
      })
      setShowCreate(false)
      loadOrders()
    } catch (err: any) {
      setFormError(err.response?.data?.error ?? 'Failed to create purchase order.')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleStatusChange(po: PurchaseOrder, status: string) {
    if (status === 'cancelled') {
      const reason = prompt('Reason for cancelling this PO?') ?? ''
      if (!confirm('Cancel this purchase order?')) return
      try {
        await updatePurchaseOrderStatus(po.id, status, reason)
        loadOrders()
      } catch (err: any) {
        alert(err.response?.data?.error ?? 'Failed to update status.')
      }
      return
    }
    try {
      await updatePurchaseOrderStatus(po.id, status)
      loadOrders()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update status.')
    }
  }

  function openReceive(po: PurchaseOrder) {
    const initial: Record<number, string> = {}
    ;(po.items ?? []).forEach((it) => {
      const remaining = it.quantity_ordered - it.quantity_received
      initial[it.id] = remaining > 0 ? String(remaining) : ''
    })
    setReceiveQtys(initial)
    setReceivingPO(po)
  }

  async function handleReceiveSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!receivingPO) return
    const payload = Object.entries(receiveQtys)
      .filter(([, v]) => v && parseInt(v, 10) > 0)
      .map(([itemId, v]) => ({ item_id: parseInt(itemId, 10), quantity_received: parseInt(v, 10) }))
    if (payload.length === 0) {
      alert('Enter at least one quantity to receive.')
      return
    }
    try {
      await receivePurchaseOrderItems(receivingPO.id, payload)
      setReceivingPO(null)
      loadOrders()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to record receiving.')
    }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Purchase Orders</h1>
            <p className="text-sm text-slate-400 mt-1">
              {orders.length} order{orders.length !== 1 ? 's' : ''}
            </p>
          </div>
          <button
            onClick={openCreate}
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            + New purchase order
          </button>
        </div>

        <div className="mb-6">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All statuses</option>
            <option value="draft">Draft</option>
            <option value="sent">Sent</option>
            <option value="confirmed">Confirmed</option>
            <option value="partially_received">Partially received</option>
            <option value="received">Received</option>
            <option value="cancelled">Cancelled</option>
          </select>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && orders.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No purchase orders yet.
          </div>
        )}

        {!isLoading && orders.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">PO #</th>
                  <th className="px-4 py-3 font-medium">Supplier</th>
                  <th className="px-4 py-3 font-medium">Warehouse</th>
                  <th className="px-4 py-3 font-medium">Items</th>
                  <th className="px-4 py-3 font-medium">Total</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {orders.map((po) => (
                  <tr key={po.id} className="border-t border-slate-800">
                    <td className="px-4 py-3 font-medium">{po.po_number}</td>
                    <td className="px-4 py-3 text-slate-400">{po.vendor?.name ?? '-'}</td>
                    <td className="px-4 py-3 text-slate-400">{po.warehouse?.name ?? '-'}</td>
                    <td className="px-4 py-3 text-slate-400">{po.items?.length ?? 0}</td>
                    <td className="px-4 py-3">₹{po.total_amount?.toLocaleString('en-IN') ?? 0}</td>
                    <td className="px-4 py-3">
                      <span className={`text-xs px-2 py-1 rounded-full ${STATUS_STYLES[po.status] ?? ''}`}>
                        {po.status.replace('_', ' ')}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right space-x-3">
                      {(NEXT_STATUS[po.status] ?? []).map((next) => (
                        <button
                          key={next}
                          onClick={() => handleStatusChange(po, next)}
                          className="text-indigo-400 hover:text-indigo-300 text-xs"
                        >
                          Mark {next.replace('_', ' ')}
                        </button>
                      ))}
                      {(po.status === 'sent' || po.status === 'confirmed' || po.status === 'partially_received') && (
                        <button
                          onClick={() => openReceive(po)}
                          className="text-emerald-400 hover:text-emerald-300 text-xs"
                        >
                          Receive
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showCreate && (
        <Modal title="New purchase order" onClose={() => setShowCreate(false)}>
          <form onSubmit={handleCreate} className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Supplier</label>
                <select
                  value={vendorId}
                  onChange={(e) => setVendorId(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="">Select supplier</option>
                  {suppliers.map((s) => (
                    <option key={s.id} value={s.id}>{s.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Warehouse</label>
                <select
                  value={warehouseId}
                  onChange={(e) => setWarehouseId(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="">Select warehouse</option>
                  {warehouses.map((w) => (
                    <option key={w.id} value={w.id}>{w.name}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Expected date</label>
                <input
                  type="date"
                  value={expectedDate}
                  onChange={(e) => setExpectedDate(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Notes</label>
                <input
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
            </div>

            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs text-slate-400">Products</label>
                <button
                  type="button"
                  onClick={addItemRow}
                  className="text-xs text-indigo-400 hover:text-indigo-300"
                >
                  + Add line
                </button>
              </div>
              <div className="space-y-2">
                {items.map((row, idx) => (
                  <div key={idx} className="grid grid-cols-12 gap-2 items-center">
                    <select
                      value={row.product_id}
                      onChange={(e) => updateItemRow(idx, 'product_id', e.target.value)}
                      className="col-span-6 bg-slate-800 border border-slate-700 rounded-lg px-2 py-2 text-sm"
                    >
                      <option value="">Product</option>
                      {products.map((p: any) => (
                        <option key={p.id} value={p.id}>{p.name}</option>
                      ))}
                    </select>
                    <input
                      type="number"
                      placeholder="Qty"
                      value={row.quantity_ordered}
                      onChange={(e) => updateItemRow(idx, 'quantity_ordered', e.target.value)}
                      className="col-span-2 bg-slate-800 border border-slate-700 rounded-lg px-2 py-2 text-sm"
                    />
                    <input
                      type="number"
                      step="any"
                      placeholder="Unit price"
                      value={row.unit_price}
                      onChange={(e) => updateItemRow(idx, 'unit_price', e.target.value)}
                      className="col-span-3 bg-slate-800 border border-slate-700 rounded-lg px-2 py-2 text-sm"
                    />
                    <button
                      type="button"
                      onClick={() => removeItemRow(idx)}
                      className="col-span-1 text-red-400 hover:text-red-300 text-xs"
                    >
                      ✕
                    </button>
                  </div>
                ))}
              </div>
            </div>

            {formError && <p className="text-red-400 text-xs">{formError}</p>}

            <button
              type="submit"
              disabled={isSaving}
              className="w-full py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors mt-2"
            >
              {isSaving ? 'Creating...' : 'Create purchase order'}
            </button>
          </form>
        </Modal>
      )}

      {receivingPO && (
        <Modal title={`Receive items - ${receivingPO.po_number}`} onClose={() => setReceivingPO(null)}>
          <form onSubmit={handleReceiveSubmit} className="space-y-3">
            {(receivingPO.items ?? []).map((it) => {
              const remaining = it.quantity_ordered - it.quantity_received
              return (
                <div key={it.id} className="flex items-center justify-between gap-3">
                  <div className="text-sm">
                    <p>{it.product?.name ?? `Product #${it.product_id}`}</p>
                    <p className="text-xs text-slate-500">
                      Ordered {it.quantity_ordered} · Received {it.quantity_received} · Remaining {remaining}
                    </p>
                  </div>
                  <input
                    type="number"
                    min={0}
                    max={remaining}
                    value={receiveQtys[it.id] ?? ''}
                    onChange={(e) => setReceiveQtys({ ...receiveQtys, [it.id]: e.target.value })}
                    className="w-24 bg-slate-800 border border-slate-700 rounded-lg px-2 py-2 text-sm"
                    disabled={remaining <= 0}
                  />
                </div>
              )
            })}
            <button
              type="submit"
              className="w-full py-2 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-white text-sm font-medium transition-colors mt-2"
            >
              Confirm receiving
            </button>
          </form>
        </Modal>
      )}
    </Layout>
  )
}
