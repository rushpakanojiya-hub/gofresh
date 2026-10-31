package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/xuri/excelize/v2"
)

// parseReportDateRange reads from/to query params (YYYY-MM-DD) and returns
// a half-open [start, end) window plus a human label for filenames/sheets.
// Defaults to the last 7 days (inclusive) when no params are given, since
// this endpoint exists for weekly/custom-range reporting.
func parseReportDateRange(c *gin.Context) (start, end time.Time, label string, err error) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	var fromDay, toDay time.Time
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if fromStr == "" {
		fromDay = today.AddDate(0, 0, -6)
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
	label = fmt.Sprintf("%s_to_%s", fromDay.Format("2006-01-02"), toDay.Format("2006-01-02"))
	return
}

type RangeSalesSummary struct {
	From             string  `json:"from"`
	To               string  `json:"to"`
	TotalOrders      int64   `json:"total_orders"`
	DeliveredOrders  int64   `json:"delivered_orders"`
	CancelledOrders  int64   `json:"cancelled_orders"`
	PendingOrders    int64   `json:"pending_orders"`
	TotalRevenue     float64 `json:"total_revenue"`
	CODRevenue       float64 `json:"cod_revenue"`
	OnlineRevenue    float64 `json:"online_revenue"`
	TotalDeliveryFee float64 `json:"total_delivery_charge"`
	TotalWalletUsed  float64 `json:"total_wallet_used"`
	AvgOrderValue    float64 `json:"avg_order_value"`
	TaxableAmount    float64 `json:"taxable_amount"`
	CGSTAmount       float64 `json:"cgst_amount"`
	SGSTAmount       float64 `json:"sgst_amount"`
	IGSTAmount       float64 `json:"igst_amount"`
	TotalOutputGST   float64 `json:"total_output_gst"`
	TotalVendorGST   float64 `json:"total_vendor_gst"`
}

func buildRangeSalesSummary(start, end time.Time, fromLabel, toLabel string) RangeSalesSummary {
	var s RangeSalesSummary
	s.From = fromLabel
	s.To = toLabel
	f := "created_at >= ? AND created_at < ?"

	database.DB.Model(&models.Order{}).Where(f, start, end).Count(&s.TotalOrders)
	database.DB.Model(&models.Order{}).Where(f+" AND status = ?", start, end, "delivered").Count(&s.DeliveredOrders)
	database.DB.Model(&models.Order{}).Where(f+" AND status = ?", start, end, "cancelled").Count(&s.CancelledOrders)
	database.DB.Model(&models.Order{}).Where(f+" AND status = ?", start, end, "pending").Count(&s.PendingOrders)

	database.DB.Model(&models.Order{}).
		Where(f+" AND status NOT IN (?, ?)", start, end, "cancelled", "returned").
		Select("COALESCE(SUM(CASE WHEN payment_method = 'online' AND payment_status <> 'paid' THEN 0 WHEN payment_method = 'cod' AND status <> 'delivered' THEN 0 ELSE total_amount + wallet_amount_used END), 0)").Scan(&s.TotalRevenue)

	database.DB.Model(&models.Order{}).
		Where(f+" AND status = ? AND payment_method = ?", start, end, "delivered", "cod").
		Select("COALESCE(SUM(total_amount + wallet_amount_used), 0)").Scan(&s.CODRevenue)

	database.DB.Model(&models.Order{}).
		Where(f+" AND status NOT IN (?, ?) AND payment_method = ?", start, end, "cancelled", "returned", "online").
		Select("COALESCE(SUM(CASE WHEN payment_method = 'online' AND payment_status <> 'paid' THEN 0 ELSE total_amount + wallet_amount_used END), 0)").Scan(&s.OnlineRevenue)

	database.DB.Model(&models.Order{}).
		Where(f+" AND status NOT IN (?, ?)", start, end, "cancelled", "returned").
		Select("COALESCE(SUM(delivery_charge), 0)").Scan(&s.TotalDeliveryFee)

	database.DB.Model(&models.Order{}).
		Where(f+" AND status NOT IN (?, ?)", start, end, "cancelled", "returned").
		Select("COALESCE(SUM(CASE WHEN payment_method = 'online' AND payment_status <> 'paid' THEN 0 WHEN payment_method = 'cod' AND status <> 'delivered' THEN 0 ELSE wallet_amount_used END), 0)").Scan(&s.TotalWalletUsed)

	var nonCancelled int64
	database.DB.Model(&models.Order{}).Where(f+" AND status NOT IN (?, ?) AND NOT (payment_method = 'online' AND payment_status <> 'paid') AND NOT (payment_method = 'cod' AND status <> 'delivered')", start, end, "cancelled", "returned").Count(&nonCancelled)
	if nonCancelled > 0 {
		s.AvgOrderValue = s.TotalRevenue / float64(nonCancelled)
	}

	type gstTotals struct {
		TaxableAmount float64
		CGSTAmount    float64
		SGSTAmount    float64
		IGSTAmount    float64
	}
	var g gstTotals
	database.DB.Table("invoices").
		Where("generated_at >= ? AND generated_at < ?", start, end).
		Select("COALESCE(SUM(taxable_amount),0) as taxable_amount, COALESCE(SUM(cgst_amount),0) as cgst_amount, COALESCE(SUM(sgst_amount),0) as sgst_amount, COALESCE(SUM(igst_amount),0) as igst_amount").
		Scan(&g)
	s.TaxableAmount = g.TaxableAmount
	s.CGSTAmount = g.CGSTAmount
	s.SGSTAmount = g.SGSTAmount
	s.IGSTAmount = g.IGSTAmount
	s.TotalOutputGST = g.CGSTAmount + g.SGSTAmount + g.IGSTAmount

	database.DB.Model(&models.VendorBill{}).
		Where("bill_date >= ? AND bill_date < ? AND voided_at IS NULL", start, end).
		Select("COALESCE(SUM(gst_amount), 0)").Scan(&s.TotalVendorGST)

	return s
}

