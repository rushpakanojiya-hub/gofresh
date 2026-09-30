import { useEffect, useState } from 'react'
import { getVendors, listVendorBankChangeRequests, requestVendorBankChange, approveVendorBankChange, rejectVendorBankChange } from '../api/finance'
import type { Vendor, VendorBankChangeRequest, VendorBankChangeRequestBody } from '../types/finance'

const emptyForm: VendorBankChangeRequestBody = { account_holder: '', account_number: '', ifsc: '' }

export default function VendorBankChangeRequests() {
  const [requests, setRequests] = useState<VendorBankChangeRequest[]>([])
  const [vendors, setVendors] = useState<Vendor[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [selectedVendorId, setSelectedVendorId] = useState(0)
  const [form, setForm] = useState<VendorBankChangeRequestBody>(emptyForm)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const [rejectingId, setRejectingId] = useState<number | null>(null)
  const [rejectReason, setRejectReason] = useState('')
  const [actionError, setActionError] = useState<string | null>(null)

  function load() {
    setIsLoading(true)
    setError(null)
    listVendorBankChangeRequests({ status: statusFilter || undefined })
      .then((res) => setRequests(res.bank_change_requests ?? []))
      .catch((err) => setError(err.response?.data?.error ?? 'Could not load bank change requests.'))
      .finally(() => setIsLoading(false))
  }

  useEffect(() => {
    load()
    getVendors().then((res) => setVendors(res.vendors ?? [])).catch(() => {})
  }, [statusFilter])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!selectedVendorId) {
      setFormError('Select a vendor.')
      return
    }
    if (!form.account_holder.trim() || !form.account_number.trim() || !form.ifsc.trim()) {
      setFormError('All fields are required.')
      return
    }
    setIsSaving(true)
    setFormError(null)
    try {
      await requestVendorBankChange(selectedVendorId, form)
      setForm(emptyForm)
      setSelectedVendorId(0)
      setShowForm(false)
      load()
    } catch (err: any) {
      setFormError(err.response?.data?.error ?? 'Could not create bank change request.')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleApprove(id: number) {
    setActionError(null)
    try {
      await approveVendorBankChange(id)
      load()
    } catch (err: any) {
      setActionError(err.response?.data?.error ?? 'Could not approve request.')
    }
  }

  async function handleReject(id: number) {
    if (!rejectReason.trim()) {
      setActionError('A rejection reason is required.')
      return
    }
    setActionError(null)
    try {
      await rejectVendorBankChange(id, rejectReason)
      setRejectingId(null)
      setRejectReason('')
      load()
    } catch (err: any) {
      setActionError(err.response?.data?.error ?? 'Could not reject request.')
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold">Vendor Bank Change Requests</h1>
          <p className="text-sm text-slate-500">
            Maker-checker: bank details only change once a different admin approves the request.
          </p>
        </div>
        <button
          onClick={() => setShowForm((s) => !s)}
          className="text-sm bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-2 rounded-lg transition-colors"
        >
          {showForm ? 'Cancel' : '+ New Request'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="border border-slate-800 rounded-xl p-5 mb-6 max-w-2xl">
          <div className="mb-4">
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">Vendor *</span>
              <select
                value={selectedVendorId}
                onChange={(e) => setSelectedVendorId(Number(e.target.value))}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              >
                <option value={0}>Select vendor...</option>
                {vendors.map((v) => (
                  <option key={v.id} value={v.id}>{v.name}</option>
                ))}
              </select>
            </label>
          </div>
          <div className="grid grid-cols-2 gap-4 mb-4">
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">New Account Holder *</span>
              <input
                value={form.account_holder}
                onChange={(e) => setForm({ ...form, account_holder: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </label>
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">New Account Number *</span>
              <input
                value={form.account_number}
                onChange={(e) => setForm({ ...form, account_number: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </label>
            <label className="block">
              <span className="text-xs text-slate-500 mb-1 block">New IFSC *</span>
              <input
                value={form.ifsc}
                onChange={(e) => setForm({ ...form, ifsc: e.target.value })}
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
            {isSaving ? 'Submitting...' : 'Submit Request'}
          </button>
        </form>
      )}

      <div className="flex gap-2 mb-4 text-sm">
        {['', 'pending', 'approved', 'rejected'].map((s) => (
          <button
            key={s}
            onClick={() => setStatusFilter(s)}
            className={`px-3 py-1.5 rounded-lg border transition-colors ${
              statusFilter === s
                ? 'border-emerald-600 bg-emerald-600/15 text-emerald-400'
                : 'border-slate-800 text-slate-400 hover:text-slate-200'
            }`}
          >
            {s === '' ? 'All' : s[0].toUpperCase() + s.slice(1)}
          </button>
        ))}
      </div>

      {actionError && <p className="text-sm text-red-400 mb-3">{actionError}</p>}

      {isLoading && <p className="text-sm text-slate-500">Loading requests...</p>}
      {!isLoading && error && (
        <div className="border border-slate-800 rounded-xl p-6 text-sm text-slate-500">{error}</div>
      )}

      {!isLoading && !error && (
        <div className="border border-slate-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-slate-900 text-slate-400 text-left">
                <th className="px-4 py-2 font-medium">Vendor</th>
                <th className="px-4 py-2 font-medium">New Account Holder</th>
                <th className="px-4 py-2 font-medium">New Account #</th>
                <th className="px-4 py-2 font-medium">New IFSC</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {requests.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-slate-500">
                    No bank change requests yet.
                  </td>
                </tr>
              )}
              {requests.map((r) => (
                <tr key={r.id} className="border-t border-slate-800">
                  <td className="px-4 py-2">{r.vendor?.name ?? `#${r.vendor_id}`}</td>
                  <td className="px-4 py-2">{r.new_account_holder}</td>
                  <td className="px-4 py-2 font-mono text-xs">{r.new_account_number}</td>
                  <td className="px-4 py-2 font-mono text-xs">{r.new_ifsc}</td>
                  <td className="px-4 py-2">
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full ${
                        r.status === 'approved'
                          ? 'bg-emerald-600/15 text-emerald-400'
                          : r.status === 'rejected'
                          ? 'bg-red-600/15 text-red-400'
                          : 'bg-amber-600/15 text-amber-400'
                      }`}
                    >
                      {r.status}
                    </span>
                    {r.status === 'rejected' && r.rejection_reason && (
                      <p className="text-xs text-slate-500 mt-1">{r.rejection_reason}</p>
                    )}
                  </td>
                  <td className="px-4 py-2 text-right">
                    {r.status === 'pending' && (
                      <div className="flex items-center justify-end gap-2">
                        {rejectingId === r.id ? (
                          <>
                            <input
                              value={rejectReason}
                              onChange={(e) => setRejectReason(e.target.value)}
                              placeholder="Reason"
                              className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1 text-xs w-32"
                            />
                            <button
                              onClick={() => handleReject(r.id)}
                              className="text-xs text-red-400 hover:text-red-300"
                            >
                              Confirm
                            </button>
                            <button
                              onClick={() => { setRejectingId(null); setRejectReason('') }}
                              className="text-xs text-slate-500 hover:text-slate-300"
                            >
                              Cancel
                            </button>
                          </>
                        ) : (
                          <>
                            <button
                              onClick={() => handleApprove(r.id)}
                              className="text-xs text-emerald-400 hover:text-emerald-300"
                            >
                              Approve
                            </button>
                            <button
                              onClick={() => setRejectingId(r.id)}
                              className="text-xs text-slate-500 hover:text-red-400"
                            >
                              Reject
                            </button>
                          </>
                        )}
                      </div>
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