package handlers

import (
"errors"
"log"
"net/http"
"strconv"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
"gorm.io/gorm"
"gorm.io/gorm/clause"
)

// freeDeliveryThreshold and flatDeliveryCharge implement a simple, common
// Indian-ecommerce delivery pricing rule: free delivery above the threshold,
// otherwise a flat charge. Swap for a real shipping/rate-card service later.
var errOrderAlreadyCancelled = errors.New("order already cancelled or no longer cancellable")

const (
freeDeliveryThreshold = 500.0
flatDeliveryCharge    = 50.0
)

// Checkout godoc
// POST /api/v1/orders/checkout (protected)
// Converts the user's current cart into an order: finds the nearest
// serviceable warehouse for the delivery address, validates + deducts
// stock from that warehouse specifically, snapshots prices, applies a
// coupon if given, creates the order + order items, and empties the cart -
// all inside a single DB transaction so a failure partway through never
// leaves stock, coupon usage, or the cart in a bad state.
func Checkout(c *gin.Context) {
userID := c.MustGet("user_id").(uint)

var req models.CheckoutRequest
// Body is optional - an empty/missing body just means "use default address + COD".
_ = c.ShouldBindJSON(&req)

paymentMethod := req.PaymentMethod
if paymentMethod == "" {
paymentMethod = models.PaymentMethodCOD
}

// Resolve the delivery address: explicit address_id, else the user's default.
var address models.Address
if req.AddressID != 0 {
if err := database.DB.First(&address, req.AddressID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
return
}
if address.UserID != userID {
c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this address"})
return
}
} else {
if err := database.DB.Where("user_id = ? AND is_default = ?", userID, true).First(&address).Error; err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "No address_id given and no default address saved. Add a delivery address first."})
return
}
}

// Find the nearest serviceable warehouse for this address BEFORE doing
// any cart/stock work, so an out-of-range order fails fast with a clear
// message instead of touching inventory or creating an order row.
if address.Lat == nil || address.Lng == nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "This address is missing location coordinates. Please re-save it with location enabled."})
return
}
nearestWarehouse, distance, err := FindNearestWarehouse(*address.Lat, *address.Lng)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check delivery serviceability"})
return
}
if nearestWarehouse == nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Sorry, no active warehouses are available right now"})
return
}
if distance > nearestWarehouse.ServiceRadiusKm {
c.JSON(http.StatusBadRequest, gin.H{"error": "Sorry, we don't deliver to this location yet"})
return
}

cart, err := getOrCreateCart(userID)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
return
}

var cartItems []models.CartItem
if err := database.DB.Preload("Product").Where("cart_id = ?", cart.ID).Find(&cartItems).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart items"})
return
}
if len(cartItems) == 0 {
c.JSON(http.StatusBadRequest, gin.H{"error": "Your cart is empty"})
return
}

var order models.Order

