import { useEffect, useState } from 'react'
import { listRiderPayouts, createRiderPayout, approveRiderPayout, payRiderPayout } from '../api/finance'
import { listDeliveryPartners } from '../api/admin'
import type { RiderPayout } from '../types/finance'
import type { DeliveryPartner } from '../types/admin'

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

const emptyForm = { delivery_partner_id: 0, period_from: daysAgoISO(7), period_to: todayISO() }

function statusBadgeClass(status: string) {
  if (status === 'paid') return 'bg-emerald-600/15 text-emerald-400'
  if (status === 'approved') return 'bg-sky-600/15 text-sky-400'
  return 'bg-amber-600/15 text-amber-400'
}

export default function RiderPayouts() {
  const [payouts, setPayouts] = useState<RiderPayout[]>([])
  const [partners, setPartners] = useState<DeliveryPartner[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState(emptyForm)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const [actioningId, setActioningId] = useState<number | null>(null)

  function load() {
    setIsLoading(true)
    setError(null)
    listRiderPayouts({ status: statusFilter || undefined })
      .then((res) => setPayouts(res.rider_payouts ?? []))
      .catch((err) => setError(err.response?.data?.error ?? 'Could not load rider payouts.'))
      .finally(() => setIsLoading(false))
  }

  useEffect(() => {
    load()
  }, [statusFilter])

  useEffect(() => {
    listDeliveryPartners()
      .then((res: any) => setPartners(res.delivery_partners ?? res ?? []))
      .catch(() => {})
  }, [])

  function partnerName(id: number) {
    return partners.find((p) => p.id === id)?.name ?? `#${id}`
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!form.delivery_partner_id) {
      setFormError('Select a delivery partner.')
      return
    }
    if (form.period_from > form.period_to) {
      setFormError('Period start must be before period end.')
      return
    }
    setIsSaving(true)
    setFormError(null)
    try {
      await createRiderPayout(form)
      setForm(emptyForm)
      setShowForm(false)
      load()
    } catch (err: any) {
      setFormError(err.response?.data?.error ?? 'Could not create payout.')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleApprove(id: number) {
    setActioningId(id)
    try {
      await approveRiderPayout(id)
      load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Could not approve payout.')
    } finally {
      setActioningId(null)
    }
  }

  async function handlePay(id: number) {
    if (!confirm('Mark this payout as paid? This should only be done once the bank transfer is complete.')) return
    setActioningId(id)
    try {
      await payRiderPayout(id)
      load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Could not mark payout as paid.')
    } finally {
      setActioningId(null)
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold">Rider Payouts</h1>
          <p className="text-sm text-slate-500">
            Delivery partner earnings by settlement period — computed as delivered orders × the standard per-delivery rate.
          </p>
        </div>
        <button
          onClick={() => setShowForm((s) => !s)}
          className="text-sm bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-2 rounded-lg transition-colors"
        >
          {showForm ? 'Cancel' : '+ New Payout'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="border border-slate-800 rounded-xl p-5 mb-6 max-w-2xl">
          <div className="grid grid-cols-3 gap-4 mb-4">
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">Delivery Partner *</span>
              <select
                value={form.delivery_partner_id}
                onChange={(e) => setForm({ ...form, delivery_partner_id: Number(e.target.value) })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              >
                <option value={0}>Select...</option>
                {partners.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({p.phone})
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">Period From</span>
              <input
                type="date"
                value={form.period_from}
                max={form.period_to}
                onChange={(e) => setForm({ ...form, period_from: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </label>
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">Period To</span>
              <input
                type="date"
                value={form.period_to}
                min={form.period_from}
                max={todayISO()}
                onChange={(e) => setForm({ ...form, period_to: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </label>
          </div>
          {formError && <p className="text-sm text-red-400 mb-3">{formError}</p>}
          <button
            type="submit"
            disabled={isSaving}
            className="text-sm bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white px-4 py-2 rounded-lg transition-colors"
          >
            {isSaving ? 'Computing...' : 'Compute Payout'}
          </button>
        </form>
      )}

      <div className="flex items-center gap-2 mb-4 text-sm">
        <span className="text-slate-500">Status</span>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5"
        >
          <option value="">All</option>
          <option value="pending">Pending</option>
          <option value="approved">Approved</option>
          <option value="paid">Paid</option>
        </select>
      </div>

      {isLoading && <p className="text-sm text-slate-500">Loading payouts...</p>}
      {!isLoading && error && (
        <div className="border border-slate-800 rounded-xl p-6 text-sm text-slate-500">{error}</div>
      )}

      {!isLoading && !error && (
        <div className="border border-slate-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-slate-900 text-slate-400 text-left">
                <th className="px-4 py-2 font-medium">Partner</th>
                <th className="px-4 py-2 font-medium">Period</th>
                <th className="px-4 py-2 font-medium text-right">Deliveries</th>
                <th className="px-4 py-2 font-medium text-right">Amount</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {payouts.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-slate-500">
                    No rider payouts yet.
                  </td>
                </tr>
              )}
              {payouts.map((p) => (
                <tr key={p.id} className="border-t border-slate-800">
                  <td className="px-4 py-2 font-medium">{partnerName(p.delivery_partner_id)}</td>
                  <td className="px-4 py-2 text-slate-400">
                    {p.period_from.slice(0, 10)} → {p.period_to.slice(0, 10)}
                  </td>
                  <td className="px-4 py-2 text-right">{p.delivered_count}</td>
                  <td className="px-4 py-2 text-right font-medium">{formatCurrency(p.amount)}</td>
                  <td className="px-4 py-2">
                    <span className={`text-xs px-2 py-0.5 rounded-full capitalize ${statusBadgeClass(p.status)}`}>
                      {p.status}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-right space-x-3">
                    {p.status === 'pending' && (
                      <button
                        onClick={() => handleApprove(p.id)}
                        disabled={actioningId === p.id}
                        className="text-xs text-sky-400 hover:text-sky-300 disabled:opacity-50 transition-colors"
                      >
                        Approve
                      </button>
                    )}
                    {p.status === 'approved' && (
                      <button
                        onClick={() => handlePay(p.id)}
                        disabled={actioningId === p.id}
                        className="text-xs text-emerald-400 hover:text-emerald-300 disabled:opacity-50 transition-colors"
                      >
                        Mark Paid
                      </button>
                    )}
                    {p.status === 'paid' && p.paid_at && (
                      <span className="text-xs text-slate-500">Paid {p.paid_at.slice(0, 10)}</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
