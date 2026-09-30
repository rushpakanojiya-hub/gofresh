import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

const navGroups = [
  {
    section: 'Dashboard',
    items: [
      { to: '/dashboard', label: 'Overview', icon: '\u25C6' },
        { to: '/control-tower', label: 'Control Tower', icon: '\u25C9' },
        { to: '/unassigned-orders', label: 'Unassigned Orders', icon: '\u26A0' },
        { to: '/delivery-management', label: 'Delivery Management', icon: '\u25B2' },
        { to: '/operations-analytics', label: 'Operations Analytics', icon: '\u25A6' },
        { to: '/audit-logs', label: 'Audit Logs', icon: '\u2637' },
    ],
  },
  {
    section: 'User Management',
    items: [
      { to: '/customers', label: 'Customers', icon: '\u25C7' },
      { to: '/delivery-partners', label: 'Delivery Partners', icon: '\u25B2' },
      { to: '/staff-roles', label: 'Staff & Roles', icon: '\u25C8' },
    ],
  },
  {
    section: 'Product Management',
    items: [
      { to: '/products', label: 'Products', icon: '\u25A3' },
      { to: '/categories', label: 'Categories', icon: '\u25A4' },
    ],
  },
  {
    section: 'Inventory',
    items: [
      { to: '/inventory', label: 'Inventory Overview', icon: '\u25A6' },
      { to: '/stock-transfers', label: 'Stock Transfers', icon: '\u21C4' },
    ],
  },
  {
    section: 'Store Management',
    items: [
      { to: '/warehouses', label: 'Warehouses', icon: '\u25A0' },
      { to: '/warehouse-staff', label: 'Warehouse Staff', icon: '\u25AB' },
      { to: '/picker-performance', label: 'Picker Performance', icon: '\u25C9' },
    ],
  },
  {
    section: 'Procurement',
    items: [
      { to: '/suppliers', label: 'Suppliers', icon: '\u25C9' },
      { to: '/purchase-orders', label: 'Purchase Orders', icon: '\u25A1' },
      { to: '/replenishment', label: 'Low Stock / Replenishment', icon: '\u26A0' },
    ],
  },
  {
    section: 'Order Management',
    items: [
      { to: '/orders', label: 'Orders', icon: '\u25A5' },
      { to: '/returns', label: 'Returns', icon: '\u21BA' },
      { to: '/payments', label: 'Payments', icon: '\u25CA' },
    ],
  },
  {
    section: 'Delivery Management',
    items: [
      { to: '/delivery-zones', label: 'Delivery Zones', icon: '\u2302' },
    ],
  },
  {
    section: 'Support',
      items: [
        { to: '/support', label: 'Support Tickets', icon: '\u2709' },
      ],
    },
]

export default function Layout({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth()
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex">
      <aside className="w-60 shrink-0 border-r border-slate-800 bg-slate-900 flex flex-col">
        <div className="px-6 py-5 border-b border-slate-800">
          <p className="text-sm font-semibold tracking-wide text-slate-100">
            Ecommerce Admin
          </p>
        </div>
        <nav className="flex-1 px-3 py-4 space-y-4 overflow-y-auto">
          {navGroups.map((group) => (
            <div key={group.section}>
              <p className="px-3 mb-1 text-[10px] font-semibold uppercase tracking-widest text-slate-600">
                {group.section}
              </p>
              <div className="space-y-1">
                {group.items.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    className={({ isActive }) =>
                      `flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                        isActive
                          ? 'bg-indigo-500/15 text-indigo-300'
                          : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800'
                      }`
                    }
                  >
                    <span className="text-xs opacity-70">{item.icon}</span>
                    {item.label}
                  </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>
        <div className="px-4 py-4 border-t border-slate-800">
          <p className="text-xs text-slate-500 mb-2 truncate">
            {user?.phone} &middot; {user?.role}
          </p>
          <button
            onClick={logout}
            className="w-full text-left text-sm px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
          >
            Log out
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto">{children}</main>
    </div>
  )
}