txErr := database.DB.Transaction(func(tx *gorm.DB) error {
itemsAmount := 0.0
orderItems := make([]models.OrderItem, 0, len(cartItems))
			pendingMovements := make([]models.StockMovement, 0, len(cartItems))

for _, ci := range cartItems {
var inventory models.Inventory
// Lock this product's inventory row AT the assigned warehouse for
// the rest of the transaction, so two concurrent checkouts on the
// same product/warehouse can't both read the same stock, both
// pass the check, and both deduct beyond what's available.
err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
Where("product_id = ? AND warehouse_id = ?", ci.ProductID, nearestWarehouse.ID).
First(&inventory).Error
if err != nil {
return errors.New("this product is not available for purchase at your nearest warehouse: " + ci.Product.Name)
}
if !inventory.InStock || inventory.Stock < ci.Quantity {
return errors.New("insufficient stock for " + ci.Product.Name)
}
				previousQty := inventory.Stock

inventory.Stock -= ci.Quantity
if inventory.Stock <= 0 {
inventory.InStock = false
}
if err := tx.Save(&inventory).Error; err != nil {
return err
}
				pendingMovements = append(pendingMovements, models.StockMovement{
					ProductID:    ci.ProductID,
					WarehouseID:  nearestWarehouse.ID,
					PreviousQty:  previousQty,
					Change:       -ci.Quantity,
					NewQty:       inventory.Stock,
					MovementType: models.MovementSale,
				})

itemsAmount += ci.Product.Price * float64(ci.Quantity)
orderItems = append(orderItems, models.OrderItem{
ProductID: ci.ProductID,
Quantity:  ci.Quantity,
Price:     ci.Product.Price,
})
}

serviceability := services.CalculateServiceability(address.Pincode, address.Lat, address.Lng)
if paymentMethod == models.PaymentMethodCOD && !serviceability.CODAvailable {
return errors.New("Cash on Delivery is not available for this address")
}
deliveryCharge := serviceability.DeliveryCharge
if itemsAmount >= freeDeliveryThreshold {
deliveryCharge = 0
}

// Platform fee: flat fee on every order (not waived above the free-delivery
// threshold). Admin-configurable via PUT /admin/settings ("platform_fee"),
// defaults to Rs.5.
platformFee := utils.GetSettingFloat("platform_fee", 5.0)

// Coupon: validated against itemsAmount (before delivery charge),
// using the transaction so the read is consistent with everything
// else happening in this checkout.
var appliedCoupon *models.Coupon
var discount float64
if req.CouponCode != "" {
coupon, d, err := ValidateCoupon(tx, req.CouponCode, itemsAmount, userID)
if err != nil {
return err
}
appliedCoupon = coupon
discount = d
}

totalBeforeWallet := itemsAmount + deliveryCharge + platformFee - discount
walletUsed := 0.0
if req.UseWallet {
userWallet, werr := utils.GetOrCreateWallet(tx, userID)
if werr != nil {
return werr
}
if userWallet.Balance > 0 {
walletUsed = userWallet.Balance
if walletUsed > totalBeforeWallet {
walletUsed = totalBeforeWallet
}
}
}
finalTotal := totalBeforeWallet - walletUsed
orderStatus := models.OrderStatusPending
paymentStatus := models.OrderPaymentStatusPending
if finalTotal <= 0 {
paymentStatus = models.OrderPaymentStatusPaid
orderStatus = models.OrderStatusConfirmed
} else if paymentMethod == models.PaymentMethodCOD {
orderStatus = models.OrderStatusConfirmed
}

warehouseID := nearestWarehouse.ID
order = models.Order{
UserID:           userID,
AddressID:        address.ID,
WarehouseID:      &warehouseID,
ItemsAmount:      itemsAmount,
DeliveryCharge:   deliveryCharge,
PlatformFee:      platformFee,
WalletAmountUsed: walletUsed,
TotalAmount:      finalTotal,
Status:           orderStatus,
PaymentMethod:    paymentMethod,
PaymentStatus:    paymentStatus,
Items:            orderItems,
}
if err := tx.Create(&order).Error; err != nil {
return err
}
			for i := range pendingMovements {
				pendingMovements[i].ReferenceID = &order.ID
			}
			if len(pendingMovements) > 0 {
				if err := tx.Create(&pendingMovements).Error; err != nil {
					return err
				}
			}

if walletUsed > 0 {
refID := order.ID
if err := utils.DebitWallet(tx, userID, walletUsed, models.WalletReasonCheckoutUse, "order", &refID, ""); err != nil {
return err
}
}

// Record coupon usage only after the order is successfully created,
// inside the same transaction - if anything above fails, the coupon
// use rolls back too, so it never gets burned on a failed checkout.
if appliedCoupon != nil {
if err := ApplyCoupon(tx, appliedCoupon, order.ID, discount); err != nil {
return err
}
}

if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
return err
}

// The real stock deduction above (under its own row lock) is now the
// source of truth, so any cart holds this user had - for these items
// or anything else left over - no longer serve a purpose.
if err := services.ReleaseAllUserReservations(tx, userID); err != nil {
return err
}

return nil
})

