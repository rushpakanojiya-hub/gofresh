package handlers

import (
"math"
"net/http"
"time"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

func reportDateRange(c *gin.Context) (start, end time.Time, err error) {
fromStr := c.Query("from")
toStr := c.Query("to")
now := time.Now()
today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

var fromDay, toDay time.Time
if fromStr == "" {
fromDay = today.AddDate(0, 0, -29)
} else {
fromDay, err = time.Parse("2006-01-02", fromStr)
if err != nil {
return
}
}
if toStr == "" {
toDay = today
} else {
toDay, err = time.Parse("2006-01-02", toStr)
if err != nil {
return
}
}
start = fromDay
end = toDay.AddDate(0, 0, 1)
return
}

// GetSalesRegister godoc
// GET /api/v1/admin/reports/sales-register?from=YYYY-MM-DD&to=YYYY-MM-DD
// One row per invoice in range: full GST-relevant breakdown for a period,
// the standard "Sales Register" a CA expects (SRS 12.24).
func GetSalesRegister(c *gin.Context) {
start, end, err := reportDateRange(c)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
return
}
var invoices []models.Invoice
database.DB.Where("generated_at >= ? AND generated_at < ?", start, end).
Order("generated_at ASC").Find(&invoices)
c.JSON(http.StatusOK, gin.H{"from": start.Format("2006-01-02"), "to": end.AddDate(0, 0, -1).Format("2006-01-02"), "sales_register": invoices})
}

// GetPurchaseRegister godoc
// GET /api/v1/admin/reports/purchase-register?from=YYYY-MM-DD&to=YYYY-MM-DD
// One row per vendor bill in range: the "Purchase Register" counterpart
// to the Sales Register, for input GST claims (SRS 12.24).
func GetPurchaseRegister(c *gin.Context) {
start, end, err := reportDateRange(c)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
return
}
var bills []models.VendorBill
database.DB.Preload("Vendor").Where("bill_date >= ? AND bill_date < ?", start, end).
Order("bill_date ASC").Find(&bills)
c.JSON(http.StatusOK, gin.H{"from": start.Format("2006-01-02"), "to": end.AddDate(0, 0, -1).Format("2006-01-02"), "purchase_register": bills})
}

// GetRiderPayableReport godoc
// GET /api/v1/admin/reports/rider-payable?from=YYYY-MM-DD&to=YYYY-MM-DD
// Per delivery partner: count of deliveries completed in range and the
// resulting payable at the flat per-delivery rate (SRS 12.11, 12.24).
// There is no separate payout/settlement table yet, so "payable" here is
// the full computed amount, not netted against any prior payment.
func GetRiderPayableReport(c *gin.Context) {
start, end, err := reportDateRange(c)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
return
}

type row struct {
DeliveryPartnerID uint    `json:"delivery_partner_id"`
Name              string  `json:"name"`
Phone             string  `json:"phone"`
DeliveredCount    int64   `json:"delivered_count"`
Payable           float64 `json:"payable"`
}
var rows []row
const perDelivery = 30.0
database.DB.Table("orders").
Select("orders.delivery_partner_id, delivery_partners.name, delivery_partners.phone, COUNT(*) as delivered_count").
Joins("JOIN delivery_partners ON delivery_partners.id = orders.delivery_partner_id").
Where("orders.status = ? AND COALESCE(orders.delivered_at, orders.updated_at) >= ? AND COALESCE(orders.delivered_at, orders.updated_at) < ?", "delivered", start, end).
Group("orders.delivery_partner_id, delivery_partners.name, delivery_partners.phone").
Scan(&rows)
for i := range rows {
rows[i].Payable = float64(rows[i].DeliveredCount) * perDelivery
}
c.JSON(http.StatusOK, gin.H{"from": start.Format("2006-01-02"), "to": end.AddDate(0, 0, -1).Format("2006-01-02"), "per_delivery_rate": perDelivery, "rider_payable": rows})
}

// GetGatewaySettlementReport godoc
// GET /api/v1/admin/reports/gateway-settlement?from=YYYY-MM-DD&to=YYYY-MM-DD
// Gross amount collected via each payment gateway in range, and refunds
// issued against those payments (SRS 12.10, 12.24). Gateway fees are not
// currently captured anywhere in the system (no fee field on Payment), so
// "net_settlement" here is gross minus refunds only, not minus fees -
// noted explicitly rather than inventing a fee figure.
func GetGatewaySettlementReport(c *gin.Context) {
start, end, err := reportDateRange(c)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
return
}
type row struct {
Gateway        string  `json:"gateway"`
TransactionCount int64 `json:"transaction_count"`
GrossAmount    float64 `json:"gross_amount"`
RefundedAmount float64 `json:"refunded_amount"`
}
var rows []row
database.DB.Table("payments").
Select("gateway, COUNT(*) as transaction_count, COALESCE(SUM(amount),0) as gross_amount, COALESCE(SUM(refunded_amount),0) as refunded_amount").
Where("status = ? AND created_at >= ? AND created_at < ?", "paid", start, end).
Group("gateway").
Scan(&rows)

