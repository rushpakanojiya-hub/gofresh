import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { getUPICollections, reviewUPICollection } from '../api/admin'

const TABS = ['verified', 'rejected'] as const
type Tab = (typeof TABS)[number]

function fmtMoney(n: number) {
  return '\u20b9' + Number(n ?? 0).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function fmtDate(s?: string | null) {
  if (!s) return '-'
  const d = new Date(s)
  return isNaN(d.getTime()) ? '-' : d.toLocaleString('en-IN')
}

export default function UPICollections() {
  const [tab, setTab] = useState<Tab>('verified')
  const [rows, setRows] = useState<any[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)

  async function load(status: Tab) {
    setIsLoading(true)
    setError(null)
    try {
      const res = await getUPICollections(status)
      setRows(res.orders ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load UPI collections.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load(tab)
  }, [tab])

  async function review(o: any, action: 'verify' | 'reject') {
    let utr: string | undefined
    if (action === 'verify') {
      const v = window.prompt('UTR / bank reference (optional). Check the bank statement first.', '')
      if (v === null) return
      utr = v.trim() || undefined
    } else if (!window.confirm('Payment NOT received? Order #' + o.id + ' (' + fmtMoney(o.total_amount) + ') will count as cash the rider is holding again.')) {
      return
    }
    setBusyId(o.id)
    try {
      await reviewUPICollection(o.id, action, utr)
      await load(tab)
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update.')
    } finally {
      setBusyId(null)
    }
  }

  const total = rows.reduce((s, o) => s + Number(o.total_amount ?? 0), 0)

  return (
    <Layout>
      <div className="p-8 space-y-4">
        <div>
          <h1 className="text-xl font-semibold">UPI Collections</h1>
          <p className="text-sm text-slate-400 mt-1">
            COD orders paid by UPI QR at the door. If a payment never reached the bank, mark it Not received.
          </p>
        </div>

        <div className="flex gap-2">
          {TABS.map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-3 py-1.5 rounded-md text-sm capitalize ${
                tab === t ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
              }`}
            >
              {t === 'verified' ? 'Paid by UPI' : 'Not received'}
            </button>
          ))}
        </div>

        {error && <div className="text-sm text-red-400">{error}</div>}

        <div className="text-sm text-slate-400">
          {rows.length} order{rows.length === 1 ? '' : 's'} - {fmtMoney(total)}
        </div>

        <div className="overflow-x-auto rounded-lg border border-slate-800">
          <table className="w-full text-sm text-left">
            <thead className="bg-slate-900 text-slate-400">
              <tr>
                <th className="px-4 py-3">Order</th>
                <th className="px-4 py-3">Rider</th>
                <th className="px-4 py-3">Amount</th>
                <th className="px-4 py-3">Delivered</th>
                <th className="px-4 py-3">Action</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <tr><td className="px-4 py-6 text-slate-400" colSpan={5}>Loading...</td></tr>
              ) : rows.length === 0 ? (
                <tr><td className="px-4 py-6 text-slate-400" colSpan={5}>Nothing here.</td></tr>
              ) : (
                rows.map((o) => (
                  <tr key={o.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">#{o.id}</td>
                    <td className="px-4 py-3">{o.delivery_partner?.name ?? (o.delivery_partner_id ? 'Rider #' + o.delivery_partner_id : '-')}</td>
                    <td className="px-4 py-3">{fmtMoney(o.total_amount)}</td>
                    <td className="px-4 py-3">{fmtDate(o.delivered_at)}</td>
                    <td className="px-4 py-3">
                      {o.upi_status !== 'rejected' ? (
                        <div className="flex gap-2">
                          <button
                            disabled={busyId === o.id}
                            onClick={() => review(o, 'reject')}
                            className="px-2 py-1 rounded-md text-xs bg-red-600 hover:bg-red-500 text-white disabled:opacity-50"
                          >
                            Not received
                          </button>
                        </div>
                      ) : (
                        <span className="text-xs text-slate-400">{o.upi_status} {fmtDate(o.upi_verified_at)}</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </Layout>
  )
}