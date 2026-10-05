import apiClient from './client'
import type {
  StockTransfer,
  OrdersResponse,
  PickingTask,
  PickingTaskItem,
  PackingTaskResponse,
  PackingTask,
  WarehouseDashboardStats,
  PickItemStatus,
  ScanResult,
  Receiving,
  ReceivingsResponse,
  Batch,
  BatchesResponse,
  ExpiringBatchesResponse,
  WarehouseInventoryResponse,
  ExceptionsResponse,
  WarehouseException,
  StaffPerformanceRow,
  WarehouseZone,
  WarehouseRack,
  WarehouseBin,
  Inventory,
  StockMovementsResponse,
  AuditLogsResponse,
  OrderInvoice,
  WarehouseNotificationsResponse,
  SubstitutionRequest,
  SubstitutionRequestsResponse,
  StoreReturnRequest,
  StoreReturnsResponse,
} from '../types/warehouse'

export const listMyStockTransfers = () =>
  apiClient.get('/warehouse/stock-transfers', { params: { limit: 100 } }).then((r) => r.data as { stock_transfers: StockTransfer[] })
export const requestStockTransfer = (data: {
  product_id: number
  to_warehouse_id: number
  quantity: number
}) => apiClient.post('/warehouse/stock-transfers', data).then((r) => r.data)
export const receiveStockTransfer = (id: number) =>
  apiClient.put(`/warehouse/stock-transfers/${id}/receive`).then((r) => r.data)
export const approveStockTransfer = (id: number) =>
  apiClient.put(`/warehouse/stock-transfers/${id}/approve`).then((r) => r.data)
export const rejectStockTransfer = (id: number) =>
  apiClient.put(`/warehouse/stock-transfers/${id}/reject`).then((r) => r.data)
export const listProducts = (params?: Record<string, any>) =>
  apiClient.get('/products', { params }).then((r) => r.data)

// ---- Dashboard ----

export const getDashboard = () =>
  apiClient.get('/warehouse/dashboard').then((r) => r.data as WarehouseDashboardStats)

// ---- Orders ----

export const listWarehouseOrders = (params: { status?: string; page?: number; limit?: number }) =>
  apiClient.get('/warehouse/orders', { params }).then((r) => r.data as OrdersResponse)

export const acceptOrder = (orderId: number) =>
  apiClient
    .put(`/warehouse/orders/${orderId}/accept`)
    .then((r) => r.data as { success: boolean; order_id: number; status: string })

export const handoverOrder = (orderId: number, data: { package_count: number; delivery_partner_id: number }) =>
  apiClient
    .post(`/warehouse/orders/${orderId}/handover`, data)
    .then((r) => r.data as { success: boolean; order_id: number; status: string })

export const getHandover = (orderId: number) =>
  apiClient.get(`/warehouse/orders/${orderId}/handover`).then((r) => r.data)

// ---- Picking ----

export const getPickingTask = (orderId: number) =>
  apiClient.get(`/warehouse/picking/${orderId}`).then((r) => r.data as PickingTask)

export const startPicking = (orderId: number) =>
  apiClient.put(`/warehouse/picking/${orderId}/start`).then((r) => r.data as PickingTask)

export const markPickItem = (
  itemId: number,
  data: { status: PickItemStatus; quantity_picked?: number; reason?: string }
) => apiClient.put(`/warehouse/picking/items/${itemId}`, data).then((r) => r.data as PickingTaskItem)

export const completePicking = (orderId: number) =>
  apiClient
    .put(`/warehouse/picking/${orderId}/complete`)
    .then((r) => r.data as { success: boolean; picking_task: PickingTask; packing_task: PackingTask })

// ---- Packing ----

export const getPackingTask = (orderId: number) =>
  apiClient.get(`/warehouse/packing/${orderId}`).then((r) => r.data as PackingTaskResponse)

export const startPacking = (orderId: number) =>
  apiClient.put(`/warehouse/packing/${orderId}/start`).then((r) => r.data as PackingTask)

export const completePacking = (orderId: number) =>
  apiClient
    .put(`/warehouse/packing/${orderId}/complete`)
    .then((r) => r.data as { success: boolean; packing_task: PackingTask; order_status: string })


// ---- Exceptions ----

export const listExceptions = (params: {
  status?: string
  priority?: string
  type?: string
  page?: number
  limit?: number
}) => apiClient.get('/warehouse/exceptions', { params }).then((r) => r.data as ExceptionsResponse)

