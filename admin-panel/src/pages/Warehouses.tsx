import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import Modal from '../components/Modal'
import ServiceAreaModal from '../components/ServiceAreaModal'
import {
  listWarehouses,
  createWarehouse,
  updateWarehouse,
  deleteWarehouse,
} from '../api/admin'
import type { Warehouse } from '../types/admin'

const emptyForm = {
  name: '',
  city: '',
  address: '',
  lat: '',
  lng: '',
  service_radius_km: '5',
  is_active: true,
  warehouse_type: 'dark_store' as 'dark_store' | 'mother_warehouse',
  status: 'open' as 'open' | 'closed' | 'paused',
  capacity: '',
  opening_time: '',
  closing_time: '',
}

const STATUS_STYLES: Record<string, string> = {
  open: 'bg-emerald-500/15 text-emerald-400',
  closed: 'bg-red-500/15 text-red-400',
  paused: 'bg-amber-500/15 text-amber-400',
}

export default function Warehouses() {
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [editingWarehouse, setEditingWarehouse] = useState<Warehouse | null>(null)
  const [serviceAreaWarehouse, setServiceAreaWarehouse] = useState<Warehouse | null>(null)

  const [form, setForm] = useState(emptyForm)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listWarehouses()
      setWarehouses(res.warehouses ?? res ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load warehouses.')
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
    setEditingWarehouse(null)
    setShowCreate(true)
  }

  function openEdit(w: Warehouse) {
    setForm({
      name: w.name,
      city: w.city,
      address: w.address ?? '',
      lat: String(w.lat),
      lng: String(w.lng),
      service_radius_km: String(w.service_radius_km ?? 5),
      is_active: w.is_active ?? true,
      warehouse_type: (w.warehouse_type as 'dark_store' | 'mother_warehouse') ?? 'dark_store',
      status: (w.status as 'open' | 'closed' | 'paused') ?? 'open',
      capacity: w.capacity != null ? String(w.capacity) : '',
      opening_time: w.opening_time ?? '',
      closing_time: w.closing_time ?? '',
    })
    setFormError(null)
    setEditingWarehouse(w)
    setShowCreate(true)
  }

  function closeModal() {
    setShowCreate(false)
    setEditingWarehouse(null)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)

    if (!form.name.trim() || !form.city.trim() || !form.lat || !form.lng) {
      setFormError('Name, city, latitude, and longitude are required.')
      return
    }

    const lat = parseFloat(form.lat)
    const lng = parseFloat(form.lng)
    if (Number.isNaN(lat) || Number.isNaN(lng)) {
      setFormError('Latitude and longitude must be valid numbers.')
      return
    }

    setIsSaving(true)
    try {
      const payload = {
        name: form.name.trim(),
        city: form.city.trim(),
        address: form.address.trim(),
        lat,
        lng,
        service_radius_km: form.service_radius_km ? parseFloat(form.service_radius_km) : 5,
        is_active: form.is_active,
        warehouse_type: form.warehouse_type,
        status: form.status,
        capacity: form.capacity ? parseInt(form.capacity, 10) : 0,
        opening_time: form.opening_time || null,
        closing_time: form.closing_time || null,
      }
      if (editingWarehouse) {
        await updateWarehouse(editingWarehouse.id, payload)
      } else {
        await createWarehouse(payload)
      }
      closeModal()
      setForm(emptyForm)
      load()
    } catch (err: any) {
      setFormError(
        err.response?.data?.error ??
          `Failed to ${editingWarehouse ? 'update' : 'create'} warehouse.`
      )
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!confirm('Delete this warehouse? This cannot be undone.')) return
    try {
      await deleteWarehouse(id)
      setWarehouses((prev) => prev.filter((w) => w.id !== id))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to delete warehouse.')
    }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Stores / Dark Stores</h1>
            <p className="text-sm text-slate-400 mt-1">
              {warehouses.length} store{warehouses.length !== 1 ? 's' : ''}
            </p>
          </div>
          <button
            onClick={openCreate}
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            + Add store
          </button>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && warehouses.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No stores yet. Add your first one to get started.
          </div>
        )}

        {!isLoading && warehouses.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Name</th>
                  <th className="px-4 py-3 font-medium">Type</th>
                  <th className="px-4 py-3 font-medium">City</th>
                  <th className="px-4 py-3 font-medium">Capacity</th>
                  <th className="px-4 py-3 font-medium">Hours</th>
                  <th className="px-4 py-3 font-medium">Radius (km)</th>
                  <th className="px-4 py-3 font-medium">Service Area</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {warehouses.map((w) => (
                  <tr key={w.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">{w.name}</td>
                    <td className="px-4 py-3">
                      <span className="text-xs px-2 py-1 rounded-full bg-slate-700 text-slate-300">
                        {w.warehouse_type === 'mother_warehouse' ? 'Mother WH' : 'Dark Store'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-400">{w.city}</td>
                    <td className="px-4 py-3">{w.capacity ?? 0}</td>
                    <td className="px-4 py-3 text-slate-400">
                      {w.opening_time && w.closing_time
                        ? `${w.opening_time.slice(0, 5)} - ${w.closing_time.slice(0, 5)}`
                        : '24x7'}
                    </td>
                    <td className="px-4 py-3">{w.service_radius_km ?? 5}</td>
                    <td className="px-4 py-3">
                      <button
                        onClick={() => setServiceAreaWarehouse(w)}
                        className={`text-xs px-2 py-1 rounded-full transition-colors ${
                          w.service_area
                            ? 'bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25'
                            : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
                        }`}
                      >
                        {w.service_area ? 'Set \u2713 (edit)' : 'Set polygon'}
                      </button>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`text-xs px-2 py-1 rounded-full ${
                          STATUS_STYLES[w.status ?? 'open'] ?? STATUS_STYLES.open
                        }`}
                      >
                        {w.status ?? 'open'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right space-x-3">
                      <button
                        onClick={() => openEdit(w)}
                        className="text-indigo-400 hover:text-indigo-300 text-xs"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(w.id)}
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
          title={editingWarehouse ? 'Edit store' : 'Add store'}
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
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Store Type</label>
                <select
                  value={form.warehouse_type}
                  onChange={(e) =>
                    setForm({ ...form, warehouse_type: e.target.value as 'dark_store' | 'mother_warehouse' })
                  }
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="dark_store">Dark Store</option>
                  <option value="mother_warehouse">Mother Warehouse</option>
                </select>
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Status</label>
                <select
                  value={form.status}
                  onChange={(e) => setForm({ ...form, status: e.target.value as 'open' | 'closed' | 'paused' })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="open">Open</option>
                  <option value="paused">Paused</option>
                  <option value="closed">Closed</option>
                </select>
              </div>
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">City</label>
              <input
                value={form.city}
                onChange={(e) => setForm({ ...form, city: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Address</label>
              <input
                value={form.address}
                onChange={(e) => setForm({ ...form, address: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Latitude</label>
                <input
                  type="number"
                  step="any"
                  value={form.lat}
                  onChange={(e) => setForm({ ...form, lat: e.target.value })}
                  placeholder="e.g. 23.0225"
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Longitude</label>
                <input
                  type="number"
                  step="any"
                  value={form.lng}
                  onChange={(e) => setForm({ ...form, lng: e.target.value })}
                  placeholder="e.g. 72.5714"
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Service Radius (km)</label>
                <input
                  type="number"
                  step="any"
                  value={form.service_radius_km}
                  onChange={(e) => setForm({ ...form, service_radius_km: e.target.value })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Capacity (orders)</label>
                <input
                  type="number"
                  value={form.capacity}
                  onChange={(e) => setForm({ ...form, capacity: e.target.value })}
                  placeholder="e.g. 200"
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Opening Time</label>
                <input
                  type="time"
                  value={form.opening_time}
                  onChange={(e) => setForm({ ...form, opening_time: e.target.value })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Closing Time</label>
                <input
                  type="time"
                  value={form.closing_time}
                  onChange={(e) => setForm({ ...form, closing_time: e.target.value })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
            </div>
            <div className="flex items-center gap-2">
              <input
                id="w_is_active"
                type="checkbox"
                checked={form.is_active}
                onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
                className="rounded border-slate-700 bg-slate-800"
              />
              <label htmlFor="w_is_active" className="text-sm text-slate-300">
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
                : editingWarehouse
                ? 'Save changes'
                : 'Add store'}
            </button>
          </form>
        </Modal>
      )}

      {serviceAreaWarehouse && (
        <ServiceAreaModal
          warehouse={serviceAreaWarehouse}
          onClose={() => setServiceAreaWarehouse(null)}
          onSaved={load}
        />
      )}
    </Layout>
  )
}
