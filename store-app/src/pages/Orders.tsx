import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { acceptOrder, listWarehouseOrders, getOrderInvoice, handoverOrder } from '../api/warehouse'
import type { Order, OrderStatus, OrderInvoice } from '../types/warehouse'
import StatusBadge from '../components/StatusBadge'
import { getErrorMessage } from '../utils/errors'

const TABS: { label: string; status?: OrderStatus }[] = [
  { label: 'New Orders', status: 'confirmed' },
  { label: 'Picking', status: 'picking' },
  { label: 'Packing', status: 'packing' },
  { label: 'Ready for Dispatch', status: 'ready_for_dispatch' },
  { label: 'Handed Over', status: 'handed_over' },
  { label: 'Completed', status: 'delivered' },
  { label: 'All', status: undefined },
]

// Where an order's action button should route the staff member.
function actionFor(order: Order): { label: string; onClick: () => void } | null {
  if (order.status === 'confirmed') return null // handled via Accept button
  if (order.status === 'picking' || order.status === 'picked') return { label: 'Go to Picking', onClick: () => {} }
  if (order.status === 'packing') return { label: 'Go to Packing', onClick: () => {} }
  return null
}

type PartnerRef = { delivery_partner_id?: number | null; delivery_partner?: { id?: number; name?: string } | null }

function partnerIdOf(order: Order): number | null {
  const o = order as unknown as PartnerRef
  return o.delivery_partner_id ?? o.delivery_partner?.id ?? null
}

