package handlers

import (
"log"
"net/http"
"strconv"
"strings"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
)

// GetAdminPayments godoc
// GET /api/v1/admin/payments (admin only)
// Combined reconciliation list: every order gets a row. Orders with a real
// Payment record (online attempts, or COD orders an admin has manually
// marked refunded) use that record's data; other COD orders have their
// fields derived from the Order itself.
// Query params: search, status, payment_method, gateway, date_from, date_to (YYYY-MM-DD), page, limit
func GetAdminPayments(c *gin.Context) {
page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
if page < 1 {
page = 1
}
if limit < 1 || limit > 100 {
limit = 20
}

search := strings.TrimSpace(c.Query("search"))
status := c.Query("status")
paymentMethod := c.Query("payment_method")
gateway := c.Query("gateway")
dateFrom := c.Query("date_from")
dateTo := c.Query("date_to")

baseFrom := `
FROM orders o
LEFT JOIN payments p ON p.order_id = o.id
LEFT JOIN users u ON u.id = o.user_id
`

var whereClauses []string
var args []interface{}
argN := 1

if search != "" {
whereClauses = append(whereClauses, `(
u.name ILIKE $`+strconv.Itoa(argN)+` OR
u.phone ILIKE $`+strconv.Itoa(argN)+` OR
CAST(o.id AS TEXT) ILIKE $`+strconv.Itoa(argN)+` OR
COALESCE(p.razorpay_payment_id, '') ILIKE $`+strconv.Itoa(argN)+`
)`)
args = append(args, "%"+search+"%")
argN++
}

if status != "" {
whereClauses = append(whereClauses, `(
CASE
WHEN p.id IS NOT NULL THEN (CASE WHEN p.status = 'created' THEN 'pending' ELSE p.status END)
ELSE o.payment_status
END
) = $`+strconv.Itoa(argN))
args = append(args, status)
argN++
}

if paymentMethod != "" {
whereClauses = append(whereClauses, `o.payment_method = $`+strconv.Itoa(argN))
args = append(args, paymentMethod)
argN++
}

if gateway != "" {
whereClauses = append(whereClauses, `COALESCE(p.gateway, CASE WHEN o.payment_method = 'cod' THEN 'cod' ELSE 'razorpay' END) = $`+strconv.Itoa(argN))
args = append(args, gateway)
argN++
}

if dateFrom != "" {
if _, err := time.Parse("2006-01-02", dateFrom); err == nil {
whereClauses = append(whereClauses, `COALESCE(p.created_at, o.created_at) >= $`+strconv.Itoa(argN)+`::date`)
args = append(args, dateFrom)
argN++
}
}
if dateTo != "" {
if _, err := time.Parse("2006-01-02", dateTo); err == nil {
whereClauses = append(whereClauses, `COALESCE(p.created_at, o.created_at) < ($`+strconv.Itoa(argN)+`::date + interval '1 day')`)
args = append(args, dateTo)
argN++
}
}

whereSQL := ""
if len(whereClauses) > 0 {
whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
}

var total int64
countQuery := "SELECT COUNT(*) " + baseFrom + whereSQL
if err := database.DB.Raw(countQuery, args...).Scan(&total).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count payments"})
return
}

selectSQL := `
SELECT
o.id AS order_id,
COALESCE(
NULLIF(p.razorpay_payment_id, ''),
CASE WHEN o.payment_method = 'cod' THEN 'COD-' || o.id::text ELSE 'TXN-' || o.id::text END
) AS transaction_id,
COALESCE(u.name, '') AS customer_name,
COALESCE(u.phone, '') AS customer_phone,
COALESCE(p.amount, o.total_amount) AS amount,
COALESCE(p.refunded_amount, 0) AS refunded_amount,
o.payment_method AS payment_method,
COALESCE(p.gateway, CASE WHEN o.payment_method = 'cod' THEN 'cod' ELSE 'razorpay' END) AS gateway,
CASE
WHEN p.id IS NOT NULL THEN (CASE WHEN p.status = 'created' THEN 'pending' ELSE p.status END)
ELSE o.payment_status
END AS status,
COALESCE(p.created_at, o.created_at) AS created_at
` + baseFrom + whereSQL + `
ORDER BY COALESCE(p.created_at, o.created_at) DESC
LIMIT $` + strconv.Itoa(argN) + ` OFFSET $` + strconv.Itoa(argN+1)

