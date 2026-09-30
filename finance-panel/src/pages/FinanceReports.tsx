import { useEffect, useState } from 'react'
import {
  getSalesRegister,
  getPurchaseRegister,
  getRiderPayable,
  getGatewaySettlement,
  getCashFlow,
  getBalanceSheet,
} from '../api/reports'
import type { RiderPayableRow, GatewaySettlementRow, CashFlowRow, BalanceSheet } from '../types/reports'

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(value)
}

function todayISO() {
  return new Date().toISOString().slice(0, 10)
}

function daysAgoISO(days: number) {
  const d = new Date()
  d.setDate(d.getDate() - days)
  return d.toISOString().slice(0, 10)
}

type TabKey = 'sales-register' | 'purchase-register' | 'rider-payable' | 'gateway-settlement' | 'cash-flow' | 'balance-sheet'

const TABS: { key: TabKey; label: string }[] = [
  { key: 'sales-register', label: 'Sales Register' },
  { key: 'purchase-register', label: 'Purchase Register' },
  { key: 'rider-payable', label: 'Rider Payable' },
  { key: 'gateway-settlement', label: 'Gateway Settlement' },
  { key: 'cash-flow', label: 'Cash Flow' },
  { key: 'balance-sheet', label: 'Balance Sheet' },
]