export const updateException = (id: number, data: { status: string; resolution?: string }) =>
  apiClient.put(`/warehouse/exceptions/${id}`, data).then((r) => r.data as WarehouseException)

// ---- Staff performance ----

export const getStaffPerformance = () =>
  apiClient
    .get('/warehouse/staff/performance')
    .then((r) => r.data as { staff_performance: StaffPerformanceRow[] })

export const getMyPerformance = () =>
  apiClient.get('/warehouse/staff/performance/me').then((r) => r.data as StaffPerformanceRow)

// ---- Warehouse locations ----

export const listZones = () =>
  apiClient.get('/warehouse/zones').then((r) => r.data as { zones: WarehouseZone[] })

export const createZone = (name: string) =>
  apiClient.post('/warehouse/zones', { name }).then((r) => r.data as WarehouseZone)

export const listRacks = (zoneId: number) =>
  apiClient.get(`/warehouse/zones/${zoneId}/racks`).then((r) => r.data as { racks: WarehouseRack[] })

export const createRack = (zoneId: number, name: string) =>
  apiClient.post(`/warehouse/zones/${zoneId}/racks`, { name }).then((r) => r.data as WarehouseRack)

export const listBins = (rackId: number) =>
  apiClient.get(`/warehouse/racks/${rackId}/bins`).then((r) => r.data as { bins: WarehouseBin[] })

export const createBin = (rackId: number, name: string) =>
  apiClient.post(`/warehouse/racks/${rackId}/bins`, { name }).then((r) => r.data as WarehouseBin)

export const getProductInventory = (productId: number) =>
  apiClient.get(`/warehouse/inventory/${productId}`).then((r) => r.data as Inventory)

export const assignProductBin = (productId: number, binId: number | null) =>
  apiClient
    .put(`/warehouse/inventory/${productId}/bin`, { bin_id: binId })
    .then((r) => r.data as Inventory)

// ---- Stock adjustment / movement ----

export const adjustStock = (
  productId: number,
  data: { new_quantity: number; reason: string; notes?: string }
) => apiClient.post(`/warehouse/inventory/${productId}/adjust`, data).then((r) => r.data as Inventory)

export const listStockMovements = (params: { product_id?: number; movement_type?: string; page?: number; limit?: number }) =>
  apiClient.get('/warehouse/stock-movements', { params }).then((r) => r.data as StockMovementsResponse)

// ---- Warehouse location deletion ----

export const deleteZone = (zoneId: number) =>
  apiClient.delete(`/warehouse/zones/${zoneId}`).then((r) => r.data)

export const deleteRack = (rackId: number) =>
  apiClient.delete(`/warehouse/racks/${rackId}`).then((r) => r.data)

export const deleteBin = (binId: number) =>
  apiClient.delete(`/warehouse/bins/${binId}`).then((r) => r.data)

// ---- Barcode scan ----

export const scanPickItem = (itemId: number, barcode: string) =>
  apiClient.put(`/warehouse/picking/items/${itemId}/scan`, { barcode }).then((r) => r.data as ScanResult)


// ---- Receiving ----

export const createReceiving = (data: {
  supplier_name: string
  reference_number?: string
  product_id: number
  expected_quantity: number
  notes?: string
}) => apiClient.post('/warehouse/receiving', data).then((r) => r.data as Receiving)

export const listReceivings = (params: { status?: string; page?: number; limit?: number }) =>
  apiClient.get('/warehouse/receiving', { params }).then((r) => r.data as ReceivingsResponse)

export const getReceiving = (id: number) =>
  apiClient.get(`/warehouse/receiving/${id}`).then((r) => r.data as Receiving)

export const markReceived = (id: number, data: { received_quantity: number; damaged_quantity: number; notes?: string }) =>
  apiClient.put(`/warehouse/receiving/${id}/receive`, data).then((r) => r.data as Receiving)

export const qcReceiving = (id: number, data: { action: 'accept' | 'reject'; accepted_quantity?: number; rejection_reason?: string }) =>
  apiClient.put(`/warehouse/receiving/${id}/qc`, data).then((r) => r.data as Receiving)

export const putAwayReceiving = (id: number, binId: number | null) =>
  apiClient.put(`/warehouse/receiving/${id}/putaway`, { bin_id: binId }).then((r) => r.data as Receiving)

// ---- Batch & Expiry ----

