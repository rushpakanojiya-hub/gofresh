import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import Modal from '../components/Modal'
import {
  listDeliveryPartners,
  createDeliveryPartner,
  updateDeliveryPartner,
  deleteDeliveryPartner,
} from '../api/admin'
import type { DeliveryPartner } from '../types/admin'

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

  const [form, setForm] = useState(emptyForm)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listDeliveryPartners()
      setPartners(res.delivery_partners ?? res.partners ?? res ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load delivery partners.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

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
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {partners.map((p) => (
                  <tr key={p.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">{p.name}</td>
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
                    <td className="px-4 py-3 text-right space-x-3">
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
    </Layout>
  )
}
