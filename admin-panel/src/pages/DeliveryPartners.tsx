import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import Modal from '../components/Modal'
import {
  listDeliveryPartners,
  createDeliveryPartner,
  updateDeliveryPartner,
  deleteDeliveryPartner,
  getPartnerOnboarding,
  getRiderWorkload,
  recordAndVerifyCODDeposit,
} from '../api/admin'
import type { DeliveryPartner } from '../types/admin'

function approvalClass(s: string) {
  if (s === 'approved') return 'bg-emerald-500/15 text-emerald-400'
  if (s === 'pending') return 'bg-amber-500/15 text-amber-400'
  if (s === 'rejected') return 'bg-red-500/15 text-red-400'
  return 'bg-slate-700 text-slate-300'
}

const emptyForm = {
  name: '',
  phone: '',
  vehicle_number: '',
  is_active: true,
}

export default function DeliveryPartners() {
  const [partners, setPartners] = useState<DeliveryPartner[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [editingPartner, setEditingPartner] = useState<DeliveryPartner | null>(null)
  const [viewing, setViewing] = useState<DeliveryPartner | null>(null)
  const [details, setDetails] = useState<any>(null)
  const [detailsLoading, setDetailsLoading] = useState(false)
  const [detailsError, setDetailsError] = useState<string | null>(null)

  const [form, setForm] = useState(emptyForm)
  const [cash, setCash] = useState<Record<number, { pending: number; limit: number; reached: boolean }>>({})
  const [depositFor, setDepositFor] = useState<DeliveryPartner | null>(null)
  const [depositAmount, setDepositAmount] = useState('')
  const [depositError, setDepositError] = useState<string | null>(null)
  const [depositSaving, setDepositSaving] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listDeliveryPartners()
      setPartners(res.delivery_partners ?? res.partners ?? res ?? [])
      try {
        const w = await getRiderWorkload()
        const m: Record<number, { pending: number; limit: number; reached: boolean }> = {}
        for (const r of w.riders ?? []) {
          m[r.partner_id] = { pending: r.pending_cod ?? 0, limit: r.cod_limit ?? 2500, reached: !!r.cod_limit_reached }
        }
        setCash(m)
      } catch {
        // cash column is optional; ignore failures
      }
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load delivery partners.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function openView(p: DeliveryPartner) {
    setViewing(p)
    setDetails(null)
    setDetailsError(null)
    setDetailsLoading(true)
    try {
      setDetails(await getPartnerOnboarding(p.id))
    } catch (err: any) {
      setDetailsError(err.response?.data?.error ?? 'Failed to load details.')
    } finally {
      setDetailsLoading(false)
    }
  }

  function openCreate() {
    setForm(emptyForm)
    setFormError(null)
    setEditingPartner(null)
    setShowCreate(true)
  }

  function openEdit(p: DeliveryPartner) {
    setForm({
      name: p.name,
      phone: p.phone,
      vehicle_number: p.vehicle_number ?? '',
      is_active: p.is_active ?? true,
    })
    setFormError(null)
    setEditingPartner(p)
    setShowCreate(true)
  }

  function closeModal() {
    setShowCreate(false)
    setEditingPartner(null)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)

    const phoneDigits = form.phone.trim()

    if (!form.name.trim() || !phoneDigits) {
      setFormError('Name and phone are required.')
      return
    }
    if (!/^\d{10}$/.test(phoneDigits)) {
      setFormError('Phone must be exactly 10 digits, numbers only.')
      return
    }

    setIsSaving(true)
    try {
      const payload = {
        name: form.name.trim(),
        phone: phoneDigits,
        vehicle_number: form.vehicle_number.trim(),
        is_active: form.is_active,
      }
      if (editingPartner) {
        await updateDeliveryPartner(editingPartner.id, payload)
      } else {
        await createDeliveryPartner(payload)
      }
      closeModal()
      setForm(emptyForm)
      load()
    } catch (err: any) {
      setFormError(
        err.response?.data?.error ??
          `Failed to ${editingPartner ? 'update' : 'create'} delivery partner.`
      )
    } finally {
      setIsSaving(false)
    }
  }

  function openDeposit(p: DeliveryPartner) {
    setDepositFor(p)
    setDepositAmount(String(Math.round((cash[p.id]?.pending ?? 0) * 100) / 100))
    setDepositError(null)
  }

  async function handleDeposit(e: React.FormEvent) {
    e.preventDefault()
    if (!depositFor) return
    const amt = parseFloat(depositAmount)
    if (!(amt > 0)) {
      setDepositError('Enter a valid amount.')
      return
    }
    setDepositSaving(true)
    setDepositError(null)
    try {
      await recordAndVerifyCODDeposit(depositFor.id, amt, 'Recorded from admin panel')
      setDepositFor(null)
      load()
    } catch (err: any) {
      setDepositError(err.response?.data?.error ?? 'Failed to record deposit.')
    } finally {
      setDepositSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!confirm('Delete this delivery partner? This cannot be undone.')) return
    try {
      await deleteDeliveryPartner(id)
      setPartners((prev) => prev.filter((p) => p.id !== id))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to delete delivery partner.')
    }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Delivery Partners</h1>
            <p className="text-sm text-slate-400 mt-1">
              {partners.length} partner{partners.length !== 1 ? 's' : ''}
            </p>
          </div>
          <button
            onClick={openCreate}
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            + Add partner
          </button>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && partners.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No delivery partners yet. Add your first one to get started.
          </div>
        )}

        {!isLoading && partners.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Name</th>
                  <th className="px-4 py-3 font-medium">Phone</th>
                  <th className="px-4 py-3 font-medium">Vehicle No.</th>
                  <th className="px-4 py-3 font-medium">Rating</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Approval</th>
                  <th className="px-4 py-3 font-medium">Cash in hand</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {partners.map((p) => (
                  <tr key={p.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        {p.profile_photo_url ? (
                          <img
                            src={p.profile_photo_url}
                            alt=""
                            className="w-8 h-8 rounded-full object-cover bg-slate-800"
                          />
                        ) : (
                          <div className="w-8 h-8 rounded-full bg-slate-800 flex items-center justify-center text-xs text-slate-400">
                            {(p.name || '?').charAt(0).toUpperCase()}
                          </div>
                        )}
                        <span>{p.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">{p.phone}</td>
                    <td className="px-4 py-3 text-slate-400">{p.vehicle_number || '-'}</td>
                    <td className="px-4 py-3">{p.rating_count ? "★ " + (p.avg_rating ?? 0).toFixed(1) + " (" + p.rating_count + ")" : "-"}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`text-xs px-2 py-1 rounded-full ${
                          p.is_active
                            ? 'bg-emerald-500/15 text-emerald-400'
                            : 'bg-slate-700 text-slate-300'
                        }`}
                      >
                        {p.is_active ? 'active' : 'inactive'}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`text-xs px-2 py-1 rounded-full ${approvalClass(
                          (p as any).approval_status ?? 'approved'
                        )}`}
                      >
                        {(p as any).approval_status ?? 'approved'}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      {cash[p.id] ? (
                        <div className="flex items-center gap-2">
                          <span
                            className={`text-xs px-2 py-1 rounded-full ${
                              cash[p.id].reached
                                ? 'bg-red-500/15 text-red-300'
                                : cash[p.id].pending >= cash[p.id].limit * 0.8
                                ? 'bg-amber-500/15 text-amber-300'
                                : 'bg-slate-800 text-slate-400'
                            }`}
                          >
                            {'\u20B9'}{Math.round(cash[p.id].pending)} / {'\u20B9'}{Math.round(cash[p.id].limit)}
                            {cash[p.id].reached ? ' - COD blocked' : ''}
                          </span>
                          {cash[p.id].pending > 0 && (
                            <button
                              onClick={() => openDeposit(p)}
                              className="text-emerald-400 hover:text-emerald-300 text-xs"
                            >
                              Record deposit
                            </button>
                          )}
                        </div>
                      ) : (
                        '-'
                      )}
                    </td>
                    <td className="px-4 py-3 text-right space-x-3">
                      <button
                        onClick={() => openView(p)}
                        className="text-slate-300 hover:text-white text-xs"
                      >
                        View
                      </button>
                      <button
                        onClick={() => openEdit(p)}
                        className="text-indigo-400 hover:text-indigo-300 text-xs"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(p.id)}
                        className="text-red-400 hover:text-red-300 text-xs"
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showCreate && (
        <Modal
          title={editingPartner ? 'Edit delivery partner' : 'Add delivery partner'}
          onClose={closeModal}
        >
          <form onSubmit={handleSubmit} className="space-y-3">
            <div>
              <label className="text-xs text-slate-400 block mb-1">Name</label>
              <input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Phone (10 digits)</label>
              <input
                value={form.phone}
                onChange={(e) =>
                  setForm({ ...form, phone: e.target.value.replace(/\D/g, '').slice(0, 10) })
                }
                maxLength={10}
                inputMode="numeric"
                placeholder="9876543210"
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Vehicle Number</label>
              <input
                value={form.vehicle_number}
                onChange={(e) => setForm({ ...form, vehicle_number: e.target.value })}
                placeholder="e.g. GJ01AB1234"
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>
            <div className="flex items-center gap-2">
              <input
                id="is_active"
                type="checkbox"
                checked={form.is_active}
                onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
                className="rounded border-slate-700 bg-slate-800"
              />
              <label htmlFor="is_active" className="text-sm text-slate-300">
                Active
              </label>
            </div>

            {formError && <p className="text-red-400 text-xs">{formError}</p>}

            <button
              type="submit"
              disabled={isSaving}
              className="w-full py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors mt-2"
            >
              {isSaving
                ? 'Saving...'
                : editingPartner
                ? 'Save changes'
                : 'Add partner'}
            </button>
          </form>
        </Modal>
      )}
      {depositFor && (
        <Modal title={`Record cash deposit: ${depositFor.name}`} onClose={() => setDepositFor(null)}>
          <form onSubmit={handleDeposit} className="space-y-3">
            <p className="text-xs text-slate-400">
              Pending cash: {'\u20B9'}{(cash[depositFor.id]?.pending ?? 0).toFixed(2)}. This records the deposit and verifies it immediately.
            </p>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Amount received</label>
              <input
                value={depositAmount}
                onChange={(e) => setDepositAmount(e.target.value)}
                inputMode="decimal"
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>
            {depositError && <p className="text-red-400 text-xs">{depositError}</p>}
            <button
              type="submit"
              disabled={depositSaving}
              className="w-full py-2 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-white text-sm font-medium transition-colors mt-2"
            >
              {depositSaving ? 'Saving...' : 'Record and verify deposit'}
            </button>
          </form>
        </Modal>
      )}
      {viewing && (
        <Modal title={`Partner details: ${viewing.name}`} onClose={() => setViewing(null)}>
          {detailsLoading && <p className="text-slate-400 text-sm">Loading...</p>}
          {detailsError && <p className="text-red-400 text-sm">{detailsError}</p>}
          {details && (
            <div className="space-y-4 text-sm">
              <div className="flex items-center gap-3">
                {details.profile_photo_url ? (
                  <a href={details.profile_photo_url} target="_blank" rel="noreferrer">
                    <img
                      src={details.profile_photo_url}
                      alt="Profile photo"
                      className="w-16 h-16 rounded-full object-cover border border-slate-700"
                    />
                  </a>
                ) : (
                  <div className="w-16 h-16 rounded-full bg-slate-800 flex items-center justify-center text-xl text-slate-400">
                    {(viewing.name || '?').charAt(0).toUpperCase()}
                  </div>
                )}
                <div>
                  <p className="font-medium">{viewing.name}</p>
                  <p className="text-xs text-slate-400">
                    {details.profile_photo_url ? 'Profile photo' : 'Profile photo not uploaded'}
                  </p>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                {[
                  ['Phone', details.phone],
                  ['Approval', details.approval_status],
                  ['Vehicle type', details.vehicle_type],
                  ['Vehicle number', details.vehicle_number],
                  ['Store ID', details.warehouse_id],
                  ['ID document', details.id_doc_type],
                  ['UPI ID', details.upi_id],
                  ['Account holder', details.bank_account_holder],
                  ['Account number', details.bank_account_no],
                  ['IFSC', details.bank_ifsc],
                ].map(([label, value]) => (
                  <div key={label}>
                    <p className="text-xs text-slate-400">{label}</p>
                    <p>{value || '-'}</p>
                  </div>
                ))}
              </div>
              <div className="grid grid-cols-2 gap-3">
                {[
                  ['Aadhaar', 'aadhaar_url'],
                  ['Driving licence', 'licence_url'],
                  ['Voter ID', 'voter_url'],
                  ['PAN card', 'pan_url'],
                  ['KYC selfie', 'selfie_url'],
                  ['RC book', 'rc_url'],
                ]
                  .filter(([, key]) => details[key])
                  .map(([label, key]) => (
                    <div key={key}>
                      <p className="text-xs text-slate-400 mb-1">{label}</p>
                      <a href={details[key]} target="_blank" rel="noreferrer">
                        <img
                          src={details[key]}
                          alt={label}
                          className="h-32 w-full rounded-lg border border-slate-700 object-cover"
                        />
                      </a>
                    </div>
                  ))}
              </div>
            </div>
          )}
        </Modal>
      )}
    </Layout>
  )
}
