export interface User {
  id: number
  name: string
  phone: string
  role: string
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface Category {
  id: number
  name: string
  image_url: string
  created_at: string
  updated_at: string
}

export interface Inventory {
  id: number
  product_id: number
  warehouse_id: number
  stock: number
  in_stock: boolean
}

export interface Product {
  id: number
  name: string
  description: string
  price: number
  image_url: string
  category_id: number
  category?: Category
  inventories?: Inventory[]
  created_at: string
  updated_at: string
}

export interface ProductListResponse {
  products: Product[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface ProductListQuery {
  search?: string
  category_id?: number
  min_price?: number
  max_price?: number
  in_stock?: boolean
  sort?: 'price_asc' | 'price_desc' | 'name_asc' | 'name_desc' | 'newest'
  page?: number
  limit?: number
}

export interface Review {
  id: number
  user_id: number
  product_id: number
  rating: number
  comment: string
  created_at: string
  updated_at: string
}

export interface CartItem {
  id: number
  cart_id: number
  product_id: number
  product?: Product
  quantity: number
  created_at: string
  updated_at: string
}

export interface CartResponse {
  id: number
  items: CartItem[]
  total_items: number
  total_amount: number
}

export interface Address {
  id: number
  user_id: number
  label: string
  full_name: string
  phone: string
  line1: string
  line2: string
  city: string
  state: string
  pincode: string
  is_default: boolean
  lat?: number | null
  lng?: number | null
  created_at: string
  updated_at: string
}

export interface AddressRequest {
  label: string
  full_name: string
  phone: string
  line1: string
  line2?: string
  city: string
  state: string
  pincode: string
  is_default: boolean
  lat?: number | null
  lng?: number | null
}

export interface OrderItem {
  id: number
  order_id: number
  product_id: number
  product?: Product
  quantity: number
  price: number
  created_at: string
}

export interface Order {
  id: number
  user_id: number
  address_id: number
  address?: Address
  items_amount: number
  delivery_charge: number
  wallet_amount_used: number
  total_amount: number
  status: 'pending' | 'confirmed' | 'shipped' | 'delivered' | 'returned' | 'cancelled'
  payment_method: 'cod' | 'online'
  payment_status: 'pending' | 'paid' | 'failed'
  delivery_partner_id?: number
  items?: OrderItem[]
  created_at: string
  updated_at: string
}

export interface OrderTracking {
  delivery_partner_name: string
  vehicle_number: string
  current_lat: number | null
  current_lng: number | null
  last_updated: string | null
  order_status: string
}

export interface OrderListResponse {
  orders: Order[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface CheckoutRequest {
  address_id?: number
  payment_method?: 'cod' | 'online'
  coupon_code?: string
  use_wallet?: boolean
}

export interface CreatePaymentOrderResponse {
  razorpay_order_id: string
  amount: number
  currency: string
  key_id: string
  order_id: number
}

export interface VerifyPaymentRequest {
  razorpay_order_id: string
  razorpay_payment_id: string
  razorpay_signature: string
}

export interface Coupon {
  id: number
  code: string
  discount_type: 'flat' | 'percentage'
  discount_value: number
  min_order_amount: number
  max_discount_amount?: number
  usage_limit: number
  used_count: number
  expiry_date: string
  is_active: boolean
}

export interface ValidateCouponRequest {
  code: string
  order_amount: number
}

export interface Wishlist {
  id: number
  user_id: number
  product_id: number
  product?: Product
  created_at: string
}

export interface Wallet {
  id: number
  user_id: number
  balance: number
  created_at: string
  updated_at: string
}

export interface WalletTransaction {
  id: number
  wallet_id: number
  type: 'credit' | 'debit'
  amount: number
  reason: string
  reference_type?: string
  reference_id?: number
  balance_after: number
  note?: string
  created_at: string
}

export interface WalletResponse {
  balance: number
  transactions: WalletTransaction[]
}

export interface ReturnRequestItemBody {
  order_item_id: number
  quantity: number
}

export interface ReturnRequestBody {
  reason: string
  items: ReturnRequestItemBody[]
}

export interface ReturnRequest {
  id: number
  order_id: number
  order?: Order
  user_id: number
  reason: string
  status: 'pending' | 'approved' | 'rejected'
  refund_amount: number
  created_at: string
  updated_at: string
}

export interface SupportTicket {
  id: number
  user_id: number
  order_id?: number | null
  subject: string
  status: 'open' | 'in_progress' | 'resolved' | 'closed'
  priority: 'low' | 'normal' | 'high' | 'urgent'
  issue_type: string
  assigned_to_staff_id?: number | null
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

export interface CreateTicketRequest {
  order_id?: number | null
  subject: string
  message: string
  issue_type?: string
}

export interface ReplyRequest {
  message: string
}
