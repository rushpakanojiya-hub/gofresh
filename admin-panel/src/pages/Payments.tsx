import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import {
  listAdminPayments,
  getAdminPaymentReconciliation,
  getAdminPaymentDetail,
  updateAdminPaymentStatus,
} from '../api/admin'
import type {
  AdminPaymentRow,
  AdminPaymentDetail,
  AdminPaymentReconciliationSummary,
} from '../types/admin'

const STATUS_OPTIONS = ['pending', 'paid', 'failed', 'refunded', 'partially_refunded']

const STATUS_STYLES: Record<string, string> = {
  pending: 'bg-amber-500/15 text-amber-300',
  paid: 'bg-emerald-500/15 text-emerald-300',
  failed: 'bg-red-500/15 text-red-300',
  refunded: 'bg-slate-700/50 text-slate-300',
  partially_refunded: 'bg-orange-500/15 text-orange-300',
  created: 'bg-amber-500/15 text-amber-300',
}

function fmtStatus(s: string) {
  return s.replace('_', ' ')
}

function fmtMoney(n: number) {
  return '\u20b9' + Number(n ?? 0).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

export default function Payments() {
  const [rows, setRows] = useState<AdminPaymentRow[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('')
  const [method, setMethod] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)

  const [summary, setSummary] = useState<AdminPaymentReconciliationSummary | null>(null)

  const [selectedOrderId, setSelectedOrderId] = useState<number | null>(null)
  const [detail, setDetail] = useState<AdminPaymentDetail | null>(null)
  const [isLoadingDetail, setIsLoadingDetail] = useState(false)
  const [newStatus, setNewStatus] = useState('')
  const [refundAmount, setRefundAmount] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listAdminPayments({
        search: search || undefined,
        status: status || undefined,
        payment_method: method || undefined,
        date_from: dateFrom || undefined,
        date_to: dateTo || undefined,
        page,
        limit: 20,
      })
      setRows(res.payments ?? [])
      setTotalPages(res.total_pages ?? 1)
      setTotal(res.total ?? 0)
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load payments.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadSummary() {
    try {
      const res = await getAdminPaymentReconciliation({
        date_from: dateFrom || undefined,
        date_to: dateTo || undefined,
      })
      setSummary(res)
    } catch {
      // non-critical, ignore
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, method, dateFrom, dateTo, page])

  useEffect(() => {
    loadSummary()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dateFrom, dateTo])

  function handleSearchSubmit(e: React.FormEvent) {
    e.preventDefault()
    setPage(1)
    load()
  }

  async function openDetail(orderId: number) {
    setSelectedOrderId(orderId)
    setIsLoadingDetail(true)
    setDetail(null)
    try {
      const res = await getAdminPaymentDetail(orderId)
      setDetail(res)
      setNewStatus(res.payment.status === 'created' ? 'pending' : res.payment.status)
      setRefundAmount(res.payment.refunded_amount ? String(res.payment.refunded_amount) : '')
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to load payment detail.')
    } finally {
      setIsLoadingDetail(false)
    }
  }

  async function handleStatusSave() {
    if (!selectedOrderId || !newStatus) return
    const needsAmount = newStatus === 'refunded' || newStatus === 'partially_refunded'
    if (needsAmount && !refundAmount.trim()) {
      alert('Refunded amount is required for this status.')
      return
    }
    setIsSaving(true)
    try {
      await updateAdminPaymentStatus(
        selectedOrderId,
        newStatus,
        needsAmount ? Number(refundAmount) : undefined
      )
      await openDetail(selectedOrderId)
      setRows((prev) =>
        prev.map((r) => (r.order_id === selectedOrderId ? { ...r, status: newStatus as any } : r))
      )
      loadSummary()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update payment status.')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="mb-6">
          <h1 className="text-xl font-semibold">Payments</h1>
          <p className="text-sm text-slate-400 mt-1">Transaction history and reconciliation across COD and online orders</p>
        </div>

        {summary && (
          <div className="grid grid-cols-5 gap-4 mb-6">
            <div className="border border-slate-800 rounded-xl p-4">
              <p className="text-xs text-slate-500 mb-1">Total Collected</p>
              <p className="text-lg font-semibold text-emerald-300">{fmtMoney(summary.total_collected)}</p>
            </div>
            <div className="border border-slate-800 rounded-xl p-4">
              <p className="text-xs text-slate-500 mb-1">Pending</p>
              <p className="text-lg font-semibold text-amber-300">{fmtMoney(summary.total_pending)}</p>
            </div>
            <div className="border border-slate-800 rounded-xl p-4">
              <p className="text-xs text-slate-500 mb-1">Refunded</p>
              <p className="text-lg font-semibold text-slate-300">{fmtMoney(summary.total_refunded)}</p>
            </div>
            <div className="border border-slate-800 rounded-xl p-4">
              <p className="text-xs text-slate-500 mb-1">Online Collected</p>
              <p className="text-lg font-semibold text-indigo-300">{fmtMoney(summary.online_collected)}</p>
            </div>
            <div className="border border-slate-800 rounded-xl p-4">
              <p className="text-xs text-slate-500 mb-1">COD Collected</p>
              <p className="text-lg font-semibold text-indigo-300">{fmtMoney(summary.cod_collected)}</p>
            </div>
          </div>
        )}

        <form onSubmit={handleSearchSubmit} className="flex flex-wrap items-center gap-2 mb-4">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search customer, order id, transaction id..."
            className="flex-1 min-w-[220px] bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-slate-500"
          />
          <select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value)
              setPage(1)
            }}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All statuses</option>
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {fmtStatus(s)}
              </option>
            ))}
          </select>
          <select
            value={method}
            onChange={(e) => {
              setMethod(e.target.value)
              setPage(1)
            }}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All methods</option>
            <option value="cod">COD</option>
            <option value="online">Online</option>
          </select>
          <input
            type="date"
            value={dateFrom}
            onChange={(e) => {
              setDateFrom(e.target.value)
              setPage(1)
            }}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          />
          <span className="text-slate-500 text-sm">to</span>
          <input
            type="date"
            value={dateTo}
            onChange={(e) => {
              setDateTo(e.target.value)
              setPage(1)
            }}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          />
          <button
            type="submit"
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            Search
          </button>
        </form>

        <div className="grid grid-cols-5 gap-6">
          <div className="col-span-3">
            {isLoading && <p className="text-slate-400">Loading...</p>}
            {error && <p className="text-red-400">{error}</p>}

            {!isLoading && !error && rows.length === 0 && (
              <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
                No transactions found.
              </div>
            )}

            {!isLoading && rows.length > 0 && (
              <div className="border border-slate-800 rounded-xl overflow-hidden">
                <table className="w-full text-sm">
                  <thead className="bg-slate-900 text-slate-400 text-xs uppercase">
                    <tr>
                      <th className="text-left px-4 py-3">Transaction</th>
                      <th className="text-left px-4 py-3">Customer</th>
                      <th className="text-right px-4 py-3">Amount</th>
                      <th className="text-left px-4 py-3">Method</th>
                      <th className="text-left px-4 py-3">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {rows.map((r) => (
                      <tr
                        key={r.order_id}
                        onClick={() => openDetail(r.order_id)}
                        className={
                          'cursor-pointer hover:bg-slate-900 transition-colors ' +
                          (selectedOrderId === r.order_id ? 'bg-slate-900' : '')
                        }
                      >
                        <td className="px-4 py-3">
                          <p className="text-slate-200 font-medium">{r.transaction_id}</p>
                          <p className="text-xs text-slate-500">
                            Order #{r.order_id} &middot; {new Date(r.created_at).toLocaleString()}
                          </p>
                        </td>
                        <td className="px-4 py-3">
                          <p className="text-slate-300">{r.customer_name || '\u2014'}</p>
                          <p className="text-xs text-slate-500">{r.customer_phone}</p>
                        </td>
                        <td className="px-4 py-3 text-right text-slate-200">{fmtMoney(r.amount)}</td>
                        <td className="px-4 py-3">
                          <span className="text-xs uppercase text-slate-400">{r.payment_method}</span>
                          {r.collected_via === 'upi' && (
                            <span className="ml-1.5 px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-500/15 text-indigo-300">UPI</span>
                          )}
                          <p className="text-xs text-slate-600">{r.gateway}</p>
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={
                              'px-2 py-0.5 rounded-md text-xs font-medium ' + (STATUS_STYLES[r.status] ?? '')
                            }
                          >
                            {fmtStatus(r.status)}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {!isLoading && rows.length > 0 && (
              <div className="flex items-center justify-between mt-4 text-sm text-slate-400">
                <span>
                  Page {page} of {totalPages} &middot; {total} total
                </span>
                <div className="flex gap-2">
                  <button
                    disabled={page <= 1}
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    className="px-3 py-1.5 rounded-lg bg-slate-800 border border-slate-700 disabled:opacity-40"
                  >
                    Prev
                  </button>
                  <button
                    disabled={page >= totalPages}
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                    className="px-3 py-1.5 rounded-lg bg-slate-800 border border-slate-700 disabled:opacity-40"
                  >
                    Next
                  </button>
                </div>
              </div>
            )}
          </div>

          <div className="col-span-2">
            {!selectedOrderId && (
              <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500 h-full flex items-center justify-center">
                Select a transaction to view details
              </div>
            )}

            {selectedOrderId && (
              <div className="border border-slate-800 rounded-xl overflow-hidden">
                <div className="px-4 py-3 bg-slate-900 border-b border-slate-800">
                  <p className="text-sm font-medium text-slate-200">Order #{selectedOrderId}</p>
                  {!detail?.has_payment_record && !isLoadingDetail && (
                    <p className="text-xs text-amber-400 mt-0.5">No gateway record yet (COD, derived from order)</p>
                  )}
                </div>

                {isLoadingDetail && <p className="text-slate-400 text-sm p-4">Loading...</p>}

                {!isLoadingDetail && detail && (
                  <div className="p-4 space-y-4">
                    <div className="grid grid-cols-2 gap-3 text-sm">
                      <div>
                        <p className="text-xs text-slate-500">Customer</p>
                        <p className="text-slate-200">{detail.customer.name || '\u2014'}</p>
                        <p className="text-xs text-slate-500">{detail.customer.phone}</p>
                      </div>
                      <div>
                        <p className="text-xs text-slate-500">Amount</p>
                        <p className="text-slate-200">{fmtMoney(detail.payment.amount)}</p>
                      </div>
                      <div>
                        <p className="text-xs text-slate-500">Gateway</p>
                        <p className="text-slate-200">{detail.payment.gateway}</p>
                      </div>
                      <div>
                        <p className="text-xs text-slate-500">Transaction Ref</p>
                        <p className="text-slate-200 break-all">
                          {detail.payment.razorpay_payment_id || detail.payment.razorpay_order_id || '\u2014'}
                        </p>
                      </div>
                      {detail.payment.refunded_amount > 0 && (
                        <div>
                          <p className="text-xs text-slate-500">Refunded Amount</p>
                          <p className="text-slate-200">{fmtMoney(detail.payment.refunded_amount)}</p>
                        </div>
                      )}
                    </div>

                    <div className="border-t border-slate-800 pt-4">
                      <p className="text-xs text-slate-500 mb-2">Update Status</p>
                      <div className="flex gap-2 mb-2">
                        <select
                          value={newStatus}
                          onChange={(e) => setNewStatus(e.target.value)}
                          className="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                        >
                          {STATUS_OPTIONS.map((s) => (
                            <option key={s} value={s}>
                              {fmtStatus(s)}
                            </option>
                          ))}
                        </select>
                      </div>
                      {(newStatus === 'refunded' || newStatus === 'partially_refunded') && (
                        <input
                          type="number"
                          step="0.01"
                          value={refundAmount}
                          onChange={(e) => setRefundAmount(e.target.value)}
                          placeholder="Refunded amount"
                          className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-2"
                        />
                      )}
                      <button
                        onClick={handleStatusSave}
                        disabled={isSaving}
                        className="w-full px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium disabled:opacity-40 transition-colors"
                      >
                        {isSaving ? 'Saving...' : 'Save Status'}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </Layout>
  )
}