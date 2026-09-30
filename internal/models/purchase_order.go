package models

import "time"

// PurchaseOrder is an admin-created order to a vendor/supplier for
// restocking a warehouse (typically the mother warehouse). It is the
// upstream request; the actual GRN/batch/expiry tracking happens in the
// warehouse receiving flow once goods arrive.
type PurchaseOrder struct {
ID               uint                 `gorm:"primaryKey" json:"id"`
PONumber         string               `gorm:"not null" json:"po_number"`
VendorID         uint                 `gorm:"not null" json:"vendor_id"`
Vendor           Vendor               `gorm:"foreignKey:VendorID" json:"vendor,omitempty"`
WarehouseID      uint                 `gorm:"not null" json:"warehouse_id"`
Warehouse        Warehouse            `gorm:"foreignKey:WarehouseID" json:"warehouse,omitempty"`
Status           string               `gorm:"default:draft" json:"status"` // draft|sent|confirmed|partially_received|received|cancelled
ExpectedDate     *time.Time           `json:"expected_date,omitempty"`
Notes            string               `json:"notes,omitempty"`
TotalAmount      float64              `gorm:"default:0" json:"total_amount"`
CreatedByID      uint                 `gorm:"not null" json:"created_by_id"`
ApprovedByID     *uint                `json:"approved_by_id,omitempty"`
ApprovedAt       *time.Time           `json:"approved_at,omitempty"`
CancelledReason  string               `json:"cancelled_reason,omitempty"`
Items            []PurchaseOrderItem  `gorm:"foreignKey:PurchaseOrderID" json:"items,omitempty"`
CreatedAt        time.Time            `json:"created_at"`
UpdatedAt        time.Time            `json:"updated_at"`
}

// PurchaseOrderItem is one product line on a purchase order.
type PurchaseOrderItem struct {
ID                uint      `gorm:"primaryKey" json:"id"`
PurchaseOrderID   uint      `gorm:"not null" json:"purchase_order_id"`
ProductID         uint      `gorm:"not null" json:"product_id"`
Product           Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
QuantityOrdered   int       `gorm:"not null" json:"quantity_ordered"`
QuantityReceived  int       `gorm:"default:0" json:"quantity_received"`
UnitPrice         float64   `gorm:"default:0" json:"unit_price"`
CreatedAt         time.Time `json:"created_at"`
UpdatedAt         time.Time `json:"updated_at"`
}

// PurchaseOrderItemRequest is one line item in a create/update PO request.
type PurchaseOrderItemRequest struct {
ProductID       uint    `json:"product_id" binding:"required"`
QuantityOrdered int     `json:"quantity_ordered" binding:"required,gt=0"`
UnitPrice       float64 `json:"unit_price"`
}

// PurchaseOrderRequest is the body for POST /admin/procurement/purchase-orders
type PurchaseOrderRequest struct {
VendorID     uint                       `json:"vendor_id" binding:"required"`
WarehouseID  uint                       `json:"warehouse_id" binding:"required"`
ExpectedDate string                     `json:"expected_date"`
Notes        string                     `json:"notes"`
Items        []PurchaseOrderItemRequest `json:"items" binding:"required,min=1"`
}

// PurchaseOrderStatusRequest is the body for PUT .../status
type PurchaseOrderStatusRequest struct {
Status string `json:"status" binding:"required"`
}

// ReceivePurchaseOrderItemsRequest records partial/full receipt of PO items.
type ReceivePurchaseOrderItemsRequest struct {
Items []struct {
ItemID           uint `json:"item_id" binding:"required"`
QuantityReceived int  `json:"quantity_received" binding:"required,gt=0"`
} `json:"items" binding:"required,min=1"`
}

// ReplenishmentSuggestion is one product that has fallen at/below its
// reorder level (or the global low-stock threshold if not configured),
// with a suggested reorder quantity to bring it back to a healthy level.
type ReplenishmentSuggestion struct {
ProductID      uint   `json:"product_id"`
ProductName    string `json:"product_name"`
WarehouseID    uint   `json:"warehouse_id"`
WarehouseName  string `json:"warehouse_name"`
CurrentStock   int    `json:"current_stock"`
ReorderLevel   int    `json:"reorder_level"`
SuggestedOrder int    `json:"suggested_order_qty"`
}
