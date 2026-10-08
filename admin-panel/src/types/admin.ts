export interface Product {
  id: number
  name: string
  description?: string
  price: number
  mrp: number
  category_id: number
  image_url?: string
  barcode?: string
  gst_percent?: number
  hsn_code?: string
  weight?: string
  subcategory_id?: number | null
  subcategory?: { id: number; name: string } | null
  created_at?: string
  inventories?: { id: number; warehouse_id: number; stock: number; in_stock: boolean }[]
}

export interface ProductCreateRequest {
  name: string
  description?: string
  price: number
  mrp: number
  category_id: number
  subcategory_id?: number | null
  image_url?: string
  stock?: number
  gst_percent?: number
  hsn_code?: string
  weight?: string
}

export interface Category {
  id: number
  name: string
}

export interface Subcategory {
  id: number
  name: string
  category_id: number
}

export interface OrderItem {
  id: number
  order_id: number
  product_id: number
  product?: Product
  quantity: number
  price: number
  created_at?: string
}

export interface Order {
  id: number
  user_id: number
  status: string
  total_amount: number
  payment_method?: string
  payment_status?: string
  collected_via?: 'cash' | 'upi' | null
  upi_status?: 'unverified' | 'verified' | 'rejected' | null
  upi_utr?: string | null
  upi_verified_at?: string | null
  created_at: string
  items?: OrderItem[]
  delivery_partner_id?: number | null
  delivery_status?: string | null
  delivery_proof_url?: string | null
}

export interface Coupon {
  id: number
  code: string
  discount_type: "flat" | "percentage"
  discount_value: number
  min_order_amount: number
  max_discount_amount?: number | null
  usage_limit: number
  used_count: number
  expiry_date: string
  is_active: boolean
}

export interface CreateCouponRequest {
  code: string
  discount_type: "flat" | "percentage"
  discount_value: number
  min_order_amount?: number
  max_discount_amount?: number | null
  usage_limit?: number
  expiry_date: string
}

export interface DeliveryPartner {
  id: number
  name: string
  phone: string
  vehicle_number?: string
  is_active?: boolean
  avg_rating?: number
  rating_count?: number
  profile_photo_url?: string
}

export interface StaffPerformance {
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
  warehouse_id: number
  warehouse_name: string
}
export interface Warehouse {
  id: number
  name: string
  city: string
  address?: string
  lat: number
  lng: number
  service_radius_km?: number
  service_area?: string
  is_active?: boolean
  warehouse_type?: 'dark_store' | 'mother_warehouse'
  status?: 'open' | 'closed' | 'paused'
  capacity?: number
  opening_time?: string | null
  closing_time?: string | null
}

export interface WarehouseStaff {
  id: number
  name: string
  phone: string
  warehouse_id: number
  role?: string
  is_active?: boolean
  status?: string
}

export interface Supplier {
  id: number
  name: string
  contact_name?: string
  phone?: string
  email?: string
  gstin?: string
  address?: string
  is_active?: boolean
  created_at?: string
}

export interface SupplierRequest {
  name: string
  contact_name?: string
  phone?: string
  email?: string
  gstin?: string
  address?: string
  is_active?: boolean
}

export interface PurchaseOrderItem {
  id: number
  purchase_order_id: number
  product_id: number
  product?: Product
  quantity_ordered: number
  quantity_received: number
  unit_price: number
}

export interface PurchaseOrder {
  id: number
  po_number: string
  vendor_id: number
  vendor?: Supplier
  warehouse_id: number
  warehouse?: Warehouse
  status: 'draft' | 'sent' | 'confirmed' | 'partially_received' | 'received' | 'cancelled'
  expected_date?: string | null
  notes?: string
  total_amount: number
  created_by_id: number
  approved_by_id?: number | null
  approved_at?: string | null
  cancelled_reason?: string
  items?: PurchaseOrderItem[]
  created_at: string
  updated_at: string
}

export interface CreatePurchaseOrderItemRequest {
  product_id: number
  quantity_ordered: number
  unit_price: number
}

export interface CreatePurchaseOrderRequest {
  vendor_id: number
  warehouse_id: number
  expected_date?: string
  notes?: string
  items: CreatePurchaseOrderItemRequest[]
}

