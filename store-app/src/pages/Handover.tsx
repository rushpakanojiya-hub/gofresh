import { useCallback, useEffect, useState } from 'react'
import { listWarehouseOrders } from '../api/warehouse'
import type { Order } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

export default function Handover() {
  const [waiting, setWaiting] = useState<Order[]>([])
  const [pickedUp, setPickedUp] = useState<Order[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (silent?: boolean) => {
    if (silent !== true) setIsLoading(true)
    setError(null)
    try {
      const [a, b] = await Promise.all([
        listWarehouseOrders({ status: 'ready_for_dispatch', page: 1, limit: 50 }),
        listWarehouseOrders({ status: 'handed_over', page: 1, limit: 20 }),
      ])
      setWaiting(a.orders ?? [])
      setPickedUp(b.orders ?? [])
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load handover orders.'))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const t = setInterval(() => load(true), 5000)
    return () => clearInterval(t)
  }, [load])

  function renderTable(rows: Order[], done: boolean) {
    return (
      <div className="border border-slate-800 rounded-xl bg-slate-900 overflow-hidden mb-8">
        <table className="w-full text-sm">
          <thead className="bg-slate-800/50 text-slate-400 text-xs uppercase">
            <tr>
              <th className="text-left px-4 py-2.5">Order</th>
              <th className="text-left px-4 py-2.5">Address</th>
              <th className="text-left px-4 py-2.5">Delivery Partner</th>
              <th className="text-left px-4 py-2.5">Payment</th>
              <th className="text-right px-4 py-2.5">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800">
            {rows.map((order) => (
              <tr key={order.id} className="hover:bg-slate-800/30">
                <td className="px-4 py-3">#{order.id}</td>
                <td className="px-4 py-3 text-slate-300">
                  {order.address ? `${order.address.city ?? ''}, ${order.address.pincode ?? ''}` : '-'}
                </td>
                <td className="px-4 py-3">
                  {order.delivery_partner ? (
                    <span className="text-slate-300">{order.delivery_partner.name}</span>
                  ) : (
                    <span className="text-rose-400 text-xs">Not assigned</span>
                  )}
                </td>
                <td className="px-4 py-3 text-slate-400 uppercase text-xs">{order.payment_method}</td>
                <td className="px-4 py-3 text-right">
                  {done ? (
                    <span className="text-xs text-emerald-300">Picked up</span>
                  ) : (
                    <span className="text-xs text-amber-300">Waiting for partner</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-5xl">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="font-display text-2xl font-semibold">Handover</h1>
          <p className="text-xs text-slate-500 mt-1">
            Delivery partners pick up ready orders themselves. This page updates automatically.
          </p>
        </div>
        <button
          onClick={() => load()}
          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

      {isLoading && <p className="text-sm text-slate-400">Loading orders...</p>}

      {!isLoading && (
        <>
          <h2 className="text-sm font-semibold text-slate-300 mb-2">Waiting for partner ({waiting.length})</h2>
          {waiting.length === 0 ? (
            <div className="border border-slate-800 rounded-xl bg-slate-900 p-6 text-center text-sm text-slate-500 mb-8">
              No orders are waiting for a delivery partner.
            </div>
          ) : (
            renderTable(waiting, false)
          )}

          <h2 className="text-sm font-semibold text-slate-300 mb-2">Picked up by partner ({pickedUp.length})</h2>
          {pickedUp.length === 0 ? (
            <div className="border border-slate-800 rounded-xl bg-slate-900 p-6 text-center text-sm text-slate-500">
              No recent pickups.
            </div>
          ) : (
            renderTable(pickedUp, true)
          )}
        </>
      )}
    </div>
  )
}