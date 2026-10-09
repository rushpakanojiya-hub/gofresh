export interface WarehouseStaff {
  id: number
  name: string
  phone: string
  warehouse_id: number
  role?: string
  is_active?: boolean
  warehouse?: Warehouse
}

export interface Warehouse {
  id: number
  name: string
  city: string
  address?: string
  lat: number
  lng: number
  service_radius_km?: number
  is_active?: boolean
}

export interface Product {
  id: number
  name: string
  price: number
}

export interface StockTransfer {
  id: number
  product_id: number
  product?: Product
  from_warehouse_id: number
  from_warehouse?: Warehouse
  to_warehouse_id: number
  to_warehouse?: Warehouse
  quantity: number
  status: 'pending' | 'in_transit' | 'received' | 'rejected' | 'cancelled'
  requested_by: number
  approved_by?: number | null
  created_at?: string
}

// ---- Orders / fulfillment ----

export type OrderStatus =
  | 'pending'
  | 'confirmed'
  | 'picking'
  | 'picked'
  | 'packing'
  | 'packed'
  | 'ready_for_dispatch'
  | 'handed_over'
  | 'shipped'
  | 'delivered'
  | 'returned'
  | 'cancelled'

export interface DeliveryPartner {
  id: number
  name: string
  phone: string
  vehicle_number?: string
}

export interface Address {
  id: number
  line1?: string
  line2?: string
  city?: string
  state?: string
  pincode?: string
}

export interface OrderItem {
  id: number
  order_id: number
  product_id: number
  product?: Product
  quantity: number
  price: number
}

export interface Order {
  id: number
  user_id: number
  address_id: number
  address?: Address
  warehouse_id?: number
  items_amount: number
  delivery_charge: number
  wallet_amount_used: number
  total_amount: number
  status: OrderStatus
  payment_method: 'cod' | 'online'
  payment_status: 'pending' | 'paid' | 'failed'
  delivery_partner_id?: number | null
  delivery_partner?: DeliveryPartner
  picker_id?: number | null
  picker_name?: string
  task_status?: string
  delayed?: boolean
  items?: OrderItem[]
  created_at: string
  updated_at: string
}

export interface OrdersResponse {
  orders: Order[]
  page: number
  limit: number
  total: number
  total_pages: number
}

// ---- Picking ----

export type PickItemStatus = 'pending' | 'picked' | 'unavailable' | 'short'
export type TaskStatus = 'pending' | 'in_progress' | 'completed'

export interface PickingTaskItem {
  id: number
  picking_task_id: number
  order_item_id: number
  product_id: number
  product?: Product
  quantity_needed: number
  quantity_picked: number
  status: PickItemStatus
  reason?: string
  created_at: string
  updated_at: string
}

export interface PickingTask {
  id: number
  order_id: number
  order?: Order
  warehouse_id: number
  picker_id?: number | null
  status: TaskStatus
  started_at?: string | null
  completed_at?: string | null
  created_at: string
  updated_at: string
  items?: PickingTaskItem[]
}

// ---- Packing ----

export interface PackingTask {
  id: number
  order_id: number
  order?: Order
  warehouse_id: number
  packer_id?: number | null
  status: TaskStatus
  started_at?: string | null
  completed_at?: string | null
  created_at: string
  updated_at: string
}

export interface PackingTaskResponse {
  packing_task: PackingTask
  picked_items: PickingTaskItem[]
}

// ---- Dashboard ----

export interface WarehouseDashboardStats {
  today_orders: number
  new_orders: number
  picking: number
  packed: number
  ready_for_dispatch: number
  completed_today: number
  cancelled_today: number
  low_stock_products: number
  out_of_stock_products: number
  pending_stock_transfers: number
  pending_receivings: number
  open_exceptions: number
  pending_handovers: number
  delayed_orders: number
  expiring_stock_batches: number
  active_staff: number
  avg_picking_minutes: number
  avg_packing_minutes: number
  fulfillment_rate: number
  items_per_hour?: number
  online_pickers?: number
  pending_putaway?: number
}

// ---- Exceptions ----

export type ExceptionType =
  | 'unavailable'
  | 'short_quantity'
  | 'damaged'
  | 'wrong_product'
  | 'barcode_mismatch'
  | 'picking_failure'
  | 'packing_failure'
  | 'order_cancellation'
  | 'delivery_partner_unavailable'
  | 'order_delayed'

export type ExceptionPriority = 'low' | 'medium' | 'high'
export type ExceptionStatus = 'open' | 'investigating' | 'resolved' | 'closed'

export interface WarehouseException {
  id: number
  order_id: number
  order?: Order
  product_id?: number | null
  product?: Product
  warehouse_id: number
  type: ExceptionType
  reason?: string
  priority: ExceptionPriority
  staff_id?: number | null
  status: ExceptionStatus
  resolution?: string
  resolved_by_id?: number | null
  resolved_at?: string | null
  created_at: string
  updated_at: string
}

