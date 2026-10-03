import apiClient from './client'
import type { Product, ProductCreateRequest, CreateCouponRequest, CreateOfferRequest, CreateBannerRequest, CreateDeliveryZoneRequest, DeliveryPartner, Warehouse, WarehouseStaff } from '../types/admin'

export const IMAGE_ORIGIN = (apiClient.defaults.baseURL ?? '').replace(/\/api\/v1\/?$/, '')

export const uploadImage = (file: File) => {
  const formData = new FormData()
  formData.append('image', file)
  return apiClient
    .post('/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    .then((r) => r.data as { image_url: string })
}

// ---- Products ----
export const listProducts = (params?: Record<string, any>) =>
  apiClient.get('/products', { params }).then((r) => r.data)

export const createProduct = (data: ProductCreateRequest) =>
  apiClient.post('/admin/products', data).then((r) => r.data)

export const updateProduct = (id: number, data: Partial<Product>) =>
  apiClient.put(`/admin/products/${id}`, data).then((r) => r.data)

export const deleteProduct = (id: number) =>
  apiClient.delete(`/admin/products/${id}`).then((r) => r.data)

export const updateInventory = (id: number, stock: number, warehouseId: number) =>
  apiClient.put(`/admin/products/${id}/inventory`, { stock, warehouse_id: warehouseId }).then((r) => r.data)

export const generateProductBarcode = (id: number) =>
  apiClient.post(`/admin/products/${id}/barcode`).then((r) => r.data as { id: number; barcode: string })

// ---- Subcategories ----
export const listSubcategories = (categoryId?: number | string) =>
  apiClient
    .get('/subcategories', { params: categoryId ? { category_id: categoryId } : {} })
    .then((r) => r.data)

export const createSubcategory = (data: { name: string; category_id: number; image_url?: string }) =>
  apiClient.post('/admin/subcategories', data).then((r) => r.data)

export const updateSubcategory = (
  id: number,
  data: { name: string; category_id: number; image_url?: string }
) => apiClient.put(`/admin/subcategories/${id}`, data).then((r) => r.data)

export const deleteSubcategory = (id: number) =>
  apiClient.delete(`/admin/subcategories/${id}`).then((r) => r.data)

// ---- Categories ----
export const listCategories = () =>
  apiClient.get('/categories').then((r) => r.data)

export const createCategory = (name: string) =>
  apiClient.post('/admin/categories', { name }).then((r) => r.data)

export const updateCategory = (id: number, name: string) =>
  apiClient.put(`/admin/categories/${id}`, { name }).then((r) => r.data)

export const deleteCategory = (id: number) =>
  apiClient.delete(`/admin/categories/${id}`).then((r) => r.data)

// ---- Orders ----
export const listOrders = (params?: Record<string, any>) =>
  apiClient.get('/admin/orders', { params }).then((r) => r.data)

export const updateOrderStatus = (id: number, status: string) =>
  apiClient.put(`/admin/orders/${id}/status`, { status }).then((r) => r.data)

export const assignDeliveryPartner = (orderId: number, deliveryPartnerId: number) =>
  apiClient.put(`/admin/orders/${orderId}/assign-delivery`, { delivery_partner_id: deliveryPartnerId }).then((r) => r.data)

// ---- Coupons ----
export const listCoupons = () =>
  apiClient.get('/admin/coupons').then((r) => r.data)

export const createCoupon = (data: CreateCouponRequest) =>
  apiClient.post('/admin/coupons', data).then((r) => r.data)

export const updateCouponStatus = (id: number, isActive: boolean) =>
  apiClient.put(`/admin/coupons/${id}/status`, { is_active: isActive }).then((r) => r.data)

// ---- Delivery Partners ----
export const listDeliveryPartners = () =>
  apiClient.get('/admin/delivery-partners').then((r) => r.data)

export const getUnassignedOrders = () =>
  apiClient.get('/admin/delivery/unassigned-orders').then((r) => r.data)

export const getActiveDeliveries = () =>
  apiClient.get('/admin/delivery/active').then((r) => r.data)

export const getRiderWorkload = () =>
  apiClient.get('/admin/delivery/rider-workload').then((r) => r.data)

export const getFailedDeliveries = () =>
  apiClient.get('/admin/delivery/failed').then((r) => r.data)

export const getOperationsReport = (from?: string, to?: string) =>
  apiClient.get('/admin/reports/operations', { params: { from, to } }).then((r) => r.data)

export const createDeliveryPartner = (data: Partial<DeliveryPartner>) =>
  apiClient.post('/admin/delivery-partners', data).then((r) => r.data)

export const updateDeliveryPartner = (id: number, data: Partial<DeliveryPartner>) =>
  apiClient.put(`/admin/delivery-partners/${id}`, data).then((r) => r.data)

export const deleteDeliveryPartner = (id: number) =>
  apiClient.delete(`/admin/delivery-partners/${id}`).then((r) => r.data)

// Records a rider COD cash deposit and verifies it right away.
export const recordAndVerifyCODDeposit = async (partnerId: number, amount: number, note: string) => {
  const created = await apiClient
    .post('/admin/finance/rider-cod-deposits', {
      delivery_partner_id: partnerId,
      amount,
      deposit_date: new Date().toLocaleDateString('en-CA'),
      note,
    })
    .then((r) => r.data)
  return apiClient
    .post(`/admin/finance/rider-cod-deposits/${created.id}/verify`)
    .then((r) => r.data)
}

// ---- Warehouses ----
export const listWarehouses = () =>
  apiClient.get('/admin/warehouses').then((r) => r.data)

export const getWarehouse = (id: number) =>
  apiClient.get(`/admin/warehouses/${id}`).then((r) => r.data)

export const createWarehouse = (data: Partial<Warehouse>) =>
  apiClient.post('/admin/warehouses', data).then((r) => r.data)

export const updateWarehouse = (id: number, data: Partial<Warehouse>) =>
  apiClient.put(`/admin/warehouses/${id}`, data).then((r) => r.data)

export const deleteWarehouse = (id: number) =>
  apiClient.delete(`/admin/warehouses/${id}`).then((r) => r.data)

export const setWarehouseServiceArea = (id: number, geojson: string) =>
  apiClient.put(`/admin/warehouses/${id}/service-area`, { geojson }).then((r) => r.data)

// ---- Warehouse Staff ----
export const listWarehouseStaff = () =>
  apiClient.get('/admin/warehouse-staff').then((r) => r.data)

export const createWarehouseStaff = (data: Partial<WarehouseStaff>) =>
  apiClient.post('/admin/warehouse-staff', data).then((r) => r.data)

export const updateWarehouseStaff = (id: number, data: Partial<WarehouseStaff>) =>
  apiClient.put(`/admin/warehouse-staff/${id}`, data).then((r) => r.data)

export const deleteWarehouseStaff = (id: number) =>
  apiClient.delete(`/admin/warehouse-staff/${id}`).then((r) => r.data)

// ---- Stock Transfers ----
export const listStockTransfers = () =>
  apiClient.get('/admin/stock-transfers').then((r) => r.data)

export const approveStockTransfer = (id: number) =>
  apiClient.put(`/admin/stock-transfers/${id}/approve`).then((r) => r.data)

export const rejectStockTransfer = (id: number) =>
  apiClient.put(`/admin/stock-transfers/${id}/reject`).then((r) => r.data)

// ---- Returns ----
export const listReturns = () =>
  apiClient.get('/admin/returns').then((r) => r.data)

export const approveReturn = (id: number) =>
  apiClient.put(`/admin/returns/${id}/approve`).then((r) => r.data)

export const rejectReturn = (id: number, reason?: string) =>
  apiClient.put(`/admin/returns/${id}/reject`, reason ? { reason } : {}).then((r) => r.data)

export const assignReturnPickup = (id: number, deliveryPartnerId: number) =>
  apiClient.put(`/admin/returns/${id}/assign-pickup`, { delivery_partner_id: deliveryPartnerId }).then((r) => r.data)

// ---- Analytics ----
export const getAnalyticsSummary = () =>
  apiClient.get('/admin/analytics/summary').then((r) => r.data)

export const getPickerPerformance = (warehouseId?: number) =>
  apiClient.get('/admin/analytics/picker-performance', { params: warehouseId ? { warehouse_id: warehouseId } : {} }).then((r) => r.data)

export const getProductPerformance = () =>
  apiClient.get('/admin/analytics/products').then((r) => r.data)

// ---- Wallet ----
export const creditWallet = (userId: number, amount: number, note?: string) =>
  apiClient.post(`/admin/wallet/credit/${userId}`, { amount, note }).then((r) => r.data)



export const cancelStockTransfer = (id: number) =>
  apiClient.put(`/admin/stock-transfers/${id}/cancel`).then((r) => r.data)

// ---- Customers ----
export const listCustomers = (params?: Record<string, any>) =>
  apiClient.get('/admin/customers', { params }).then((r) => r.data)

export const getCustomer = (id: number) =>
  apiClient.get(`/admin/customers/${id}`).then((r) => r.data)

export const blockCustomer = (id: number) =>
  apiClient.put(`/admin/customers/${id}/block`).then((r) => r.data)

export const unblockCustomer = (id: number) =>
  apiClient.put(`/admin/customers/${id}/unblock`).then((r) => r.data)

// ---- Inventory Overview ----
export const getInventoryOverview = (params?: Record<string, any>) =>
  apiClient.get('/admin/inventory', { params }).then((r) => r.data)

// ---- Staff & Roles ----
export const listAdminStaff = () =>
  apiClient.get('/admin/staff').then((r) => r.data)

export const updateStaffRole = (id: number, adminRole: string) =>
  apiClient.put(`/admin/staff/${id}/role`, { admin_role: adminRole }).then((r) => r.data)

// ---- Settings ----
export const getSettings = () =>
  apiClient.get('/admin/settings').then((r) => r.data)

export const updateSettings = (settings: Record<string, string>) =>
  apiClient.put('/admin/settings', { settings }).then((r) => r.data)

export const deleteCoupon = (id: number) =>
  apiClient.delete(`/admin/coupons/${id}`).then((r) => r.data)

// ---- Audit Logs ----
export const getAuditLogs = (params?: Record<string, any>) =>
  apiClient.get('/admin/audit-logs', { params }).then((r) => r.data)

// ---- Notifications ----
export const broadcastNotification = (title: string, body: string) =>
  apiClient.post('/admin/notifications/broadcast', { title, body }).then((r) => r.data)

// ---- Offers ----
export const listOffers = () =>
  apiClient.get('/admin/offers').then((r) => r.data)

export const createOffer = (data: CreateOfferRequest) =>
  apiClient.post('/admin/offers', data).then((r) => r.data)

export const updateOfferStatus = (id: number, isActive: boolean) =>
  apiClient.put(`/admin/offers/${id}/status`, { is_active: isActive }).then((r) => r.data)

export const deleteOffer = (id: number) =>
  apiClient.delete(`/admin/offers/${id}`).then((r) => r.data)

// ---- Banners ----
export const listBanners = () =>
  apiClient.get('/admin/banners').then((r) => r.data)

export const createBanner = (data: CreateBannerRequest) =>
  apiClient.post('/admin/banners', data).then((r) => r.data)

export const updateBanner = (id: number, data: Partial<CreateBannerRequest> & { is_active?: boolean }) =>
  apiClient.put(`/admin/banners/${id}`, data).then((r) => r.data)

export const deleteBanner = (id: number) =>
  apiClient.delete(`/admin/banners/${id}`).then((r) => r.data)

// ---- Delivery Zones ----
export const listDeliveryZones = () =>
  apiClient.get('/admin/delivery-zones').then((r) => r.data)

export const createDeliveryZone = (data: CreateDeliveryZoneRequest) =>
  apiClient.post('/admin/delivery-zones', data).then((r) => r.data)

export const updateDeliveryZone = (id: number, data: Partial<CreateDeliveryZoneRequest> & { is_active?: boolean }) =>
  apiClient.put(`/admin/delivery-zones/${id}`, data).then((r) => r.data)

export const deleteDeliveryZone = (id: number) =>
  apiClient.delete(`/admin/delivery-zones/${id}`).then((r) => r.data)

// ---- Control Tower ----
export const getControlTowerOverview = () =>
  apiClient.get('/admin/control-tower').then((r) => r.data)

export const updatePlatformPause = (paused: boolean, reason: string) =>
  apiClient.put('/admin/control-tower/platform-pause', { paused, reason }).then((r) => r.data)

export const updateCityStatus = (city: string, status: 'open' | 'paused' | 'closed', reason: string) =>
  apiClient.put('/admin/control-tower/city-status', { city, status, reason }).then((r) => r.data)

export const getLiveOperations = () =>
  apiClient.get('/admin/control-tower/operations').then((r) => r.data)

export const getControlTowerSettings = () =>
  apiClient.get('/admin/control-tower/settings').then((r) => r.data)

export const updateCodEnabled = (enabled: boolean, reason: string) =>
  apiClient.put('/admin/control-tower/settings/cod', { enabled, reason }).then((r) => r.data)

// ---- Support Tickets ----
export const listSupportTickets = (status?: string) =>
  apiClient.get('/admin/support/tickets', { params: status ? { status } : {} }).then((r) => r.data)

export const getSupportTicketMessages = (id: number) =>
  apiClient.get(`/admin/support/tickets/${id}/messages`).then((r) => r.data)

export const replyToSupportTicket = (id: number, message: string) =>
  apiClient.post(`/admin/support/tickets/${id}/messages`, { message }).then((r) => r.data)

export const updateSupportTicketStatus = (id: number, status: string) =>
  apiClient.put(`/admin/support/tickets/${id}/status`, { status }).then((r) => r.data)
export const updateSupportTicket = (id: number, body: { issue_type?: string; assigned_to_staff_id?: number | null; priority?: string }) =>
  apiClient.put(`/admin/support/tickets/${id}`, body).then((r) => r.data)

export const listAdminPayments = (params: {
  search?: string
  status?: string
  payment_method?: string
  gateway?: string
  date_from?: string
  date_to?: string
  page?: number
  limit?: number
}) => apiClient.get('/admin/payments', { params }).then((r) => r.data)

export const getAdminPaymentReconciliation = (params: { date_from?: string; date_to?: string }) =>
  apiClient.get('/admin/payments/reconciliation', { params }).then((r) => r.data)

export const getDashboardOverview = () =>
  apiClient.get('/admin/analytics/dashboard').then((r) => r.data)

export const getAdminPaymentDetail = (orderId: number) =>
  apiClient.get(`/admin/payments/${orderId}`).then((r) => r.data)

export const updateAdminPaymentStatus = (orderId: number, status: string, refundedAmount?: number) =>
  apiClient
    .put(`/admin/payments/${orderId}/status`, {
      status,
      ...(refundedAmount !== undefined ? { refunded_amount: refundedAmount } : {}),
    })
    .then((r) => r.data)
export const searchAdminInvoices = (params: {
  invoice_number?: string
  order_id?: string
  payment_status?: string
  date_from?: string
  date_to?: string
  page?: number
  limit?: number
}) => apiClient.get('/admin/invoices', { params }).then((r) => r.data)

export const getAdminInvoiceById = (id: number) =>
  apiClient.get(`/admin/invoices/${id}`).then((r) => r.data)

// Downloads the invoice PDF and triggers a browser save - a plain <a href>
// can't be used because the request needs the Authorization header, so we
// fetch it as a blob and hand the browser a local object URL instead.
export const downloadAdminInvoicePDF = async (id: number, invoiceNumber?: string) => {
  const res = await apiClient.get(`/admin/invoices/${id}/pdf`, { responseType: 'blob' })
  const url = window.URL.createObjectURL(new Blob([res.data]))
  const link = document.createElement('a')
  link.href = url
  const disposition = res.headers['content-disposition'] as string | undefined
  const match = disposition?.match(/filename="(.+)"/)
  link.download = match?.[1] ?? `${invoiceNumber ?? 'invoice-' + id}.pdf`
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.URL.revokeObjectURL(url)
}

// ---- Suppliers (Procurement) ----
export const listSuppliers = () =>
  apiClient.get('/admin/procurement/suppliers').then((r) => r.data)

export const createSupplier = (data: import('../types/admin').SupplierRequest) =>
  apiClient.post('/admin/procurement/suppliers', data).then((r) => r.data)

export const updateSupplier = (id: number, data: import('../types/admin').SupplierRequest) =>
  apiClient.put(`/admin/procurement/suppliers/${id}`, data).then((r) => r.data)

export const deleteSupplier = (id: number) =>
  apiClient.delete(`/admin/procurement/suppliers/${id}`).then((r) => r.data)

// ---- Purchase Orders ----
export const listPurchaseOrders = (params?: Record<string, any>) =>
  apiClient.get('/admin/procurement/purchase-orders', { params }).then((r) => r.data)

export const getPurchaseOrder = (id: number) =>
  apiClient.get(`/admin/procurement/purchase-orders/${id}`).then((r) => r.data)

export const createPurchaseOrder = (data: import('../types/admin').CreatePurchaseOrderRequest) =>
  apiClient.post('/admin/procurement/purchase-orders', data).then((r) => r.data)

export const updatePurchaseOrderStatus = (id: number, status: string, reason?: string) =>
  apiClient
    .put(`/admin/procurement/purchase-orders/${id}/status`, { status, ...(reason ? { reason } : {}) })
    .then((r) => r.data)

export const receivePurchaseOrderItems = (
  id: number,
  items: { item_id: number; quantity_received: number }[]
) => apiClient.post(`/admin/procurement/purchase-orders/${id}/receive`, { items }).then((r) => r.data)

// ---- Replenishment ----
export const getReplenishmentSuggestions = (warehouseId?: number) =>
  apiClient
    .get('/admin/procurement/replenishment', { params: warehouseId ? { warehouse_id: warehouseId } : {} })
    .then((r) => r.data)

export const updateProductReorderLevel = (productId: number, reorderLevel: number) =>
  apiClient
    .put(`/admin/products/${productId}/reorder-level`, { reorder_level: reorderLevel })
    .then((r) => r.data)

export const getPartnerOnboarding = (id: number) =>
  apiClient.get(`/admin/delivery-partners/${id}/onboarding`).then((r) => r.data)
