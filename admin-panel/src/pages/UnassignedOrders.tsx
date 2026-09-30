import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { getUnassignedOrders, listDeliveryPartners, assignDeliveryPartner } from '../api/admin'
import type { UnassignedOrder } from '../types/admin'

export default function UnassignedOrders() {
  const [orders, setOrders] = useState<UnassignedOrder[]>([])
  const [count, setCount] = useState(0)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [partners, setPartners] = useState<any[]>([])

  const [assigningOrderId, setAssigningOrderId] = useState<number | null>(null)
  const [selectedPartnerId, setSelectedPartnerId] = useState('')
  const [isAssigning, setIsAssigning] = useState(false)
  const [assignError, setAssignError] = useState<string | null>(null)

  const [toast, setToast] = useState<{ kind: 'success' | 'error'; message: string } | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await getUnassignedOrders()
      setOrders(res.orders ?? [])
      setCount(res.unassigned_count ?? 0)
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load unassigned orders.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadPartners() {
    try {
      const res = await listDeliveryPartners()
      const list = res.delivery_partners ?? res.partners ?? res ?? []
      // Only show partners who are active and online - offline/inactive
      // ones would just get rejected by the backend anyway.
      setPartners(list.filter((p: any) => p.is_active && p.is_online))
    } catch {
      // non-fatal - dropdown just won't populate
    }
  }

  useEffect(() => {
    load()
    loadPartners()
  }, [])

  function showToast(kind: 'success' | 'error', message: string) {
    setToast({ kind, message })
    setTimeout(() => setToast(null), 4000)
  }

  function openAssignModal(orderId: number) {
    setAssigningOrderId(orderId)
    setSelectedPartnerId('')
    setAssignError(null)
  }

  async function confirmAssign() {
    if (!assigningOrderId || !selectedPartnerId) return
    setIsAssigning(true)
    setAssignError(null)
    try {
      await assignDeliveryPartner(assigningOrderId, Number(selectedPartnerId))
      showToast('success', `Order #${assigningOrderId} assigned successfully.`)
      setAssigningOrderId(null)
      await load()
    } catch (err: any) {
      setAssignError(err.response?.data?.error ?? 'Failed to assign delivery partner.')
    } finally {
      setIsAssigning(false)
    }
  }

  function formatDate(value: string | null) {
    if (!value) return '-'
    return new Date(value).toLocaleString()
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Unassigned Orders</h1>
            <p className="text-sm text-slate-400 mt-1">
              Orders where every delivery partner rejected or the offer expired - need manual assignment.
            </p>
          </div>
          <div className="border border-slate-800 rounded-xl px-4 py-2 bg-slate-900">
            <p className="text-xs text-slate-500">Unassigned</p>
            <p className="text-lg font-semibold text-red-300">{count}</p>
          </div>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && orders.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No unassigned orders right now.
          </div>
        )}

        {!isLoading && orders.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Order</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Assignment</th>
                  <th className="px-4 py-3 font-medium">Warehouse</th>
                  <th className="px-4 py-3 font-medium">Area</th>
                  <th className="px-4 py-3 font-medium">Attempts</th>
                  <th className="px-4 py-3 font-medium">Expired/Created</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {orders.map((o) => (
                  <tr key={o.order_id} className="border-t border-slate-800">
                    <td className="px-4 py-3 text-slate-200">#{o.order_id}</td>
                    <td className="px-4 py-3 text-slate-400">{o.order_status.replace('_', ' ')}</td>
                    <td className="px-4 py-3">
                      <span
                        className={
                          'px-2 py-0.5 rounded-md text-xs font-medium ' +
                          (o.delivery_assignment_status === 'expired'
                            ? 'bg-amber-500/15 text-amber-300'
                            : 'bg-red-500/15 text-red-300')
                        }
                      >
                        {o.delivery_assignment_status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-400">{o.warehouse}</td>
                    <td className="px-4 py-3 text-slate-400">{o.customer_area}</td>
                    <td className="px-4 py-3 text-slate-400">{o.attempted_partner_count}</td>
                    <td className="px-4 py-3 text-slate-500 text-xs">
                      {formatDate(o.assignment_expiry)}
                      <br />
                      <span className="text-slate-600">created {formatDate(o.created_at)}</span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => openAssignModal(o.order_id)}
                        className="text-xs px-3 py-1.5 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white font-medium transition-colors"
                      >
                        Assign
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {assigningOrderId && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 w-full max-w-sm">
            <h3 className="text-sm font-semibold text-slate-200 mb-1">Assign delivery partner</h3>
            <p className="text-xs text-slate-500 mb-4">Order #{assigningOrderId}</p>

            <label className="text-xs text-slate-400 block mb-1">Delivery partner</label>
            <select
              value={selectedPartnerId}
              onChange={(e) => setSelectedPartnerId(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-1"
            >
              <option value="">Select a partner...</option>
              {partners.map((p: any) => (
                <option key={p.id} value={p.id}>
                  {p.name || p.phone}
                </option>
              ))}
            </select>
            {partners.length === 0 && (
              <p className="text-xs text-amber-400 mb-3">No active/online partners available right now.</p>
            )}

            {assignError && <p className="text-red-400 text-xs mt-2 mb-2">{assignError}</p>}

            <div className="flex gap-2 pt-3">
              <button
                onClick={() => setAssigningOrderId(null)}
                disabled={isAssigning}
                className="flex-1 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm transition-colors disabled:opacity-40"
              >
                Cancel
              </button>
              <button
                onClick={confirmAssign}
                disabled={isAssigning || !selectedPartnerId}
                className="flex-1 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium disabled:opacity-40 transition-colors"
              >
                {isAssigning ? 'Assigning...' : 'Assign'}
              </button>
            </div>
          </div>
        </div>
      )}

      {toast && (
        <div
          className={`fixed bottom-6 right-6 px-4 py-3 rounded-lg text-sm font-medium shadow-lg z-50 ${
            toast.kind === 'success' ? 'bg-emerald-500 text-white' : 'bg-red-500 text-white'
          }`}
        >
          {toast.message}
        </div>
      )}
    </Layout>
  )
}
