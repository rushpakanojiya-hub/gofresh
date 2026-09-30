import apiClient from './client'
import type { RevenueSummary, Expense, ExpenseListResponse, ExpenseFormInput, Payroll, PayrollListResponse, PayrollFormInput, ProfitLoss, PaymentReconciliation, GSTSummary, Vendor, VendorRequest, VendorBill, VendorBillRequest, VendorBillHoldRequest, VendorBillVoidRequest, Account, AccountRequest, LedgerEntry, ManualJournalEntryRequest, TrialBalance, BankTransaction, BankTransactionRequest, BankTransactionMatchRequest, BankTransactionVoidRequest, FinanceDashboard, VendorBankChangeRequest, VendorBankChangeRequestBody } from '../types/finance'

export async function getRevenue(from: string, to: string): Promise<RevenueSummary> {
  const { data } = await apiClient.get<RevenueSummary>('/admin/finance/revenue', {
    params: { from, to },
  })
  return data
}

export async function listExpenses(params: {
  category?: string
  warehouse_id?: number
  from?: string
  to?: string
  page?: number
  limit?: number
}): Promise<ExpenseListResponse> {
  const { data } = await apiClient.get<ExpenseListResponse>('/admin/finance/expenses', { params })
  return data
}

export async function createExpense(body: ExpenseFormInput): Promise<Expense> {
  const { data } = await apiClient.post<Expense>('/admin/finance/expenses', body)
  return data
}

export async function updateExpense(id: number, body: ExpenseFormInput): Promise<Expense> {
  const { data } = await apiClient.put<Expense>(`/admin/finance/expenses/${id}`, body)
  return data
}

export async function deleteExpense(id: number): Promise<void> {
  await apiClient.delete(`/admin/finance/expenses/${id}`)
}

export async function listPayroll(params: {
  staff_id?: number
  status?: string
  month?: number
  year?: number
  page?: number
  limit?: number
}): Promise<PayrollListResponse> {
  const { data } = await apiClient.get<PayrollListResponse>('/admin/finance/payroll', { params })
  return data
}

export async function createPayroll(body: PayrollFormInput): Promise<Payroll> {
  const { data } = await apiClient.post<Payroll>('/admin/finance/payroll', body)
  return data
}

export async function updatePayroll(id: number, body: PayrollFormInput): Promise<Payroll> {
  const { data } = await apiClient.put<Payroll>(`/admin/finance/payroll/${id}`, body)
  return data
}

export async function deletePayroll(id: number): Promise<void> {
  await apiClient.delete(`/admin/finance/payroll/${id}`)
}



export async function getProfitLoss(from: string, to: string): Promise<ProfitLoss> {
  const { data } = await apiClient.get<ProfitLoss>('/admin/finance/profit-loss', {
    params: { from, to },
  })
  return data
}


export async function getPaymentReconciliation(dateFrom: string, dateTo: string): Promise<PaymentReconciliation> {
  const { data } = await apiClient.get<PaymentReconciliation>('/admin/payments/reconciliation', {
    params: { date_from: dateFrom, date_to: dateTo },
  })
  return data
}


export async function getGSTSummary(from: string, to: string): Promise<GSTSummary> {
  const { data } = await apiClient.get<GSTSummary>('/admin/finance/gst', {
    params: { from, to },
  })
  return data
}


// ---- Vendors ----

export async function getVendors(isActive?: boolean): Promise<{ vendors: Vendor[] }> {
  const { data } = await apiClient.get('/admin/finance/vendors', {
    params: isActive === undefined ? {} : { is_active: isActive },
  })
  return data
}

export async function createVendor(payload: VendorRequest): Promise<Vendor> {
  const { data } = await apiClient.post<Vendor>('/admin/finance/vendors', payload)
  return data
}

export async function updateVendor(id: number, payload: VendorRequest): Promise<Vendor> {
  const { data } = await apiClient.put<Vendor>(`/admin/finance/vendors/${id}`, payload)
  return data
}

export async function deleteVendor(id: number): Promise<void> {
  await apiClient.delete(`/admin/finance/vendors/${id}`)
}

// ---- Vendor Bills ----

export async function getVendorBills(params: {
  vendor_id?: number
  status?: string
  page?: number
}): Promise<{ bills: VendorBill[]; total: number; total_pages: number; total_outstanding: number }> {
  const { data } = await apiClient.get('/admin/finance/vendor-bills', { params })
  return data
}

export async function createVendorBill(payload: VendorBillRequest): Promise<VendorBill> {
  const { data } = await apiClient.post<VendorBill>('/admin/finance/vendor-bills', payload)
  return data
}