export interface ExceptionsResponse {
  exceptions: WarehouseException[]
  page: number
  limit: number
  total: number
  total_pages: number
}

// ---- Staff performance ----

export interface StaffPerformanceRow {
  staff_id: number
  staff_name: string
  orders_picked: number
  orders_packed: number
  avg_picking_minutes: number
  avg_packing_minutes: number
  total_items_picked: number
  clean_picks: number
  accuracy_rate: number
  exceptions_caused: number
  role?: string
  items_per_hour?: number
}

// ---- Warehouse locations ----

export interface WarehouseZone {
  id: number
  warehouse_id: number
  name: string
  storage_type?: string
  created_at: string
  updated_at: string
}

export interface WarehouseRack {
  id: number
  zone_id: number
  name: string
  created_at: string
  updated_at: string
}

export interface WarehouseBin {
  id: number
  rack_id: number
  rack?: WarehouseRack & { zone?: WarehouseZone }
  name: string
  created_at: string
  updated_at: string
}

export interface Inventory {
  id: number
  product_id: number
  product?: Product
  warehouse_id: number
  bin_id?: number | null
  bin?: WarehouseBin | null
  stock: number
  in_stock: boolean
  created_at: string
  updated_at: string
}

// ---- Stock movements ----

export type MovementType =
  | 'receive'
  | 'sale'
  | 'adjustment'
  | 'transfer'
  | 'damaged'
  | 'expired'
  | 'return'
  | 'correction'

export type AdjustReason =
  | 'damaged'
  | 'expired'
  | 'counting_error'
  | 'lost'
  | 'found'
  | 'manual_correction'
  | 'other'

export interface StockMovement {
  id: number
  product_id: number
  product?: Product
  warehouse_id: number
  previous_qty: number
  change: number
  new_qty: number
  movement_type: MovementType
  reason?: string
  staff_id?: number | null
  reference_id?: number | null
  notes?: string
  created_at: string
}

export interface StockMovementsResponse {
  movements: StockMovement[]
  page: number
  limit: number
  total: number
  total_pages: number
}

// ---- Barcode scan ----

export interface ScanResult {
  match: boolean
  item_id: number
  product_id: number
  product_name: string
}

// ---- Receiving ----

export type ReceivingStatus = 'pending' | 'received' | 'accepted' | 'rejected' | 'put_away'

export interface Receiving {
  id: number
  warehouse_id: number
  supplier_name: string
  reference_number?: string
  product_id: number
  product?: Product
  expected_quantity: number
  received_quantity: number
  damaged_quantity: number
  accepted_quantity: number
  status: ReceivingStatus
  bin_id?: number | null
  bin?: WarehouseBin | null
  created_by_staff_id: number
  received_by_staff_id?: number | null
  qc_by_staff_id?: number | null
  put_away_by_staff_id?: number | null
  notes?: string
  rejection_reason?: string
  received_at?: string | null
  qc_at?: string | null
  put_away_at?: string | null
  created_at: string
  updated_at: string
}

export interface ReceivingsResponse {
  receivings: Receiving[]
  page: number
  limit: number
  total: number
  total_pages: number
}

// ---- Batch & Expiry ----

export interface Batch {
  id: number
  product_id: number
  product?: Product
  warehouse_id: number
  batch_number: string
  manufacture_date?: string | null
  expiry_date: string
  quantity: number
  bin_id?: number | null
  bin?: WarehouseBin | null
  created_by_staff_id: number
  receiving_id?: number | null
  created_at: string
  updated_at: string
}

