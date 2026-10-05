package services

import (
"fmt"
"time"

"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"gorm.io/gorm"
)

// PostSalesLedgerEntry records the double-entry ledger lines for one sale,
// triggered at the exact moment revenue is recognized:
//   - Online: right after payment verification (payment_handler.go)
//   - COD:    right after delivery confirmation (ConfirmDelivery)
// It is idempotent per order (checked via reference_type="sale",
// reference_id=orderID) so a retry or duplicate call never double-posts.
//
// Balanced entry:
//   Debit  Bank/Cash (order total actually collected)
//   Debit  Customer Wallet Liability (wallet amount redeemed)
//   Debit  Discount Given (discount amount)
//   Credit Product Sales (taxable amount)
//   Credit GST Payable (CGST+SGST+IGST)
// Debit total = TotalAmount + WalletUsed + DiscountAmount
// Credit total = TaxableAmount + totalGST
// These are equal because TaxableAmount + GST = ItemsAmount + DeliveryCharge
// + PlatformFee = TotalAmount + WalletUsed + DiscountAmount (invoice math).
func PostSalesLedgerEntry(orderID uint) error {
var invoice models.Invoice
if err := database.DB.Where("order_id = ?", orderID).First(&invoice).Error; err != nil {
return fmt.Errorf("no invoice found for order %d: %w", orderID, err)
}

// Idempotency check.
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "sale", orderID).First(&existing).Error; err == nil {
return nil
}

var order models.Order
if err := database.DB.First(&order, orderID).Error; err != nil {
return fmt.Errorf("order not found: %w", err)
}

cashOrBankCode := "1002" // Bank
if order.PaymentMethod == models.PaymentMethodCOD && !(order.CollectedVia != nil && *order.CollectedVia == "upi") {
cashOrBankCode = "1001" // Cash
}

totalGST := invoice.CGSTAmount + invoice.SGSTAmount + invoice.IGSTAmount
transactionRef := fmt.Sprintf("SALE-%d", orderID)

type line struct {
code   string
lType  string
amount float64
desc   string
}
var lines []line

lines = append(lines, line{cashOrBankCode, "debit", invoice.TotalAmount, fmt.Sprintf("Sale for order #%d", orderID)})
if invoice.WalletUsed > 0 {
lines = append(lines, line{"2005", "debit", invoice.WalletUsed, fmt.Sprintf("Wallet redeemed on order #%d", orderID)})
}
if invoice.DiscountAmount > 0 {
lines = append(lines, line{"5002", "debit", invoice.DiscountAmount, fmt.Sprintf("Discount on order #%d", orderID)})
}
lines = append(lines, line{"4001", "credit", invoice.TaxableAmount, fmt.Sprintf("Sale for order #%d", orderID)})
if totalGST > 0 {
lines = append(lines, line{"2002", "credit", totalGST, fmt.Sprintf("GST on order #%d", orderID)})
}
var orderItems []models.OrderItem
database.DB.Preload("Product").Where("order_id = ?", orderID).Find(&orderItems)
var totalCOGS float64
for _, it := range orderItems {
if it.Product.CostPrice > 0 {
totalCOGS += it.Product.CostPrice * float64(it.Quantity)
}
}
if totalCOGS > 0 {
lines = append(lines, line{"5001", "debit", totalCOGS, fmt.Sprintf("COGS for order #%d", orderID)})
lines = append(lines, line{"1004", "credit", totalCOGS, fmt.Sprintf("Inventory reduction for order #%d", orderID)})
}

return database.DB.Transaction(func(tx *gorm.DB) error {
for _, l := range lines {
if l.amount <= 0 {
continue
}
var account models.Account
if err := tx.Where("code = ?", l.code).First(&account).Error; err != nil {
return fmt.Errorf("chart of accounts missing code %s: %w", l.code, err)
}
entry := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      account.ID,
Type:           l.lType,
Amount:         l.amount,
Description:    l.desc,
ReferenceType:  "sale",
ReferenceID:    &orderID,
EntryDate:      invoice.GeneratedAt,
CreatedByID:    nil, // system-generated, same convention as invoice audit log
}
if err := tx.Create(&entry).Error; err != nil {
return fmt.Errorf("failed to create ledger entry: %w", err)
}
}
return nil
})
}

