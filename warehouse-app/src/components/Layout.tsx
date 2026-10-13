import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { getDashboard, listWarehouseNotifications } from '../api/warehouse'
import { useAuth } from '../context/AuthContext'
import Icon from './Icon'

const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: 'home' },
  { to: '/orders', label: 'Orders', icon: 'cart' },
  { to: '/inventory', label: 'Inventory', icon: 'box' },
  { to: '/stock-transfers', label: 'Stock Transfers', icon: 'repeat' },
  { to: '/locations', label: 'Locations', icon: 'pin' },
  { to: '/stock-operations', label: 'Stock Operations', icon: 'layers' },
  { to: '/receiving', label: 'Receiving', icon: 'download' },
  { to: '/batches', label: 'Batch & Expiry', icon: 'calendar' },
  { to: '/exceptions', label: 'Exceptions', icon: 'alert' },
  { to: '/substitutions', label: 'Substitution', icon: 'shuffle' },
  { to: '/performance', label: 'Performance', icon: 'chart' },
  { to: '/staff', label: 'Staff', icon: 'users' },
  { to: '/notifications', label: 'Notifications', icon: 'bell' },
  { to: '/audit-logs', label: 'Audit Logs', icon: 'file' },
]

const hiddenForMother = ['/orders', '/substitutions']

export default function Layout({ children }: { children: ReactNode }) {
  const { staff, logout } = useAuth()
  const [openExceptions, setOpenExceptions] = useState(0)
  const [unread, setUnread] = useState(0)
  useEffect(() => {
    let alive = true
    const load = () => {
      getDashboard().then((d) => { if (alive) setOpenExceptions(d.open_exceptions ?? 0) }).catch(() => {})
      listWarehouseNotifications({ is_read: false, page: 1, limit: 1 }).then((n) => { if (alive) setUnread(n.unread_count ?? 0) }).catch(() => {})
    }
    load()
    const t = setInterval(load, 30000)
    return () => { alive = false; clearInterval(t) }
  }, [])
  const isMother = (staff?.warehouse as unknown as { warehouse_type?: string } | undefined)?.warehouse_type === 'mother_warehouse'
  const warehouseLabel = staff?.warehouse?.name ?? `Warehouse #${staff?.warehouse_id ?? '—'}`
  const today = new Date().toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' })
  const initials = (staff?.name ?? '?').split(' ').map((p) => p[0]).slice(0, 2).join('').toUpperCase()
  const roleText = (staff?.role ?? '').replace(/_/g, ' ')

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex">
      <aside className="w-60 shrink-0 border-r border-slate-800 bg-white flex flex-col">
        <div className="px-5 py-5 flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-indigo-500 text-white flex items-center justify-center">
            <Icon name="home" size={22} />
          </div>
          <div>
            <p className="text-lg font-bold leading-none">GoFresh</p>
            <p className="text-xs text-slate-400 mt-1">Warehouse Panel</p>
          </div>
        </div>
        <nav className="flex-1 px-3 py-2 space-y-1">
          {navItems
            .filter((item) => !(isMother && hiddenForMother.includes(item.to)))
            .map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm transition-colors ${
                    isActive
                      ? 'bg-indigo-100 text-indigo-700 font-semibold'
                      : 'text-slate-300 hover:bg-slate-950 hover:text-slate-100'
                  }`
                }
              >
                <Icon name={item.icon} size={18} />
                <span className="flex-1">{item.label}</span>
                {item.to === '/exceptions' && openExceptions > 0 && (
                  <span className="min-w-5 h-5 px-1.5 rounded-full bg-rose-500 text-white text-xs font-semibold flex items-center justify-center">{openExceptions}</span>
                )}
              </NavLink>
            ))}
        </nav>
        <div className="px-3 py-4 border-t border-slate-800">
          <div className="flex items-center gap-3 px-3 py-3 mb-2 rounded-xl bg-slate-950">
            <div className="w-9 h-9 rounded-full bg-indigo-500 text-white text-xs font-bold flex items-center justify-center">{initials}</div>
            <div className="min-w-0">
              <p className="text-sm font-semibold truncate">{staff?.name}</p>
              <p className="text-xs text-slate-400 capitalize truncate">{roleText}</p>
            </div>
          </div>
          <button
            onClick={logout}
            className="w-full flex items-center gap-2 text-sm px-3 py-2 rounded-xl text-slate-300 hover:bg-slate-950 transition-colors"
          >
            <Icon name="logout" size={16} /> Log out
          </button>
        </div>
      </aside>
      <div className="flex-1 min-w-0 flex flex-col">
        <header className="h-16 shrink-0 bg-white border-b border-slate-800 px-6 flex items-center justify-end gap-4">
          <span className="text-sm text-slate-300 mr-auto">{warehouseLabel}</span>
          <span className="text-sm text-slate-300 flex items-center gap-2">
            <Icon name="calendar" size={16} /> {today}
          </span>
          <NavLink to="/notifications" className="text-slate-300 hover:text-slate-100">
            <span className="relative inline-block"><Icon name="bell" size={20} />{unread > 0 && <span className="absolute -top-2 -right-2 min-w-4 h-4 px-1 rounded-full bg-rose-500 text-white text-[10px] font-semibold flex items-center justify-center">{unread}</span>}</span>
          </NavLink>
          <div className="flex items-center gap-2">
            <div className="w-9 h-9 rounded-full bg-indigo-500 text-white text-xs font-bold flex items-center justify-center">{initials}</div>
            <div className="leading-tight">
              <p className="text-sm font-semibold">{staff?.name}</p>
              <p className="text-xs text-slate-400 capitalize">{roleText}</p>
            </div>
          </div>
        </header>
        <main className="flex-1 min-w-0">{children}</main>
      </div>
    </div>
  )
}