import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import Modal from '../components/Modal'
import {
  listWarehouseStaff,
  createWarehouseStaff,
  updateWarehouseStaff,
  deleteWarehouseStaff,
  listWarehouses,
} from '../api/admin'
import type { WarehouseStaff, Warehouse } from '../types/admin'

const ROLES = [
  { value: 'picker', label: 'Picker' },
  { value: 'packer', label: 'Packer' },
  { value: 'inventory_staff', label: 'Inventory Staff' },
  { value: 'putter', label: 'Putter' },
  { value: 'supervisor', label: 'Supervisor' },
  { value: 'warehouse_manager', label: 'Warehouse Manager' },
]

const emptyForm = {
  name: '',
  phone: '',
  warehouse_id: '',
  role: 'picker',
  status: 'active',
}

function roleLabel(role?: string) {
  return ROLES.find((r) => r.value === role)?.label ?? role ?? 'Picker'
}

export default function WarehouseStaffPage() {
  const [staff, setStaff] = useState<WarehouseStaff[]>([])
  const [warehouses, setWarehouses] = useState<Warehouse[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [editingStaff, setEditingStaff] = useState<WarehouseStaff | null>(null)

  const [form, setForm] = useState(emptyForm)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const [staffRes, warehousesRes] = await Promise.all([
        listWarehouseStaff(),
        listWarehouses(),
      ])
      setStaff(staffRes.warehouse_staff ?? staffRes.staff ?? staffRes ?? [])
      setWarehouses(warehousesRes.warehouses ?? warehousesRes ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load warehouse staff.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  function warehouseName(id: number) {
    return warehouses.find((w) => w.id === id)?.name ?? `#${id}`
  }

  function openCreate() {
    setForm(emptyForm)
    setFormError(null)
    setEditingStaff(null)
    setShowCreate(true)
  }

  function openEdit(s: WarehouseStaff) {
    setForm({
      name: s.name,
      phone: s.phone,
      warehouse_id: String(s.warehouse_id),
      role: s.role ?? 'picker',
      status: s.status ?? 'active',
    })
    setFormError(null)
    setEditingStaff(s)
    setShowCreate(true)
  }

  function closeModal() {
    setShowCreate(false)
    setEditingStaff(null)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)

    if (!form.name.trim() || !form.phone.trim() || !form.warehouse_id) {
      setFormError('Name, phone, and warehouse are required.')
      return
    }

    setIsSaving(true)
    try {
      const payload = {
        name: form.name.trim(),
        phone: form.phone.trim(),
        warehouse_id: parseInt(form.warehouse_id, 10),
        role: form.role,
        status: form.status,
      }
      if (editingStaff) {
        await updateWarehouseStaff(editingStaff.id, payload)
      } else {
        await createWarehouseStaff(payload)
      }
      closeModal()
      setForm(emptyForm)
      load()
    } catch (err: any) {
      setFormError(
        err.response?.data?.error ??
          `Failed to ${editingStaff ? 'update' : 'create'} staff member.`
      )
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!confirm('Delete this staff member? This cannot be undone.')) return
    try {
      await deleteWarehouseStaff(id)
      setStaff((prev) => prev.filter((s) => s.id !== id))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to delete staff member.')
    }
  }

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Warehouse Staff</h1>
            <p className="text-sm text-slate-400 mt-1">
              {staff.length} staff member{staff.length !== 1 ? 's' : ''}
            </p>
          </div>
          <button
            onClick={openCreate}
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            + Add staff
          </button>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && staff.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No warehouse staff yet. Add your first one to get started.
          </div>
        )}

        {!isLoading && staff.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Name</th>
                  <th className="px-4 py-3 font-medium">Phone</th>
                  <th className="px-4 py-3 font-medium">Warehouse</th>
                  <th className="px-4 py-3 font-medium">Role</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {staff.map((s) => (
                  <tr key={s.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">{s.name}</td>
                    <td className="px-4 py-3">{s.phone}</td>
                    <td className="px-4 py-3">{warehouseName(s.warehouse_id)}</td>
                    <td className="px-4 py-3">
                      <span className="text-xs px-2 py-1 rounded-full bg-indigo-500/15 text-indigo-300">
                        {roleLabel(s.role)}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`text-xs px-2 py-1 rounded-full ${
                          s.status === 'active'
                            ? 'bg-emerald-500/15 text-emerald-400'
                            : 'bg-slate-700 text-slate-300'
                        }`}
                      >
                        {s.status ?? 'active'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right space-x-3">
                      <button
                        onClick={() => openEdit(s)}
                        className="text-indigo-400 hover:text-indigo-300 text-xs"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(s.id)}
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
          title={editingStaff ? 'Edit staff member' : 'Add staff member'}
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
              <label className="text-xs text-slate-400 block mb-1">Phone</label>
              <input
                value={form.phone}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Warehouse</label>
              <select
                value={form.warehouse_id}
                onChange={(e) => setForm({ ...form, warehouse_id: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              >
                <option value="">Select a warehouse</option>
                {warehouses.map((w) => (
                  <option key={w.id} value={w.id}>
                    {w.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Role</label>
              <select
                value={form.role}
                onChange={(e) => setForm({ ...form, role: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              >
                {ROLES.map((r) => (
                  <option key={r.value} value={r.value}>
                    {r.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Status</label>
              <select
                value={form.status}
                onChange={(e) => setForm({ ...form, status: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              >
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
              </select>
            </div>

            {formError && <p className="text-red-400 text-xs">{formError}</p>}

            <button
              type="submit"
              disabled={isSaving}
              className="w-full py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors mt-2"
            >
              {isSaving
                ? 'Saving...'
                : editingStaff
                ? 'Save changes'
                : 'Add staff'}
            </button>
          </form>
        </Modal>
      )}
    </Layout>
  )
}