export interface ReplenishmentSuggestion {
  product_id: number
  product_name: string
  warehouse_id: number
  warehouse_name: string
  current_stock: number
  reorder_level: number
  suggested_order_qty: number
}

export interface StockTransfer {
  id: number
  product_id: number
  from_warehouse_id: number
  to_warehouse_id: number
  quantity: number
  status: string
  created_at?: string
}

export interface ReturnRequestItem {
  id: number
  order_item_id: number
  order_item?: OrderItem
  quantity: number
  refund_amount: number
}

export interface ReturnRequest {
  id: number
  order_id: number
  reason?: string
  status: string
  items?: ReturnRequestItem[]
  created_at?: string
  refund_amount: number
  customer_name?: string
  customer_phone?: string
  rejection_reason?: string
  pickup_status?: string
  delivery_partner_id?: number
  delivery_partner_name?: string
  delivery_partner_phone?: string
}

export interface AnalyticsSummary {
  total_users: number
  total_orders: number
  total_sales: number
  pending_orders: number
  delivered_orders: number
  cancelled_orders: number
}

export interface ProductPerformance {
  product_id: number
  product_name: string
  units_sold: number
  total_revenue: number
}

export interface PaginatedResponse<_T> {
  page: number
  limit: number
  total: number
  total_pages: number
  [key: string]: any // covers "products", "orders", etc. - key name varies by endpoint
}

export interface Wallet {
  id: number
  user_id: number
  balance: number
  created_at: string
  updated_at: string
}

// ---- Customers ----
export interface CustomerSummary {
  id: number
  name: string
  phone: string
  is_blocked: boolean
  created_at: string
  total_orders: number
  total_spent: number
  last_order_at?: string | null
}

export interface CustomerDetail extends CustomerSummary {
  orders: Order[]
  addresses: {
    id: number
    label: string
    full_name: string
    phone: string
    line1: string
    line2?: string
    city: string
    state: string
    pincode: string
    is_default: boolean
  }[]
  wallet?: { id: number; balance: number } | null
  wallet_transactions?: { id: number; type: string; amount: number; created_at: string }[]
}

// ---- Inventory Overview ----
export interface InventoryRow {
  product_id: number
  product_name: string
  category_name: string
  warehouse_id: number
  warehouse_name: string
  stock: number
  reserved: number
  available: number
  in_stock: boolean
}

export interface InventoryOverviewResponse {
  total_skus: number
  total_available_stock: number
  total_reserved_stock: number
  low_stock_count: number
  out_of_stock_count: number
  damaged_stock: number
  expired_stock: number
  rows: InventoryRow[]
  page: number
  limit: number
  total: number
  total_pages: number
}

// ---- Staff & Roles ----
export interface StaffMember {
  id: number
  name: string
  phone: string
  admin_role?: string
  created_at: string
}

export const ADMIN_ROLES = [
  { value: 'super_admin', label: 'Super Admin' },
  { value: 'admin', label: 'Admin' },
  { value: 'ops_manager', label: 'Operations Manager' },
  { value: 'warehouse_manager', label: 'Warehouse Manager' },
  { value: 'support_agent', label: 'Support Agent' },
  { value: 'finance_manager', label: 'Finance Manager' },
]

// ---- Settings ----
export interface SettingsMap {
  [key: string]: string
}

export const SETTINGS_LABELS: Record<string, string> = {
  free_delivery_threshold: 'Free Delivery Threshold (Rs)',
  flat_delivery_charge: 'Flat Delivery Charge (Rs)',
  min_order_amount: 'Minimum Order Amount (Rs)',
  cancellation_window_minutes: 'Cancellation Window (minutes)',
  company_name: 'Company Name',
  support_phone: 'Support Phone',
  support_email: 'Support Email',
  gst_percentage: 'GST Percentage (%)',
}


// ---- Audit Logs ----
export interface AuditLog {
  id: number
  admin_id: number
  admin_phone: string
  action: string
  entity_type: string
  entity_id: string
  details: string
  created_at: string
}

export interface AuditLogsResponse {
  logs: AuditLog[]
  page: number
  limit: number
  total: number
  total_pages: number
}

// ---- Offers ----
export interface Offer {
  id: number
  title: string
  description: string
  image_url: string
  discount_text: string
  category_id?: number | null
  start_date: string
  end_date: string
  is_active: boolean
  created_at: string
}

