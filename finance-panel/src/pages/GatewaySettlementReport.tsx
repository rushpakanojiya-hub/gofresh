import { useEffect, useState } from 'react'
import { getGatewaySettlement } from '../api/reports'
import type { GatewaySettlementRow } from '../types/reports'

function toDateInputValue(d: Date) {
  return d.toISOString().slice(0, 10)
}

function startOfMonth() {
  const d = new Date()
  d.setDate(1)
  return toDateInputValue(d)
}

function today() {
  return toDateInputValue(new Date())
}

export default function GatewaySettlementReport() {
  const defaultFrom = startOfMonth()
  const defaultTo = today()

  const [from, setFrom] = useState(defaultFrom)
  const [to, setTo] = useState(defaultTo)
  const [rows, setRows] = useState<GatewaySettlementRow[]>([])
  const [note, setNote] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function load(fromDate = from, toDate = to) {
    setLoading(true)
    setError('')
    try {
      const data = await getGatewaySettlement(fromDate, toDate)
      setRows(data.gateway_settlement ?? [])
      setNote(data.note ?? '')
    } catch (err: any) {
      setError(err?.response?.data?.message ?? 'Failed to load gateway settlement report')
      setRows([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function handleReset() {
    setFrom(defaultFrom)
    setTo(defaultTo)
    load(defaultFrom, defaultTo)
  }

  const totalTransactions = rows.reduce((sum, r) => sum + r.transaction_count, 0)
  const totalGross = rows.reduce((sum, r) => sum + r.gross_amount, 0)
  const totalRefunded = rows.reduce((sum, r) => sum + r.refunded_amount, 0)
  const totalNet = totalGross - totalRefunded

  return (
    <div>
      <h1 className="text-2xl font-semibold text-white">Gateway Settlement Report</h1>
      <p className="mt-1 text-sm text-gray-400">
        Gross vs refunded amounts by payment gateway for the selected period.
        {note ? ` ${note}` : ''}
      </p>

      <div className="mt-6 flex flex-wrap items-end gap-4">
        <div>
          <label className="mb-1 block text-sm text-gray-400">From</label>
          <input
            type="date"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm text-gray-400">To</label>
          <input
            type="date"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <button
          onClick={() => load()}
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
              <th className="px-4 py-3 font-medium">Gateway</th>
              <th className="px-4 py-3 font-medium">Transactions</th>
              <th className="px-4 py-3 font-medium">Gross Amount</th>
              <th className="px-4 py-3 font-medium">Refunded Amount</th>
              <th className="px-4 py-3 font-medium">Net Amount</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && !loading && (
              <tr>
                <td colSpan={5} className="px-4 py-6 text-center text-gray-500">
                  No data for the selected period.
                </td>
              </tr>
            )}
            {rows.map((r) => (
              <tr key={r.gateway} className="border-t border-gray-800">
                <td className="px-4 py-3 text-white">{r.gateway}</td>
                <td className="px-4 py-3 text-gray-300">{r.transaction_count}</td>
                <td className="px-4 py-3 text-gray-300">₹{r.gross_amount.toFixed(2)}</td>
                <td className="px-4 py-3 text-gray-300">₹{r.refunded_amount.toFixed(2)}</td>
                <td className="px-4 py-3 text-emerald-400">₹{(r.gross_amount - r.refunded_amount).toFixed(2)}</td>
              </tr>
            ))}
          </tbody>
          {rows.length > 0 && (
            <tfoot>
              <tr className="border-t border-gray-700 bg-gray-900 font-medium">
                <td className="px-4 py-3 text-white">Total</td>
                <td className="px-4 py-3 text-white">{totalTransactions}</td>
                <td className="px-4 py-3 text-white">₹{totalGross.toFixed(2)}</td>
                <td className="px-4 py-3 text-white">₹{totalRefunded.toFixed(2)}</td>
                <td className="px-4 py-3 text-emerald-400">₹{totalNet.toFixed(2)}</td>
              </tr>
            </tfoot>
          )}
        </table>
      </div>
    </div>
  )
}
