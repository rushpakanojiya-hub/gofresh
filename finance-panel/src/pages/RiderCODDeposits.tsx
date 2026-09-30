import { useEffect, useState } from 'react'
import { listRiderCODDeposits, createRiderCODDeposit, verifyRiderCODDeposit } from '../api/finance'
import { listDeliveryPartners } from '../api/admin'
import type { RiderCODDeposit } from '../types/finance'
import type { DeliveryPartner } from '../types/admin'

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(value)
}

function todayISO() {
  return new Date().toISOString().slice(0, 10)
}

const emptyForm = { delivery_partner_id: 0, amount: 0, deposit_date: todayISO(), note: '' }

function statusBadgeClass(status: string) {
  return status === 'verified' ? 'bg-emerald-600/15 text-emerald-400' : 'bg-amber-600/15 text-amber-400'
}

export default function RiderCODDeposits() {
  const [deposits, setDeposits] = useState<RiderCODDeposit[]>([])
  const [partners, setPartners] = useState<DeliveryPartner[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState(emptyForm)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const [verifyingId, setVerifyingId] = useState<number | null>(null)

  function load() {
    setIsLoading(true)
    setError(null)
    listRiderCODDeposits({ status: statusFilter || undefined })
      .then((res) => setDeposits(res.rider_cod_deposits ?? []))
      .catch((err) => setError(err.response?.data?.error ?? 'Could not load COD deposits.'))
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
    if (!form.amount || form.amount <= 0) {
      setFormError('Enter a deposit amount greater than zero.')
      return
    }
    setIsSaving(true)
    setFormError(null)
    try {
      await createRiderCODDeposit(form)
      setForm(emptyForm)
      setShowForm(false)
      load()
    } catch (err: any) {
      setFormError(err.response?.data?.error ?? 'Could not record deposit.')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleVerify(id: number) {
    if (!confirm('Confirm this cash deposit has been received in the bank?')) return
    setVerifyingId(id)
    try {
      await verifyRiderCODDeposit(id)
      load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Could not verify deposit.')
    } finally {
      setVerifyingId(null)
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold">Rider COD Deposits</h1>
          <p className="text-sm text-slate-500">
            Cash a delivery partner has deposited into the company bank from COD collections. Verifying moves the amount from Cash to Bank.
          </p>
        </div>
        <button
          onClick={() => setShowForm((s) => !s)}
          className="text-sm bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-2 rounded-lg transition-colors"
        >
          {showForm ? 'Cancel' : '+ Record Deposit'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="border border-slate-800 rounded-xl p-5 mb-6 max-w-2xl">
          <div className="grid grid-cols-2 gap-4 mb-4">
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
              <span className="text-xs text-slate-500 mb-1 block">Amount *</span>
              <input
                type="number"
                value={form.amount || ''}
                onChange={(e) => setForm({ ...form, amount: Number(e.target.value) })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </label>
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">Deposit Date</span>
              <input
                type="date"
                value={form.deposit_date}
                max={todayISO()}
                onChange={(e) => setForm({ ...form, deposit_date: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </label>
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">Note</span>
              <input
                value={form.note}
                onChange={(e) => setForm({ ...form, note: e.target.value })}
                placeholder="Optional"
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
            {isSaving ? 'Saving...' : 'Record Deposit'}
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
          <option value="verified">Verified</option>
        </select>
      </div>

      {isLoading && <p className="text-sm text-slate-500">Loading deposits...</p>}
      {!isLoading && error && (
        <div className="border border-slate-800 rounded-xl p-6 text-sm text-slate-500">{error}</div>
      )}

      {!isLoading && !error && (
        <div className="border border-slate-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-slate-900 text-slate-400 text-left">
                <th className="px-4 py-2 font-medium">Partner</th>
                <th className="px-4 py-2 font-medium">Deposit Date</th>
                <th className="px-4 py-2 font-medium text-right">Amount</th>
                <th className="px-4 py-2 font-medium">Note</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {deposits.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-slate-500">
                    No COD deposits yet.
                  </td>
                </tr>
              )}
              {deposits.map((d) => (
                <tr key={d.id} className="border-t border-slate-800">
                  <td className="px-4 py-2 font-medium">{partnerName(d.delivery_partner_id)}</td>
                  <td className="px-4 py-2 text-slate-400">{d.deposit_date.slice(0, 10)}</td>
                  <td className="px-4 py-2 text-right font-medium">{formatCurrency(d.amount)}</td>
                  <td className="px-4 py-2 text-slate-400">{d.note || '—'}</td>
                  <td className="px-4 py-2">
                    <span className={`text-xs px-2 py-0.5 rounded-full capitalize ${statusBadgeClass(d.status)}`}>
                      {d.status}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-right">
                    {d.status === 'pending' ? (
                      <button
                        onClick={() => handleVerify(d.id)}
                        disabled={verifyingId === d.id}
                        className="text-xs text-sky-400 hover:text-sky-300 disabled:opacity-50 transition-colors"
                      >
                        Verify
                      </button>
                    ) : (
                      d.verified_at && <span className="text-xs text-slate-500">Verified {d.verified_at.slice(0, 10)}</span>
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