export async function payVendorBill(id: number, amount: number): Promise<VendorBill> {
  const { data } = await apiClient.post<VendorBill>(`/admin/finance/vendor-bills/${id}/pay`, { amount })
  return data
}

export async function voidVendorBill(id: number, payload: VendorBillVoidRequest): Promise<VendorBill> {
  const { data } = await apiClient.post<VendorBill>(`/admin/finance/vendor-bills/${id}/void`, payload)
  return data
}

export async function holdVendorBill(id: number, payload: VendorBillHoldRequest): Promise<VendorBill> {
  const { data } = await apiClient.post<VendorBill>(`/admin/finance/vendor-bills/${id}/hold`, payload)
  return data
}

export async function disputeVendorBill(id: number, payload: VendorBillHoldRequest): Promise<VendorBill> {
  const { data } = await apiClient.post<VendorBill>(`/admin/finance/vendor-bills/${id}/dispute`, payload)
  return data
}

export async function releaseHoldVendorBill(id: number): Promise<VendorBill> {
  const { data } = await apiClient.post<VendorBill>(`/admin/finance/vendor-bills/${id}/release-hold`, {})
  return data
}

// ---- Chart of Accounts ----

export async function getAccounts(type?: string): Promise<{ accounts: Account[] }> {
  const { data } = await apiClient.get('/admin/finance/accounts', {
    params: type ? { type } : {},
  })
  return data
}

export async function createAccount(payload: AccountRequest): Promise<Account> {
  const { data } = await apiClient.post<Account>('/admin/finance/accounts', payload)
  return data
}

export async function updateAccount(id: number, payload: AccountRequest): Promise<Account> {
  const { data } = await apiClient.put<Account>(`/admin/finance/accounts/${id}`, payload)
  return data
}

// ---- Ledger ----

export async function getLedgerEntries(params: {
  account_id?: number
  from?: string
  to?: string
  page?: number
}): Promise<{ entries: LedgerEntry[]; total: number; total_pages: number }> {
  const { data } = await apiClient.get('/admin/finance/ledger', { params })
  return data
}

export async function createManualJournalEntry(
  payload: ManualJournalEntryRequest
): Promise<{ transaction_ref: string; entries: LedgerEntry[] }> {
  const { data } = await apiClient.post('/admin/finance/ledger', payload)
  return data
}

export async function getTrialBalance(asOf?: string): Promise<TrialBalance> {
  const { data } = await apiClient.get<TrialBalance>('/admin/finance/ledger/trial-balance', {
    params: asOf ? { as_of: asOf } : {},
  })
  return data
}

// ---- Bank Transactions ----

export async function getBankTransactions(params: {
  status?: string
  from?: string
  to?: string
  page?: number
}): Promise<{ transactions: BankTransaction[]; total: number; total_pages: number; unmatched_count: number }> {
  const { data } = await apiClient.get('/admin/finance/bank-transactions', { params })
  return data
}

export async function createBankTransaction(payload: BankTransactionRequest): Promise<BankTransaction> {
  const { data } = await apiClient.post<BankTransaction>('/admin/finance/bank-transactions', payload)
  return data
}

export async function matchBankTransaction(
  id: number,
  payload: BankTransactionMatchRequest
): Promise<BankTransaction> {
  const { data } = await apiClient.post<BankTransaction>(`/admin/finance/bank-transactions/${id}/match`, payload)
  return data
}

export async function ignoreBankTransaction(id: number): Promise<BankTransaction> {
  const { data } = await apiClient.post<BankTransaction>(`/admin/finance/bank-transactions/${id}/ignore`)
  return data
}

export async function voidBankTransaction(id: number, payload: BankTransactionVoidRequest): Promise<BankTransaction> {
  const { data } = await apiClient.post<BankTransaction>(`/admin/finance/bank-transactions/${id}/void`, payload)
  return data
}

export async function getFinanceDashboard(): Promise<FinanceDashboard> {
  const { data } = await apiClient.get<FinanceDashboard>('/admin/finance/dashboard')
  return data
}

export async function listAdminPayments(params: { status?: string; gateway?: string; payment_method?: string; page?: number; limit?: number }): Promise<{ payments: import('../types/finance').AdminPaymentRow[]; total: number }> {
  const { data } = await apiClient.get('/admin/payments', { params })
  return data
}

export async function getAdminPaymentDetail(orderId: number): Promise<{ payment: { id: number; is_settled: boolean } }> {
  const { data } = await apiClient.get(`/admin/payments/${orderId}`)
  return data
}

