import { useEffect, useState } from 'react'
import { listAdminPayments, getAdminPaymentDetail, settleGatewayPayment } from '../api/finance'
import type { AdminPaymentRow } from '../types/finance'

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 2,
  }).format(value)
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('en-IN')
}

const PAGE_SIZE = 20

export default function AdminPaymentsDetail() {
  const [status, setStatus] = useState('')
  const [gateway, setGateway] = useState('')
  const [paymentMethod, setPaymentMethod] = useState('')
  const [page, setPage] = useState(1)

  const [rows, setRows] = useState<AdminPaymentRow[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [settlingOrderId, setSettlingOrderId] = useState<number | null>(null)
  const [rowMessage, setRowMessage] = useState<{ orderId: number; text: string } | null>(null)

  async function load(pageNum = page) {
    setLoading(true)
    setError('')
    try {
      const params: any = { page: pageNum, limit: PAGE_SIZE }
      if (status) params.status = status
      if (gateway) params.gateway = gateway
      if (paymentMethod) params.payment_method = paymentMethod
      const data = await listAdminPayments(params)
      setRows(data.payments ?? [])
      setTotal(data.total ?? 0)
    } catch (err: any) {
      setError(err?.response?.data?.message ?? 'Failed to load payments')
      setRows([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    setPage(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function handleApply() {
    setPage(1)
    load(1)
  }

  function handleReset() {
    setStatus('')
    setGateway('')
    setPaymentMethod('')
    setPage(1)
    load(1)
  }

  function goToPage(newPage: number) {
    setPage(newPage)
    load(newPage)
  }

  async function handleSettle(orderId: number) {
    setSettlingOrderId(orderId)
    setRowMessage(null)
    try {
      const detail = await getAdminPaymentDetail(orderId)
      if (detail.payment.is_settled) {
        setRowMessage({ orderId, text: 'Already settled.' })
      } else {
        await settleGatewayPayment(detail.payment.id)
        setRowMessage({ orderId, text: 'Settled successfully.' })
        load(page)
      }
    } catch (err: any) {
      setRowMessage({ orderId, text: err?.response?.data?.message ?? 'Settle failed.' })
    } finally {
      setSettlingOrderId(null)
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div>
      <h1 className="text-2xl font-semibold text-white">Payments — Detailed List</h1>
      <p className="mt-1 text-sm text-gray-400">Individual transaction-level view of all payments.</p>

      <div className="mt-6 flex flex-wrap items-end gap-4">
        <div>
          <label className="mb-1 block text-sm text-gray-400">Status</label>
          <input
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            placeholder="e.g. success, failed"
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm text-gray-400">Gateway</label>
          <input
            value={gateway}
            onChange={(e) => setGateway(e.target.value)}
            placeholder="e.g. razorpay"
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm text-gray-400">Payment Method</label>
          <input
            value={paymentMethod}
            onChange={(e) => setPaymentMethod(e.target.value)}
            placeholder="e.g. upi, card"
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <button
          onClick={handleApply}
          disabled={loading}
          className="rounded bg-emerald-600 px-4 py-2 font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
        >
          {loading ? 'Loading…' : 'Apply'}
        </button>
        <button
          onClick={handleReset}
          disabled={loading}
          className="rounded border border-gray-700 px-4 py-2 font-medium text-gray-300 hover:bg-gray-800 disabled:opacity-50"
        >
          Reset
        </button>
      </div>

      {error && <div className="mt-4 rounded border border-red-800 bg-red-950 px-4 py-2 text-red-300">{error}</div>}

      <div className="mt-6 overflow-x-auto rounded border border-gray-800">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-900 text-gray-400">
            <tr>
              <th className="px-4 py-3 font-medium">Order</th>
              <th className="px-4 py-3 font-medium">Transaction ID</th>
              <th className="px-4 py-3 font-medium">Customer</th>
              <th className="px-4 py-3 font-medium">Amount</th>
              <th className="px-4 py-3 font-medium">Refunded</th>
              <th className="px-4 py-3 font-medium">Method</th>
              <th className="px-4 py-3 font-medium">Gateway</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium">Date</th>
              <th className="px-4 py-3 font-medium">Action</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && !loading && (
              <tr>
                <td colSpan={10} className="px-4 py-6 text-center text-gray-500">
                  No payments found.
                </td>
              </tr>
            )}
            {rows.map((r) => (
              <tr key={r.order_id} className="border-t border-gray-800 align-top">
                <td className="px-4 py-3 text-white">#{r.order_id}</td>
                <td className="px-4 py-3 text-gray-300">{r.transaction_id}</td>
                <td className="px-4 py-3 text-gray-300">
                  <div>{r.customer_name}</div>
                  <div className="text-xs text-gray-500">{r.customer_phone}</div>
                </td>
                <td className="px-4 py-3 text-white">{formatCurrency(r.amount)}</td>
                <td className="px-4 py-3 text-gray-300">{formatCurrency(r.refunded_amount)}</td>
                <td className="px-4 py-3 text-gray-300">{r.payment_method}</td>
                <td className="px-4 py-3 text-gray-300">{r.gateway}</td>
                <td className="px-4 py-3 text-gray-300">{r.status}</td>
                <td className="px-4 py-3 text-gray-400">{formatDate(r.created_at)}</td>
                <td className="px-4 py-3">
                  {r.gateway.toLowerCase() === 'cod' ? (
                    <span className="text-xs text-gray-600">—</span>
                  ) : (
                    <>
                      <button
                        onClick={() => handleSettle(r.order_id)}
                        disabled={settlingOrderId === r.order_id}
                        className="rounded border border-emerald-700 px-3 py-1 text-xs font-medium text-emerald-400 hover:bg-emerald-950 disabled:opacity-50"
                      >
                        {settlingOrderId === r.order_id ? 'Checking…' : 'Settle'}
                      </button>
                      {rowMessage && rowMessage.orderId === r.order_id && (
                        <div className="mt-1 text-xs text-gray-400">{rowMessage.text}</div>
                      )}
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {total > 0 && (
        <div className="mt-4 flex items-center justify-between text-sm text-gray-400">
          <div>
            Page {page} of {totalPages} ({total} total)
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => goToPage(page - 1)}
              disabled={page <= 1 || loading}
              className="rounded border border-gray-700 px-3 py-1 hover:bg-gray-800 disabled:opacity-50"
            >
              Prev
            </button>
            <button
              onClick={() => goToPage(page + 1)}
              disabled={page >= totalPages || loading}
              className="rounded border border-gray-700 px-3 py-1 hover:bg-gray-800 disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