export default function FinanceReports() {
  const [tab, setTab] = useState<TabKey>('sales-register')
  const [from, setFrom] = useState(daysAgoISO(29))
  const [to, setTo] = useState(todayISO())
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [salesRegister, setSalesRegister] = useState<any[]>([])
  const [purchaseRegister, setPurchaseRegister] = useState<any[]>([])
  const [riderPayable, setRiderPayable] = useState<RiderPayableRow[]>([])
  const [gatewaySettlement, setGatewaySettlement] = useState<GatewaySettlementRow[]>([])
  const [cashFlow, setCashFlow] = useState<{ rows: CashFlowRow[]; net: number }>({ rows: [], net: 0 })
  const [balanceSheet, setBalanceSheet] = useState<BalanceSheet | null>(null)

  useEffect(() => {
    let cancelled = false
    setIsLoading(true)
    setError(null)

    const load = async () => {
      try {
        if (tab === 'sales-register') setSalesRegister(await getSalesRegister(from, to))
        else if (tab === 'purchase-register') setPurchaseRegister(await getPurchaseRegister(from, to))
        else if (tab === 'rider-payable') setRiderPayable((await getRiderPayable(from, to)).rider_payable)
        else if (tab === 'gateway-settlement') setGatewaySettlement((await getGatewaySettlement(from, to)).gateway_settlement)
        else if (tab === 'cash-flow') {
          const res = await getCashFlow(from, to)
          setCashFlow({ rows: res.by_category, net: res.net_cash_flow })
        } else if (tab === 'balance-sheet') setBalanceSheet(await getBalanceSheet(to))
      } catch (err: any) {
        if (!cancelled) setError(err.response?.data?.error ?? 'Could not load report.')
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    }
    load()

    return () => {
      cancelled = true
    }
  }, [tab, from, to])

  const totalInflow = cashFlow.rows.reduce((sum, r) => sum + r.inflow, 0)
  const totalOutflow = cashFlow.rows.reduce((sum, r) => sum + r.outflow, 0)
  const totalLiabilitiesAndEquity = balanceSheet
    ? balanceSheet.liabilities.total + balanceSheet.equity.total
    : 0

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold">Finance Reports</h1>
          <p className="text-sm text-slate-500">Sales & Purchase Registers, Rider Payable, Gateway Settlement, Cash Flow, Balance Sheet.</p>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <input type="date" value={from} max={to} onChange={(e) => setFrom(e.target.value)} className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5" />
          <span className="text-slate-500">to</span>
          <input type="date" value={to} min={from} max={todayISO()} onChange={(e) => setTo(e.target.value)} className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5" />
        </div>
      </div>

      <div className="flex gap-1 mb-6 border-b border-slate-800">
        {TABS.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-3 py-2 text-sm rounded-t-lg transition-colors ${
              tab === t.key ? 'bg-slate-900 text-emerald-400 border-b-2 border-emerald-500' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {isLoading && <p className="text-sm text-slate-500">Loading report...</p>}
      {!isLoading && error && <div className="border border-slate-800 rounded-xl p-6 text-sm text-slate-500">{error}</div>}

      {!isLoading && !error && tab === 'sales-register' && (
        <ReportTable
          rows={salesRegister}
          columns={[
            { key: 'invoice_number', label: 'Invoice #' },
            { key: 'customer_name', label: 'Customer' },
            { key: 'taxable_amount', label: 'Taxable', format: formatCurrency },
            { key: 'cgst_amount', label: 'CGST', format: formatCurrency },
            { key: 'sgst_amount', label: 'SGST', format: formatCurrency },
            { key: 'igst_amount', label: 'IGST', format: formatCurrency },
            { key: 'total_amount', label: 'Total', format: formatCurrency },
          ]}
          emptyLabel="No invoices in this range."
        />
      )}

      {!isLoading && !error && tab === 'purchase-register' && (
        <ReportTable
          rows={purchaseRegister}
          columns={[
            { key: 'bill_number', label: 'Bill #' },
            { key: 'vendor', label: 'Vendor', format: (v: any) => v?.name ?? '\u2014' },
            { key: 'amount', label: 'Amount', format: formatCurrency },
            { key: 'gst_amount', label: 'GST', format: formatCurrency },
            { key: 'amount_paid', label: 'Paid', format: formatCurrency },
          ]}
          emptyLabel="No vendor bills in this range."
        />
      )}

      {!isLoading && !error && tab === 'rider-payable' && (
        <ReportTable
          rows={riderPayable}
          columns={[
            { key: 'name', label: 'Rider' },
            { key: 'phone', label: 'Phone' },
            { key: 'delivered_count', label: 'Deliveries' },
            { key: 'payable', label: 'Payable', format: formatCurrency },
          ]}
          emptyLabel="No deliveries in this range."
        />
      )}

      {!isLoading && !error && tab === 'gateway-settlement' && (
        <ReportTable
          rows={gatewaySettlement}
          columns={[
            { key: 'gateway', label: 'Gateway' },
            { key: 'transaction_count', label: 'Transactions' },
            { key: 'gross_amount', label: 'Gross', format: formatCurrency },
            { key: 'refunded_amount', label: 'Refunded', format: formatCurrency },
          ]}
          emptyLabel="No settled payments in this range."
        />
      )}

      {!isLoading && !error && tab === 'cash-flow' && (
        <>
          <ReportTable
            rows={cashFlow.rows}
            columns={[
              { key: 'reference_type', label: 'Category' },
              { key: 'inflow', label: 'Inflow', format: formatCurrency },
              { key: 'outflow', label: 'Outflow', format: formatCurrency },
            ]}
            emptyLabel="No cash movement in this range."
          />
          <div className="mt-4 flex flex-wrap gap-4">
            <div className="border border-slate-800 rounded-xl p-4 inline-block">
              <p className="text-xs text-slate-500 mb-1">Total Inflows</p>
              <p className="text-lg font-semibold text-emerald-400">{formatCurrency(totalInflow)}</p>
            </div>
            <div className="border border-slate-800 rounded-xl p-4 inline-block">
              <p className="text-xs text-slate-500 mb-1">Total Outflows</p>
              <p className="text-lg font-semibold text-red-400">{formatCurrency(totalOutflow)}</p>
            </div>
            <div className="border border-slate-800 rounded-xl p-4 inline-block">
              <p className="text-xs text-slate-500 mb-1">Net Cash Flow</p>
              <p className={`text-lg font-semibold ${cashFlow.net >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>
                {formatCurrency(cashFlow.net)}
              </p>
            </div>
          </div>
        </>
      )}

      {!isLoading && !error && tab === 'balance-sheet' && balanceSheet && (
        <div className="space-y-6">
          <div className={`text-xs px-3 py-2 rounded-lg inline-block ${balanceSheet.balances ? 'bg-emerald-600/15 text-emerald-400' : 'bg-red-600/15 text-red-400'}`}>
            {balanceSheet.balances ? 'Balanced \u2713' : 'Out of balance \u2717'} {'\u2014'} as of {balanceSheet.as_of}
          </div>
          <div className="grid grid-cols-3 gap-6">
            <BalanceSheetSection title="Assets" total={balanceSheet.assets.total} accounts={balanceSheet.assets.accounts} />
            <BalanceSheetSection title="Liabilities" total={balanceSheet.liabilities.total} accounts={balanceSheet.liabilities.accounts} />
            <div>
              <h3 className="text-sm font-semibold mb-2 text-slate-300">Equity</h3>
              <div className="border border-slate-800 rounded-xl overflow-hidden">
                <div className="flex justify-between px-3 py-2 text-sm border-b border-slate-800 last:border-b-0">
                  <span className="text-slate-400">Retained Earnings</span>
                  <span>{formatCurrency(balanceSheet.equity.retained_earnings)}</span>
                </div>
              </div>
              <p className="text-xs text-slate-500 mt-2 px-1">Total: {formatCurrency(balanceSheet.equity.total)}</p>
            </div>
          </div>
          <div className="flex flex-wrap gap-4 pt-2 border-t border-slate-800">
            <div className="border border-slate-800 rounded-xl p-4 inline-block">
              <p className="text-xs text-slate-500 mb-1">Total Assets</p>
              <p className="text-lg font-semibold">{formatCurrency(balanceSheet.assets.total)}</p>
            </div>
            <div className="border border-slate-800 rounded-xl p-4 inline-block">
              <p className="text-xs text-slate-500 mb-1">Total Liabilities + Equity</p>
              <p className="text-lg font-semibold">{formatCurrency(totalLiabilitiesAndEquity)}</p>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function BalanceSheetSection({ title, total, accounts }: { title: string; total: number; accounts: { code: string; name: string; balance: number }[] }) {
  return (
    <div>
      <h3 className="text-sm font-semibold mb-2 text-slate-300">{title}</h3>
      <div className="border border-slate-800 rounded-xl overflow-hidden">
        {accounts.map((a) => (
          <div key={a.code} className="flex justify-between px-3 py-2 text-sm border-b border-slate-800 last:border-b-0">
            <span className="text-slate-400">{a.name}</span>
            <span>{formatCurrency(a.balance)}</span>
          </div>
        ))}
      </div>
      <p className="text-xs text-slate-500 mt-2 px-1">Total: {formatCurrency(total)}</p>
    </div>
  )
}

function ReportTable({
  rows,
  columns,
  emptyLabel,
}: {
  rows: any[]
  columns: { key: string; label: string; format?: (v: any) => string }[]
  emptyLabel: string
}) {
  if (rows.length === 0) {
    return <div className="border border-slate-800 rounded-xl p-6 text-sm text-slate-500">{emptyLabel}</div>
  }
  return (
    <div className="border border-slate-800 rounded-xl overflow-hidden overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="bg-slate-900 text-slate-400 text-xs uppercase">
          <tr>
            {columns.map((c) => (
              <th key={c.key} className="px-4 py-2 text-left font-medium">{c.label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i} className="border-t border-slate-800">
              {columns.map((c) => (
                <td key={c.key} className="px-4 py-2">
                  {c.format ? c.format(row[c.key]) : String(row[c.key] ?? '\u2014')}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