// PostVendorPaymentLedgerEntry records the double-entry ledger lines for one
// vendor bill payment: Debit Vendor Payable, Credit Bank. Each call to
// PayVendorBill can record a partial payment, so this posts once per
// payment (not once per bill) - the transaction ref includes a timestamp
// so multiple payments against the same bill each get their own entry.
func PostVendorPaymentLedgerEntry(billID uint, amount float64) error {
if amount <= 0 {
return nil
}

var vendorPayable, bank models.Account
if err := database.DB.Where("code = ?", "2001").First(&vendorPayable).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2001 (Vendor Payable): %w", err)
}
if err := database.DB.Where("code = ?", "1002").First(&bank).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1002 (Bank): %w", err)
}

transactionRef := fmt.Sprintf("VBILLPAY-%d-%d", billID, time.Now().UnixNano())
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      vendorPayable.ID,
Type:           "debit",
Amount:         amount,
Description:    fmt.Sprintf("Payment against vendor bill #%d", billID),
ReferenceType:  "vendor_bill_payment",
ReferenceID:    &billID,
EntryDate:      now,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create debit ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      bank.ID,
Type:           "credit",
Amount:         amount,
Description:    fmt.Sprintf("Payment against vendor bill #%d", billID),
ReferenceType:  "vendor_bill_payment",
ReferenceID:    &billID,
EntryDate:      now,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create credit ledger entry: %w", err)
}
return nil
})
}


// PostVendorBillLedgerEntry records the double-entry ledger lines for one
// vendor bill at creation time: Debit Inventory for the goods-value portion,
// Debit GST Input Credit (ITC) for the GST portion (claiming the input
// credit on this purchase), and Credit Vendor Payable for the full amount
// owed (amount + GST). This is the mirror image of PostDebitNoteLedgerEntry,
// which reverses these same three legs when goods are returned to a vendor.
//
// Idempotent: if an entry already exists for this bill, it's a no-op, so
// this is safe to call from CreateVendorBill without a separate "posted"
// flag on VendorBill.
func PostVendorBillLedgerEntry(billID uint) error {
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "vendor_bill", billID).First(&existing).Error; err == nil {
return nil
}

var bill models.VendorBill
if err := database.DB.First(&bill, billID).Error; err != nil {
return fmt.Errorf("vendor bill not found: %w", err)
}

var vendorPayable, inventory models.Account
if err := database.DB.Where("code = ?", "2001").First(&vendorPayable).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2001 (Vendor Payable): %w", err)
}
if err := database.DB.Where("code = ?", "1004").First(&inventory).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1004 (Inventory): %w", err)
}

transactionRef := fmt.Sprintf("VENDORBILL-%d", billID)

return database.DB.Transaction(func(tx *gorm.DB) error {
debitInventory := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      inventory.ID,
Type:           "debit",
Amount:         bill.Amount,
Description:    fmt.Sprintf("Vendor bill %s", bill.BillNumber),
ReferenceType:  "vendor_bill",
ReferenceID:    &billID,
EntryDate:      bill.BillDate,
}
if err := tx.Create(&debitInventory).Error; err != nil {
return fmt.Errorf("failed to create inventory ledger entry: %w", err)
}

if bill.GSTAmount > 0 {
var gstITC models.Account
if err := tx.Where("code = ?", "1005").First(&gstITC).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1005 (GST Input Credit): %w", err)
}
debitGST := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      gstITC.ID,
Type:           "debit",
Amount:         bill.GSTAmount,
Description:    fmt.Sprintf("Vendor bill %s - GST ITC", bill.BillNumber),
ReferenceType:  "vendor_bill",
ReferenceID:    &billID,
EntryDate:      bill.BillDate,
}
if err := tx.Create(&debitGST).Error; err != nil {
return fmt.Errorf("failed to create GST-ITC ledger entry: %w", err)
}
}

creditVendorPayable := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      vendorPayable.ID,
Type:           "credit",
Amount:         bill.Amount + bill.GSTAmount,
Description:    fmt.Sprintf("Vendor bill %s", bill.BillNumber),
ReferenceType:  "vendor_bill",
ReferenceID:    &billID,
EntryDate:      bill.BillDate,
}
if err := tx.Create(&creditVendorPayable).Error; err != nil {
return fmt.Errorf("failed to create vendor-payable ledger entry: %w", err)
}

