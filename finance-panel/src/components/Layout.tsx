import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

const NAV_ITEMS = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/revenue', label: 'Revenue' },
  { to: '/mis', label: 'Weekly MIS' },
  { to: '/payments', label: 'Payments \u0026 Refunds' },
  { to: '/finance/payments-detailed', label: 'Payments - Detailed List' },
  { to: '/finance/general-ledger', label: 'General Ledger' },
  { to: '/finance/vendor-bank-change-requests', label: 'Vendor Bank Changes' },
  { to: '/finance/mismatch-center', label: 'Mismatch Center' },
  { to: '/finance/rider-payouts', label: 'Rider Payouts' },
  { to: '/finance/rider-cod-deposits', label: 'Rider COD Deposits' },
  { to: '/finance/audit-logs', label: 'Audit Logs' },
  { to: '/finance/rider-payable-report', label: 'Rider Payable Report' },
  { to: '/finance/gateway-settlement-report', label: 'Gateway Settlement Report' },
  { to: '/expenses', label: 'Expenses' },
  { to: '/payroll', label: 'Payroll' },
  { to: '/profit-loss', label: 'Profit \u0026 Loss' },
  { to: '/gst', label: 'GST' },
  { to: '/invoices', label: 'Invoices' },
  { to: '/reports', label: 'Reports' },
  { to: '/reports/range', label: 'Custom Range Report' },
  { to: '/reports/finance', label: 'Finance Reports' },
  { to: '/operations', label: 'Operations' },
  { to: '/settings', label: 'Settings' },
]

const ACCOUNTING_NAV_ITEMS = [
  { to: '/accounting/vendors', label: 'Vendors' },
  { to: '/accounting/vendor-bills', label: 'Vendor Bills' },
  { to: '/accounting/accounts', label: 'Chart of Accounts' },
  { to: '/accounting/ledger', label: 'Ledger' },
  { to: '/accounting/bank-reconciliation', label: 'Bank Reconciliation' },
]

function navLinkClass({ isActive }: { isActive: boolean }) {
  return `block px-3 py-2 rounded-lg text-sm transition-colors ${
    isActive
      ? 'bg-emerald-600/15 text-emerald-400 font-medium'
      : 'text-slate-400 hover:bg-slate-800 hover:text-slate-100'
  }`
}

export default function Layout() {
  const navigate = useNavigate()
  const { user, logout } = useAuth()

  function handleLogout() {
    logout()
    navigate('/login')
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex">
      <aside className="w-56 shrink-0 border-r border-slate-800 flex flex-col">
        <div className="px-4 py-5 border-b border-slate-800">
          <p className="font-mono text-[10px] tracking-widest text-emerald-500 uppercase">
            Finance & Accounting
          </p>
          <h1 className="text-sm font-semibold mt-1">Finance Panel</h1>
        </div>
        <nav className="flex-1 px-2 py-4 space-y-1 overflow-y-auto">
          {NAV_ITEMS.map((item) => (
            <NavLink key={item.to} to={item.to} className={navLinkClass}>
              {item.label}
            </NavLink>
          ))}

          <p className="px-3 pt-4 pb-1 text-[10px] tracking-widest text-slate-600 uppercase">
            Accounting
          </p>
          {ACCOUNTING_NAV_ITEMS.map((item) => (
            <NavLink key={item.to} to={item.to} className={navLinkClass}>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="px-4 py-4 border-t border-slate-800">
          {user && (
            <p className="text-xs text-slate-500 truncate mb-2">{user.phone ?? user.name}</p>
          )}
          <button
            onClick={handleLogout}
            className="w-full text-left text-xs text-slate-400 hover:text-red-400 transition-colors"
          >
            Log out
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  )
}
