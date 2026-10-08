package services

import (
"fmt"
"strings"
"time"

"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
"gorm.io/gorm"
"gorm.io/gorm/clause"
)

// GenerateInvoiceIfNotExists creates the Invoice + InvoiceItem snapshot
// for an order, the first time it's called for that order. Safe to call
// more than once (e.g. once from Checkout for COD, once from
// VerifyPayment for online) - OrderID has a unique index, and this
// function checks for an existing row inside the transaction under a
// lock before creating one, so concurrent calls cannot create duplicates.
//
// Fire-and-forget by design: an invoice failure should never block the
// order/payment flow that triggered it. Errors are returned so the caller
// can log them, but callers should not fail the request on error.
func GenerateInvoiceIfNotExists(orderID uint) (*models.Invoice, error) {
var invoice models.Invoice

txErr := database.DB.Transaction(func(tx *gorm.DB) error {
// Already generated - nothing to do. This check happens inside
// the transaction so a concurrent call blocks on the row lock
// below rather than racing this SELECT.
if err := tx.Where("order_id = ?", orderID).First(&invoice).Error; err == nil {
return nil
}

var order models.Order
if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
Preload("Items.Product").Preload("Address").
First(&order, orderID).Error; err != nil {
return fmt.Errorf("order not found: %w", err)
}
// cancelled-order guard: a cancelled order is never billed.
if order.Status == models.OrderStatusCancelled {
return fmt.Errorf("order %d is cancelled: invoice not generated", orderID)
}

// Re-check after acquiring the lock in case another goroutine
// created it between the first check and the lock being granted.
if err := tx.Where("order_id = ?", orderID).First(&invoice).Error; err == nil {
return nil
}

// Atomic invoice number: a dedicated Postgres sequence, not COUNT(*)+1.
// Two concurrent invoice generations for two *different* orders could
// both read the same COUNT() before either commits and then collide on
// the unique invoice_number constraint - nextval() is atomic across
// concurrent transactions and can never hand out the same value twice.
// CREATE SEQUENCE IF NOT EXISTS is self-healing: the sequence is created
// automatically the first time this runs, so there's no manual DB step
// required after deploying this - useful since AutoMigrate doesn't manage
// sequences and production migrations aren't always run immediately.
if err := tx.Exec("CREATE SEQUENCE IF NOT EXISTS invoice_number_seq START 1").Error; err != nil {
return fmt.Errorf("failed to ensure invoice sequence exists: %w", err)
}
// Same self-healing idea for columns: this project's production deploy
// runs with GIN_MODE=release, which intentionally skips GORM AutoMigrate
// (see internal/database/database.go) so schema changes go through
// versioned migrations instead - but until that migration is actually
// applied to prod, these ADD COLUMN IF NOT EXISTS statements make the
// invoice snapshot fields self-provisioning rather than a hard failure.
// Schema patches for these columns now run once at startup (see
// EnsureInvoiceSchemaPatches in database.go) instead of inside this
// per-invoice transaction - running ALTER TABLE here took an ACCESS
// EXCLUSIVE lock on every invoice generation and deadlocked against
// AutoAssignDeliveryPartner's concurrent row updates on orders.
var seqVal int64
if err := tx.Raw("SELECT nextval('invoice_number_seq')").Scan(&seqVal).Error; err != nil {
return fmt.Errorf("failed to allocate invoice number: %w", err)
}
invoiceNumber := fmt.Sprintf("INV-%d-%06d", time.Now().Year(), seqVal)

var orderCoupon models.OrderCoupon
discountAmount := 0.0
if err := tx.Where("order_id = ?", order.ID).First(&orderCoupon).Error; err == nil {
discountAmount = orderCoupon.DiscountAmount
}

// Section 15(3) CGST Act: trade discounts must reduce the taxable
// value before GST is computed. Apply the coupon discount
// proportionally across line items via a ratio on ItemsAmount.
discountRatio := 1.0
if order.ItemsAmount > 0 {
netItemsAmount := order.ItemsAmount - discountAmount
if netItemsAmount < 0 {
netItemsAmount = 0
}
discountRatio = netItemsAmount / order.ItemsAmount
}

// Gateway transaction reference for online payments only - COD has no
// gateway leg, so this stays blank rather than a fabricated value.
paymentReference := ""
if order.PaymentMethod == models.PaymentMethodOnline {
var payment models.Payment
if err := tx.Where("order_id = ?", order.ID).First(&payment).Error; err == nil {
paymentReference = payment.RazorpayPaymentID
}
}

// GST calculation. Product.Price is treated as GST-inclusive (MRP),
// which is standard for Indian retail listings - so the tax amount
// is derived out of the line total rather than added on top of it.
// Place of supply is the delivery address's state; if it matches the
// seller's registered state (config.SellerState) this is an
// intra-state sale (CGST+SGST, split equally), otherwise inter-state
// (IGST, full amount). Comparison is case/whitespace-insensitive
// since state names are free text on both sides.
sellerState := strings.TrimSpace(strings.ToLower(config.AppConfig.SellerState))
buyerState := strings.TrimSpace(strings.ToLower(order.Address.State))
isInterState := sellerState != "" && buyerState != "" && sellerState != buyerState

totalTaxable := 0.0
totalGST := 0.0
type gstLine struct {
percent float64
amount  float64
}
itemGST := make([]gstLine, len(order.Items))

for i, item := range order.Items {
lineTotal := item.Price * float64(item.Quantity) * discountRatio
gstPercent := item.Product.GSTPercent
taxableLine := lineTotal
gstLineAmount := 0.0
if gstPercent > 0 {
taxableLine = lineTotal / (1 + gstPercent/100)
gstLineAmount = lineTotal - taxableLine
}
totalTaxable += taxableLine
totalGST += gstLineAmount
itemGST[i] = gstLine{percent: gstPercent, amount: gstLineAmount}
}

// Delivery charge follows the same tax treatment as the goods it's
// delivering (composite supply, Section 15(2)(c) CGST Act) rather than
// being left untaxed - taxed at the highest GST rate among the order's
// items, since a single delivery charge can't be split per-item. Orders
// with no GST-rated items (maxGSTPercent stays 0) leave delivery untaxed too.
maxGSTPercent := 0.0
for _, g := range itemGST {
if g.percent > maxGSTPercent {
maxGSTPercent = g.percent
}
}
deliveryTaxable := order.DeliveryCharge
deliveryGST := 0.0
if maxGSTPercent > 0 && order.DeliveryCharge > 0 {
deliveryTaxable = order.DeliveryCharge / (1 + maxGSTPercent/100)
deliveryGST = order.DeliveryCharge - deliveryTaxable
}
totalTaxable += deliveryTaxable
totalGST += deliveryGST

// Platform fee gets the same composite-supply tax treatment as delivery
// charge - taxed at the order's highest item GST rate.
platformFeeTaxable := order.PlatformFee
platformFeeGST := 0.0
if maxGSTPercent > 0 && order.PlatformFee > 0 {
platformFeeTaxable = order.PlatformFee / (1 + maxGSTPercent/100)
platformFeeGST = order.PlatformFee - platformFeeTaxable
}
totalTaxable += platformFeeTaxable
totalGST += platformFeeGST

cgstAmount := 0.0
sgstAmount := 0.0
igstAmount := 0.0
if isInterState {
igstAmount = totalGST
} else {
cgstAmount = totalGST / 2
sgstAmount = totalGST / 2
}

invoice = models.Invoice{
InvoiceNumber:     invoiceNumber,
OrderID:           order.ID,
CustomerName:      order.Address.FullName,
CustomerPhone:     order.Address.Phone,
AddressLine1:      order.Address.Line1,
AddressLine2:      order.Address.Line2,
AddressCity:       order.Address.City,
AddressState:      order.Address.State,
AddressPincode:    order.Address.Pincode,
ItemsAmount:       order.ItemsAmount,
DiscountAmount:    discountAmount,
DeliveryCharge:    order.DeliveryCharge,
PlatformFee:       order.PlatformFee,
WalletUsed:        order.WalletAmountUsed,
IsInterState:      isInterState,
TaxableAmount:     totalTaxable,
CGSTAmount:        cgstAmount,
SGSTAmount:        sgstAmount,
IGSTAmount:        igstAmount,
TotalAmount:       order.TotalAmount,
PaymentMethod:     order.PaymentMethod,
PaymentReference:  paymentReference,
GeneratedAt:       time.Now(),
}
if err := tx.Create(&invoice).Error; err != nil {
return fmt.Errorf("failed to create invoice: %w", err)
}

for i, item := range order.Items {
invoiceItem := models.InvoiceItem{
InvoiceID:   invoice.ID,
ProductID:   item.ProductID,
ProductName: item.Product.Name,
SKU:         item.Product.Barcode,
HSNCode:     item.Product.HSNCode,
Quantity:    item.Quantity,
Price:       item.Price,
GSTPercent:  itemGST[i].percent,
GSTAmount:   itemGST[i].amount,
}
if err := tx.Create(&invoiceItem).Error; err != nil {
return fmt.Errorf("failed to create invoice item: %w", err)
}
}

// System-generated event, not a specific admin/staff action - recorded
// with admin_id 0 / "system" so it's still visible in the audit trail
// (spec explicitly asks that invoice generation be logged) without
// attributing it to a person who didn't trigger it.
utils.LogAudit(0, "system", "generate_invoice", "invoice", fmt.Sprint(invoice.ID),
fmt.Sprintf("order_id=%d invoice_number=%s total=%.2f", order.ID, invoiceNumber, order.TotalAmount))

return nil
})

if txErr != nil {
return nil, txErr
}
return &invoice, nil
}

