import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

const navItems = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/orders', label: 'Orders' },
  { to: '/inventory', label: 'Inventory' },
  { to: '/stock-transfers', label: 'Stock Transfers' },
  { to: '/locations', label: 'Locations' },
  { to: '/stock-operations', label: 'Stock Operations' },
  { to: '/receiving', label: 'Receiving' },
  { to: '/batches', label: 'Batch & Expiry' },
  { to: '/exceptions', label: 'Exceptions' },
  { to: '/substitutions', label: 'Substitution' },
  { to: '/performance', label: 'Performance' },
  { to: '/staff', label: 'Staff' },
  { to: '/notifications', label: 'Notifications' },
  { to: '/audit-logs', label: 'Audit Logs' },
]

const hiddenForMother = ['/orders', '/substitutions']

export default function Layout({ children }: { children: ReactNode }) {
  const { staff, logout } = useAuth()
  const isMother = (staff?.warehouse as unknown as { warehouse_type?: string } | undefined)?.warehouse_type === 'mother_warehouse'
  const warehouseLabel = staff?.warehouse?.name ?? `WAREHOUSE #${staff?.warehouse_id ?? '—'}`

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex">
      <aside className="w-60 shrink-0 border-r border-slate-800 bg-slate-900 flex flex-col">
        <div className="px-5 py-5 border-b border-slate-800">
          <p className="font-mono text-[10px] tracking-widest text-indigo-500 uppercase mb-1">
            {warehouseLabel}
          </p>
          <p className="font-display text-xl leading-none">Warehouse Panel</p>
        </div>
        <nav className="flex-1 px-2 py-4 space-y-0.5">
          {navItems.filter((item) => !(isMother && hiddenForMother.includes(item.to))).map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `flex items-center gap-2.5 pl-3 pr-3 py-2 text-sm border-l-2 transition-colors ${
                  isActive
                    ? 'border-indigo-500 bg-indigo-500/10 text-indigo-300 font-medium'
                    : 'border-transparent text-slate-400 hover:bg-slate-800 hover:text-slate-200'
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="px-4 py-4 border-t border-slate-800">
          <p className="text-xs font-medium text-slate-200">{staff?.name}</p>
          <p className="font-mono text-xs text-slate-500 mb-3">{staff?.phone}</p>
          <button
            onClick={logout}
            className="w-full text-xs px-3 py-2 border border-slate-700 hover:border-slate-600 hover:bg-slate-800 transition-colors"
          >
            Log out
          </button>
        </div>
      </aside>
      <main className="flex-1 min-w-0">{children}</main>
    </div>
  )
}

