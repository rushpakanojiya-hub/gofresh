import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { listOrders, updateOrderStatus, assignDeliveryPartner, listDeliveryPartners } from '../api/admin'
import type { Order, DeliveryPartner } from '../types/admin'

const STATUS_OPTIONS = [
  'pending',
  'confirmed',
  'picking',
  'picked',
  'packing',
  'packed',
  'ready_for_dispatch',
  'handed_over',
  'shipped',
  'delivered',
  'returned',
  'cancelled',
]

const STATUS_COLORS: Record<string, string> = {
  pending: 'bg-amber-500/15 text-amber-300',
  confirmed: 'bg-blue-500/15 text-blue-300',
  picking: 'bg-sky-500/15 text-sky-300',
  picked: 'bg-sky-500/15 text-sky-300',
  packing: 'bg-cyan-500/15 text-cyan-300',
  packed: 'bg-cyan-500/15 text-cyan-300',
  ready_for_dispatch: 'bg-violet-500/15 text-violet-300',
  handed_over: 'bg-purple-500/15 text-purple-300',
  shipped: 'bg-indigo-500/15 text-indigo-300',
  delivered: 'bg-emerald-500/15 text-emerald-300',
  returned: 'bg-orange-500/15 text-orange-300',
  cancelled: 'bg-red-500/15 text-red-300',
}

export default function Orders() {
  const [orders, setOrders] = useState<Order[]>([])
  const [partners, setPartners] = useState<DeliveryPartner[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [assigningId, setAssigningId] = useState<number | null>(null)
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const [ordersRes, partnersRes] = await Promise.all([
        listOrders({ page, limit: 20, ...(statusFilter ? { status: statusFilter } : {}) }),
        listDeliveryPartners(),
      ])
      setOrders(ordersRes.orders ?? [])
      setTotalPages(ordersRes.total_pages ?? 1)
      setTotal(ordersRes.total ?? (ordersRes.orders ?? []).length)
      setPartners(partnersRes.delivery_partners ?? partnersRes.partners ?? partnersRes ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load orders.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter, page])

  useEffect(() => {
    setPage(1)
  }, [statusFilter])

  async function handleStatusChange(id: number, status: string) {
    try {
      await updateOrderStatus(id, status)
      setOrders((prev) => prev.map((o) => (o.id === id ? { ...o, status } : o)))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update order status.')
    }
  }

  async function handleAssignDelivery(orderId: number, partnerId: string) {
    if (!partnerId) return
    const deliveryPartnerId = parseInt(partnerId, 10)
    setAssigningId(orderId)
    try {
      await assignDeliveryPartner(orderId, deliveryPartnerId)
      setOrders((prev) =>
        prev.map((o) => (o.id === orderId ? { ...o, delivery_partner_id: deliveryPartnerId } : o)),
      )
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to assign delivery partner.')
    } finally {
      setAssigningId(null)
    }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Orders</h1>
            <p className="text-sm text-slate-400 mt-1">
              {total} order{total !== 1 ? 's' : ''}
            </p>
          </div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All statuses</option>
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && orders.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No orders {statusFilter ? `with status "${statusFilter}"` : 'yet'}.
          </div>
        )}

        {!isLoading && orders.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Order</th>
                  <th className="px-4 py-3 font-medium">User</th>
                  <th className="px-4 py-3 font-medium">Products</th>
                  <th className="px-4 py-3 font-medium">Total</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Payment</th>
                  <th className="px-4 py-3 font-medium">Update</th>
                  <th className="px-4 py-3 font-medium">Assign Delivery</th>
                  <th className="px-4 py-3 font-medium">Delivery Proof</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((o) => (
                  <tr key={o.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">#{o.id}</td>
                    <td className="px-4 py-3">User {o.user_id}</td>
                    <td className="px-4 py-3 max-w-xs">
                      {o.items && o.items.length > 0 ? (
                        <div className="space-y-0.5">
                          {o.items.map((it) => (
                            <div key={it.id} className="text-slate-300">
                              {it.product?.name ?? `Product #${it.product_id}`}
                              <span className="text-slate-500"> × {it.quantity}</span>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <span className="text-xs text-slate-500">No items</span>
                      )}
                    </td>
                    <td className="px-4 py-3">₹{o.total_amount}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`px-2 py-1 rounded-md text-xs font-medium ${
                          STATUS_COLORS[o.status] ?? 'bg-slate-700 text-slate-300'
                        }`}
                      >
                        {o.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`text-xs px-2 py-0.5 rounded-md ${o.payment_status === "paid" ? "bg-emerald-500/15 text-emerald-300" : "bg-amber-500/15 text-amber-300"}`}>
                        {o.payment_status ?? "-"} ({o.payment_method ?? "-"})
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <select
                        defaultValue=""
                        onChange={(e) => {
                          if (e.target.value) handleStatusChange(o.id, e.target.value)
                        }}
                        className="bg-slate-800 border border-slate-700 rounded-md px-2 py-1 text-xs"
                      >
                        <option value="">Change status...</option>
                        {STATUS_OPTIONS.map((s) => (
                          <option key={s} value={s}>
                            {s}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-4 py-3">
                      {(o.status === 'confirmed' || o.status === 'shipped' || o.status === 'ready_for_dispatch' || o.status === 'delivered') ? (
                        <div className="space-y-1">
                          {o.delivery_partner_id ? (
                            <div className={o.delivery_status === 'assigned' ? "text-xs text-amber-300" : "text-xs text-emerald-300"}>
                              {o.delivery_status === 'assigned' ? 'Requested: ' : 'Assigned: '}
                              {partners.find((p) => p.id === o.delivery_partner_id)?.name ?? `Partner #${o.delivery_partner_id}`}
                              {o.delivery_status === 'assigned' ? ' (awaiting response)' : ''}
                            </div>
                          ) : null}
                          <select
                            value={o.delivery_partner_id ?? ''}
                            disabled={assigningId === o.id}
                            onChange={(e) => handleAssignDelivery(o.id, e.target.value)}
                            className="bg-slate-800 border border-slate-700 rounded-md px-2 py-1 text-xs disabled:opacity-50"
                          >
                            <option value="">
                              {assigningId === o.id ? 'Assigning...' : o.delivery_partner_id ? 'Reassign partner...' : 'Assign partner...'}
                            </option>
                            {partners.map((p) => (
                              <option key={p.id} value={p.id}>
                                {p.name} ({p.phone})
                              </option>
                            ))}
                          </select>
                        </div>
                      ) : (
                        <span className="text-xs text-slate-500">-</span>
                      )}
                    </td>                    <td className="px-4 py-3">
                      {o.delivery_proof_url ? (
                        <a href={o.delivery_proof_url} target="_blank" rel="noreferrer">
                          <img
                            src={o.delivery_proof_url}
                            alt="Delivery proof"
                            className="h-12 w-12 rounded object-cover border border-slate-700"
                          />
                        </a>
                      ) : (
                        <span className="text-xs text-slate-500">-</span>
                      )}
                    </td>

                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {!isLoading && totalPages > 1 && (
          <div className="flex items-center justify-between mt-4 text-sm text-slate-400">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="px-3 py-1.5 rounded-md border border-slate-700 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-800"
            >
              Previous
            </button>
            <span>
              Page {page} of {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="px-3 py-1.5 rounded-md border border-slate-700 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-800"
            >
              Next
            </button>
          </div>
        )}      </div>
    </Layout>
  )
}

