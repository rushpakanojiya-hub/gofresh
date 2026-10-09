import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { getDashboard, listWarehouseOrders } from '../api/warehouse'
import type { Order, WarehouseDashboardStats } from '../types/warehouse'
import StatusBadge from '../components/StatusBadge'
import { useAuth } from '../context/AuthContext'
import { getErrorMessage } from '../utils/errors'

const TABS: { key: string; label: string; statuses: string[] }[] = [
  { key: 'live', label: 'Live Orders', statuses: ['confirmed', 'picking', 'picked', 'packing', 'packed', 'ready_for_dispatch'] },
  { key: 'picking', label: 'Picking', statuses: ['picking', 'picked'] },
  { key: 'packing', label: 'Packing', statuses: ['packing', 'packed'] },
  { key: 'ready', label: 'Ready', statuses: ['ready_for_dispatch'] },
  { key: 'dispatched', label: 'Dispatched', statuses: ['handed_over'] },
]

type Tone = 'ok' | 'warn' | 'bad'
const DOT: Record<Tone, string> = { ok: 'bg-emerald-500', warn: 'bg-amber-400', bad: 'bg-rose-500' }
const TEXT: Record<Tone, string> = { ok: 'text-emerald-300', warn: 'text-amber-300', bad: 'text-rose-300' }

function TopCard({ label, value, hint, color }: { label: string; value: number | string; hint?: string; color: string }) {
  return (
    <div className="border border-slate-800 bg-slate-900 rounded-xl p-4 flex items-center gap-4">
      <div className={`w-12 h-12 rounded-xl flex items-center justify-center text-white text-lg font-bold ${color}`}>
        {String(label).charAt(0)}
      </div>
      <div>
        <p className="text-xs text-slate-400">{label}</p>
        <p className="font-mono text-2xl font-semibold text-slate-100">
          {value}
          {hint ? <span className="font-sans text-xs font-normal text-slate-500 ml-2">{hint}</span> : null}
        </p>
      </div>
    </div>
  )
}

function Panel({ title, linkLabel, onLink, children }: { title: string; linkLabel?: string; onLink?: () => void; children: React.ReactNode }) {
  return (
    <div className="border border-slate-800 bg-slate-900 rounded-xl p-4">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold">{title}</h2>
        {linkLabel && (
          <button onClick={onLink} className="text-xs text-red-300 hover:text-red-200">
            {linkLabel} &rarr;
          </button>
        )}
      </div>
      {children}
    </div>
  )
}

function Row({ label, value, tone }: { label: string; value: string | number; tone?: Tone }) {
  return (
    <div className="flex items-center justify-between py-2 border-b border-slate-800 last:border-0 text-sm">
      <span className="flex items-center gap-2 text-slate-300">
        {tone && <span className={`w-2 h-2 rounded-full ${DOT[tone]}`} />}
        {label}
      </span>
      <span className={`font-mono font-semibold ${tone ? TEXT[tone] : 'text-slate-100'}`}>{value}</span>
    </div>
  )
}