export interface CreateOfferRequest {
  title: string
  description?: string
  image_url?: string
  discount_text?: string
  category_id?: number | null
  start_date: string
  end_date: string
}

// ---- Banners ----
export interface Banner {
  id: number
  image_url: string
  title: string
  link_type: 'product' | 'category' | 'url' | 'none'
  link_value: string
  display_order: number
  is_active: boolean
  created_at: string
}

export interface CreateBannerRequest {
  image_url: string
  title?: string
  link_type?: string
  link_value?: string
  display_order?: number
}

// ---- Delivery Zones ----
export interface DeliveryZone {
  id: number
  name: string
  city: string
  pincodes: string
  delivery_charge: number
  is_cod_available: boolean
  estimated_days: number
  is_active: boolean
  created_at: string
}

export interface CreateDeliveryZoneRequest {
  name: string
  city?: string
  pincodes: string
  delivery_charge?: number
  is_cod_available?: boolean
  estimated_days?: number
}

// ---- Support Tickets ----
export interface CityStoreSummary {
  city: string
  total_stores: number
  open_stores: number
  paused_stores: number
  closed_stores: number
}

export interface ControlTowerSettings {
  platform_orders_paused: boolean
  cod_enabled: boolean
}

export interface ControlTowerOverview {
  platform_orders_paused: boolean
  total_stores: number
  paused_or_closed_stores: number
  open_support_tickets: number
  cities: CityStoreSummary[]
}

export interface LiveOperations {
  active_orders: number
  picking: number
  packed: number
  ready_for_pickup: number
  out_for_delivery: number
  active_riders: number
  delayed: number | null
  failed: number | null
}

export interface OperationsReport {
  range_from: string
  range_to: string
  total_orders: number
  delivered_orders: number
  cancelled_orders: number
  returned_orders: number
  cancellation_rate_percent: number | null
  refund_rate_percent: number | null
  avg_assignment_minutes: number | null
  avg_delivery_minutes: number | null
  assignment_sample_size: number
  delivery_sample_size: number
  sla_target_minutes: number
  avg_confirm_to_deliver_minutes: number | null
  sla_adherence_percent: number | null
  sla_sample_size: number
  delayed_orders: number
  avg_picking_minutes: number | null
  avg_packing_minutes: number | null
  picking_sample_size: number
  packing_sample_size: number
  orders_per_minute: number | null
  total_stores: number
  open_stores: number
  paused_stores: number
  closed_stores: number
  total_riders: number
  online_riders: number
}

export interface ActiveDelivery {
  order_id: number
  order_status: string
  delivery_status: string
  delivery_partner_id: number
  partner_name: string
  partner_phone: string
  customer_area: string
  assigned_at: string
}

export interface RiderWorkload {
  partner_id: number
  name: string
  phone: string
  is_online: boolean
  is_active: boolean
  active_orders: number
  max_active_orders: number
  pending_cod: number
  cod_limit: number
  cod_limit_reached: boolean
}

export interface FailedDelivery {
  order_id: number
  partner_id?: number
  partner_name?: string
  customer_area: string
  returned_at: string
}

export interface UnassignedOrder {
  order_id: number
  order_status: string
  delivery_assignment_status: string
  warehouse: string
  customer_area: string
  attempted_partner_count: number
  assignment_expiry: string | null
  created_at: string
}

export interface UnassignedOrdersResponse {
  unassigned_count: number
  orders: UnassignedOrder[]
}

export interface SupportTicket {
  id: number
  user_id: number
  order_id?: number | null
  subject: string
  status: 'open' | 'in_progress' | 'resolved' | 'closed'
  priority: 'low' | 'normal' | 'high' | 'urgent'
  issue_type?: string
  assigned_to_staff_id?: number | null
  assigned_to_staff?: { id: number; name?: string; phone?: string; role?: string } | null
  created_at: string
  updated_at: string
}

export interface SupportMessage {
  id: number
  ticket_id: number
  sender_id: number
  sender_type: 'customer' | 'admin'
  message: string
  created_at: string
}

