import { useEffect, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { getDashboard, listMyStockTransfers, listWarehouseNotifications } from '../api/warehouse'
import { useAuth } from '../context/AuthContext'
import Icon from '../components/Icon'
import type { WarehouseDashboardStats, StockTransfer, WarehouseNotification } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

type Tone = 'default' | 'warning' | 'danger' | 'success'

const TONES: Record<Tone, { bg: string; fg: string }> = {
  default: { bg: '#dcfce7', fg: '#16a34a' },
  success: { bg: '#dcfce7', fg: '#16a34a' },
  warning: { bg: '#ffedd5', fg: '#ea580c' },
  danger: { bg: '#ffe4e6', fg: '#e11d48' },
}

const PALETTES: Record<string, { bg: string; fg: string }> = {
  green: { bg: '#dcfce7', fg: '#16a34a' },
  blue: { bg: '#dbeafe', fg: '#2563eb' },
  purple: { bg: '#ede9fe', fg: '#7c3aed' },
  orange: { bg: '#ffedd5', fg: '#ea580c' },
  red: { bg: '#ffe4e6', fg: '#e11d48' },
}

function StatCard({
  label,
  value,
  suffix,
  icon,
  tone = 'default',
  color,
  onClick,
}: {
  label: string
  value: number | string
  suffix?: string
  icon: string
  tone?: Tone
  color?: string
  onClick?: () => void
}) {
  const c = (color && PALETTES[color]) || TONES[tone]
  return (
    <div
      onClick={onClick}
      className={`bg-white border border-slate-800 rounded-2xl p-5 flex items-center gap-4 ${onClick ? 'cursor-pointer hover:shadow-md transition-shadow' : ''}`}
    >
      <div className="w-12 h-12 rounded-full flex items-center justify-center shrink-0" style={{ background: c.bg, color: c.fg }}>
        <Icon name={icon} size={22} />
      </div>
      <div>
        <p className="text-sm text-slate-300">{label}</p>
        <p className="text-3xl font-bold leading-tight">
          {value}
          {suffix ? <span className="text-sm font-normal text-slate-400 ml-1">{suffix}</span> : null}
        </p>
      </div>
    </div>
  )
}

function timeAgo(iso: string): string {
  const mins = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60000))
  if (mins < 60) return `${mins} min ago`
  const hrs = Math.round(mins / 60)
  if (hrs < 24) return `${hrs} hr ago`
  return `${Math.round(hrs / 24)} d ago`
}