export default function Dashboard() {
  const navigate = useNavigate()
  const { staff } = useAuth()
  const [stats, setStats] = useState<WarehouseDashboardStats | null>(null)
  const [orders, setOrders] = useState<Order[]>([])
  const [tab, setTab] = useState('live')
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const load = useCallback(async () => {
    setError(null)
    try {
      const [s, o] = await Promise.all([getDashboard(), listWarehouseOrders({ page: 1, limit: 100 })])
      setStats(s)
      setOrders(o.orders ?? [])
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load dashboard.'))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const t = setInterval(load, 10000)
    return () => clearInterval(t)
  }, [load])

  const counts = useMemo(() => {
    const out: Record<string, number> = {}
    TABS.forEach((t) => {
      out[t.key] = orders.filter((o) => t.statuses.includes(o.status)).length
    })
    return out
  }, [orders])

  const active = TABS.find((t) => t.key === tab) ?? TABS[0]
  const rows = orders.filter((o) => active.statuses.includes(o.status)).slice(0, 8)

  function goToOrder(o: Order) {
    if (o.status === 'picking' || o.status === 'picked') navigate(`/picking/${o.id}`)
    else if (o.status === 'packing' || o.status === 'packed') navigate(`/packing/${o.id}`)
    else navigate('/orders')
  }

  function slaOf(o: Order): { text: string; tone: Tone } {
    if (o.status === 'handed_over' || o.status === 'delivered') return { text: '-', tone: 'ok' }
    const mins = Math.max(0, Math.round((Date.now() - new Date(o.created_at).getTime()) / 60000))
    if (o.delayed) return { text: `${mins} mins`, tone: 'bad' }
    return { text: `${mins} mins`, tone: mins < 15 ? 'ok' : mins < 30 ? 'warn' : 'bad' }
  }

  const warehouseName = staff?.warehouse?.name ?? 'Store'
  const todayLabel = new Date().toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })

  const inProgress = stats ? Math.max(0, stats.today_orders - stats.completed_today - stats.cancelled_today) : 0
  const rate = stats?.fulfillment_rate ?? 0
  const healthy = !!stats && stats.delayed_orders === 0 && stats.open_exceptions === 0 && rate >= 90

  return (
    <div className="p-6 max-w-7xl">
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="font-display text-2xl font-semibold">Dashboard</h1>
          <p className="text-sm text-slate-400 mt-1">
            {warehouseName} <span className="text-slate-600">|</span> <span className="text-emerald-400">&bull; Online</span>
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-300">Today, {todayLabel}</span>
          <button onClick={load} className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors">
            Refresh
          </button>
        </div>
      </div>

      {isLoading && <p className="text-sm text-slate-400">Loading dashboard...</p>}
      {error && !isLoading && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">{error}</div>
      )}

      {stats && (
        <>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
            <TopCard label="Total Orders" value={stats.today_orders} color="bg-blue-600" />
            <TopCard label="Delivered" value={stats.completed_today} color="bg-emerald-600" />
            <TopCard label="In Progress" value={inProgress} color="bg-amber-500" />
            <TopCard label="Cancelled" value={stats.cancelled_today} color="bg-rose-600" />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-4 mb-4 items-start">
            <div className="border border-slate-800 bg-slate-900 rounded-xl overflow-hidden">
              <div className="flex gap-1 border-b border-slate-800 px-3 overflow-x-auto">
                {TABS.map((t) => (
                  <button
                    key={t.key}
                    onClick={() => setTab(t.key)}
                    className={`px-3 py-3 text-sm whitespace-nowrap border-b-2 transition-colors ${
                      tab === t.key ? 'border-red-400 text-red-300 font-medium' : 'border-transparent text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {t.label}
                    {t.key !== 'live' && t.key !== 'dispatched' ? ` (${counts[t.key]})` : ''}
                  </button>
                ))}
              </div>
              {rows.length === 0 ? (
                <p className="text-sm text-slate-500 text-center py-10">No orders in this view.</p>
              ) : (
                <table className="w-full text-sm">
                  <thead className="text-left text-xs text-slate-400 uppercase tracking-wide">
                    <tr>
                      <th className="px-4 py-3">Order ID</th>
                      <th className="px-4 py-3">Time</th>
                      <th className="px-4 py-3">Items</th>
                      <th className="px-4 py-3">Status</th>
                      <th className="px-4 py-3">SLA</th>
                      <th className="px-4 py-3 text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((o) => {
                      const sla = slaOf(o)
                      return (
                        <tr key={o.id} className="border-t border-slate-800 hover:bg-slate-800/40">
                          <td className="px-4 py-3 font-medium">#{o.id}</td>
                          <td className="px-4 py-3 text-slate-400">
                            {new Date(o.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                          </td>
                          <td className="px-4 py-3 text-slate-400">{o.items?.length ?? 0} items</td>
                          <td className="px-4 py-3"><StatusBadge status={o.status} /></td>
                          <td className={`px-4 py-3 font-mono text-xs ${TEXT[sla.tone]}`}>{sla.text}</td>
                          <td className="px-4 py-3 text-right">
                            <button
                              onClick={() => goToOrder(o)}
                              className="px-3 py-1.5 rounded-lg border border-slate-700 hover:bg-slate-800 text-xs font-medium transition-colors"
                            >
                              View
                            </button>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
              <div className="px-4 py-3 border-t border-slate-800">
                <button onClick={() => navigate('/orders')} className="text-xs text-red-300 hover:text-red-200">
                  View All Orders &rarr;
                </button>
              </div>
            </div>

            <Panel title="Store Health">
              <div className="flex justify-end -mt-8 mb-2">
                <span className={`text-xs px-2 py-1 rounded-full ${healthy ? 'bg-emerald-500/15 text-emerald-300' : 'bg-amber-500/15 text-amber-300'}`}>
                  {healthy ? 'Good' : 'Needs attention'}
                </span>
              </div>
              <Row label="Fulfillment Rate" value={`${rate.toFixed(0)}%`} tone={rate >= 90 ? 'ok' : rate >= 75 ? 'warn' : 'bad'} />
              <Row label="Avg Picking Time" value={`${stats.avg_picking_minutes.toFixed(1)} min`} />
              <Row label="Avg Packing Time" value={`${stats.avg_packing_minutes.toFixed(1)} min`} />
              <Row label="Delayed Orders" value={stats.delayed_orders} tone={stats.delayed_orders === 0 ? 'ok' : 'bad'} />
              <Row label="Open Exceptions" value={stats.open_exceptions} tone={stats.open_exceptions === 0 ? 'ok' : 'bad'} />
            </Panel>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Panel title="Inventory Snapshot" linkLabel="View All" onLink={() => navigate('/inventory')}>
              <Row label="Low Stock" value={stats.low_stock_products} tone={stats.low_stock_products > 0 ? 'warn' : 'ok'} />
              <Row label="Out of Stock" value={stats.out_of_stock_products} tone={stats.out_of_stock_products > 0 ? 'bad' : 'ok'} />
              <Row label="Expiring Batches" value={stats.expiring_stock_batches} tone={stats.expiring_stock_batches > 0 ? 'warn' : 'ok'} />
            </Panel>
            <Panel title="Inbound (Today)" linkLabel="View" onLink={() => navigate('/receiving')}>
              <Row label="Pending Receivings" value={stats.pending_receivings} />
              <Row label="Pending Putaway" value={stats.pending_putaway ?? 0} />
              <Row label="Pending Stock Transfers" value={stats.pending_stock_transfers} />
            </Panel>
            <Panel title="Store Performance" linkLabel="View" onLink={() => navigate('/performance')}>
              <Row label="Order Fill Rate" value={`${rate.toFixed(0)}%`} tone={rate >= 90 ? 'ok' : 'warn'} />
              <Row label="Items / Hour" value={(stats.items_per_hour ?? 0).toFixed(1)} />
              <Row label="Active Staff" value={stats.active_staff} />
            </Panel>
          </div>
        </>
      )}
    </div>
  )
}