export interface BatchesResponse {
  batches: Batch[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface ExpiringBatchesResponse {
  batches: Batch[]
  days: number
  count: number
}

// ---- Staff overview ----

export interface StaffOverviewRow {
  id: number
  name: string
  phone: string
  role: string
  is_active: boolean
  current_task?: string | null
  orders_handled: number
  last_activity?: string | null
}

// ---- Warehouse Inventory ----

export type WarehouseStockStatus = 'in_stock' | 'low' | 'out'

export interface WarehouseInventoryRow {
  product_id: number
  product_name: string
  barcode?: string
  image_url?: string
  category_id: number
  category_name: string
  stock: number
  reserved: number
  available: number
  in_stock: boolean
  stock_status: WarehouseStockStatus
  bin_id?: number | null
  bin_name?: string
  rack_name?: string
  zone_name?: string
  expired_qty: number
  last_damaged_at?: string | null
  last_damaged_qty?: number
}

export interface WarehouseInventoryResponse {
  rows: WarehouseInventoryRow[]
  page: number
  limit: number
  total: number
  total_pages: number
  in_stock_count: number
  low_stock_count: number
  out_of_stock_count: number
  damaged_count: number
  expired_count: number
  low_stock_threshold: number
}

export interface InvoiceItem {
  id: number
  product_id: number
  product_name: string
  quantity: number
  price: number
}

export interface OrderInvoice {
  invoice_number: string
  order_id: number
  order_status: OrderStatus
  customer_name: string
  customer_phone: string
  items_amount: number
  delivery_charge: number
  wallet_used: number
  total_amount: number
  payment_method: 'cod' | 'online'
  payment_status: 'pending' | 'paid' | 'failed'
  generated_at: string
  items: InvoiceItem[]
}

// ---- Audit logs ----

export interface WarehouseAuditLog {
  id: number
  warehouse_id: number
  staff_id: number
  staff_name: string
  action: string
  entity_type: string
  entity_id: string
  before_value?: string
  after_value?: string
  created_at: string
}

export interface AuditLogsResponse {
  audit_logs: WarehouseAuditLog[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface WarehouseNotification {
  id: number
  warehouse_id: number
  type: string
  title: string
  message: string
  order_id?: number
  product_id?: number
  is_read: boolean
  read_at?: string
  created_at: string
}

export interface WarehouseNotificationsResponse {
  notifications: WarehouseNotification[]
  unread_count: number
  page: number
  limit: number
  total: number
  total_pages: number
}

// ---- Substitution ----

export type SubstitutionStatus = 'pending' | 'approved' | 'rejected'

export interface SubstitutionRequest {
  id: number
  order_id: number
  order?: { id: number; status: string }
  picking_task_item_id?: number
  original_product_id: number
  original_product?: Product
  substitute_product_id: number
  substitute_product?: Product
  quantity: number
  reason?: string
  warehouse_id: number
  requested_by_id: number
  status: SubstitutionStatus
  decided_by_id?: number
  decision_note?: string
  decided_at?: string
  created_at: string
  updated_at: string
}

export interface SubstitutionRequestsResponse {
  substitution_requests: SubstitutionRequest[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface ReturnRequestItem {
id: number
return_request_id: number
order_item_id: number
order_item?: {
id: number
product_id: number
quantity: number
price: number
product?: { id: number; name: string; image_url?: string }
}
quantity: number
refund_amount: number
}

export interface StoreReturnRequest {
id: number
order_id: number
user_id: number
reason: string
status: string
refund_amount: number
items?: ReturnRequestItem[]
processed_by?: number
rejection_reason?: string
image_url?: string
created_at: string
updated_at: string
customer_name: string
customer_phone: string
}

export interface StoreReturnsResponse {
return_requests: StoreReturnRequest[]
}

// ---- Picking monitor ----

export interface PickingMonitorTask {
  task_id: number
  order_id: number
  picker_id: number | null
  picker_name: string
  picker_online: boolean
  status: TaskStatus
  created_at: string
  started_at: string | null
  waiting_minutes: number
  items_total: number
  items_done: number
  stale: boolean
  delayed: boolean
}

export interface PickingMonitorPicker {
  staff_id: number
  name: string
  online: boolean
  active_tasks: number
  state: 'offline' | 'idle' | 'busy'
}

export interface PickingMonitorData {
  tasks: PickingMonitorTask[]
  pickers: PickingMonitorPicker[]
}

// ---- Putaway ----

export interface PutawayBinRef {
  id: number
  name?: string
  rack?: { name?: string; zone?: { name?: string } }
}

export interface PutawayTaskRow {
  id: number
  receiving_id: number
  product_id: number
  product?: { id: number; name: string }
  receiving?: { id: number; supplier_name?: string }
  quantity: number
  suggested_bin_id?: number | null
  suggested_bin?: PutawayBinRef | null
  actual_bin_id?: number | null
  actual_bin?: PutawayBinRef | null
  putter_id?: number | null
  putter_name: string
  status: 'pending' | 'in_progress' | 'completed'
  wrong_location: boolean
  note?: string
  created_at: string
  assigned_at?: string | null
  started_at?: string | null
  completed_at?: string | null
}

export interface PutawayData {
  tasks: PutawayTaskRow[]
  putters: { id: number; name: string }[]
  summary: {
    pending: number
    in_progress: number
    completed_today: number
    wrong_location_7d: number
  }
}

// ---- Location occupancy ----

export interface OccupancyBin {
  id: number
  name: string
  product_count: number
  total_qty: number
  products: { product_id: number; name: string; stock: number }[]
}

export interface OccupancyData {
  zones: {
    id: number
    name: string
    storage_type?: string
    racks: { id: number; name: string; bins: OccupancyBin[] }[]
  }[]
  summary: {
    zones: number
    racks: number
    bins: number
    occupied_bins: number
    unassigned_products: number
  }
}