export async function settleGatewayPayment(paymentId: number): Promise<any> {
  const { data } = await apiClient.post(`/admin/payments/${paymentId}/settle-gateway`)
  return data
}

export async function listRiderPayouts(params: { status?: string; delivery_partner_id?: number }): Promise<{ rider_payouts: import('../types/finance').RiderPayout[] }> {
  const { data } = await apiClient.get('/admin/finance/rider-payouts', { params })
  return data
}

export async function createRiderPayout(payload: { delivery_partner_id: number; period_from: string; period_to: string }): Promise<import('../types/finance').RiderPayout> {
  const { data } = await apiClient.post('/admin/finance/rider-payouts', payload)
  return data
}

export async function approveRiderPayout(id: number): Promise<import('../types/finance').RiderPayout> {
  const { data } = await apiClient.post(`/admin/finance/rider-payouts/${id}/approve`)
  return data
}

export async function payRiderPayout(id: number): Promise<import('../types/finance').RiderPayout> {
  const { data } = await apiClient.post(`/admin/finance/rider-payouts/${id}/pay`)
  return data
}

export async function listRiderCODDeposits(params: { status?: string; delivery_partner_id?: number }): Promise<{ rider_cod_deposits: import('../types/finance').RiderCODDeposit[] }> {
  const { data } = await apiClient.get('/admin/finance/rider-cod-deposits', { params })
  return data
}

export async function createRiderCODDeposit(payload: { delivery_partner_id: number; amount: number; deposit_date: string; note?: string }): Promise<import('../types/finance').RiderCODDeposit> {
  const { data } = await apiClient.post('/admin/finance/rider-cod-deposits', payload)
  return data
}

export async function verifyRiderCODDeposit(id: number): Promise<import('../types/finance').RiderCODDeposit> {
  const { data } = await apiClient.post(`/admin/finance/rider-cod-deposits/${id}/verify`)
  return data
}

// ---- Weekly MIS ----

export async function getWeeklyMIS(weekStart?: string): Promise<import('../types/finance').WeeklyMIS> {
  const { data } = await apiClient.get('/admin/finance/mis', {
    params: weekStart ? { week_start: weekStart } : {},
  })
  return data
}

export async function upsertMISManualEntry(payload: {
  id?: number
  sheet: string
  week_start: string
  row_key: string
  data: Record<string, any>
}): Promise<{ id: number }> {
  const { data } = await apiClient.post('/admin/finance/mis/manual-entry', payload)
  return data
}

export async function deleteMISManualEntry(id: number): Promise<void> {
  await apiClient.delete(`/admin/finance/mis/manual-entry/${id}`)
}

export async function updateMISExpenseApproval(
  id: number,
  payload: {
    category: string
    up_to_25k: string
    range_25k_1l: string
    range_1l_5l: string
    above_5l: string
    required_documents: string
    approver: string
  }
): Promise<{ updated: boolean }> {
  const { data } = await apiClient.put(`/admin/finance/mis/expense-approval/${id}`, payload)
  return data
}

export async function exportWeeklyMIS(weekStart: string): Promise<Blob> {
  const { data } = await apiClient.get('/admin/finance/mis/weekly-template/export', {
    params: { week_start: weekStart },
    responseType: 'blob',
  })
  return data
}

export async function exportMonthlyMIS(month: string): Promise<Blob> {
  const { data } = await apiClient.get('/admin/finance/mis/monthly/export', {
    params: { month },
    responseType: 'blob',
  })
  return data
}

// ---- Vendor Bank Change Requests ----

export async function listVendorBankChangeRequests(params: {
  status?: string
  vendor_id?: number
}): Promise<{ bank_change_requests: VendorBankChangeRequest[] }> {
  const { data } = await apiClient.get('/admin/finance/vendor-bank-change-requests', { params })
  return data
}

export async function requestVendorBankChange(
  vendorId: number,
  payload: VendorBankChangeRequestBody
): Promise<VendorBankChangeRequest> {
  const { data } = await apiClient.post(`/admin/finance/vendors/${vendorId}/bank-change-request`, payload)
  return data
}

export async function approveVendorBankChange(id: number): Promise<{ vendor: any; bank_change_request: VendorBankChangeRequest }> {
  const { data } = await apiClient.post(`/admin/finance/vendor-bank-change-requests/${id}/approve`)
  return data
}

export async function rejectVendorBankChange(id: number, reason: string): Promise<VendorBankChangeRequest> {
  const { data } = await apiClient.post(`/admin/finance/vendor-bank-change-requests/${id}/reject`, { reason })
  return data
}