args = append(args, limit, (page-1)*limit)

rows := []models.AdminPaymentRow{}
if err := database.DB.Raw(selectSQL, args...).Scan(&rows).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch payments"})
return
}

totalPages := int((total + int64(limit) - 1) / int64(limit))
if totalPages < 1 {
totalPages = 1
}

c.JSON(http.StatusOK, models.AdminPaymentListResponse{
Payments:   rows,
Page:       page,
Limit:      limit,
Total:      total,
TotalPages: totalPages,
})
}

// GetAdminPaymentDetail godoc
// GET /api/v1/admin/payments/:order_id (admin only)
func GetAdminPaymentDetail(c *gin.Context) {
orderID, err := strconv.Atoi(c.Param("order_id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
return
}

var order models.Order
if err := database.DB.Preload("Address").Preload("Items.Product").First(&order, orderID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
return
}

var customer models.User
database.DB.First(&customer, order.UserID)

var payment models.Payment
hasPayment := true
if err := database.DB.Where("order_id = ?", orderID).First(&payment).Error; err != nil {
hasPayment = false
gateway := "cod"
if order.PaymentMethod == models.PaymentMethodOnline {
gateway = "razorpay"
}
payment = models.Payment{
OrderID:  order.ID,
Amount:   order.TotalAmount,
Currency: "INR",
Status:   order.PaymentStatus,
Gateway:  gateway,
}
}

c.JSON(http.StatusOK, gin.H{
"order":              order,
"customer":           gin.H{"id": customer.ID, "name": customer.Name, "phone": customer.Phone},
"payment":            payment,
"has_payment_record": hasPayment,
})
}

// UpdateAdminPaymentStatus godoc
// PUT /api/v1/admin/payments/:order_id/status (admin only)
// body: { "status": "refunded", "refunded_amount": 499.00 }
// Upserts the Payment row - COD orders that never had one get one created here.
func UpdateAdminPaymentStatus(c *gin.Context) {
orderID, err := strconv.Atoi(c.Param("order_id"))
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
return
}

var req models.AdminPaymentStatusUpdateRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

var order models.Order
if err := database.DB.First(&order, orderID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
return
}

if (req.Status == models.PaymentStatusRefunded || req.Status == models.PaymentStatusPartiallyRefunded) && req.RefundedAmount == nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "refunded_amount is required when status is refunded or partially_refunded"})
return
}

var payment models.Payment
if err := database.DB.Where("order_id = ?", orderID).First(&payment).Error; err != nil {
gateway := "cod"
if order.PaymentMethod == models.PaymentMethodOnline {
gateway = "razorpay"
}
payment = models.Payment{
OrderID:  order.ID,
Amount:   order.TotalAmount,
Currency: "INR",
Gateway:  gateway,
}
}

previousRefundedAmount := payment.RefundedAmount
payment.Status = req.Status
if req.RefundedAmount != nil {
// 12.28.6: a refund can never exceed the original payment amount -
// reject rather than silently clamp, so a typo/bug surfaces as an
// error instead of quietly capping the refund.
if *req.RefundedAmount < 0 {
c.JSON(http.StatusBadRequest, gin.H{"error": "refunded_amount cannot be negative"})
return
}
if *req.RefundedAmount > payment.Amount {
c.JSON(http.StatusBadRequest, gin.H{"error": "refunded_amount cannot exceed the original payment amount"})
return
}
payment.RefundedAmount = *req.RefundedAmount
}

if err := database.DB.Save(&payment).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment status"})
return
}

// Post the incremental refund (not the cumulative total) to the ledger:
// Debit Customer Refund Payable, Credit Bank/Cash. Only fires when this
// call actually increased the refunded amount.
refundDelta := payment.RefundedAmount - previousRefundedAmount
if refundDelta > 0 {
if err := services.PostRefundLedgerEntry(order.ID, refundDelta, order.PaymentMethod); err != nil {
log.Printf("failed to post refund ledger entry for order %d: %v", order.ID, err)
}
}

