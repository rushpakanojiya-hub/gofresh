import { useCallback, useEffect, useState } from 'react'
import { listStoreReturns, approveStoreReturn, rejectStoreReturn } from '../api/warehouse'
import type { StoreReturnRequest } from '../types/warehouse'
import StatusBadge from '../components/StatusBadge'
import { getErrorMessage } from '../utils/errors'

const TABS: { value: string; label: string }[] = [
  { value: '', label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'approved', label: 'Approved' },
  { value: 'rejected', label: 'Rejected' },
]

export default function Returns() {
  const [statusFilter, setStatusFilter] = useState('')
  const [returns, setReturns] = useState<StoreReturnRequest[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [decidingId, setDecidingId] = useState<number | null>(null)
  const [rejectingId, setRejectingId] = useState<number | null>(null)
  const [rejectReason, setRejectReason] = useState('')

  const load = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listStoreReturns({ status: statusFilter || undefined })
      setReturns(res.return_requests ?? [])
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load return requests.'))
    } finally {
      setIsLoading(false)
    }
  }, [statusFilter])

  useEffect(() => {
    load()
  }, [load])

  async function handleApprove(id: number) {
    setDecidingId(id)
    setError(null)
    try {
      await approveStoreReturn(id)
      await load()
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to approve return.'))
    } finally {
      setDecidingId(null)
    }
  }

  async function handleReject(id: number) {
    setDecidingId(id)
    setError(null)
    try {
      await rejectStoreReturn(id, rejectReason || undefined)
      setRejectingId(null)
      setRejectReason('')
      await load()
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to reject return.'))
    } finally {
      setDecidingId(null)
    }
  }

  return (
    <div className="p-6">
      <h1 className="font-display text-2xl font-semibold mb-1">Return Requests</h1>
      <p className="text-xs text-slate-500 mb-5">
        Approve or reject customer return requests for orders fulfilled from your warehouse.
      </p>

      <div className="flex gap-1 mb-4">
        {TABS.map((tab) => (
          <button
            key={tab.value}
            onClick={() => setStatusFilter(tab.value)}
            className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
              statusFilter === tab.value
                ? 'bg-red-500/20 text-red-300'
                : 'text-slate-400 hover:bg-slate-800 hover:text-slate-200'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="text-sm text-slate-400">Loading...</div>
      ) : returns.length === 0 ? (
        <div className="text-sm text-slate-500 border border-slate-800 rounded-xl p-6 text-center">
          No return requests found.
        </div>
      ) : (
        <div className="space-y-3">
          {returns.map((req) => (
            <div key={req.id} className="border border-slate-800 rounded-xl bg-slate-900 p-4">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1">
                  <p className="text-xs text-slate-500 mb-1">
                    Order #{req.order_id} &middot; {req.customer_name} ({req.customer_phone}) &middot;{' '}
                    {new Date(req.created_at).toLocaleString()}
                  </p>
                  <p className="text-sm text-slate-200">Reason: {req.reason}</p>
                  <p className="text-xs text-slate-500 mt-1">Refund amount: &#8377;{req.refund_amount}</p>
                  {req.items && req.items.length > 0 && (
                    <ul className="text-xs text-slate-400 mt-2 space-y-0.5">
                      {req.items.map((it) => (
                        <li key={it.id}>
                          {it.order_item?.product?.name ?? `Product #${it.order_item?.product_id}`} &times; {it.quantity}
                        </li>
                      ))}
                    </ul>
                  )}
                  {req.rejection_reason && (
                    <p className="text-xs text-rose-400 mt-1">Rejection reason: {req.rejection_reason}</p>
                  )}
                </div>

                {req.image_url && (
                  <a href={req.image_url} target="_blank" rel="noreferrer" className="shrink-0">
                    <img
                      src={req.image_url}
                      alt="Return evidence"
                      className="w-20 h-20 object-cover rounded-lg border border-slate-700"
                    />
                  </a>
                )}

                <StatusBadge status={req.status} />
              </div>

              {req.status === 'pending' && (
                <div className="mt-3">
                  {rejectingId === req.id ? (
                    <div className="flex gap-2 items-center">
                      <input
                        type="text"
                        value={rejectReason}
                        onChange={(e) => setRejectReason(e.target.value)}
                        placeholder="Rejection reason (optional)"
                        className="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-xs"
                      />
                      <button
                        disabled={decidingId === req.id}
                        onClick={() => handleReject(req.id)}
                        className="px-3 py-1.5 rounded-lg bg-rose-600/80 hover:bg-rose-600 text-white text-xs font-medium disabled:opacity-50"
                      >
                        Confirm Reject
                      </button>
                      <button
                        onClick={() => { setRejectingId(null); setRejectReason('') }}
                        className="px-3 py-1.5 rounded-lg text-xs font-medium text-slate-400 hover:text-slate-200"
                      >
                        Cancel
                      </button>
                    </div>
                  ) : (
                    <div className="flex gap-2">
                      <button
                        disabled={decidingId === req.id}
                        onClick={() => handleApprove(req.id)}
                        className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium disabled:opacity-50"
                      >
                        Approve
                      </button>
                      <button
                        disabled={decidingId === req.id}
                        onClick={() => setRejectingId(req.id)}
                        className="px-3 py-1.5 rounded-lg bg-rose-600/80 hover:bg-rose-600 text-white text-xs font-medium disabled:opacity-50"
                      >
                        Reject
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
