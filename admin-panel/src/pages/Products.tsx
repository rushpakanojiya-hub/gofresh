import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import Layout from '../components/Layout'
import Modal from '../components/Modal'
import {
  listProducts,
  createProduct,
  updateProduct,
  deleteProduct,
  updateInventory,
  listCategories,
  listSubcategories,
  uploadImage,
  generateProductBarcode,
  IMAGE_ORIGIN,
} from '../api/admin'
import type { Product, Category, Subcategory } from '../types/admin'

const emptyForm = {
  name: '',
  description: '',
  price: '',
  mrp: '',
  category_id: '',
  subcategory_id: '',
  stock: '',
  image_url: '',
  gst_percent: '0',
  hsn_code: '',
}

export default function Products() {
  const [products, setProducts] = useState<Product[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [categories, setCategories] = useState<Category[]>([])
  const [subcategories, setSubcategories] = useState<Subcategory[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [editingProduct, setEditingProduct] = useState<Product | null>(null)

  const [form, setForm] = useState(emptyForm)
  const [isSaving, setIsSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [generatingBarcodeId, setGeneratingBarcodeId] = useState<number | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      let allProducts: Product[] = []
      let page = 1
      while (true) {
        const res = await listProducts({ limit: 200, page })
        const batch = res.products ?? []
        allProducts = allProducts.concat(batch)
        if (batch.length === 0 || batch.length < 20) break
        page++
        if (page > 100) break
      }
      const categoriesRes = await listCategories()
      setProducts(allProducts)
      setCategories(categoriesRes.categories ?? categoriesRes ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load products.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadSubcategoriesFor(categoryId: string) {
    if (!categoryId) {
      setSubcategories([])
      return
    }
    try {
      const res = await listSubcategories(categoryId)
      setSubcategories(res.subcategories ?? res ?? [])
    } catch {
      setSubcategories([])
    }
  }

  useEffect(() => {
    load()
  }, [])

  useEffect(() => {
    loadSubcategoriesFor(form.category_id)
  }, [form.category_id])

  function openCreate() {
    setForm(emptyForm)
    setFormError(null)
    setEditingProduct(null)
    setShowCreate(true)
  }

  function openEdit(p: Product) {
    setForm({
      name: p.name,
      description: p.description ?? '',
      price: String(p.price),
      mrp: p.mrp ? String(p.mrp) : '',
      category_id: String(p.category_id),
      subcategory_id: p.subcategory_id ? String(p.subcategory_id) : '',
      stock: '',
      image_url: p.image_url ?? '',
      gst_percent: String(p.gst_percent ?? 0),
      hsn_code: p.hsn_code ?? '',
    })
    setFormError(null)
    setEditingProduct(p)
    setShowCreate(true)
  }

  function closeModal() {
    setShowCreate(false)
    setEditingProduct(null)
  }

  function handleCategoryChange(categoryId: string) {
    setForm((f) => ({ ...f, category_id: categoryId, subcategory_id: '' }))
  }

  async function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setIsUploading(true)
    setFormError(null)
    try {
      const { image_url } = await uploadImage(file)
      setForm((f) => ({ ...f, image_url }))
    } catch (err: any) {
      setFormError(err.response?.data?.error ?? 'Failed to upload image.')
    } finally {
      setIsUploading(false)
      e.target.value = ''
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)

    if (!form.name.trim() || !form.price || !form.category_id) {
      setFormError('Name, price, and category are required.')
      return
    }

    setIsSaving(true)
    try {
      const subcategoryId = form.subcategory_id ? parseInt(form.subcategory_id, 10) : null
      if (editingProduct) {
        await updateProduct(editingProduct.id, {
          name: form.name.trim(),
          description: form.description.trim(),
          price: parseFloat(form.price),
          mrp: form.mrp ? parseFloat(form.mrp) : parseFloat(form.price),
          category_id: parseInt(form.category_id, 10),
          subcategory_id: subcategoryId,
          image_url: form.image_url.trim(),
          gst_percent: form.gst_percent ? parseFloat(form.gst_percent) : 0,
          hsn_code: form.hsn_code.trim(),
        })
      } else {
        await createProduct({
          name: form.name.trim(),
          description: form.description.trim(),
          price: parseFloat(form.price),
          mrp: form.mrp ? parseFloat(form.mrp) : parseFloat(form.price),
          category_id: parseInt(form.category_id, 10),
          subcategory_id: subcategoryId,
          image_url: form.image_url.trim(),
          gst_percent: form.gst_percent ? parseFloat(form.gst_percent) : 0,
          hsn_code: form.hsn_code.trim(),
          stock: form.stock ? parseInt(form.stock, 10) : 0,
        })
      }
      closeModal()
      setForm(emptyForm)
      load()
    } catch (err: any) {
      setFormError(
        err.response?.data?.error ??
          `Failed to ${editingProduct ? 'update' : 'create'} product.`
      )
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDelete(id: number) {
    if (!confirm('Delete this product? This cannot be undone.')) return
    try {
      await deleteProduct(id)
      setProducts((prev) => prev.filter((p) => p.id !== id))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to delete product.')
    }
  }

  async function handleStockChange(id: number, newStock: number, warehouseId: number) {
    if (Number.isNaN(newStock) || newStock < 0) return
    try {
      await updateInventory(id, newStock, warehouseId)
      setProducts((prev) =>
        prev.map((p) =>
          p.id === id
            ? {
                ...p,
                inventories: p.inventories?.length
                  ? p.inventories.map((inv, i) =>
                      i === 0 ? { ...inv, stock: newStock } : inv
                    )
                  : [{ id: 0, warehouse_id: warehouseId, stock: newStock, in_stock: newStock > 0 }],
              }
            : p
        )
      )
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update stock.')
    }
  }

  async function handleGenerateBarcode(product: Product) {
    setGeneratingBarcodeId(product.id)
    try {
      const { barcode } = await generateProductBarcode(product.id)
      setProducts((prev) => prev.map((p) => (p.id === product.id ? { ...p, barcode } : p)))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to generate barcode.')
    } finally {
      setGeneratingBarcodeId(null)
    }
  }

  async function handlePrintLabel(product: Product) {
    if (!product.barcode) return
    const qrDataUrl = await QRCode.toDataURL(product.barcode, { width: 500, margin: 2 })

    const win = window.open('', '_blank', 'width=500,height=650')
    if (!win) return
    win.document.write(`
      <html>
        <head>
          <title>Label - ${product.name}</title>
          <style>
            body { font-family: sans-serif; text-align: center; padding: 24px; }
            img { width: 380px; height: 380px; }
            h2 { font-size: 20px; margin: 12px 0 4px; }
            p { font-size: 16px; color: #444; margin: 0 0 12px; letter-spacing: 2px; }
          </style>
        </head>
        <body>
          <img src="${qrDataUrl}" alt="barcode" />
          <h2>${product.name}</h2>
          <p>${product.barcode}</p>
        </body>
      </html>
    `)
    win.document.close()
    win.focus()
    win.onload = () => win.print()
  }

  function categoryName(id: number) {
    return categories.find((c) => c.id === id)?.name ?? '-'
  }
  const filteredProducts = products.filter((p) =>
    p.name.toLowerCase().includes(searchQuery.toLowerCase())
  )


  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Products</h1>
            <p className="text-sm text-slate-400 mt-1">
              {products.length} product{products.length !== 1 ? 's' : ''}
            </p>
          </div>
          <input
            
type="text"
            
value={searchQuery}
            
onChange={(e) => setSearchQuery(e.target.value)}
            
placeholder="Search products..."
            
className="bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm w-64 mr-3"
          />
          <button
            onClick={openCreate}
            className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors"
          >
            + Add product
          </button>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && products.length === 0 && (
          <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
            No products yet. Add your first one to get started.
          </div>
        )}

        {!isLoading && products.length > 0 && (
          <div className="border border-slate-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-900 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">Image</th>
                  <th className="px-4 py-3 font-medium">Name</th>
                  <th className="px-4 py-3 font-medium">Category</th>
                  <th className="px-4 py-3 font-medium">Price</th>
                  <th className="px-4 py-3 font-medium">Stock</th>
                  <th className="px-4 py-3 font-medium">Barcode</th>
                  <th className="px-4 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {filteredProducts.map((p) => (
                  <tr key={p.id} className="border-t border-slate-800">
                    <td className="px-4 py-3">
                      {p.image_url ? (
                        <img
                          src={
                            p.image_url.startsWith('http')
                              ? p.image_url
                              : `${IMAGE_ORIGIN}${p.image_url}`
                          }
                          alt={p.name}
                          className="w-10 h-10 rounded-md object-cover border border-slate-700"
                        />
                      ) : (
                        <div className="w-10 h-10 rounded-md bg-slate-800 border border-slate-700" />
                      )}
                    </td>
                    <td className="px-4 py-3">{p.name}</td>
                    <td className="px-4 py-3 text-slate-400">
                      {categoryName(p.category_id)}
                      {p.subcategory?.name ? ` / ${p.subcategory.name}` : ''}
                    </td>
                    <td className="px-4 py-3">
                      <div>₹{p.price}</div>
                      {p.mrp > p.price && (
                        <div className="text-xs text-slate-500">
                          <span className="line-through">₹{p.mrp}</span>{' '}
                          <span className="text-emerald-400">{Math.round(((p.mrp - p.price) / p.mrp) * 100)}% OFF</span>
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <input
                        type="number"
                        defaultValue={p.inventories?.[0]?.stock ?? 0}
                        min={0}
                        className="w-20 bg-slate-800 border border-slate-700 rounded-md px-2 py-1 text-sm"
                        onBlur={(e) =>
                          handleStockChange(
                            p.id,
                            parseInt(e.target.value, 10),
                            p.inventories?.[0]?.warehouse_id ?? 1
                          )
                        }
                      />
                    </td>
                    <td className="px-4 py-3">
                      {p.barcode ? (
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-mono text-slate-300">{p.barcode}</span>
                          <button
                            onClick={() => handlePrintLabel(p)}
                            className="text-indigo-400 hover:text-indigo-300 text-xs"
                          >
                            Print
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => handleGenerateBarcode(p)}
                          disabled={generatingBarcodeId === p.id}
                          className="text-indigo-400 hover:text-indigo-300 text-xs disabled:opacity-40"
                        >
                          {generatingBarcodeId === p.id ? 'Generating...' : 'Generate'}
                        </button>
                      )}
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
          title={editingProduct ? 'Edit product' : 'Add product'}
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
              <label className="text-xs text-slate-400 block mb-1">Description</label>
              <textarea
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                rows={2}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Selling Price (₹)</label>
                <input
                  type="number"
                  value={form.price}
                  onChange={(e) => setForm({ ...form, price: e.target.value })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">MRP (₹) - optional</label>
                <input
                  type="number"
                  value={form.mrp}
                  onChange={(e) => setForm({ ...form, mrp: e.target.value })}
                  placeholder="Same as price if blank"
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
                {form.mrp && form.price && parseFloat(form.mrp) > parseFloat(form.price) && (
                  <p className="text-xs text-emerald-400 mt-1">
                    {Math.round(((parseFloat(form.mrp) - parseFloat(form.price)) / parseFloat(form.mrp)) * 100)}% OFF
                  </p>
                )}
              </div>
              {!editingProduct && (
                <div>
                  <label className="text-xs text-slate-400 block mb-1">Stock</label>
                  <input
                    type="number"
                    value={form.stock}
                    onChange={(e) => setForm({ ...form, stock: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                  />
                </div>
              )}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Category</label>
                <select
                  value={form.category_id}
                  onChange={(e) => handleCategoryChange(e.target.value)}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="">Select a category</option>
                  {categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Subcategory</label>
                <select
                  value={form.subcategory_id}
                  onChange={(e) => setForm({ ...form, subcategory_id: e.target.value })}
                  disabled={!form.category_id || subcategories.length === 0}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm disabled:opacity-40"
                >
                  <option value="">
                    {form.category_id ? 'None' : 'Select category first'}
                  </option>
                  {subcategories.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">GST %</label>
                <input
                  type="number"
                  step="0.01"
                  min={0}
                  max={100}
                  value={form.gst_percent}
                  onChange={(e) => setForm({ ...form, gst_percent: e.target.value })}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">HSN Code</label>
                <input
                  type="text"
                  value={form.hsn_code}
                  onChange={(e) => setForm({ ...form, hsn_code: e.target.value })}
                  placeholder="e.g. 8544"
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
            </div>
            <div>
              <label className="text-xs text-slate-400 block mb-1">Image</label>
              <div className="flex items-center gap-3">
                {form.image_url && (
                  <img
                    src={
                      form.image_url.startsWith('http')
                        ? form.image_url
                        : `${IMAGE_ORIGIN}${form.image_url}`
                    }
                    alt="preview"
                    className="w-12 h-12 rounded-md object-cover border border-slate-700"
                  />
                )}
                <label className="flex-1">
                  <input
                    type="file"
                    accept="image/jpeg,image/jpg,image/png,image/webp"
                    onChange={handleFileSelect}
                    disabled={isUploading}
                    className="w-full text-xs text-slate-400 file:mr-3 file:py-2 file:px-3 file:rounded-lg file:border-0 file:bg-slate-800 file:text-slate-200 file:text-xs hover:file:bg-slate-700 file:cursor-pointer"
                  />
                  {isUploading && (
                    <p className="text-xs text-slate-400 mt-1">Uploading...</p>
                  )}
                </label>
              </div>
            </div>

            {formError && <p className="text-red-400 text-xs">{formError}</p>}

            <button
              type="submit"
              disabled={isSaving || isUploading}
              className="w-full py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium transition-colors mt-2"
            >
              {isSaving
                ? 'Saving...'
                : editingProduct
                ? 'Save changes'
                : 'Create product'}
            </button>
          </form>
        </Modal>
      )}
    </Layout>
  )
}
