import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { listWarehouseOrders } from '../api/warehouse'
import { NavLink } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

const navItems: { to: string; label: string; managerOnly?: boolean; inventoryToo?: boolean; putterToo?: boolean }[] = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/orders', label: 'Order Queue' },
  { to: '/picking-monitor', label: 'Picking Monitor', managerOnly: true },
  { to: '/putaway', label: 'Putaway', inventoryToo: true, putterToo: true },
  { to: '/receiving', label: 'Receiving', inventoryToo: true },
  { to: '/locations', label: 'Locations', inventoryToo: true },
  { to: '/racks', label: 'Racks & Bins', inventoryToo: true },
  { to: '/substitutions', label: 'Substitution' },
  { to: '/returns', label: 'Returns' },
  { to: '/inventory', label: 'Inventory' },
  { to: '/exceptions', label: 'Exceptions' },
  { to: '/staff', label: 'Staff' },
  { to: '/checkin-qr', label: 'Store QR' },
  { to: '/performance', label: 'Performance' },
]

export default function Layout({ children }: { children: ReactNode }) {
  const { staff, logout } = useAuth()

  const [newCount, setNewCount] = useState(0)
  useEffect(() => {
    if (!staff) return
    let alive = true
    const NEW_VIEW = ['confirmed', 'picking', 'picked', 'packing', 'packed', 'ready_for_dispatch']
    const loadCount = async () => {
      try {
        const data = await listWarehouseOrders({ page: 1, limit: 100 })
        if (alive) setNewCount((data.orders ?? []).filter((o) => NEW_VIEW.includes(o.status)).length)
      } catch {
        // badge is optional; ignore errors
      }
    }
    loadCount()
    const timer = setInterval(loadCount, 5000)
    return () => {
      alive = false
      clearInterval(timer)
    }
  }, [staff])
  const warehouseLabel = staff?.warehouse?.name ?? `WAREHOUSE #${staff?.warehouse_id ?? '-'}`
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex">
      <aside className="w-60 shrink-0 border-r border-slate-800 bg-slate-900 flex flex-col">
        <div className="px-5 py-5 border-b border-slate-800">
          <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">
            {warehouseLabel}
          </p>
          <p className="font-display text-xl leading-none">Store Staff App</p>
        </div>
        <nav className="flex-1 px-2 py-4 space-y-0.5">
          {navItems.filter((item) => (!item.managerOnly && !item.inventoryToo) || ['warehouse_manager', 'supervisor'].includes(staff?.role ?? '') || (item.inventoryToo && staff?.role === 'inventory_staff') || (item.putterToo && staff?.role === 'putter')).map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `flex items-center gap-2.5 pl-3 pr-3 py-2 text-sm border-l-2 transition-colors ${
                  isActive
                    ? 'border-red-500 bg-red-500/10 text-red-300 font-medium'
                    : 'border-transparent text-slate-400 hover:bg-slate-800 hover:text-slate-200'
                }`
              }
            >
              <span className="flex-1">{item.label}</span>
              {item.to === '/orders' && newCount > 0 && (
                <span className="min-w-[20px] h-5 px-1.5 rounded-full bg-red-500 text-white text-[11px] font-semibold flex items-center justify-center">
                  {newCount}
                </span>
              )}
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

