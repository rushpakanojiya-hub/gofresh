export interface WarehouseRevenue {
  warehouse_id: number | null
  warehouse_name: string
  revenue: number
  order_count: number
}

export interface ProductRevenue {
  product_id: number
  product_name: string
  revenue: number
  quantity: number
}

export interface RevenueSummary {
  from: string
  to: string
  gross_sales: number
  net_sales: number
  discounts: number
  delivery_charge: number
  platform_fee: number
  order_count: number
  by_warehouse: WarehouseRevenue[]
  by_product: ProductRevenue[]
}

export interface Expense {
  id: number
  amount: number
  category: string
  expense_date: string
  warehouse_id?: number | null
  warehouse?: { id: number; name: string }
  note?: string
  receipt_url?: string
  added_by_id: number
  created_at: string
  updated_at: string
}

export const EXPENSE_CATEGORIES = [
  'rent',
  'utilities',
  'salaries',
  'packaging',
  'marketing',
  'maintenance',
  'misc',
] as const

export interface ExpenseListResponse {
  expenses: Expense[]
  page: number
  limit: number
  total: number
  total_pages: number
  total_amount: number
}

export interface ExpenseFormInput {
  amount: number
  category: string
  expense_date: string
  warehouse_id?: number | null
  note?: string
  receipt_url?: string
}

export interface Payroll {
  id: number
  staff_id: number
  staff?: { id: number; name: string; phone: string; warehouse_id: number }
  amount: number
  month: number
  year: number
  status: string
  payment_method?: string
  note?: string
  paid_by_id?: number | null
  paid_at?: string | null
  created_at: string
  updated_at: string
}

export const PAYROLL_PAYMENT_METHODS = ['cash', 'bank', 'upi'] as const

export interface PayrollListResponse {
  payroll: Payroll[]
  page: number
  limit: number
  total: number
  total_pages: number
  total_pending: number
  total_paid: number
}

export interface PayrollFormInput {
  staff_id: number
  amount: number
  month: number
  year: number
  status?: string
  payment_method?: string
  note?: string
}

export interface ProfitLoss {
  from: string
  to: string
  revenue: number
  cogs: number
  cost_price_coverage: number
  gross_profit: number
  operating_expenses: number
  ebitda: number
  net_profit: number
}