return nil
})
}

// ReverseVendorBillLedgerEntry reverses the ledger entries created by
// PostVendorBillLedgerEntry when a vendor bill is voided: Credit Inventory,
// Credit GST Input Credit (if any), Debit Vendor Payable - the exact mirror
// of the original posting. Idempotent per bill via reference_type="vendor_bill_void".
// No-op if the original bill was never posted to the ledger in the first place.
func ReverseVendorBillLedgerEntry(billID uint) error {
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "vendor_bill_void", billID).First(&existing).Error; err == nil {
return nil
}

var original models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "vendor_bill", billID).First(&original).Error; err != nil {
// Bill was never posted to the ledger (e.g. void before posting ran) - nothing to reverse.
return nil
}

var bill models.VendorBill
if err := database.DB.First(&bill, billID).Error; err != nil {
return fmt.Errorf("vendor bill not found: %w", err)
}

var vendorPayable, inventory models.Account
if err := database.DB.Where("code = ?", "2001").First(&vendorPayable).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2001 (Vendor Payable): %w", err)
}
if err := database.DB.Where("code = ?", "1004").First(&inventory).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1004 (Inventory): %w", err)
}

transactionRef := fmt.Sprintf("VENDORBILLVOID-%d", billID)
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
creditInventory := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      inventory.ID,
Type:           "credit",
Amount:         bill.Amount,
Description:    fmt.Sprintf("Void vendor bill %s", bill.BillNumber),
ReferenceType:  "vendor_bill_void",
ReferenceID:    &billID,
EntryDate:      now,
}
if err := tx.Create(&creditInventory).Error; err != nil {
return fmt.Errorf("failed to create inventory reversal ledger entry: %w", err)
}

if bill.GSTAmount > 0 {
var gstITC models.Account
if err := tx.Where("code = ?", "1005").First(&gstITC).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1005 (GST Input Credit): %w", err)
}
creditGST := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      gstITC.ID,
Type:           "credit",
Amount:         bill.GSTAmount,
Description:    fmt.Sprintf("Void vendor bill %s - GST ITC reversal", bill.BillNumber),
ReferenceType:  "vendor_bill_void",
ReferenceID:    &billID,
EntryDate:      now,
}
if err := tx.Create(&creditGST).Error; err != nil {
return fmt.Errorf("failed to create GST-ITC reversal ledger entry: %w", err)
}
}

debitVendorPayable := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      vendorPayable.ID,
Type:           "debit",
Amount:         bill.Amount + bill.GSTAmount,
Description:    fmt.Sprintf("Void vendor bill %s", bill.BillNumber),
ReferenceType:  "vendor_bill_void",
ReferenceID:    &billID,
EntryDate:      now,
}
if err := tx.Create(&debitVendorPayable).Error; err != nil {
return fmt.Errorf("failed to create vendor-payable reversal ledger entry: %w", err)
}

return nil
})
}
// PostExpenseLedgerEntry records the double-entry ledger lines for one
// expense at creation time: Debit Operating Expenses, Credit Bank. This
// posts once, at CreateExpense - if the expense amount is later edited via
// UpdateExpense, the ledger is NOT automatically corrected (that requires a
// proper reversal/adjustment entry, which is out of scope for this phase;
// tracked separately). Idempotent per expense via reference_type="expense".
func PostExpenseLedgerEntry(expenseID uint) error {
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "expense", expenseID).First(&existing).Error; err == nil {
return nil
}

var expense models.Expense
if err := database.DB.First(&expense, expenseID).Error; err != nil {
return fmt.Errorf("expense not found: %w", err)
}
if expense.Amount <= 0 {
return nil
}

var opex, bank models.Account
if err := database.DB.Where("code = ?", "5003").First(&opex).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 5003 (Operating Expenses): %w", err)
}
if err := database.DB.Where("code = ?", "1002").First(&bank).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1002 (Bank): %w", err)
}

transactionRef := fmt.Sprintf("EXPENSE-%d", expenseID)