if txErr != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": txErr.Error()})
return
}

database.DB.Preload("Items.Product").Preload("Address").Preload("Warehouse").First(&order, order.ID)

message := "Your order #" + strconv.Itoa(int(order.ID)) + " has been placed successfully!"
utils.SendNotification(order.Address.Phone, message, "order_placed", &order.ID)
services.SendPushToUser(order.UserID, "Order Placed", message)
if order.Status == models.OrderStatusConfirmed {
                // Order confirmed at checkout time (COD, or zero-total) -
                // try to auto-assign the nearest available delivery
                // partner right away instead of waiting for warehouse
                // packing to complete.
                go services.AutoAssignDeliveryPartner(order.ID)
if order.WarehouseID != nil {
services.NotifyWarehouse(*order.WarehouseID, models.WhNotifyNewOrder,
"New order #"+strconv.Itoa(int(order.ID)),
"A new order has been confirmed and is ready to accept.", &order.ID, nil)
}
// COD orders are billable immediately - no separate payment-confirmation
// step is coming, so the invoice must be generated here rather than
// waiting for something that will never happen for COD.
if order.PaymentMethod == models.PaymentMethodCOD {
if _, err := services.GenerateInvoiceIfNotExists(order.ID); err != nil {
log.Printf("failed to generate invoice for COD order %d: %v", order.ID, err)
}
}
}

c.JSON(http.StatusCreated, order)
}

// GetCheckoutEstimate godoc
// GET /api/v1/orders/checkout/estimate?address_id=X (protected)
// Returns items_amount, delivery_charge, platform_fee, and the resulting
// estimated_total for the caller's current cart against a given delivery
// address, computed with the exact same rules Checkout uses, so the
// frontend can show an accurate total before placing the order instead of
// only summing item subtotals.
func GetCheckoutEstimate(c *gin.Context) {
    userID := c.MustGet("user_id").(uint)

    addressIDStr := c.Query("address_id")
    var address models.Address
    if addressIDStr != "" {
        if err := database.DB.First(&address, addressIDStr).Error; err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
            return
        }
        if address.UserID != userID {
            c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this address"})
            return
        }
    } else {
        if err := database.DB.Where("user_id = ? AND is_default = ?", userID, true).First(&address).Error; err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "No address_id given and no default address saved."})
            return
        }
    }

    cart, err := getOrCreateCart(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
        return
    }
    var cartItems []models.CartItem
    if err := database.DB.Preload("Product").Where("cart_id = ?", cart.ID).Find(&cartItems).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart items"})
        return
    }

    itemsAmount := 0.0
    for _, ci := range cartItems {
        itemsAmount += ci.Product.Price * float64(ci.Quantity)
    }

    serviceability := services.CalculateServiceability(address.Pincode, address.Lat, address.Lng)
    deliveryCharge := serviceability.DeliveryCharge
    if itemsAmount >= freeDeliveryThreshold {
        deliveryCharge = 0
    }
    platformFee := utils.GetSettingFloat("platform_fee", 5.0)

    estimatedTotal := itemsAmount + deliveryCharge + platformFee

    c.JSON(http.StatusOK, gin.H{
        "items_amount":    itemsAmount,
        "delivery_charge": deliveryCharge,
        "platform_fee":    platformFee,
        "estimated_total": estimatedTotal,
        "cod_available":   serviceability.CODAvailable,
        "estimated_days":  serviceability.EstimatedDays,
        "serviceable":     serviceability.Serviceable,
        "zone_id":         serviceability.ZoneID,
    })
}

// GetOrders godoc
// GET /api/v1/orders (protected) - ?page=&limit=
func GetOrders(c *gin.Context) {
userID := c.MustGet("user_id").(uint)

page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
if page < 1 {
page = 1
}
if limit < 1 || limit > 100 {
limit = 20
}

var total int64
database.DB.Model(&models.Order{}).Where("user_id = ?", userID).Count(&total)

var orders []models.Order
if err := database.DB.
Preload("Items.Product").
Preload("Address").
Where("user_id = ?", userID).
Order("created_at DESC").
Offset((page - 1) * limit).
Limit(limit).
Find(&orders).Error; err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load orders"})
return
}

