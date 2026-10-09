import { useEffect, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { getDashboard } from '../api/warehouse'
import type { WarehouseDashboardStats } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

function StatCard({
  label,
  value,
  suffix,
  tone = 'default',
  icon,
}: {
  label: string
  value: number | string
  suffix?: string
  tone?: 'default' | 'warning' | 'danger' | 'success'
  icon?: React.ReactNode
}) {
  const toneClass =
    tone === 'warning'
      ? 'text-red-300'
      : tone === 'danger'
        ? 'text-rose-300'
        : tone === 'success'
          ? 'text-emerald-300'
          : 'text-slate-100'

  const ringClass =
    tone === 'warning'
      ? 'ring-red-500/20'
      : tone === 'danger'
        ? 'ring-rose-500/20'
        : tone === 'success'
          ? 'ring-emerald-500/20'
          : 'ring-slate-700/30'

  return (
    <div className={`relative border border-slate-800 bg-gradient-to-b from-slate-900 to-slate-900/60 rounded-xl p-4 overflow-hidden ring-1 ${ringClass} hover:border-slate-700 transition-colors`}>
      <div className="flex items-start justify-between mb-2">
        <p className="text-xs text-slate-400">{label}</p>
        {icon && <span className={`opacity-60 ${toneClass}`}>{icon}</span>}
      </div>
      <p className={`font-mono text-2xl font-semibold ${toneClass}`}>
        {value}
        {suffix ? <span className="font-sans text-sm font-normal text-slate-500 ml-1">{suffix}</span> : null}
      </p>
    </div>
  )
}

const IconOrders = (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4Z"/><path d="M3 6h18"/><path d="M16 10a4 4 0 0 1-8 0"/></svg>
)
const IconClock = (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
)
const IconCheck = (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
)
const IconAlert = (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>
)
const IconTruck = (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 18V6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v11a1 1 0 0 0 1 1h2"/><path d="M15 18H9"/><path d="M19 18h2a1 1 0 0 0 1-1v-3.65a1 1 0 0 0-.22-.624l-3.48-4.35A1 1 0 0 0 17.52 8H14"/><circle cx="17" cy="18" r="2"/><circle cx="7" cy="18" r="2"/></svg>
)
const IconUsers = (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
)

export default function Dashboard() {
  const navigate = useNavigate()
  const [stats, setStats] = useState<WarehouseDashboardStats | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const load = useCallback(async () => {
    setError(null)
    setIsLoading(true)
    try {
      const data = await getDashboard()
      setStats(data)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load dashboard.'))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const interval = setInterval(load, 30000)
    return () => clearInterval(interval)
  }, [load])

  const alerts: { label: string; tone: 'warning' | 'danger'; onClick?: () => void }[] = []
  if (stats) {
    if (stats.new_orders > 0) {
      alerts.push({
        label: `${stats.new_orders} new order(s) waiting to be accepted`,
        tone: 'warning',
        onClick: () => navigate('/orders?status=confirmed'),
      })
    }
    if (stats.open_exceptions > 0) {
      alerts.push({
        label: `${stats.open_exceptions} exception(s) need attention`,
        tone: 'danger',
        onClick: () => navigate('/exceptions'),
      })
    }
    if (stats.pending_handovers > 0) {
      alerts.push({
        label: `${stats.pending_handovers} order(s) ready for handover`,
        tone: 'warning',
        onClick: () => navigate('/handover'),
      })
    }
    if (stats.delayed_orders > 0) {
      alerts.push({
        label: `${stats.delayed_orders} order(s) running behind schedule`,
        tone: 'danger',
        onClick: () => navigate('/orders'),
      })
    }
  }

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-1">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">Overview</p>
          <h1 className="font-display text-2xl font-semibold">Dashboard</h1>
        </div>
        <button
          onClick={load}
          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors flex items-center gap-1.5"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></svg>
          Refresh
        </button>
      </div>

      {isLoading && <p className="text-sm text-slate-400 mt-6">Loading dashboard...</p>}
      {error && !isLoading && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mt-6">
          {error}
        </div>
      )}

      {stats && (
        <>
          {alerts.length > 0 && (
            <div className="mt-6 mb-6 space-y-2">
              {alerts.map((a, i) => (
                <button
                  key={i}
                  onClick={a.onClick}
                  disabled={!a.onClick}
                  className={`w-full text-left text-sm px-4 py-2.5 rounded-lg border flex items-center gap-2 ${
                    a.tone === 'danger'
                      ? 'border-rose-900 bg-rose-950/40 text-rose-300'
                      : 'border-red-900 bg-red-950/30 text-red-300'
                  } ${a.onClick ? 'hover:brightness-125 cursor-pointer' : 'cursor-default'}`}
                >
                  {a.label}
                </button>
              ))}
            </div>
          )}

          <p className="text-xs uppercase tracking-wide text-slate-500 mb-2 mt-6">Today</p>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
            <StatCard label="Today's Orders" value={stats.today_orders} icon={IconOrders} />
            <StatCard label="New Orders" value={stats.new_orders} tone={stats.new_orders > 0 ? 'warning' : 'default'} icon={IconAlert} />
            <StatCard label="Picking" value={stats.picking} icon={IconClock} />
            <StatCard label="Packed" value={stats.packed} icon={IconCheck} />
            <StatCard label="Ready for Dispatch" value={stats.ready_for_dispatch} tone="success" icon={IconTruck} />
            <StatCard label="Completed Today" value={stats.completed_today} tone="success" icon={IconCheck} />
            <StatCard label="Cancelled Today" value={stats.cancelled_today} tone={stats.cancelled_today > 0 ? 'danger' : 'default'} icon={IconAlert} />
            <StatCard label="Fulfillment Rate" value={stats.fulfillment_rate.toFixed(0)} suffix="%" tone="success" icon={IconCheck} />
          </div>

          <p className="text-xs uppercase tracking-wide text-slate-500 mb-2">Operational Alerts</p>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
            <StatCard label="Open Exceptions" value={stats.open_exceptions} tone={stats.open_exceptions > 0 ? 'danger' : 'default'} icon={IconAlert} />
            <StatCard label="Pending Handovers" value={stats.pending_handovers} tone={stats.pending_handovers > 0 ? 'warning' : 'default'} icon={IconTruck} />
            <StatCard label="Delayed Orders" value={stats.delayed_orders} tone={stats.delayed_orders > 0 ? 'danger' : 'default'} icon={IconClock} />
          </div>

          <p className="text-xs uppercase tracking-wide text-slate-500 mb-2">Performance</p>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <StatCard label="Avg Picking Time" value={stats.avg_picking_minutes.toFixed(1)} suffix="min" icon={IconClock} />
            <StatCard label="Avg Packing Time" value={stats.avg_packing_minutes.toFixed(1)} suffix="min" icon={IconClock} />
            <StatCard label="Active Staff" value={stats.active_staff} icon={IconUsers} />
            <StatCard label="Items / Hour" value={(stats.items_per_hour ?? 0).toFixed(1)} suffix="items/hr" icon={IconClock} />
            <StatCard label="Online Pickers" value={stats.online_pickers ?? 0} icon={IconUsers} />
            <StatCard label="Pending Putaway" value={stats.pending_putaway ?? 0} icon={IconClock} />
          </div>
        </>
      )}
    </div>
  )
}