// GetRangeSalesReport godoc
// GET /api/v1/admin/reports/range-sales?from=YYYY-MM-DD&to=YYYY-MM-DD
func GetRangeSalesReport(c *gin.Context) {
	start, end, _, err := parseReportDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
		return
	}
	fromLabel := start.Format("2006-01-02")
	toLabel := end.AddDate(0, 0, -1).Format("2006-01-02")
	c.JSON(http.StatusOK, gin.H{"summary": buildRangeSalesSummary(start, end, fromLabel, toLabel)})
}

// ExportRangeSalesReport godoc
// GET /api/v1/admin/reports/range-sales/export?from=YYYY-MM-DD&to=YYYY-MM-DD
// Builds a multi-sheet Excel workbook covering a date range, meant to be
// handed to the company's CA: Summary, Orders, GST By Rate, GST By HSN,
// and Vendor GST (purchase-side GST paid to vendors) for that period.
func ExportRangeSalesReport(c *gin.Context) {
	start, end, _, err := parseReportDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
		return
	}
	fromLabel := start.Format("2006-01-02")
	toLabel := end.AddDate(0, 0, -1).Format("2006-01-02")

	summary := buildRangeSalesSummary(start, end, fromLabel, toLabel)

	var orders []models.Order
	database.DB.
		Where("created_at >= ? AND created_at < ?", start, end).
		Order("created_at ASC").
		Preload("User").
		Find(&orders)

	type hsnRow struct {
		HSNCode       string  `json:"hsn_code"`
		TaxableAmount float64 `json:"taxable_amount"`
		GSTAmount     float64 `json:"gst_amount"`
		Quantity      int64   `json:"quantity"`
	}
	var byHSN []hsnRow
	database.DB.Table("invoice_items").
		Joins("JOIN invoices ON invoices.id = invoice_items.invoice_id").
		Where("invoices.generated_at >= ? AND invoices.generated_at < ?", start, end).
		Select("COALESCE(NULLIF(invoice_items.hsn_code, ''), 'Not set') as hsn_code, COALESCE(SUM(invoice_items.price * invoice_items.quantity - invoice_items.gst_amount),0) as taxable_amount, COALESCE(SUM(invoice_items.gst_amount),0) as gst_amount, COALESCE(SUM(invoice_items.quantity),0) as quantity").
		Group("COALESCE(NULLIF(invoice_items.hsn_code, ''), 'Not set')").
		Order("gst_amount DESC").
		Scan(&byHSN)

	type rateRow struct {
		GSTPercent    float64 `json:"gst_percent"`
		TaxableAmount float64 `json:"taxable_amount"`
		GSTAmount     float64 `json:"gst_amount"`
	}
	var byRate []rateRow
	database.DB.Table("invoice_items").
		Joins("JOIN invoices ON invoices.id = invoice_items.invoice_id").
		Where("invoices.generated_at >= ? AND invoices.generated_at < ?", start, end).
		Select("invoice_items.gst_percent, COALESCE(SUM(invoice_items.price * invoice_items.quantity - invoice_items.gst_amount),0) as taxable_amount, COALESCE(SUM(invoice_items.gst_amount),0) as gst_amount").
		Group("invoice_items.gst_percent").
		Order("invoice_items.gst_percent ASC").
		Scan(&byRate)

	var vendorBills []models.VendorBill
	database.DB.
		Where("bill_date >= ? AND bill_date < ? AND voided_at IS NULL", start, end).
		Preload("Vendor").
		Order("bill_date ASC").
		Find(&vendorBills)

	xf := excelize.NewFile()
	defer xf.Close()

	summarySheet := "Summary"
	xf.SetSheetName("Sheet1", summarySheet)
	summaryHeader := []interface{}{
		"From", "To", "Total Orders", "Delivered", "Cancelled", "Pending",
		"Total Revenue", "COD Revenue", "Online Revenue", "Delivery Charges", "Wallet Used", "Avg Order Value",
		"Taxable Amount", "CGST", "SGST", "IGST", "Total Output GST (Sales)", "Total Vendor GST (Purchases)",
	}
	summaryDataRow := []interface{}{
		summary.From, summary.To, summary.TotalOrders, summary.DeliveredOrders, summary.CancelledOrders, summary.PendingOrders,
		summary.TotalRevenue, summary.CODRevenue, summary.OnlineRevenue, summary.TotalDeliveryFee, summary.TotalWalletUsed, summary.AvgOrderValue,
		summary.TaxableAmount, summary.CGSTAmount, summary.SGSTAmount, summary.IGSTAmount, summary.TotalOutputGST, summary.TotalVendorGST,
	}
	xf.SetSheetRow(summarySheet, "A1", &summaryHeader)
	xf.SetSheetRow(summarySheet, "A2", &summaryDataRow)
	for col := 'A'; col <= 'R'; col++ {
		xf.SetColWidth(summarySheet, string(col), string(col), 20)
	}

	ordersSheet := "Orders"
	xf.NewSheet(ordersSheet)
	ordersHeader := []interface{}{"Order ID", "Date", "Customer Name", "Customer Phone", "Items Amount", "Delivery Charge", "Wallet Used", "Total Amount", "Payment Method", "Payment Status", "Order Status"}
	xf.SetSheetRow(ordersSheet, "A1", &ordersHeader)
	for i, o := range orders {
		row := []interface{}{
			o.ID, o.CreatedAt.Format("2006-01-02 15:04:05"), o.User.Name, o.User.Phone,
			o.ItemsAmount, o.DeliveryCharge, o.WalletAmountUsed, o.TotalAmount,
			o.PaymentMethod, o.PaymentStatus, o.Status,
		}
		r := row
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		xf.SetSheetRow(ordersSheet, cell, &r)
	}
	for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"} {
		xf.SetColWidth(ordersSheet, col, col, 18)
	}

	gstRateSheet := "GST By Rate"
	xf.NewSheet(gstRateSheet)
	xf.SetSheetRow(gstRateSheet, "A1", &[]interface{}{"GST Rate (%)", "Taxable Amount", "GST Amount"})
	for i, r := range byRate {
		row := []interface{}{r.GSTPercent, r.TaxableAmount, r.GSTAmount}
		rr := row
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		xf.SetSheetRow(gstRateSheet, cell, &rr)
	}
	for _, col := range []string{"A", "B", "C"} {
		xf.SetColWidth(gstRateSheet, col, col, 18)
	}

	gstHsnSheet := "GST By HSN"
	xf.NewSheet(gstHsnSheet)
	xf.SetSheetRow(gstHsnSheet, "A1", &[]interface{}{"HSN Code", "Quantity", "Taxable Amount", "GST Amount"})
	for i, r := range byHSN {
		row := []interface{}{r.HSNCode, r.Quantity, r.TaxableAmount, r.GSTAmount}
		rr := row
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		xf.SetSheetRow(gstHsnSheet, cell, &rr)
	}
	for _, col := range []string{"A", "B", "C", "D"} {
		xf.SetColWidth(gstHsnSheet, col, col, 18)
	}

	vendorGstSheet := "Vendor GST"
	xf.NewSheet(vendorGstSheet)
	xf.SetSheetRow(vendorGstSheet, "A1", &[]interface{}{"Vendor Name", "Bill Number", "Bill Date", "Amount", "GST Amount", "Amount Paid"})
	for i, b := range vendorBills {
		vendorName := ""
		if b.Vendor.ID != 0 {
			vendorName = b.Vendor.Name
		}
		row := []interface{}{vendorName, b.BillNumber, b.BillDate.Format("2006-01-02"), b.Amount, b.GSTAmount, b.AmountPaid}
		rr := row
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		xf.SetSheetRow(vendorGstSheet, cell, &rr)
	}
	for _, col := range []string{"A", "B", "C", "D", "E", "F"} {
		xf.SetColWidth(vendorGstSheet, col, col, 18)
	}

	xf.SetActiveSheet(0)

	filename := fmt.Sprintf("sales-report-%s.xlsx", fmt.Sprintf("%s_to_%s", fromLabel, toLabel))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := xf.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Excel file"})
	}
}