export default function Orders() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeStatus = searchParams.get('status') ?? 'confirmed'
  const page = parseInt(searchParams.get('page') ?? '1', 10)

  const [orders, setOrders] = useState<Order[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [acceptingId, setAcceptingId] = useState<number | null>(null)
  const [handingId, setHandingId] = useState<number | null>(null)
  const [invoiceTarget, setInvoiceTarget] = useState<number | null>(null)
  const [invoice, setInvoice] = useState<OrderInvoice | null>(null)
  const [invoiceLoading, setInvoiceLoading] = useState(false)
  const [invoiceError, setInvoiceError] = useState<string | null>(null)

  const load = useCallback(async (silent?: boolean) => {
    if (silent !== true) setIsLoading(true)
    setError(null)
    try {
      const params: { status?: string; page: number; limit: number } = { page, limit: 20 }
      const tab = TABS.find((t) => (t.status ?? 'all') === activeStatus)
      if (tab?.status) params.status = tab.status
      const data = await listWarehouseOrders(params)
      setOrders(data.orders ?? [])
      setTotalPages(data.total_pages || 1)
      setTotal(data.total)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load orders.'))
    } finally {
      setIsLoading(false)
    }
  }, [activeStatus, page])

  useEffect(() => {
    load()
    const t = setInterval(() => load(true), 5000)
    return () => clearInterval(t)
  }, [load])

  function setTab(status?: OrderStatus) {
    setSearchParams(status ? { status, page: '1' } : { status: 'all', page: '1' })
  }

  function setPage(p: number) {
    setSearchParams({ status: activeStatus, page: String(p) })
  }

  async function handleAccept(order: Order) {
    setAcceptingId(order.id)
    setError(null)
    try {
      await acceptOrder(order.id)
      await load()
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to accept order.'))
    } finally {
      setAcceptingId(null)
    }
  }

  async function handleHandover(order: Order) {
    const partnerId = partnerIdOf(order)
    if (!partnerId) {
      setError('No delivery partner assigned to this order yet.')
      return
    }
    const input = window.prompt('Number of packages handed over to the partner?', '1')
    if (input === null) return
    const count = parseInt(input, 10)
    if (!Number.isFinite(count) || count < 1) {
      setError('Package count must be at least 1.')
      return
    }
    setHandingId(order.id)
    setError(null)
    try {
      await handoverOrder(order.id, { package_count: count, delivery_partner_id: partnerId })
      await load(true)
    } catch (err) {
      setError(getErrorMessage(err, 'Handover failed.'))
    } finally {
      setHandingId(null)
    }
  }

  function goToTask(order: Order) {
    if (order.status === 'picking' || order.status === 'picked') navigate(`/picking/${order.id}`)
    else if (order.status === 'packing' || order.status === 'packed') navigate(`/packing/${order.id}`)
  }

  async function openInvoice(order: Order) {
    setInvoiceTarget(order.id)
    setInvoice(null)
    setInvoiceError(null)
    setInvoiceLoading(true)
    try {
      const data = await getOrderInvoice(order.id)
      setInvoice(data)
    } catch (err) {
      setInvoiceError(getErrorMessage(err, 'Failed to load invoice.'))
    } finally {
      setInvoiceLoading(false)
    }
  }

  return (
    <div className="p-6 max-w-6xl">
      <h1 className="font-display text-2xl font-semibold mb-4">Orders</h1>

      <div className="flex gap-1 border-b border-slate-800 mb-4 overflow-x-auto">
        {TABS.map((tab) => {
          const key = tab.status ?? 'all'
          const isActive = key === activeStatus
          return (
            <button
              key={key}
              onClick={() => setTab(tab.status)}
              className={`px-3 py-2 text-sm whitespace-nowrap border-b-2 transition-colors ${
                isActive
                  ? 'border-red-400 text-red-300 font-medium'
                  : 'border-transparent text-slate-400 hover:text-slate-200'
              }`}
            >
              {tab.label}
            </button>
          )
        })}
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

      {isLoading ? (
        <p className="text-sm text-slate-400">Loading orders...</p>
      ) : orders.length === 0 ? (
        <div className="border border-slate-800 rounded-xl bg-slate-900 p-10 text-center">
          <p className="text-sm text-slate-400">No orders in this view.</p>
        </div>
      ) : (
        <div className="border border-slate-800 rounded-xl bg-slate-900 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-900/60 border-b border-slate-800 text-left text-xs text-slate-400 uppercase tracking-wide">
              <tr>
                <th className="px-4 py-3">Order</th>
                <th className="px-4 py-3">Created</th>
                <th className="px-4 py-3">Items</th>
                <th className="px-4 py-3">Total</th>
                <th className="px-4 py-3">Payment</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3 text-right">Action</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => {
                const itemCount = order.items?.length ?? 0
                const totalQty = order.items?.reduce((sum, i) => sum + i.quantity, 0) ?? 0
                const action = actionFor(order)
                return (
                  <tr key={order.id} className="border-b border-slate-800 last:border-0 hover:bg-slate-800/40">
                    <td className="px-4 py-3 font-medium">#{order.id}</td>
                    <td className="px-4 py-3 text-slate-400">
                      {new Date(order.created_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-slate-400">
                      {itemCount} lines &middot; {totalQty} qty
                    </td>
                    <td className="px-4 py-3">₹{order.total_amount.toFixed(2)}</td>
                    <td className="px-4 py-3 text-slate-400 uppercase text-xs">
                      {order.payment_method} &middot; {order.payment_status}
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={order.status} />
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex justify-end items-center gap-2">
                        {order.status !== 'confirmed' && order.status !== 'pending' && (
                          <button
                            onClick={() => openInvoice(order)}
                            className="px-2.5 py-1.5 rounded-lg border border-slate-700 hover:bg-slate-800 text-xs font-medium transition-colors text-slate-300"
                          >
                            Invoice
                          </button>
                        )}
                        {order.status === 'confirmed' ? (
                        <button
                          onClick={() => handleAccept(order)}
                          disabled={acceptingId === order.id}
                          className="px-3 py-1.5 rounded-lg bg-red-500 hover:bg-red-400 text-white text-xs font-medium transition-colors disabled:opacity-50"
                        >
                          {acceptingId === order.id ? 'Accepting...' : 'Accept'}
                        </button>
                      ) : order.status === 'ready_for_dispatch' ? (
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-amber-300">
                            {order.delivery_partner ? order.delivery_partner.name : 'No partner assigned'}
                          </span>
                          <button
                            onClick={() => handleHandover(order)}
                            disabled={handingId === order.id || !partnerIdOf(order)}
                            className="px-3 py-1.5 rounded-lg bg-emerald-500/20 text-emerald-300 hover:bg-emerald-500/30 text-xs font-medium transition-colors disabled:opacity-40"
                          >
                            {handingId === order.id ? 'Handing over...' : 'Hand over'}
                          </button>
                        </div>
                      ) : order.status === 'handed_over' ? (
                        <span className="text-xs text-emerald-300">
                          Picked up{order.delivery_partner ? ` by ${order.delivery_partner.name}` : ''}
                        </span>
                      ) : action ? (
                        <button
                          onClick={() => goToTask(order)}
                          className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium transition-colors"
                        >
                          {action.label}
                        </button>
                        ) : (
                          <span className="text-xs text-slate-600">&mdash;</span>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {!isLoading && orders.length > 0 && (
        <div className="flex items-center justify-between mt-4 text-xs text-slate-400">
          <span>
            Page {page} of {totalPages} &middot; {total} total
          </span>
          <div className="flex gap-2">
            <button
              onClick={() => setPage(page - 1)}
              disabled={page <= 1}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 disabled:opacity-40 transition-colors"
            >
              Previous
            </button>
            <button
              onClick={() => setPage(page + 1)}
              disabled={page >= totalPages}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 disabled:opacity-40 transition-colors"
            >
              Next
            </button>
          </div>
        </div>
      )}

      {invoiceTarget !== null && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-lg max-h-[85vh] flex flex-col overflow-hidden">
            <div className="flex items-center justify-between p-6 pb-4 border-b border-slate-800 shrink-0">
              <h2 className="text-base font-semibold">Invoice â€” Order #{invoiceTarget}</h2>
              <button
                onClick={() => setInvoiceTarget(null)}
                className="text-slate-400 hover:text-slate-200 text-sm"
              >
                Close
              </button>
            </div>

            <div className="p-6 pt-4 overflow-y-auto">
            {invoiceLoading ? (
              <p className="text-sm text-slate-400">Loading invoice...</p>
            ) : invoiceError ? (
              <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-3 py-2">
                {invoiceError}
              </div>
            ) : invoice ? (
              <div className="text-sm space-y-4">
                <div className="flex items-center justify-between border border-slate-800 rounded-lg p-3">
                  <div>
                    <p className="text-slate-400 text-xs uppercase mb-1">Invoice Number</p>
                    <p className="font-medium">{invoice.invoice_number}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-slate-400 text-xs uppercase mb-1">Payment</p>
                    <p className="font-medium uppercase text-xs">
                      {invoice.payment_method} &middot; {invoice.payment_status}
                    </p>
                  </div>
                </div>

                <div>
                  <p className="text-slate-400 text-xs uppercase mb-2">Items</p>
                  <div className="border border-slate-800 rounded-lg divide-y divide-slate-800">
                    {invoice.items.map((item) => (
                      <div key={item.id} className="flex justify-between px-3 py-2">
                        <span>
                          {item.product_name} <span className="text-slate-500">x{item.quantity}</span>
                        </span>
                        <span>₹{(item.price * item.quantity).toFixed(2)}</span>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="border border-slate-800 rounded-lg p-3 space-y-1">
                  <div className="flex justify-between text-slate-400">
                    <span>Items Amount</span>
                    <span>₹{invoice.items_amount.toFixed(2)}</span>
                  </div>
                  <div className="flex justify-between text-slate-400">
                    <span>Delivery Charge</span>
                    <span>₹{invoice.delivery_charge.toFixed(2)}</span>
                  </div>
                  {invoice.wallet_used > 0 && (
                    <div className="flex justify-between text-slate-400">
                      <span>Wallet Used</span>
                      <span>-₹{invoice.wallet_used.toFixed(2)}</span>
                    </div>
                  )}
                  <div className="flex justify-between font-semibold pt-1 border-t border-slate-800">
                    <span>Total</span>
                    <span>₹{invoice.total_amount.toFixed(2)}</span>
                  </div>
                </div>

                <p className="text-xs text-slate-500">
                  Generated {new Date(invoice.generated_at).toLocaleString()}. Warehouse staff cannot edit
                  invoice values.
                </p>
              </div>
            ) : null}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}



