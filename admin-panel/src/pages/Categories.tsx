import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import Modal from '../components/Modal'
import {
  listCategories,
  createCategory,
  updateCategory,
  deleteCategory,
  listSubcategories,
  createSubcategory,
  updateSubcategory,
  deleteSubcategory,
} from '../api/admin'
import type { Category, Subcategory } from '../types/admin'

export default function Categories() {
  const [categories, setCategories] = useState<Category[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  const [editingCategory, setEditingCategory] = useState<Category | null>(null)
  const [editName, setEditName] = useState('')
  const [editError, setEditError] = useState<string | null>(null)
  const [isEditSaving, setIsEditSaving] = useState(false)

  const [selectedCategoryId, setSelectedCategoryId] = useState<string>('')
  const [subcategories, setSubcategories] = useState<Subcategory[]>([])
  const [isSubLoading, setIsSubLoading] = useState(false)
  const [subError, setSubError] = useState<string | null>(null)
  const [newSubName, setNewSubName] = useState('')
  const [isSubSaving, setIsSubSaving] = useState(false)

  const [editingSub, setEditingSub] = useState<Subcategory | null>(null)
  const [editSubName, setEditSubName] = useState('')
  const [editSubError, setEditSubError] = useState<string | null>(null)
  const [isEditSubSaving, setIsEditSubSaving] = useState(false)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listCategories()
      const cats = res.categories ?? res ?? []
      setCategories(cats)
      if (!selectedCategoryId && cats.length > 0) {
        setSelectedCategoryId(String(cats[0].id))
      }
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load categories.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadSubcategories(categoryId: string) {
    if (!categoryId) {
      setSubcategories([])
      return
    }
    setIsSubLoading(true)
    setSubError(null)
    try {
      const res = await listSubcategories(categoryId)
      setSubcategories(res.subcategories ?? res ?? [])
    } catch (err: any) {
      setSubError(err.response?.data?.error ?? 'Failed to load subcategories.')
    } finally {
      setIsSubLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  useEffect(() => {
    loadSubcategories(selectedCategoryId)
  }, [selectedCategoryId])

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (!newName.trim()) return
    setIsSaving(true)
    try {
      await createCategory(newName.trim())
      setNewName('')
      load()
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to create category.')
    } finally {
      setIsSaving(false)
    }
  }

  function openEdit(c: Category) {
    setEditingCategory(c)
    setEditName(c.name)
    setEditError(null)
  }

  async function handleEditSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!editingCategory) return
    if (!editName.trim()) {
      setEditError('Category name is required.')
      return
    }
    setIsEditSaving(true)
    try {
      await updateCategory(editingCategory.id, editName.trim())
      setEditingCategory(null)
      load()
    } catch (err: any) {
      setEditError(err.response?.data?.error ?? 'Failed to update category.')
    } finally {
      setIsEditSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!confirm('Delete this category?')) return
    try {
      await deleteCategory(id)
      setCategories((prev) => prev.filter((c) => c.id !== id))
      if (selectedCategoryId === String(id)) {
        setSelectedCategoryId('')
      }
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to delete category.')
    }
  }

  async function handleAddSub(e: React.FormEvent) {
    e.preventDefault()
    if (!newSubName.trim() || !selectedCategoryId) return
    setIsSubSaving(true)
    try {
      await createSubcategory({
        name: newSubName.trim(),
        category_id: parseInt(selectedCategoryId, 10),
      })
      setNewSubName('')
      loadSubcategories(selectedCategoryId)
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to create subcategory.')
    } finally {
      setIsSubSaving(false)
    }
  }

  function openEditSub(s: Subcategory) {
    setEditingSub(s)
    setEditSubName(s.name)
    setEditSubError(null)
  }

  async function handleEditSubSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!editingSub) return
    if (!editSubName.trim()) {
      setEditSubError('Subcategory name is required.')
      return
    }
    setIsEditSubSaving(true)
    try {
      await updateSubcategory(editingSub.id, {
        name: editSubName.trim(),
        category_id: editingSub.category_id,
      })
      setEditingSub(null)
      loadSubcategories(selectedCategoryId)
    } catch (err: any) {
      setEditSubError(err.response?.data?.error ?? 'Failed to update subcategory.')
    } finally {
      setIsEditSubSaving(false)
    }
  }

  async function handleDeleteSub(id: number) {
    if (!confirm('Delete this subcategory?')) return
    try {
      await deleteSubcategory(id)
      setSubcategories((prev) => prev.filter((s) => s.id !== id))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to delete subcategory.')
    }
  }

  return (
    <Layout>
      <div className="p-8 max-w-3xl">
        <h1 className="text-xl font-semibold mb-1">Categories</h1>
        <p className="text-sm text-slate-400 mb-6">
          {categories.length} categor{categories.length !== 1 ? 'ies' : 'y'}
        </p>
        <form onSubmit={handleAdd} className="flex gap-2 mb-6">
          <input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="New category name"
            className="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          />
          <button
            type="submit"
            disabled={isSaving}
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            Add
          </button>
        </form>
        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}
        {!isLoading && categories.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No categories yet.
          </div>
        )}
        <div className="space-y-2 mb-10">
          {categories.map((c) => (
            <div
              key={c.id}
              onClick={() => setSelectedCategoryId(String(c.id))}
              className={`flex items-center justify-between border rounded-lg px-4 py-3 cursor-pointer ${
                selectedCategoryId === String(c.id)
                  ? 'border-indigo-500 bg-slate-900'
                  : 'border-slate-800'
              }`}
            >
              <span className="text-sm">{c.name}</span>
              <div className="flex items-center gap-3">
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    openEdit(c)
                  }}
                  className="text-indigo-400 hover:text-indigo-300 text-xs"
                >
                  Edit
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    handleDelete(c.id)
                  }}
                  className="text-red-400 hover:text-red-300 text-xs"
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>

        {selectedCategoryId && (
          <div className="border-t border-slate-800 pt-6">
            <h2 className="text-lg font-semibold mb-1">
              Subcategories of{' '}
              {categories.find((c) => String(c.id) === selectedCategoryId)?.name}
            </h2>
            <p className="text-sm text-slate-400 mb-4">
              {subcategories.length} subcategor{subcategories.length !== 1 ? 'ies' : 'y'}
            </p>
            <form onSubmit={handleAddSub} className="flex gap-2 mb-4">
              <input
                value={newSubName}
                onChange={(e) => setNewSubName(e.target.value)}
                placeholder="New subcategory name"
                className="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
              <button
                type="submit"
                disabled={isSubSaving}
                className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
              >
                Add
              </button>
            </form>
            {isSubLoading && <p className="text-slate-400">Loading...</p>}
            {subError && <p className="text-red-400">{subError}</p>}
            {!isSubLoading && subcategories.length === 0 && (
              <div className="border border-dashed border-slate-800 rounded-xl p-8 text-center text-slate-500">
                No subcategories yet.
              </div>
            )}
            <div className="space-y-2">
              {subcategories.map((s) => (
                <div
                  key={s.id}
                  className="flex items-center justify-between border border-slate-800 rounded-lg px-4 py-3"
                >
                  <span className="text-sm">{s.name}</span>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => openEditSub(s)}
                      className="text-indigo-400 hover:text-indigo-300 text-xs"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDeleteSub(s.id)}
                      className="text-red-400 hover:text-red-300 text-xs"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {editingCategory && (
        <Modal title="Edit category" onClose={() => setEditingCategory(null)}>
          <form onSubmit={handleEditSubmit} className="space-y-3">
            <div>
              <label className="text-xs text-slate-400 block mb-1">Category name</label>
              <input
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>

            {editError && <p className="text-red-400 text-xs">{editError}</p>}

            <button
              type="submit"
              disabled={isEditSaving}
              className="w-full py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors mt-2"
            >
              {isEditSaving ? 'Saving...' : 'Save changes'}
            </button>
          </form>
        </Modal>
      )}

      {editingSub && (
        <Modal title="Edit subcategory" onClose={() => setEditingSub(null)}>
          <form onSubmit={handleEditSubSubmit} className="space-y-3">
            <div>
              <label className="text-xs text-slate-400 block mb-1">Subcategory name</label>
              <input
                value={editSubName}
                onChange={(e) => setEditSubName(e.target.value)}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
              />
            </div>

            {editSubError && <p className="text-red-400 text-xs">{editSubError}</p>}

            <button
              type="submit"
              disabled={isEditSubSaving}
              className="w-full py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors mt-2"
            >
              {isEditSubSaving ? 'Saving...' : 'Save changes'}
            </button>
          </form>
        </Modal>
      )}
    </Layout>
  )
}