var existingCount int64
database.DB.Model(&models.LedgerEntry{}).Where("transaction_ref = ?", transactionRef).Count(&existingCount)
if existingCount > 0 {
return nil
}

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      opex.ID,
Type:           "debit",
Amount:         expense.Amount,
Description:    fmt.Sprintf("Expense: %s", expense.Category),
ReferenceType:  "expense",
ReferenceID:    &expenseID,
EntryDate:      expense.ExpenseDate,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create debit ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      bank.ID,
Type:           "credit",
Amount:         expense.Amount,
Description:    fmt.Sprintf("Expense: %s", expense.Category),
ReferenceType:  "expense",
ReferenceID:    &expenseID,
EntryDate:      expense.ExpenseDate,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create credit ledger entry: %w", err)
}
return nil
})
}

// PostExpenseAdjustmentLedgerEntry posts an adjusting double-entry line
// when a paid expense's amount is edited after PostExpenseLedgerEntry has
// already run for it, so the general ledger stays in sync with the
// Expense record instead of going stale. deltaAmount is
// newAmount-oldAmount: positive means the expense grew (post an extra
// Debit Opex / Credit Bank for the increase), negative means it shrank
// (post the reverse - Debit Bank / Credit Opex - for the decrease).
// No-op if deltaAmount is zero.
func PostExpenseAdjustmentLedgerEntry(expenseID uint, deltaAmount float64) error {
if deltaAmount == 0 {
return nil
}

var expense models.Expense
if err := database.DB.First(&expense, expenseID).Error; err != nil {
return fmt.Errorf("expense not found: %w", err)
}

var opex, bank models.Account
if err := database.DB.Where("code = ?", "5003").First(&opex).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 5003 (Operating Expenses): %w", err)
}
if err := database.DB.Where("code = ?", "1002").First(&bank).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1002 (Bank): %w", err)
}

amount := deltaAmount
increased := true
if amount < 0 {
amount = -amount
increased = false
}

transactionRef := fmt.Sprintf("EXPENSEADJ-%d-%d", expenseID, time.Now().UnixNano())
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
opexEntry := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      opex.ID,
Type:           "debit",
Amount:         amount,
Description:    fmt.Sprintf("Adjustment for expense: %s", expense.Category),
ReferenceType:  "expense_adjustment",
ReferenceID:    &expenseID,
EntryDate:      now,
}
bankEntry := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      bank.ID,
Type:           "credit",
Amount:         amount,
Description:    fmt.Sprintf("Adjustment for expense: %s", expense.Category),
ReferenceType:  "expense_adjustment",
ReferenceID:    &expenseID,
EntryDate:      now,
}
if !increased {
// Expense amount decreased - reverse the direction: Debit Bank, Credit Opex.
opexEntry.Type = "credit"
bankEntry.Type = "debit"
}
if err := tx.Create(&opexEntry).Error; err != nil {
return fmt.Errorf("failed to create opex adjustment ledger entry: %w", err)
}
if err := tx.Create(&bankEntry).Error; err != nil {
return fmt.Errorf("failed to create bank adjustment ledger entry: %w", err)
}
return nil
})
}

// PostRefundLedgerEntry records the double-entry ledger lines for one
// customer refund: Debit Customer Refund Payable, Credit Bank/Cash. Called
// with only the newly-added refund delta (not the cumulative refunded
// total), so a payment that moves partially_refunded -> refunded twice
// posts two separate entries, one per incremental amount actually paid
// out - never a single entry for the full cumulative refund.
func PostRefundLedgerEntry(orderID uint, deltaAmount float64, paymentMethod string) error {
if deltaAmount <= 0 {
return nil
}

bankCode := "1002" // Bank
if paymentMethod == models.PaymentMethodCOD {
bankCode = "1001" // Cash
}

var refundPayable, bank models.Account
if err := database.DB.Where("code = ?", "2004").First(&refundPayable).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2004 (Customer Refund Payable): %w", err)
}
if err := database.DB.Where("code = ?", bankCode).First(&bank).Error; err != nil {
return fmt.Errorf("chart of accounts missing code %s: %w", bankCode, err)
}