export interface PaymentReconciliation {
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

export interface GSTByHSN {
  hsn_code: string
  taxable_amount: number
  gst_amount: number
  quantity: number
}

export interface GSTByRate {
  gst_percent: number
  taxable_amount: number
  gst_amount: number
}

export interface GSTSummary {
  from: string
  to: string
  taxable_amount: number
  cgst_amount: number
  sgst_amount: number
  igst_amount: number
  total_gst: number
  invoice_count: number
  by_hsn: GSTByHSN[]
  by_rate: GSTByRate[]
}

export interface Vendor {
  id: number
  name: string
  contact_name?: string
  phone?: string
  email?: string
  gstin?: string
  address?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface VendorRequest {
  name: string
  contact_name?: string
  phone?: string
  email?: string
  gstin?: string
  address?: string
  is_active?: boolean
}

export interface VendorBill {
  id: number
  vendor_id: number
  vendor?: Vendor
  bill_number?: string
  amount: number
  gst_amount: number
  amount_paid: number
  hold_status: string
  hold_reason?: string
  voided_at?: string
  void_reason?: string
  voided_by_id?: number
  bill_date: string
  due_date?: string
  note?: string
  created_by_id: number
  created_at: string
  updated_at: string
  status: 'unpaid' | 'partially_paid' | 'paid' | 'on_hold' | 'disputed' | 'voided'
}

export interface VendorBillRequest {
  vendor_id: number
  bill_number?: string
  amount: number
  gst_amount?: number
  bill_date: string
  due_date?: string
  note?: string
}

export interface Account {
  id: number
  code: string
  name: string
  type: 'asset' | 'liability' | 'equity' | 'revenue' | 'expense'
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface AccountRequest {
  code: string
  name: string
  type: string
  is_active?: boolean
}

export interface LedgerEntry {
  id: number
  transaction_ref: string
  account_id: number
  account?: Account
  type: 'debit' | 'credit'
  amount: number
  description?: string
  reference_type?: string
  reference_id?: number
  entry_date: string
  created_by_id: number
  created_at: string
}

export interface LedgerEntryLine {
  account_id: number
  type: 'debit' | 'credit'
  amount: number
  description?: string
}

export interface ManualJournalEntryRequest {
  entry_date: string
  lines: LedgerEntryLine[]
}

export interface TrialBalanceRow {
  account_id: number
  account_code: string
  account_name: string
  account_type: string
  total_debit: number
  total_credit: number
}

export interface TrialBalance {
  as_of: string
  accounts: TrialBalanceRow[]
  total_debit: number
  total_credit: number
  is_balanced: boolean
}

export interface BankTransaction {
  id: number
  transaction_date: string
  description?: string
  amount: number
  reference_number?: string
  status: 'unmatched' | 'matched' | 'ignored' | 'voided'
  matched_type?: string
  matched_id?: number
  matched_at?: string
  matched_by_id?: number
  note?: string
  voided_at?: string
  void_reason?: string
  voided_by_id?: number
  created_by_id: number
  created_at: string
  updated_at: string
}

export interface BankTransactionRequest {
  transaction_date: string
  description?: string
  amount: number
  reference_number?: string
}

export interface BankTransactionMatchRequest {
  matched_type: string
  matched_id?: number
  note?: string
}

export interface VendorBillHoldRequest {
  reason: string
}

export interface VendorBillVoidRequest {
  reason: string
}

export interface BankTransactionVoidRequest {
  reason: string
}

export interface FinanceDashboard {
  period_start: string
  revenue: {
    total_revenue: number
    cogs: number
    gross_profit: number
    expenses: number
    net_profit: number
  }
  accounts_payable: {
    vendor_payable: number
  }
  accounts_receivable: {
    gateway_pending: number
    cod_pending: number
  }
  gst: {
    output_gst: number
    vendor_gst: number
    net_gst_payable: number
  }
  bank_balance: number
  pending_approvals: {
    expenses: number
    journal_entries: number
    bank_changes: number
  }
}

export interface AdminPaymentRow {
  order_id: number
  transaction_id: string
  customer_name: string
  customer_phone: string
  amount: number
  refunded_amount: number
  payment_method: string
  gateway: string
  status: string
  created_at: string
}

export interface RiderPayout {
  id: number
  delivery_partner_id: number
  period_from: string
  period_to: string
  delivered_count: number
  amount: number
  status: string
  approved_by_id?: number
  approved_at?: string
  paid_at?: string
  created_by_id: number
  created_at: string
}

export interface RiderCODDeposit {
  id: number
  delivery_partner_id: number
  amount: number
  deposit_date: string
  status: string
  note?: string
  verified_by_id?: number
  verified_at?: string
  created_by_id: number
  created_at: string
}

// ---- Weekly MIS ----

export interface MISRow {
  label: string
  current: number
  previous: number
  growth_pct: number
}

export interface MISManualEntry {
  id: number
  row_key: string
  data: Record<string, any>
}

export interface MISExpenseApproval {
  id: number
  category: string
  up_to_25k: string
  range_25k_1l: string
  range_1l_5l: string
  above_5l: string
  required_documents: string
  approver: string
}

export interface VendorSettlementRow {
  vendor_id: number
  vendor_name: string
  gross_sales: number
  commission: number
  discount: number
  returns: number
  delivery_recovery: number
  other_charges: number
  net_payable: number
  amount_paid: number
  balance: number
  status: string
}

export interface WeeklyMIS {
  week_start: string
  week_end: string
  prev_week_start: string
  prev_week_end: string
  revenue_mis: MISRow[]
  vendor_expense_mis: MISRow[]
  vendor_settlement: VendorSettlementRow[]
  revenue_by_vendor: MISManualEntry[]
  vendor_pl: MISManualEntry[]
  vendor_reconciliation: MISManualEntry[]
  expense_approval: MISExpenseApproval[]
}

export interface VendorBankChangeRequest {
  id: number
  vendor_id: number
  vendor?: Vendor
  new_account_holder: string
  new_account_number: string
  new_ifsc: string
  status: 'pending' | 'approved' | 'rejected'
  requested_by_id: number
  approved_by_id?: number
  approved_at?: string
  rejection_reason?: string
  created_at: string
}

export interface VendorBankChangeRequestBody {
  account_holder: string
  account_number: string
  ifsc: string
}