export interface AdminPaymentRow {
  order_id: number
  transaction_id: string
  customer_name: string
  customer_phone: string
  amount: number
  refunded_amount: number
  payment_method: 'cod' | 'online'
  collected_via?: 'cash' | 'upi' | null
  gateway: string
  status: 'pending' | 'paid' | 'failed' | 'refunded' | 'partially_refunded'
  created_at: string
}

export interface AdminPaymentListResponse {
  payments: AdminPaymentRow[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface AdminPaymentDetail {
  order: any
  customer: { id: number; name: string; phone: string }
  payment: {
    id?: number
    order_id: number
    razorpay_order_id?: string
    razorpay_payment_id?: string
    amount: number
    currency: string
    status: string
    gateway: string
    refunded_amount: number
    created_at?: string
    updated_at?: string
  }
  has_payment_record: boolean
}

export interface AdminPaymentReconciliationSummary {
  total_collected: number
  total_pending: number
  total_refunded: number
  count_paid: number
  count_pending: number
  count_failed: number
  count_refunded: number
  online_collected: number
  cod_collected: number
}

export interface DashboardStats {
  total_users: number
  new_users_today: number
  total_orders: number
  orders_today: number
  total_sales: number
  revenue_today: number
  avg_order_value: number
  pending_orders: number
  confirmed_orders: number
  shipped_orders: number
  delivered_orders: number
  cancelled_orders: number
  returned_orders: number
  total_products: number
  low_stock_products: number
  out_of_stock_products: number
  active_delivery_partners: number
  total_warehouses: number
  open_support_tickets: number
  pending_payment_amount: number
}

export interface DashboardTrendPoint {
  date: string
  revenue: number
  orders: number
}

export interface DashboardUserPoint {
  date: string
  count: number
}

export interface DashboardStatusCount {
  status: string
  count: number
}

export interface DashboardPaymentSplit {
  method: string
  revenue: number
  count: number
}

export interface DashboardProductRow {
  product_id: number
  product_name: string
  units_sold: number
  total_revenue: number
}

export interface DashboardWarehouseRevenue {
  warehouse_name: string
  revenue: number
}

export interface DashboardCharts {
  sales_trend: DashboardTrendPoint[]
  user_growth: DashboardUserPoint[]
  orders_by_status: DashboardStatusCount[]
  payment_split: DashboardPaymentSplit[]
  top_products: DashboardProductRow[]
  revenue_by_warehouse: DashboardWarehouseRevenue[]
  tickets_by_status: DashboardStatusCount[]
}

export interface DashboardOverview {
  stats: DashboardStats
  charts: DashboardCharts
}

export interface InvoiceItem {
  id: number
  invoice_id: number
  product_id: number
  product_name: string
  sku?: string
  quantity: number
  price: number
}

export interface InvoiceSeller {
  company_name: string
  address: string
  gstin: string
  contact_number: string
  email: string
  state: string
  state_code: string
  fssai_number: string
}

export interface AdminInvoice {
  id?: number
  invoice_number: string
  order_id: number
  order_status: string
  customer_name: string
  customer_phone: string
  address_line1: string
  address_line2?: string
  address_city: string
  address_state: string
  address_pincode: string
  place_of_supply: string
  items_amount: number
  discount_amount: number
  delivery_charge: number
  wallet_used: number
  total_amount: number
  payment_method: string
  payment_reference?: string
  payment_status: string
  generated_at: string
  items: InvoiceItem[]
  seller: InvoiceSeller
  tax_status: string
}

export interface AdminInvoiceListResponse {
  invoices: AdminInvoiceListItem[]
  page: number
  limit: number
  total: number
  total_pages: number
}

// Shape returned by GET /admin/invoices (search/list) - raw DB rows,
// different from AdminInvoice (which is the formatted GET /admin/invoices/:id
// detail shape). Notably this has wallet_amount_used, not wallet_used, and
// no order_status/seller/tax_status since those are computed only for detail view.
export interface AdminInvoiceListItem {
  id: number
  invoice_number: string
  order_id: number
  customer_name: string
  customer_phone: string
  address_line1: string
  address_line2?: string
  address_city: string
  address_state: string
  address_pincode: string
  items_amount: number
  discount_amount: number
  delivery_charge: number
  wallet_amount_used: number
  total_amount: number
  payment_method: string
  payment_reference?: string
  generated_at: string
  created_at: string
}