transactionRef := fmt.Sprintf("REFUND-%d-%d", orderID, time.Now().UnixNano())
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      refundPayable.ID,
Type:           "debit",
Amount:         deltaAmount,
Description:    fmt.Sprintf("Refund for order #%d", orderID),
ReferenceType:  "refund",
ReferenceID:    &orderID,
EntryDate:      now,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create debit ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      bank.ID,
Type:           "credit",
Amount:         deltaAmount,
Description:    fmt.Sprintf("Refund for order #%d", orderID),
ReferenceType:  "refund",
ReferenceID:    &orderID,
EntryDate:      now,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create credit ledger entry: %w", err)
}
return nil
})
}

// PostWalletRefundLedgerEntry books a refund that was credited to the
// customer's in-app wallet rather than paid out via bank/cash. No money
// actually leaves the business, so unlike PostRefundLedgerEntry (which
// credits Bank/Cash), this credits Customer Wallet Liability (2005) -
// the business now owes the customer that amount as store credit.
func PostWalletRefundLedgerEntry(orderID uint, deltaAmount float64) error {
if deltaAmount <= 0 {
return nil
}

var refundPayable, walletLiability models.Account
if err := database.DB.Where("code = ?", "2004").First(&refundPayable).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2004 (Customer Refund Payable): %w", err)
}
if err := database.DB.Where("code = ?", "2005").First(&walletLiability).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2005 (Customer Wallet Liability): %w", err)
}

transactionRef := fmt.Sprintf("WALLET-REFUND-%d-%d", orderID, time.Now().UnixNano())
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      refundPayable.ID,
Type:           "debit",
Amount:         deltaAmount,
Description:    fmt.Sprintf("Refund to wallet for order #%d", orderID),
ReferenceType:  "refund",
ReferenceID:    &orderID,
EntryDate:      now,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create debit ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef,
AccountID:      walletLiability.ID,
Type:           "credit",
Amount:         deltaAmount,
Description:    fmt.Sprintf("Refund to wallet for order #%d", orderID),
ReferenceType:  "refund",
ReferenceID:    &orderID,
EntryDate:      now,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create credit ledger entry: %w", err)
}
return nil
})
}

// PostGatewaySettlementLedgerEntry records the gateway fee deduction for
// one online payment (SRS 12.10): Debit Gateway Fees (expense), Credit
// Bank - the fee is deducted from what the gateway actually pays out, so
// this reduces the Bank balance already recorded at sale time rather than
// re-booking the whole gross amount.
func PostGatewaySettlementLedgerEntry(paymentID uint, feeAmount float64) error {
if feeAmount <= 0 {
return nil
}
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "gateway_settlement", paymentID).First(&existing).Error; err == nil {
return nil
}

var gatewayFees, bank models.Account
if err := database.DB.Where("code = ?", "5005").First(&gatewayFees).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 5005 (Gateway Fees): %w", err)
}
if err := database.DB.Where("code = ?", "1002").First(&bank).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1002 (Bank): %w", err)
}

transactionRef := fmt.Sprintf("GWSETTLE-%d", paymentID)
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: gatewayFees.ID, Type: "debit", Amount: feeAmount,
Description: fmt.Sprintf("Gateway fee for payment #%d", paymentID),
ReferenceType: "gateway_settlement", ReferenceID: &paymentID, EntryDate: now,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create gateway fee ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: bank.ID, Type: "credit", Amount: feeAmount,
Description: fmt.Sprintf("Gateway fee for payment #%d", paymentID),
ReferenceType: "gateway_settlement", ReferenceID: &paymentID, EntryDate: now,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create bank ledger entry: %w", err)
}
return nil
})
}

// PostRiderPayoutAccrualLedgerEntry recognizes a rider payout as owed
// (SRS 12.11), the moment it's approved: Debit Rider Delivery Expense,
// Credit Rider Payable.
func PostRiderPayoutAccrualLedgerEntry(payoutID uint) error {
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "rider_payout_accrual", payoutID).First(&existing).Error; err == nil {
return nil
}

var payout models.RiderPayout
if err := database.DB.First(&payout, payoutID).Error; err != nil {
return fmt.Errorf("rider payout not found: %w", err)
}
if payout.Amount <= 0 {
return nil
}

var expense, payable models.Account
if err := database.DB.Where("code = ?", "5004").First(&expense).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 5004 (Rider Delivery Expense): %w", err)
}
if err := database.DB.Where("code = ?", "2003").First(&payable).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2003 (Rider Payable): %w", err)
}