c.JSON(http.StatusOK, gin.H{
"from": start.Format("2006-01-02"), "to": end.AddDate(0, 0, -1).Format("2006-01-02"),
"note": "gateway fees are not tracked in this system; net_settlement is gross minus refunds only",
"gateway_settlement": rows,
})
}

// GetCashFlowReport godoc
// GET /api/v1/admin/reports/cash-flow?from=YYYY-MM-DD&to=YYYY-MM-DD
// Net movement through Cash (1001) and Bank (1002) ledger accounts in
// range, broken down by reference_type so each inflow/outflow category is
// visible (SRS 12.24).
func GetCashFlowReport(c *gin.Context) {
start, end, err := reportDateRange(c)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
return
}
type row struct {
ReferenceType string  `json:"reference_type"`
Inflow        float64 `json:"inflow"`
Outflow       float64 `json:"outflow"`
}
var rows []row
database.DB.Table("ledger_entries").
Joins("JOIN accounts ON accounts.id = ledger_entries.account_id").
Where("accounts.code IN ? AND ledger_entries.entry_date >= ? AND ledger_entries.entry_date < ?", []string{"1001", "1002"}, start, end).
Select(`ledger_entries.reference_type,
COALESCE(SUM(CASE WHEN ledger_entries.type = 'debit' THEN ledger_entries.amount ELSE 0 END),0) as inflow,
COALESCE(SUM(CASE WHEN ledger_entries.type = 'credit' THEN ledger_entries.amount ELSE 0 END),0) as outflow`).
Group("ledger_entries.reference_type").
Scan(&rows)

var netCashFlow float64
for _, r := range rows {
netCashFlow += r.Inflow - r.Outflow
}
c.JSON(http.StatusOK, gin.H{
"from": start.Format("2006-01-02"), "to": end.AddDate(0, 0, -1).Format("2006-01-02"),
"by_category": rows,
"net_cash_flow": netCashFlow,
})
}

// GetBalanceSheet godoc
// GET /api/v1/admin/reports/balance-sheet?as_of=YYYY-MM-DD
// Assets = Liabilities + Equity, as of a point in time (SRS 12.24). There
// is no dedicated Equity account seeded in the Chart of Accounts, so
// "Retained Earnings" is computed as cumulative (revenue - expense) from
// the ledger up to as_of, which is what makes the sheet balance -
// standard practice when a formal equity account hasn't been set up.
func GetBalanceSheet(c *gin.Context) {
asOfStr := c.Query("as_of")
asOf := time.Now()
if asOfStr != "" {
parsed, err := time.Parse("2006-01-02", asOfStr)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid as_of date format, use YYYY-MM-DD"})
return
}
asOf = parsed.AddDate(0, 0, 1)
}

type accountBalance struct {
Code    string  `json:"code"`
Name    string  `json:"name"`
Balance float64 `json:"balance"`
}

sumByType := func(accType string) ([]accountBalance, float64) {
var balances []accountBalance
database.DB.Table("accounts").
Select(`accounts.code, accounts.name,
COALESCE(SUM(CASE WHEN ledger_entries.type = 'debit' THEN ledger_entries.amount ELSE -ledger_entries.amount END),0) as balance`).
Joins("LEFT JOIN ledger_entries ON ledger_entries.account_id = accounts.id AND ledger_entries.entry_date < ?", asOf).
Where("accounts.type = ?", accType).
Group("accounts.code, accounts.name").
Order("accounts.code ASC").
Scan(&balances)
var total float64
for _, b := range balances {
total += b.Balance
}
return balances, total
}

assets, totalAssets := sumByType("asset")
liabilities, totalLiabilities := sumByType("liability")
_, totalRevenue := sumByType("revenue")
_, totalExpense := sumByType("expense")
// Liability accounts carry credit-natural balances - the opposite way
// round from assets - so normalize sign (same as retained earnings below)
// so a liability reads as a positive number when money is owed, and so
// Assets == Liabilities + Equity holds using positive-normal figures on
// both sides.
for i := range liabilities {
liabilities[i].Balance = -liabilities[i].Balance
}
totalLiabilities = -totalLiabilities
// Revenue/expense accounts carry credit/debit-natural balances the
// opposite way round from assets - normalize sign so retained earnings
// reads as a positive number when the business is profitable.
retainedEarnings := -totalRevenue - totalExpense

c.JSON(http.StatusOK, gin.H{
"as_of": asOf.AddDate(0, 0, -1).Format("2006-01-02"),
"assets": gin.H{"accounts": assets, "total": totalAssets},
"liabilities": gin.H{"accounts": liabilities, "total": totalLiabilities},
"equity": gin.H{"retained_earnings": retainedEarnings, "total": retainedEarnings},
"balances": math.Abs(totalAssets-(totalLiabilities+retainedEarnings)) < 0.01,
})
}

