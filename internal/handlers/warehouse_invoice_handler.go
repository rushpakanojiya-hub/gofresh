package handlers

import (
"net/http"
"strconv"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

// sellerDetails returns the configured seller/business details for
// display on invoices - every field is env-configured (see
// internal/config/config.go), never invented. A blank config field is
// passed through as blank rather than substituted with a placeholder, so
// the frontend/PDF can show "Not configured" instead of fake data.
func sellerDetails() gin.H {
cfg := config.AppConfig
return gin.H{
"company_name":   cfg.SellerCompanyName,
"address":        cfg.SellerAddress,
"gstin":          cfg.SellerGSTIN,
"contact_number": cfg.SellerContactNumber,
"email":          cfg.SellerEmail,
"state":          cfg.SellerState,
"state_code":     cfg.SellerStateCode,
"fssai_number":   cfg.SellerFSSAINumber,
}
}

// invoiceResponse is the shared read-only shape returned to every caller
// (customer, warehouse, admin) - same fields regardless of role, since an
// invoice is immutable and there's nothing role-specific to hide from any
// of them once they're authorized to see it at all.
func invoiceResponse(invoice models.Invoice, orderStatus, paymentStatus string) gin.H {
return gin.H{
"invoice_number":    invoice.InvoiceNumber,
"order_id":          invoice.OrderID,
"order_status":      orderStatus,
"customer_name":     invoice.CustomerName,
"customer_phone":    invoice.CustomerPhone,
"address_line1":     invoice.AddressLine1,
"address_line2":     invoice.AddressLine2,
"address_city":      invoice.AddressCity,
"address_state":     invoice.AddressState,
"address_pincode":   invoice.AddressPincode,
"place_of_supply":   invoice.AddressState,
"items_amount":      invoice.ItemsAmount,
"discount_amount":   invoice.DiscountAmount,
"delivery_charge":   invoice.DeliveryCharge,
"wallet_used":       invoice.WalletUsed,
"total_amount":      invoice.TotalAmount,
"payment_method":    invoice.PaymentMethod,
"payment_reference": invoice.PaymentReference,
"payment_status":    paymentStatus,
"generated_at":      invoice.GeneratedAt,
"items":             invoice.Items,
"seller":            sellerDetails(),
// This project has no tax/GST calculation configured anywhere in the
// order flow, so no tax line is computed or shown - reported honestly
// rather than inventing a rate. Set up real tax logic before this can
// say anything else here.
"tax_status": "Configuration required - no GST/tax rate is configured for this store",
}
}

// GetOrderInvoice godoc
// GET /api/v1/warehouse/orders/:id/invoice (warehouse staff only)
// Read-only view of the invoice for an order fulfilled by the caller's
// warehouse: invoice number, status, line items, and totals. Warehouse
// staff have no write access here - price, tax, discount, grand total
// and invoice number can only ever be set by GenerateInvoiceIfNotExists,
// never edited through this or any other warehouse endpoint.
func GetOrderInvoice(c *gin.Context) {
warehouseID := c.MustGet("warehouse_id").(uint)
orderID := c.Param("id")

var order models.Order
if err := database.DB.Where("warehouse_id = ?", warehouseID).First(&order, orderID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found for your warehouse"})
return
}

var invoice models.Invoice
err := database.DB.Where("order_id = ?", order.ID).Preload("Items").First(&invoice).Error
if err != nil {
// Orders paid COD are invoiced at confirmation and online orders right
// after payment verification, so a missing invoice on an order that
// has already reached the warehouse is unexpected - generate it
// on-demand instead of forcing the warehouse to work around a gap.
generated, genErr := services.GenerateInvoiceIfNotExists(order.ID)
if genErr != nil || generated == nil {
c.JSON(http.StatusNotFound, gin.H{"error": "No invoice found for this order yet"})
return
}
database.DB.Where("order_id = ?", order.ID).Preload("Items").First(&invoice)
}

c.JSON(http.StatusOK, invoiceResponse(invoice, order.Status, order.PaymentStatus))
}

// GetMyOrderInvoice godoc
// GET /api/v1/orders/:id/invoice (protected - order owner only)
// A customer can only ever see the invoice for an order that belongs to
// them - the WHERE clause below is the IDOR guard, not just a 404 lookup.
func GetMyOrderInvoice(c *gin.Context) {
userID := c.MustGet("user_id").(uint)
orderID := c.Param("id")

var order models.Order
if err := database.DB.Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
return
}
// cancelled-order guard: no invoice is shown for a cancelled order.
if order.Status == models.OrderStatusCancelled {
c.JSON(http.StatusNotFound, gin.H{"error": "No invoice for a cancelled order"})
return
}

if order.PaymentStatus != models.OrderPaymentStatusPaid && order.PaymentMethod != models.PaymentMethodCOD {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invoice is not available until payment is confirmed"})
return
}

var invoice models.Invoice
if err := database.DB.Where("order_id = ?", order.ID).Preload("Items").First(&invoice).Error; err != nil {
// Invoice generation is triggered at Checkout (COD) / VerifyPayment
// (online), so a missing invoice here is unexpected - self-heal by
// generating on demand instead of leaving the customer stuck.
generated, genErr := services.GenerateInvoiceIfNotExists(order.ID)
if genErr != nil || generated == nil {
c.JSON(http.StatusNotFound, gin.H{"error": "No invoice found for this order yet"})
return
}
database.DB.Where("order_id = ?", order.ID).Preload("Items").First(&invoice)
}

c.JSON(http.StatusOK, invoiceResponse(invoice, order.Status, order.PaymentStatus))
}

