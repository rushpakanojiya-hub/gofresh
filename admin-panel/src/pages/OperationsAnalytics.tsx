import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { getOperationsReport } from '../api/admin'
import type { OperationsReport } from '../types/admin'

function formatMinutes(value: number | null): string {
  if (value === null) return 'No data yet'
  if (value < 1) return `${Math.round(value * 60)}s`
  return `${value.toFixed(1)} min`
}

function formatPercent(value: number | null): string {
  if (value === null) return 'No data yet'
  return `${value.toFixed(1)}%`
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function daysAgoISO(days: number): string {
  const d = new Date()
  d.setDate(d.getDate() - days)
  return d.toISOString().slice(0, 10)
}

export default function OperationsAnalytics() {
  const [from, setFrom] = useState(daysAgoISO(7))
  const [to, setTo] = useState(todayISO())
  const [report, setReport] = useState<OperationsReport | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await getOperationsReport(from, to)
      setReport(res)
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load operations report.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function applyRange() {
    load()
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Operations Analytics</h1>
            <p className="text-sm text-slate-400 mt-1">
              Assignment/delivery time, SLA adherence, cancellation/refund rate - computed strictly from real order timestamps.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <input
              type="date"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
            />
            <span className="text-slate-500 text-sm">to</span>
            <input
              type="date"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
            />
            <button
              onClick={applyRange}
              className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
            >
              Apply
            </button>
          </div>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && report && (
          <div className="space-y-8">
            {/* Order Volume */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">Order Volume</h2>
              <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Total Orders</p>
                  <p className="text-lg font-semibold text-slate-100">{report.total_orders}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Delivered</p>
                  <p className="text-lg font-semibold text-emerald-300">{report.delivered_orders}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Cancelled</p>
                  <p className="text-lg font-semibold text-red-300">{report.cancelled_orders}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Returned</p>
                  <p className="text-lg font-semibold text-amber-300">{report.returned_orders}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Orders/min</p>
                  <p className="text-lg font-semibold text-slate-100">
                    {report.orders_per_minute !== null ? report.orders_per_minute.toFixed(3) : 'No data yet'}
                  </p>
                </div>
              </div>
            </section>

            {/* Rates */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">Cancellation &amp; Refund Rate</h2>
              <div className="grid grid-cols-2 gap-3">
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Cancellation Rate</p>
                  <p className="text-lg font-semibold text-red-300">{formatPercent(report.cancellation_rate_percent)}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Refund Rate</p>
                  <p className="text-lg font-semibold text-amber-300">{formatPercent(report.refund_rate_percent)}</p>
                </div>
              </div>
            </section>

            {/* Timing */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">Assignment &amp; Delivery Time</h2>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Avg Assignment Time</p>
                  <p className="text-lg font-semibold text-slate-100">{formatMinutes(report.avg_assignment_minutes)}</p>
                  <p className="text-xs text-slate-600 mt-1">n={report.assignment_sample_size}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Avg Delivery Time</p>
                  <p className="text-lg font-semibold text-slate-100">{formatMinutes(report.avg_delivery_minutes)}</p>
                  <p className="text-xs text-slate-600 mt-1">n={report.delivery_sample_size}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Avg Picking Time</p>
                  <p className="text-lg font-semibold text-slate-100">{formatMinutes(report.avg_picking_minutes)}</p>
                  <p className="text-xs text-slate-600 mt-1">n={report.picking_sample_size}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Avg Packing Time</p>
                  <p className="text-lg font-semibold text-slate-100">{formatMinutes(report.avg_packing_minutes)}</p>
                  <p className="text-xs text-slate-600 mt-1">n={report.packing_sample_size}</p>
                </div>
              </div>
            </section>

            {/* SLA */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">SLA / ETA Adherence</h2>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">SLA Target</p>
                  <p className="text-lg font-semibold text-slate-100">{report.sla_target_minutes} min</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Actual Avg (confirm→deliver)</p>
                  <p className="text-lg font-semibold text-slate-100">{formatMinutes(report.avg_confirm_to_deliver_minutes)}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">SLA Adherence</p>
                  <p className="text-lg font-semibold text-emerald-300">{formatPercent(report.sla_adherence_percent)}</p>
                  <p className="text-xs text-slate-600 mt-1">n={report.sla_sample_size}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Delayed Orders</p>
                  <p className="text-lg font-semibold text-red-300">{report.delayed_orders}</p>
                </div>
              </div>
            </section>

            {/* Availability */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">Store &amp; Rider Availability</h2>
              <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Total Stores</p>
                  <p className="text-lg font-semibold text-slate-100">{report.total_stores}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Open</p>
                  <p className="text-lg font-semibold text-emerald-300">{report.open_stores}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Paused</p>
                  <p className="text-lg font-semibold text-amber-300">{report.paused_stores}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Total Riders</p>
                  <p className="text-lg font-semibold text-slate-100">{report.total_riders}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Online Riders</p>
                  <p className="text-lg font-semibold text-emerald-300">{report.online_riders}</p>
                </div>
              </div>
            </section>
          </div>
        )}
      </div>
    </Layout>
  )
}
