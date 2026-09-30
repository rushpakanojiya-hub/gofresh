import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { listReturns, approveReturn, rejectReturn, assignReturnPickup, listDeliveryPartners } from '../api/admin'
import type { ReturnRequest, DeliveryPartner } from '../types/admin'

export default function Returns() {
  const [returns, setReturns] = useState<ReturnRequest[]>([])
  const [partners, setPartners] = useState<DeliveryPartner[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actingId, setActingId] = useState<number | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')
  const [rejectingId, setRejectingId] = useState<number | null>(null)
  const [rejectReason, setRejectReason] = useState('')
  const [assigningId, setAssigningId] = useState<number | null>(null)
  const [selectedPartnerId, setSelectedPartnerId] = useState('')

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listReturns()
      setReturns(res.return_requests ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load returns.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadPartners() {
    try {
      const res = await listDeliveryPartners()
      const list = res.delivery_partners ?? res.partners ?? res ?? []
      setPartners(list.filter((p: DeliveryPartner) => p.is_active !== false))
    } catch {
      // Non-fatal - assign UI just won't have options if this fails.
    }
  }

  useEffect(() => {
    load()
    loadPartners()
  }, [])

  async function handleApprove(id: number) {
    if (!confirm('Approve this return? This will credit the customer wallet and restock inventory.')) return
    setActingId(id)
    try {
      await approveReturn(id)
      await load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to approve return.')
    } finally {
      setActingId(null)
    }
  }

  function openReject(id: number) {
    setRejectingId(id)
    setRejectReason('')
  }

  async function confirmReject() {
    if (!rejectingId) return
    setActingId(rejectingId)
    try {
      await rejectReturn(rejectingId, rejectReason.trim() || undefined)
      setRejectingId(null)
      await load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to reject return.')
    } finally {
      setActingId(null)
    }
  }

  function openAssign(id: number) {
    setAssigningId(id)
    setSelectedPartnerId('')
  }

  async function confirmAssign() {
    if (!assigningId || !selectedPartnerId) return
    setActingId(assigningId)
    try {
      await assignReturnPickup(assigningId, Number(selectedPartnerId))
      setAssigningId(null)
      await load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to assign pickup partner.')
    } finally {
      setActingId(null)
    }
  }

  function statusColor(status: string) {
    if (status === 'approved') return 'bg-emerald-500/15 text-emerald-400'
    if (status === 'rejected') return 'bg-red-500/15 text-red-400'
    return 'bg-amber-500/15 text-amber-400'
  }

  function pickupStatusColor(status: string) {
    if (status === 'handed_over') return 'bg-emerald-500/15 text-emerald-400'
    if (status === 'picked_up' || status === 'arrived' || status === 'en_route') return 'bg-blue-500/15 text-blue-400'
    if (status === 'accepted') return 'bg-indigo-500/15 text-indigo-400'
    return 'bg-amber-500/15 text-amber-400'
  }

  const filtered = returns.filter((r) => {
    if (statusFilter && r.status !== statusFilter) return false
    if (search) {
      const s = search.toLowerCase()
      const matchesOrder = String(r.order_id).includes(s)
      const matchesCustomer = (r.customer_name ?? '').toLowerCase().includes(s) || (r.customer_phone ?? '').includes(s)
      if (!matchesOrder && !matchesCustomer) return false
    }
    return true
  })

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Returns / Refunds</h1>
            <p className="text-sm text-slate-400 mt-1">
              {filtered.length} of {returns.length} return{returns.length !== 1 ? 's' : ''}
            </p>
          </div>
          <div className="flex gap-3">
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
            >
              <option value="">All statuses</option>
              <option value="pending">Pending</option>
              <option value="approved">Approved</option>
              <option value="rejected">Rejected</option>
            </select>
            <input
              type="text"
              placeholder="Search order # or customer..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm w-64"
            />
          </div>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}
        {!isLoading && !error && filtered.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No return requests found.
          </div>
        )}

        {!isLoading && filtered.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Order</th>
                  <th className="px-4 py-3 font-medium">Customer</th>
                  <th className="px-4 py-3 font-medium">Product</th>
                  <th className="px-4 py-3 font-medium">Qty</th>
                  <th className="px-4 py-3 font-medium">Refund Amount</th>
                  <th className="px-4 py-3 font-medium">Reason</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Pickup Partner</th>
                  <th className="px-4 py-3 font-medium">Date</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((r) => (
                  <tr key={r.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">#{r.order_id}</td>
                    <td className="px-4 py-3 text-slate-300">
                      {r.customer_name || '-'}
                      <div className="text-xs text-slate-500">{r.customer_phone}</div>
                    </td>
                    <td className="px-4 py-3 text-slate-400">
                      {r.items && r.items.length > 0
                        ? r.items.map((it) => `${it.order_item?.product?.name ?? 'Unknown'} x${it.quantity}`).join(', ')
                        : '-'}
                    </td>
                    <td className="px-4 py-3">{r.items?.reduce((sum, it) => sum + it.quantity, 0) ?? 0}</td>
                    <td className="px-4 py-3 font-semibold text-slate-100">₹{r.refund_amount?.toFixed(2) ?? '0.00'}</td>
                    <td className="px-4 py-3 text-slate-400 max-w-xs truncate">{r.reason ?? '-'}</td>
                    <td className="px-4 py-3">
                      <span className={`text-xs px-2 py-1 rounded-full ${statusColor(r.status)}`}>
                        {r.status}
                      </span>
                      {r.status === 'rejected' && r.rejection_reason && (
                        <div className="text-xs text-slate-500 mt-1">{r.rejection_reason}</div>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {r.delivery_partner_name ? (
                        <div>
                          <div className="text-slate-200">{r.delivery_partner_name}</div>
                          <div className="text-xs text-slate-500">{r.delivery_partner_phone}</div>
                          {r.pickup_status && (
                            <span className={`text-xs px-2 py-0.5 rounded-full inline-block mt-1 ${pickupStatusColor(r.pickup_status)}`}>
                              {r.pickup_status.replace('_', ' ')}
                            </span>
                          )}
                        </div>
                      ) : r.status === 'approved' ? (
                        <button
                          onClick={() => openAssign(r.id)}
                          className="text-indigo-400 hover:text-indigo-300 text-xs"
                        >
                          Assign partner
                        </button>
                      ) : (
                        <span className="text-slate-600 text-xs">-</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-slate-500 text-xs">
                      {r.created_at ? new Date(r.created_at).toLocaleDateString() : '-'}
                    </td>
                    <td className="px-4 py-3 text-right space-x-3">
                      {r.status === 'pending' && (
                        <>
                          <button
                            onClick={() => handleApprove(r.id)}
                            disabled={actingId === r.id}
                            className="text-emerald-400 hover:text-emerald-300 text-xs disabled:opacity-50"
                          >
                            Approve
                          </button>
                          <button
                            onClick={() => openReject(r.id)}
                            disabled={actingId === r.id}
                            className="text-red-400 hover:text-red-300 text-xs disabled:opacity-50"
                          >
                            Reject
                          </button>
                        </>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {rejectingId && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 w-full max-w-sm">
            <h3 className="text-sm font-semibold text-slate-200 mb-3">Reject return request</h3>
            <textarea
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              placeholder="Reason for rejection (optional but recommended)"
              rows={3}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-4"
            />
            <div className="flex gap-2">
              <button
                onClick={() => setRejectingId(null)}
                className="flex-1 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={confirmReject}
                disabled={actingId === rejectingId}
                className="flex-1 py-2 rounded-lg bg-red-500 hover:bg-red-400 text-white text-sm font-medium disabled:opacity-40 transition-colors"
              >
                Confirm Reject
              </button>
            </div>
          </div>
        </div>
      )}

      {assigningId && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 w-full max-w-sm">
            <h3 className="text-sm font-semibold text-slate-200 mb-3">Assign pickup partner</h3>
            <select
              value={selectedPartnerId}
              onChange={(e) => setSelectedPartnerId(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-4"
            >
              <option value="">Select a delivery partner</option>
              {partners.map((p) => (
                <option key={p.id} value={p.id}>{p.name} - {p.phone}</option>
              ))}
            </select>
            <div className="flex gap-2">
              <button
                onClick={() => setAssigningId(null)}
                className="flex-1 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={confirmAssign}
                disabled={!selectedPartnerId || actingId === assigningId}
                className="flex-1 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium disabled:opacity-40 transition-colors"
              >
                Assign
              </button>
            </div>
          </div>
        </div>
      )}
    </Layout>
  )
}