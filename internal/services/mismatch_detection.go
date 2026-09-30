package services

import (
"fmt"
"time"

"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// DetectMismatches runs every mismatch check and returns the combined,
// live result set. Nothing here is persisted - each call re-queries
// Orders/Payments/LedgerEntry/BankTransaction fresh, so results always
// reflect the current state of the data. Dismissed items are still
// returned (IsDismissed=true) so the UI can show/hide them without a
// second round trip; callers that only want open items should filter
// client-side or pass dismissed=false to the handler.
func DetectMismatches() ([]models.MismatchItem, error) {
var items []models.MismatchItem

checks := []func() ([]models.MismatchItem, error){
detectOrderPaymentAmountMismatch,
detectSaleNotPostedToLedger,
detectRefundLedgerMismatch,
detectBankUnmatched,
detectDuplicatePaymentRef,
detectRefundExceedsPayment,
}

for _, check := range checks {
found, err := check()
if err != nil {
return nil, err
}
items = append(items, found...)
}

if err := attachDismissalStatus(items); err != nil {
return nil, err
}

return items, nil
}

// attachDismissalStatus mutates items in place, setting IsDismissed=true
// for any (CheckType, EntityID) pair that has a matching MismatchDismissal.
func attachDismissalStatus(items []models.MismatchItem) error {
if len(items) == 0 {
return nil
}
var dismissals []models.MismatchDismissal
if err := database.DB.Find(&dismissals).Error; err != nil {
return fmt.Errorf("failed to load dismissals: %w", err)
}
dismissed := make(map[string]bool, len(dismissals))
for _, d := range dismissals {
dismissed[fmt.Sprintf("%s-%d", d.CheckType, d.EntityID)] = true
}
for i := range items {
key := fmt.Sprintf("%s-%d", items[i].CheckType, items[i].EntityID)
items[i].IsDismissed = dismissed[key]
}
return nil
}

// detectOrderPaymentAmountMismatch finds online orders whose Payment.Amount
// does not equal Order.TotalAmount.
func detectOrderPaymentAmountMismatch() ([]models.MismatchItem, error) {
type row struct {
OrderID     uint
TotalAmount float64
PaymentID   uint
PayAmount   float64
}
var rows []row
err := database.DB.Table("orders o").
Select("o.id as order_id, o.total_amount, p.id as payment_id, p.amount as pay_amount").
Joins("JOIN payments p ON p.order_id = o.id").
Where("o.payment_method = ? AND ABS(o.total_amount - p.amount) > 0.01", "online").
Scan(&rows).Error
if err != nil {
return nil, fmt.Errorf("order-payment amount check failed: %w", err)
}

now := time.Now()
items := make([]models.MismatchItem, 0, len(rows))
for _, r := range rows {
orderID := r.OrderID
items = append(items, models.MismatchItem{
CheckType:      "order_payment_amount",
Severity:       "high",
EntityType:     "payment",
EntityID:       r.PaymentID,
OrderID:        &orderID,
Description:    fmt.Sprintf("Order #%d total is %.2f but linked payment is %.2f", r.OrderID, r.TotalAmount, r.PayAmount),
DetectedAmount: r.TotalAmount - r.PayAmount,
DetectedAt:     now,
})
}
return items, nil
}

// detectSaleNotPostedToLedger finds orders that should have revenue
// recognized (online+paid, or delivered) but have no LedgerEntry with
// reference_type="sale" for that order - mirrors the trigger points in
// PostSalesLedgerEntry (payment verification / delivery confirmation).
func detectSaleNotPostedToLedger() ([]models.MismatchItem, error) {
type row struct {
OrderID     uint
TotalAmount float64
Status      string
}
var rows []row
err := database.DB.Table("orders o").
Select("o.id as order_id, o.total_amount, o.status").
Where(`
(
  (o.payment_method = 'online' AND o.payment_status = 'paid')
  OR (o.payment_method = 'cod' AND o.status = 'delivered')
)
AND NOT EXISTS (
  SELECT 1 FROM ledger_entries le
  WHERE le.reference_type = 'sale' AND le.reference_id = o.id
)
`).
Scan(&rows).Error
if err != nil {
return nil, fmt.Errorf("sale-not-posted check failed: %w", err)
}

now := time.Now()
items := make([]models.MismatchItem, 0, len(rows))
for _, r := range rows {
orderID := r.OrderID
items = append(items, models.MismatchItem{
CheckType:      "sale_not_posted",
Severity:       "medium",
EntityType:     "order",
EntityID:       r.OrderID,
OrderID:        &orderID,
Description:    fmt.Sprintf("Order #%d (%s, total %.2f) has no sale ledger entry", r.OrderID, r.Status, r.TotalAmount),
DetectedAmount: r.TotalAmount,
DetectedAt:     now,
})
}
return items, nil
}

// detectRefundLedgerMismatch finds payments where RefundedAmount doesn't
// match the sum of "refund"-type debit ledger entries for that order -
// PostRefundLedgerEntry/PostWalletRefundLedgerEntry key their entries on
// reference_type="refund", reference_id=orderID (not payment ID).
func detectRefundLedgerMismatch() ([]models.MismatchItem, error) {
type row struct {
PaymentID      uint
OrderID        uint
RefundedAmount float64
LedgerTotal    float64
}
var rows []row
err := database.DB.Table("payments p").
Select(`p.id as payment_id, p.order_id, p.refunded_amount,
COALESCE((SELECT SUM(le.amount) FROM ledger_entries le
  WHERE le.reference_type = 'refund' AND le.reference_id = p.order_id AND le.type = 'debit'), 0) as ledger_total`).
Where("p.refunded_amount > 0").
Scan(&rows).Error
if err != nil {
return nil, fmt.Errorf("refund-ledger check failed: %w", err)
}

now := time.Now()
items := make([]models.MismatchItem, 0)
for _, r := range rows {
if diff := r.RefundedAmount - r.LedgerTotal; diff > 0.01 || diff < -0.01 {
orderID := r.OrderID
items = append(items, models.MismatchItem{
CheckType:      "refund_ledger_mismatch",
Severity:       "high",
EntityType:     "payment",
EntityID:       r.PaymentID,
OrderID:        &orderID,
Description:    fmt.Sprintf("Order #%d payment shows %.2f refunded but ledger has %.2f in refund entries", r.OrderID, r.RefundedAmount, r.LedgerTotal),
DetectedAmount: r.RefundedAmount - r.LedgerTotal,
DetectedAt:     now,
})
}
}
return items, nil
}

// detectBankUnmatched surfaces BankTransaction rows still sitting at
// status="unmatched" - already tracked by the bank reconciliation screen,
// but pulled into the Mismatch Center too for a single unified view.
func detectBankUnmatched() ([]models.MismatchItem, error) {
var transactions []models.BankTransaction
if err := database.DB.Where("status = ?", "unmatched").Find(&transactions).Error; err != nil {
return nil, fmt.Errorf("bank-unmatched check failed: %w", err)
}

now := time.Now()
items := make([]models.MismatchItem, 0, len(transactions))
for _, t := range transactions {
items = append(items, models.MismatchItem{
CheckType:      "bank_unmatched",
Severity:       "low",
EntityType:     "bank_transaction",
EntityID:       t.ID,
Description:    fmt.Sprintf("Bank transaction on %s for %.2f (%s) is unmatched", t.TransactionDate.Format("2006-01-02"), t.Amount, t.Description),
DetectedAmount: t.Amount,
DetectedAt:     now,
})
}
return items, nil
}

// detectDuplicatePaymentRef finds RazorpayPaymentID values that appear on
// more than one Payment row - should never happen if the gateway is
// behaving, so this is a data-integrity guard rather than an expected
// reconciliation scenario.
func detectDuplicatePaymentRef() ([]models.MismatchItem, error) {
type row struct {
RazorpayPaymentID string
PaymentIDs        string // comma-joined, since GORM Scan can't give us []uint directly from string_agg
Cnt               int
}
var rows []row
err := database.DB.Table("payments").
Select("razorpay_payment_id, string_agg(id::text, ',') as payment_ids, count(*) as cnt").
Where("razorpay_payment_id IS NOT NULL AND razorpay_payment_id != ''").
Group("razorpay_payment_id").
Having("count(*) > 1").
Scan(&rows).Error
if err != nil {
return nil, fmt.Errorf("duplicate-payment-ref check failed: %w", err)
}

now := time.Now()
items := make([]models.MismatchItem, 0, len(rows))
for _, r := range rows {
items = append(items, models.MismatchItem{
CheckType:      "duplicate_payment_ref",
Severity:       "high",
EntityType:     "payment",
EntityID:       0,
Description:    fmt.Sprintf("Razorpay payment ID %s appears on %d payment rows (ids: %s)", r.RazorpayPaymentID, r.Cnt, r.PaymentIDs),
DetectedAmount: 0,
DetectedAt:     now,
})
}
return items, nil
}

// detectRefundExceedsPayment is a data-integrity guard: RefundedAmount
// should never exceed Amount (UpdateAdminPaymentStatus already rejects
// this going forward), but this surfaces any pre-existing bad data.
func detectRefundExceedsPayment() ([]models.MismatchItem, error) {
type row struct {
PaymentID      uint
OrderID        uint
Amount         float64
RefundedAmount float64
}
var rows []row
err := database.DB.Table("payments").
Select("id as payment_id, order_id, amount, refunded_amount").
Where("refunded_amount > amount").
Scan(&rows).Error
if err != nil {
return nil, fmt.Errorf("refund-exceeds-payment check failed: %w", err)
}

now := time.Now()
items := make([]models.MismatchItem, 0, len(rows))
for _, r := range rows {
orderID := r.OrderID
items = append(items, models.MismatchItem{
CheckType:      "refund_exceeds_payment",
Severity:       "high",
EntityType:     "payment",
EntityID:       r.PaymentID,
OrderID:        &orderID,
Description:    fmt.Sprintf("Order #%d payment amount %.2f but refunded_amount %.2f exceeds it", r.OrderID, r.Amount, r.RefundedAmount),
DetectedAmount: r.RefundedAmount - r.Amount,
DetectedAt:     now,
})
}
return items, nil
}