// Batch-fetch coupon discounts for all orders on this page in one query
// (instead of N+1) and attach them to each order.
if len(orders) > 0 {
orderIDs := make([]uint, len(orders))
for i, o := range orders {
orderIDs[i] = o.ID
}
var orderCoupons []models.OrderCoupon
database.DB.Where("order_id IN ?", orderIDs).Find(&orderCoupons)
discountByOrderID := make(map[uint]float64, len(orderCoupons))
for _, oc := range orderCoupons {
discountByOrderID[oc.OrderID] = oc.DiscountAmount
}
for i := range orders {
orders[i].CouponDiscount = discountByOrderID[orders[i].ID]
}
}

totalPages := int((total + int64(limit) - 1) / int64(limit))
c.JSON(http.StatusOK, models.OrderListResponse{
Orders:     orders,
Page:       page,
Limit:      limit,
Total:      total,
TotalPages: totalPages,
})
}

// GetOrderByID godoc
// GET /api/v1/orders/:id (protected)
func GetOrderByID(c *gin.Context) {
userID := c.MustGet("user_id").(uint)
orderID := c.Param("id")

var order models.Order
if err := database.DB.Preload("Items.Product").Preload("Address").Preload("DeliveryPartner").First(&order, orderID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
return
}
if order.UserID != userID {
c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this order"})
return
}

var orderCoupon models.OrderCoupon
if err := database.DB.Where("order_id = ?", order.ID).First(&orderCoupon).Error; err == nil {
order.CouponDiscount = orderCoupon.DiscountAmount
}

c.JSON(http.StatusOK, order)
}