function MotherPanels() {
  const navigate = useNavigate()
  const [transfers, setTransfers] = useState<StockTransfer[]>([])
  const [notes, setNotes] = useState<WarehouseNotification[]>([])

  useEffect(() => {
    listMyStockTransfers().then((r) => setTransfers(r.stock_transfers ?? [])).catch(() => {})
    listWarehouseNotifications({ page: 1, limit: 5 }).then((r) => setNotes(r.notifications ?? [])).catch(() => {})
  }, [])

  const days: { key: string; label: string; count: number }[] = []
  for (let i = 6; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    days.push({
      key: d.toLocaleDateString('en-CA'),
      label: d.toLocaleDateString('en-IN', { day: '2-digit', month: 'short' }),
      count: 0,
    })
  }
  for (const t of transfers) {
    if (!t.created_at) continue
    const k = new Date(t.created_at).toLocaleDateString('en-CA')
    const day = days.find((x) => x.key === k)
    if (day) day.count += 1
  }
  const max = Math.max(1, ...days.map((d) => d.count))

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
      <div className="lg:col-span-2 bg-white border border-slate-800 rounded-2xl p-5">
        <div className="flex items-center gap-2 mb-4">
          <span className="text-indigo-500"><Icon name="chart" size={18} /></span>
          <h2 className="font-semibold">Stock Transfers (last 7 days)</h2>
        </div>
        <div className="flex items-end gap-3 h-40">
          {days.map((d) => (
            <div key={d.key} className="flex-1 flex flex-col items-center justify-end h-full">
              <span className="text-xs text-slate-400 mb-1">{d.count}</span>
              <div className="w-full rounded-t-md bg-indigo-500" style={{ height: `${(d.count / max) * 100}%`, minHeight: d.count > 0 ? 4 : 2, opacity: d.count > 0 ? 1 : 0.25 }} />
            </div>
          ))}
        </div>
        <div className="flex gap-3 mt-2">
          {days.map((d) => (
            <span key={d.key} className="flex-1 text-center text-xs text-slate-400">{d.label}</span>
          ))}
        </div>
      </div>

      <div className="bg-white border border-slate-800 rounded-2xl p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <span className="text-indigo-500"><Icon name="bell" size={18} /></span>
            <h2 className="font-semibold">Recent Activity</h2>
          </div>
          <button onClick={() => navigate('/notifications')} className="text-xs text-indigo-600 hover:underline">View All</button>
        </div>
        {notes.length === 0 ? (
          <p className="text-sm text-slate-400">No recent activity.</p>
        ) : (
          <ul className="space-y-3">
            {notes.map((n) => (
              <li key={n.id} className="flex items-start gap-3">
                <span className="mt-1 w-2 h-2 rounded-full shrink-0" style={{ background: n.is_read ? '#a9b2ad' : '#16a34a' }} />
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{n.title}</p>
                  <p className="text-xs text-slate-400">{timeAgo(n.created_at)}</p>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function isMotherWarehouse(): boolean {
  try {
    return JSON.parse(localStorage.getItem('warehouse_staff') ?? '{}')?.warehouse?.warehouse_type === 'mother_warehouse'
  } catch {
    return false
  }
}

export default function Dashboard() {
  const navigate = useNavigate()
  const { staff } = useAuth()
  const [stats, setStats] = useState<WarehouseDashboardStats | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const mother = isMotherWarehouse()

  const load = useCallback(async () => {
    setError(null)
    try {
      setStats(await getDashboard())
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

  const hour = new Date().getHours()
  const greeting = hour < 12 ? 'Good Morning' : hour < 17 ? 'Good Afternoon' : 'Good Evening'
  const wh = staff?.warehouse as unknown as { name?: string; city?: string } | undefined

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-start justify-between gap-4 mb-6">
        <div>
          <h1 className="text-3xl font-bold">{greeting}, {staff?.name}</h1>
          <div className="w-12 h-1 rounded bg-indigo-500 mt-2 mb-2" />
          <p className="text-sm text-slate-300">Here&apos;s what&apos;s happening at your warehouse today.</p>
        </div>
        <div className="flex items-center gap-3">
          <svg className="hidden xl:block" width="230" height="96" viewBox="0 0 230 96" fill="none"><ellipse cx="115" cy="88" rx="108" ry="7" fill="#dcfce7"/><rect x="34" y="30" width="162" height="56" rx="3" fill="#e5e9e7"/><path d="M26 34L115 8l89 26z" fill="#cfd8d3"/><rect x="92" y="46" width="46" height="40" rx="2" fill="#ffffff" stroke="#a9b2ad"/><path d="M96 54h38M96 62h38M96 70h38" stroke="#d3d9d6" stroke-width="2"/><rect x="146" y="64" width="22" height="22" fill="#fbbf24"/><rect x="170" y="70" width="18" height="16" fill="#f59e0b"/><rect x="40" y="62" width="40" height="22" rx="3" fill="#16a34a"/><rect x="72" y="66" width="14" height="18" rx="2" fill="#15803d"/><circle cx="52" cy="86" r="4" fill="#334155"/><circle cx="74" cy="86" r="4" fill="#334155"/></svg>
          <div className="bg-white border border-slate-800 rounded-2xl px-5 py-3 flex items-center gap-3">
            <span className="text-indigo-500"><Icon name="pin" size={20} /></span>
            <div>
              <p className="text-xs text-slate-400">Warehouse</p>
              <p className="text-sm font-semibold">{wh?.name ?? `#${staff?.warehouse_id ?? ''}`}</p>
              {wh?.city ? <p className="text-xs text-slate-400 capitalize">{wh.city}</p> : null}
            </div>
          </div>
          <button onClick={load} className="text-sm px-4 py-2 rounded-xl bg-white border border-slate-800 hover:bg-slate-950 transition-colors">
            Refresh
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-4 text-sm px-4 py-3 rounded-xl border border-rose-900 bg-rose-950 text-rose-300">{error}</div>
      )}
      {isLoading && !stats && <p className="text-sm text-slate-400">Loading...</p>}

      {stats && (
        <>
          {!mother && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
              <StatCard icon="cart" label="Today's Orders" value={stats.today_orders} />
              <StatCard icon="box" label="New Orders" value={stats.new_orders} tone={stats.new_orders > 0 ? 'warning' : 'default'} onClick={() => navigate('/orders?status=confirmed')} />
              <StatCard icon="layers" label="Picking" value={stats.picking} />
              <StatCard icon="box" label="Packed" value={stats.packed} />
              <StatCard icon="download" label="Ready for Dispatch" value={stats.ready_for_dispatch} tone="success" onClick={() => navigate('/orders?status=ready_for_dispatch')} />
              <StatCard icon="chart" label="Completed Today" value={stats.completed_today} tone="success" />
              <StatCard icon="alert" label="Cancelled Today" value={stats.cancelled_today} tone={stats.cancelled_today > 0 ? 'danger' : 'default'} />
              <StatCard icon="chart" label="Fulfillment Rate" value={stats.fulfillment_rate.toFixed(0)} suffix="%" tone="success" />
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard icon="download" label="Pending Receivings" color="blue" value={stats.pending_receivings} tone={stats.pending_receivings > 0 ? 'warning' : 'default'} onClick={() => navigate('/receiving')} />
            <StatCard icon="repeat" label="Pending Stock Transfers" color="purple" value={stats.pending_stock_transfers} tone={stats.pending_stock_transfers > 0 ? 'warning' : 'default'} onClick={() => navigate('/stock-transfers')} />
            <StatCard icon="box" label="Low Stock Products" color="orange" value={stats.low_stock_products} tone={stats.low_stock_products > 0 ? 'warning' : 'default'} onClick={() => navigate('/inventory')} />
            <StatCard icon="alert" label="Out of Stock Products" value={stats.out_of_stock_products} tone={stats.out_of_stock_products > 0 ? 'danger' : 'default'} onClick={() => navigate('/inventory')} />
            <StatCard icon="calendar" label="Expiring Batches (7d)" color="orange" value={stats.expiring_stock_batches} tone={stats.expiring_stock_batches > 0 ? 'warning' : 'default'} onClick={() => navigate('/batches')} />
            <StatCard icon="alert" label="Open Exceptions" value={stats.open_exceptions} tone={stats.open_exceptions > 0 ? 'danger' : 'default'} onClick={() => navigate('/exceptions')} />
            <StatCard icon="users" label="Active Staff" color="blue" value={stats.active_staff} />
            {!mother && <StatCard icon="box" label="Pending Handovers" value={stats.pending_handovers} tone={stats.pending_handovers > 0 ? 'warning' : 'default'} />}
            {!mother && <StatCard icon="alert" label="Delayed Orders" value={stats.delayed_orders} tone={stats.delayed_orders > 0 ? 'danger' : 'default'} />}
            {!mother && <StatCard icon="chart" label="Avg Picking Time" value={stats.avg_picking_minutes.toFixed(1)} suffix="min" />}
            {!mother && <StatCard icon="chart" label="Avg Packing Time" value={stats.avg_packing_minutes.toFixed(1)} suffix="min" />}
          </div>
        {mother && <MotherPanels />}
        </>
      )}
    </div>
  )
}