export const createBatch = (data: {
  product_id: number
  batch_number: string
  manufacture_date?: string
  expiry_date: string
  quantity: number
  bin_id?: number | null
}) => apiClient.post('/warehouse/batches', data).then((r) => r.data as Batch)

export const listBatches = (params: { product_id?: number; expiring_within_days?: number; page?: number; limit?: number }) =>
  apiClient.get('/warehouse/batches', { params }).then((r) => r.data as BatchesResponse)

export const listExpiringBatches = (days = 7) =>
  apiClient.get('/warehouse/batches/expiring', { params: { days } }).then((r) => r.data as ExpiringBatchesResponse)

export const adjustBatchQuantity = (id: number, data: { quantity: number; reason: string }) =>
  apiClient.put(`/warehouse/batches/${id}/quantity`, data).then((r) => r.data as Batch)

export const deleteBatch = (id: number) =>
  apiClient.delete(`/warehouse/batches/${id}`).then((r) => r.data)
// ---- Staff overview ----

export const getStaffOverview = () =>
  apiClient.get('/warehouse/staff', { params: { limit: 100 } }).then((r) => r.data as { staff: import('../types/warehouse').StaffOverviewRow[] })


// ---- Warehouse Inventory ----

export const getWarehouseInventory = (params: {
  search?: string
  category_id?: number
  stock_status?: string
  zone_id?: number
  rack_id?: number
  bin_id?: number
  page?: number
  limit?: number
}) => apiClient.get('/warehouse/inventory', { params }).then((r) => r.data as WarehouseInventoryResponse)

export const getOrderInvoice = (orderId: number) =>
  apiClient.get(`/warehouse/orders/${orderId}/invoice`).then((r) => r.data as OrderInvoice)



// ---- Audit logs ----

export const listAuditLogs = (params: {
  action?: string
  entity_type?: string
  staff_id?: number
  page?: number
  limit?: number
}) => apiClient.get('/warehouse/audit-logs', { params }).then((r) => r.data as AuditLogsResponse)
export const listWarehouseNotifications = (params: {
  is_read?: boolean
  type?: string
  page?: number
  limit?: number
}) => apiClient.get('/warehouse/notifications', { params }).then((r) => r.data as WarehouseNotificationsResponse)

export const markNotificationRead = (id: number) =>
  apiClient.put(`/warehouse/notifications/${id}/read`).then((r) => r.data)

export const markAllNotificationsRead = () =>
  apiClient.put('/warehouse/notifications/read-all').then((r) => r.data)

// ---- Substitution ----

export const listSubstitutionRequests = (params: {
  status?: string
  order_id?: number
  page?: number
  limit?: number
}) => apiClient.get('/warehouse/substitutions', { params }).then((r) => r.data as SubstitutionRequestsResponse)

export const getSubstitutionRequest = (id: number) =>
  apiClient.get(`/warehouse/substitutions/${id}`).then((r) => r.data as SubstitutionRequest)

export const createSubstitutionRequest = (data: {
  order_id: number
  picking_task_item_id?: number
  original_product_id: number
  substitute_product_id: number
  quantity: number
  reason?: string
}) => apiClient.post('/warehouse/substitutions', data).then((r) => r.data as SubstitutionRequest)

export const approveSubstitutionRequest = (id: number, note?: string) =>
  apiClient.put(`/warehouse/substitutions/${id}/approve`, { note }).then((r) => r.data as SubstitutionRequest)

export const rejectSubstitutionRequest = (id: number, note?: string) =>
  apiClient.put(`/warehouse/substitutions/${id}/reject`, { note }).then((r) => r.data as SubstitutionRequest)

// ---- Returns ----

export const listStoreReturns = (params: { status?: string }) =>
  apiClient.get('/warehouse/returns', { params }).then((r) => r.data as StoreReturnsResponse)

export const approveStoreReturn = (id: number) =>
  apiClient.put(`/warehouse/returns/${id}/approve`).then((r) => r.data as StoreReturnRequest)

export const rejectStoreReturn = (id: number, reason?: string) =>
  apiClient.put(`/warehouse/returns/${id}/reject`, { reason }).then((r) => r.data as StoreReturnRequest)

export const getCheckinQR = () =>
  apiClient
    .get('/warehouse/checkin-qr')
    .then((r) => r.data as { token: string; expires_at: string; ttl_seconds: number; warehouse_id: number })