// CancelOrder godoc
// PUT /api/v1/orders/:id/cancel (protected)
// Only pending or confirmed orders can be cancelled; stock is restored to
// the warehouse the order was fulfilled from.
func CancelOrder(c *gin.Context) {
userID := c.MustGet("user_id").(uint)
orderID := c.Param("id")

var order models.Order
if err := database.DB.Preload("Items.Product").Preload("Address").Preload("DeliveryPartner").First(&order, orderID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
return
}
if order.UserID != userID {
c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this order"})
return
}
if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusConfirmed {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending or confirmed orders can be cancelled"})
return
}

var payment models.Payment
var gatewayRefundAmount float64

txErr := database.DB.Transaction(func(tx *gorm.DB) error {
// Re-fetch the order under a row-level lock so two concurrent cancel
// requests for the same order cannot both pass the earlier status
// check and both restock inventory / refund wallet / refund gateway.
// The second request blocks here until the first commits, then sees
// the now-cancelled status and bails out cleanly instead of
// double-processing the refund.
var lockedOrder models.Order
if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedOrder, order.ID).Error; err != nil {
return err
}
if lockedOrder.Status != models.OrderStatusPending && lockedOrder.Status != models.OrderStatusConfirmed {
return errOrderAlreadyCancelled
}

for _, item := range order.Items {
var inventory models.Inventory
q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id = ?", item.ProductID)
if order.WarehouseID != nil {
// Known-good case: restore to the exact warehouse this order
// was fulfilled from.
q = q.Where("warehouse_id = ?", *order.WarehouseID)
}
// Legacy fallback: older orders placed before warehouse routing
// don't have a WarehouseID, so restore to the first available row
// for this product - the combined total across warehouses ends
// up correct either way.
if err := q.Order("id").First(&inventory).Error; err == nil {
previousQty := inventory.Stock
inventory.Stock += item.Quantity
inventory.InStock = true
if err := tx.Save(&inventory).Error; err != nil {
return err
}
movement := models.StockMovement{
ProductID:    item.ProductID,
WarehouseID:  inventory.WarehouseID,
PreviousQty:  previousQty,
Change:       item.Quantity,
NewQty:       inventory.Stock,
MovementType: models.MovementReturn,
Reason:       "Order cancelled",
ReferenceID:  &order.ID,
}
if err := tx.Create(&movement).Error; err != nil {
return err
}
}
}
if order.WalletAmountUsed > 0 {
refID := order.ID
if err := utils.CreditWallet(tx, userID, order.WalletAmountUsed, models.WalletReasonOrderRefund, "order", &refID, "Refund for cancelled order"); err != nil {
return err
}
}

// Reinstate the coupon (if any) so a single-use coupon isn't permanently
// burned by a cancelled order. OrderCoupon itself is left in place as a
// historical record of what was applied.
var orderCoupon models.OrderCoupon
if err := tx.Where("order_id = ?", order.ID).First(&orderCoupon).Error; err == nil {
if err := tx.Model(&models.Coupon{}).Where("id = ? AND used_count > 0", orderCoupon.CouponID).
UpdateColumn("used_count", gorm.Expr("used_count - 1")).Error; err != nil {
return err
}
}

// Refund the online-gateway portion of the payment (if any) back to
// the customer's wallet, since there is no live Razorpay refund
// integration wired up yet. Only applies to orders that actually
// reached "paid" - unpaid/failed online orders have nothing to refund.
if order.PaymentMethod == models.PaymentMethodOnline {
if err := tx.Where("order_id = ?", order.ID).First(&payment).Error; err == nil {
if payment.Status == models.PaymentStatusPaid {
gatewayRefundAmount = payment.Amount - payment.RefundedAmount
if gatewayRefundAmount > 0 {
refID := order.ID
if err := utils.CreditWallet(tx, userID, gatewayRefundAmount, models.WalletReasonOrderRefund, "order", &refID, "Gateway refund for cancelled order"); err != nil {
return err
}
payment.RefundedAmount = payment.Amount
payment.Status = models.PaymentStatusRefunded
if err := tx.Save(&payment).Error; err != nil {
return err
}
}
}
}
}

return tx.Model(&order).Updates(map[string]interface{}{
"status":                      models.OrderStatusCancelled,
"payment_status":              models.OrderPaymentStatusRefunded,
"delivery_partner_id":         nil,
"delivery_assignment_status":  nil,
"delivery_status":             nil,
"delivery_assignment_expires_at": nil,
}).Error
})
if txErr != nil {
if errors.Is(txErr, errOrderAlreadyCancelled) {
c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending or confirmed orders can be cancelled"})
return
}
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
return
}

totalWalletRefund := gatewayRefundAmount + order.WalletAmountUsed
if totalWalletRefund > 0 {
if err := services.PostWalletRefundLedgerEntry(order.ID, totalWalletRefund); err != nil {
log.Printf("failed to post refund ledger entry for order %d: %v", order.ID, err)
}
}

order.Status = models.OrderStatusCancelled

message := "Your order #" + orderID + " has been cancelled."
utils.SendNotification(order.Address.Phone, message, "order_cancelled", &order.ID)
services.SendPushToUser(order.UserID, "Order Cancelled", message)
if order.WarehouseID != nil {
services.NotifyWarehouse(*order.WarehouseID, models.WhNotifyOrderCancelled,
"Order #"+orderID+" cancelled",
"Customer cancelled this order before it left the warehouse.", &order.ID, nil)
}

// The delivery partner (if one had already been assigned before
// cancellation) is no longer responsible for this order - the DB
// fields were cleared above, but this in-memory order snapshot still
// has the pre-cancellation partner, so use it to notify them.
if order.DeliveryPartner != nil {
utils.SendNotification(order.DeliveryPartner.Phone, "Order #"+orderID+" was cancelled by the customer and is no longer assigned to you.", "delivery_order_cancelled", &order.ID)
}
c.JSON(http.StatusOK, order)
}