switch req.Status {
case models.PaymentStatusPaid:
order.PaymentStatus = models.OrderPaymentStatusPaid
case models.PaymentStatusFailed:
order.PaymentStatus = models.OrderPaymentStatusFailed
case models.PaymentStatusCreated:
order.PaymentStatus = models.OrderPaymentStatusPending
case models.PaymentStatusRefunded:
order.PaymentStatus = models.OrderPaymentStatusRefunded
case models.PaymentStatusPartiallyRefunded:
order.PaymentStatus = models.OrderPaymentStatusPartiallyRefunded
}
database.DB.Save(&order)

if refundDelta > 0 && (req.Status == models.PaymentStatusRefunded || req.Status == models.PaymentStatusPartiallyRefunded) {
go services.SendPushToUserWithData(order.UserID, "Refund Initiated", "Your refund has been initiated.", services.OrderPushData(order.ID))
go services.SendPushToUserWithData(order.UserID, "Refund Completed", "Your refund has been completed.", services.OrderPushData(order.ID))
}

adminID := c.MustGet("user_id").(uint)
adminPhone := c.MustGet("phone").(string)
utils.LogAudit(adminID, adminPhone, "update_payment_status", "payment", strconv.Itoa(orderID), "status: "+req.Status)

c.JSON(http.StatusOK, payment)
}

// GetAdminPaymentReconciliation godoc
// GET /api/v1/admin/payments/reconciliation (admin only)
// Query params: date_from, date_to (YYYY-MM-DD)
func GetAdminPaymentReconciliation(c *gin.Context) {
dateFrom := c.Query("date_from")
dateTo := c.Query("date_to")

baseFrom := `
FROM orders o
LEFT JOIN payments p ON p.order_id = o.id
`
var whereClauses []string
var args []interface{}
argN := 1
if dateFrom != "" {
if _, err := time.Parse("2006-01-02", dateFrom); err == nil {
whereClauses = append(whereClauses, `COALESCE(p.created_at, o.created_at) >= $`+strconv.Itoa(argN)+`::date`)
args = append(args, dateFrom)
argN++
}
}
if dateTo != "" {
if _, err := time.Parse("2006-01-02", dateTo); err == nil {
whereClauses = append(whereClauses, `COALESCE(p.created_at, o.created_at) < ($`+strconv.Itoa(argN)+`::date + interval '1 day')`)
args = append(args, dateTo)
argN++
}
}
whereSQL := ""
if len(whereClauses) > 0 {
whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
}

query := `
SELECT
COALESCE(SUM(CASE WHEN eff_status = 'paid' THEN amt ELSE 0 END), 0) AS total_collected,
COALESCE(SUM(CASE WHEN eff_status = 'pending' THEN amt ELSE 0 END), 0) AS total_pending,
COALESCE(SUM(CASE WHEN eff_status IN ('refunded','partially_refunded') THEN refunded_amt ELSE 0 END), 0) AS total_refunded,
COUNT(*) FILTER (WHERE eff_status = 'paid') AS count_paid,
COUNT(*) FILTER (WHERE eff_status = 'pending') AS count_pending,
COUNT(*) FILTER (WHERE eff_status = 'failed') AS count_failed,
COUNT(*) FILTER (WHERE eff_status IN ('refunded','partially_refunded')) AS count_refunded,
COALESCE(SUM(CASE WHEN eff_status = 'paid' AND payment_method = 'online' THEN amt ELSE 0 END), 0) AS online_collected,
COALESCE(SUM(CASE WHEN eff_status = 'paid' AND payment_method = 'cod' THEN amt ELSE 0 END), 0) AS cod_collected
FROM (
SELECT
o.payment_method,
COALESCE(p.amount, o.total_amount) AS amt,
COALESCE(p.refunded_amount, 0) AS refunded_amt,
CASE
WHEN p.id IS NOT NULL THEN (CASE WHEN p.status = 'created' THEN 'pending' ELSE p.status END)
ELSE o.payment_status
END AS eff_status
` + baseFrom + whereSQL + `
) sub
`

var summary models.AdminPaymentReconciliationSummary
if err := database.DB.Raw(query, args...).Scan(&summary).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compute reconciliation summary"})
return
}

c.JSON(http.StatusOK, summary)
}