transactionRef := fmt.Sprintf("RIDERACCRUAL-%d", payoutID)
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: expense.ID, Type: "debit", Amount: payout.Amount,
Description: fmt.Sprintf("Rider payout #%d accrual", payoutID),
ReferenceType: "rider_payout_accrual", ReferenceID: &payoutID, EntryDate: now,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create rider expense ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: payable.ID, Type: "credit", Amount: payout.Amount,
Description: fmt.Sprintf("Rider payout #%d accrual", payoutID),
ReferenceType: "rider_payout_accrual", ReferenceID: &payoutID, EntryDate: now,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create rider payable ledger entry: %w", err)
}
return nil
})
}

// PostRiderPayoutSettlementLedgerEntry records actually paying out a rider
// (SRS 12.11): Debit Rider Payable, Credit Bank. Only meaningful after
// PostRiderPayoutAccrualLedgerEntry has already run for the same payout.
func PostRiderPayoutSettlementLedgerEntry(payoutID uint) error {
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "rider_payout_settlement", payoutID).First(&existing).Error; err == nil {
return nil
}

var payout models.RiderPayout
if err := database.DB.First(&payout, payoutID).Error; err != nil {
return fmt.Errorf("rider payout not found: %w", err)
}
if payout.Amount <= 0 {
return nil
}

var payable, bank models.Account
if err := database.DB.Where("code = ?", "2003").First(&payable).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 2003 (Rider Payable): %w", err)
}
if err := database.DB.Where("code = ?", "1002").First(&bank).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1002 (Bank): %w", err)
}

transactionRef := fmt.Sprintf("RIDERPAY-%d", payoutID)
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: payable.ID, Type: "debit", Amount: payout.Amount,
Description: fmt.Sprintf("Rider payout #%d settlement", payoutID),
ReferenceType: "rider_payout_settlement", ReferenceID: &payoutID, EntryDate: now,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create rider payable ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: bank.ID, Type: "credit", Amount: payout.Amount,
Description: fmt.Sprintf("Rider payout #%d settlement", payoutID),
ReferenceType: "rider_payout_settlement", ReferenceID: &payoutID, EntryDate: now,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create bank ledger entry: %w", err)
}
return nil
})
}

// PostRiderCODDepositLedgerEntry moves the recorded balance of a verified
// COD deposit from Cash (where COD sales are already booked, since the
// rider physically holds the cash) into Bank (SRS 12.9).
func PostRiderCODDepositLedgerEntry(depositID uint) error {
var existing models.LedgerEntry
if err := database.DB.Where("reference_type = ? AND reference_id = ?", "rider_cod_deposit", depositID).First(&existing).Error; err == nil {
return nil
}

var deposit models.RiderCODDeposit
if err := database.DB.First(&deposit, depositID).Error; err != nil {
return fmt.Errorf("rider COD deposit not found: %w", err)
}

var cash, bank models.Account
if err := database.DB.Where("code = ?", "1001").First(&cash).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1001 (Cash): %w", err)
}
if err := database.DB.Where("code = ?", "1002").First(&bank).Error; err != nil {
return fmt.Errorf("chart of accounts missing code 1002 (Bank): %w", err)
}

transactionRef := fmt.Sprintf("CODDEPOSIT-%d", depositID)
now := time.Now()

return database.DB.Transaction(func(tx *gorm.DB) error {
debit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: bank.ID, Type: "debit", Amount: deposit.Amount,
Description: fmt.Sprintf("Rider COD deposit #%d", depositID),
ReferenceType: "rider_cod_deposit", ReferenceID: &depositID, EntryDate: now,
}
if err := tx.Create(&debit).Error; err != nil {
return fmt.Errorf("failed to create bank ledger entry: %w", err)
}
credit := models.LedgerEntry{
TransactionRef: transactionRef, AccountID: cash.ID, Type: "credit", Amount: deposit.Amount,
Description: fmt.Sprintf("Rider COD deposit #%d", depositID),
ReferenceType: "rider_cod_deposit", ReferenceID: &depositID, EntryDate: now,
}
if err := tx.Create(&credit).Error; err != nil {
return fmt.Errorf("failed to create cash ledger entry: %w", err)
}
return nil
})
}