// SearchInvoices godoc
// GET /api/v1/admin/invoices?invoice_number=&order_id=&payment_status=&date_from=&date_to=&page=&limit= (admin only)
func SearchInvoices(c *gin.Context) {
page := 1
limit := 20
if p := c.Query("page"); p != "" {
if v, err := strconv.Atoi(p); err == nil && v > 0 {
page = v
}
}
if l := c.Query("limit"); l != "" {
if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
limit = v
}
}

db := database.DB.Model(&models.Invoice{}).Joins("JOIN orders ON orders.id = invoices.order_id")
if invoiceNumber := c.Query("invoice_number"); invoiceNumber != "" {
db = db.Where("invoices.invoice_number ILIKE ?", "%"+invoiceNumber+"%")
}
if orderID := c.Query("order_id"); orderID != "" {
db = db.Where("invoices.order_id = ?", orderID)
}
if paymentStatus := c.Query("payment_status"); paymentStatus != "" {
db = db.Where("orders.payment_status = ?", paymentStatus)
}
if dateFrom := c.Query("date_from"); dateFrom != "" {
if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
db = db.Where("invoices.generated_at >= ?", t)
}
}
if dateTo := c.Query("date_to"); dateTo != "" {
if t, err := time.Parse("2006-01-02", dateTo); err == nil {
db = db.Where("invoices.generated_at < ?", t.AddDate(0, 0, 1))
}
}

var total int64
db.Count(&total)

var invoices []models.Invoice
offset := (page - 1) * limit
if err := db.Select("invoices.*").
Order("invoices.generated_at DESC").Offset(offset).Limit(limit).
Find(&invoices).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search invoices"})
return
}

c.JSON(http.StatusOK, gin.H{
"invoices":    invoices,
"page":        page,
"limit":       limit,
"total":       total,
"total_pages": int((total + int64(limit) - 1) / int64(limit)),
})
}

// GetAdminInvoiceByID godoc
// GET /api/v1/admin/invoices/:id (admin only)
func GetAdminInvoiceByID(c *gin.Context) {
id := c.Param("id")

var invoice models.Invoice
if err := database.DB.Preload("Items").First(&invoice, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
return
}

var order models.Order
database.DB.First(&order, invoice.OrderID)

c.JSON(http.StatusOK, invoiceResponse(invoice, order.Status, order.PaymentStatus))
}
