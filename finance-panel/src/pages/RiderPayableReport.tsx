import { useEffect, useState } from 'react'
import { getRiderPayable } from '../api/reports'
import type { RiderPayableRow } from '../types/reports'

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

export default function RiderPayableReport() {
  const [from, setFrom] = useState(startOfMonth())
  const [to, setTo] = useState(today())
  const [rows, setRows] = useState<RiderPayableRow[]>([])
  const [perDeliveryRate, setPerDeliveryRate] = useState<number>(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await getRiderPayable(from, to)
      setRows(data.rider_payable ?? [])
      setPerDeliveryRate(data.per_delivery_rate ?? 0)
    } catch (err: any) {
      setError(err?.response?.data?.message ?? 'Failed to load rider payable report')
      setRows([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const totalDelivered = rows.reduce((sum, r) => sum + r.delivered_count, 0)
  const totalPayable = rows.reduce((sum, r) => sum + r.payable, 0)

  return (
    <div>
      <h1 className="text-2xl font-semibold text-white">Rider Payable Report</h1>
      <p className="mt-1 text-sm text-gray-400">
        Delivery partner earnings for the selected period, at ₹{perDeliveryRate.toFixed(2)} per delivery.
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
          onClick={load}
          disabled={loading}
          className="rounded bg-emerald-600 px-4 py-2 font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
        >
          {loading ? 'Loading…' : 'Apply'}
        </button>
      </div>

      {error && <div className="mt-4 rounded border border-red-800 bg-red-950 px-4 py-2 text-red-300">{error}</div>}

      <div className="mt-6 overflow-x-auto rounded border border-gray-800">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-900 text-gray-400">
            <tr>
              <th className="px-4 py-3 font-medium">Delivery Partner</th>
              <th className="px-4 py-3 font-medium">Phone</th>
              <th className="px-4 py-3 font-medium">Delivered Count</th>
              <th className="px-4 py-3 font-medium">Payable</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && !loading && (
              <tr>
                <td colSpan={4} className="px-4 py-6 text-center text-gray-500">
                  No data for the selected period.
                </td>
              </tr>
            )}
            {rows.map((r) => (
              <tr key={r.delivery_partner_id} className="border-t border-gray-800">
                <td className="px-4 py-3 text-white">{r.name}</td>
                <td className="px-4 py-3 text-gray-300">{r.phone}</td>
                <td className="px-4 py-3 text-gray-300">{r.delivered_count}</td>
                <td className="px-4 py-3 text-emerald-400">₹{r.payable.toFixed(2)}</td>
              </tr>
            ))}
          </tbody>
          {rows.length > 0 && (
            <tfoot>
              <tr className="border-t border-gray-700 bg-gray-900 font-medium">
                <td className="px-4 py-3 text-white" colSpan={2}>
                  Total
                </td>
                <td className="px-4 py-3 text-white">{totalDelivered}</td>
                <td className="px-4 py-3 text-emerald-400">₹{totalPayable.toFixed(2)}</td>
              </tr>
            </tfoot>
          )}
        </table>
      </div>
    </div>
  )
}
