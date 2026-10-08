package handlers

import (
"bytes"
"fmt"
"net/http"
"strings"

"github.com/gin-gonic/gin"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
"github.com/jung-kurt/gofpdf"
"github.com/skip2/go-qrcode"
)

// buildInvoicePDF renders a GST-style tax invoice matching the standard
// marketplace invoice layout: seller header, Bill To/Ship To, item table
// with HSN/GST columns, totals, and an "Order Delivered From" +
// "E-commerce Platform Info" footer.
//
// GST is calculated per line item from each product's GSTPercent
// (treated as already included in Price/MRP) and split into CGST+SGST
// for intra-state orders or IGST for inter-state orders, based on
// invoice.IsInterState (set at generation time by comparing the
// delivery address state against the seller's registered state). HSN is
// left blank per item since the product catalog has no HSN field.
//
// warehouseAddress is the "Order Delivered From" address - passed in
// separately since Invoice doesn't snapshot the warehouse (only the
// customer's delivery address), and warehouse address can differ from
// order to order.
func buildInvoicePDF(invoice models.Invoice, warehouse models.Warehouse) ([]byte, error) {
cfg := config.AppConfig

pdf := gofpdf.New("P", "mm", "A4", "")
pdf.AddPage()
pdf.SetMargins(12, 12, 12)

// QR code encodes invoice number, order ID, and total - lets a
// warehouse or delivery staff quickly verify the invoice against the
// order without typing anything. Placed top-right so it doesn't
// interfere with the seller/address text on the left.
qrContent := fmt.Sprintf("Invoice: %s\nOrder: #%d\nAmount: Rs.%.2f", invoice.InvoiceNumber, invoice.OrderID, invoice.TotalAmount)
if qrPNG, err := qrcode.Encode(qrContent, qrcode.Medium, 256); err == nil {
qrReader := bytes.NewReader(qrPNG)
opts := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
pdf.RegisterImageOptionsReader("invoice_qr", opts, qrReader)
pdf.ImageOptions("invoice_qr", 174, 12, 20, 20, false, opts, 0, "")
}

// ---- Seller header ----
pdf.SetFont("Arial", "B", 12)
notConfigured := func(v string) string {
if v == "" {
return "Not configured"
}
return v
}
pdf.CellFormat(150, 6, "Seller Name: "+notConfigured(cfg.SellerCompanyName), "", 1, "L", false, 0, "")
pdf.SetFont("Arial", "", 9)
pdf.MultiCell(150, 5, notConfigured(cfg.SellerAddress), "", "L", false)
pdf.SetFont("Arial", "B", 9)
pdf.CellFormat(0, 5, "GSTIN: "+notConfigured(cfg.SellerGSTIN), "", 1, "L", false, 0, "")
pdf.CellFormat(0, 5, "FSSAI: "+notConfigured(cfg.SellerFSSAINumber), "", 1, "L", false, 0, "")
pdf.Ln(2)
pdf.SetDrawColor(0, 0, 0)
pdf.Line(12, pdf.GetY(), 198, pdf.GetY())
pdf.Ln(3)

// ---- Title ----
pdf.SetFont("Arial", "B", 12)
pdf.CellFormat(0, 7, "TAX INVOICE / BILL OF SUPPLY", "", 1, "C", false, 0, "")
pdf.Line(12, pdf.GetY(), 198, pdf.GetY())
pdf.Ln(3)

// ---- Invoice meta row ----
pdf.SetFont("Arial", "", 9)
metaY := pdf.GetY()
pdf.CellFormat(95, 5, "Invoice No.: "+invoice.InvoiceNumber, "", 0, "L", false, 0, "")
placeOfSupply := invoice.AddressState
if placeOfSupply == "" {
placeOfSupply = "-"
}
pdf.CellFormat(0, 5, "Place Of Supply : "+placeOfSupply, "", 1, "L", false, 0, "")
pdf.SetXY(12, metaY+5)
pdf.CellFormat(95, 5, fmt.Sprintf("Order No.: %d", invoice.OrderID), "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, "Date : "+invoice.GeneratedAt.Format("02-01-2006"), "", 1, "L", false, 0, "")
if invoice.PaymentReference != "" {
pdf.CellFormat(0, 5, "Payment Reference: "+invoice.PaymentReference, "", 1, "L", false, 0, "")
}
totalGST := invoice.CGSTAmount + invoice.SGSTAmount + invoice.IGSTAmount
taxType := "No GST Applicable"
if totalGST > 0 {
if invoice.IsInterState {
taxType = "IGST (Inter-State)"
} else {
taxType = "CGST + SGST (Intra-State)"
}
}
pdf.CellFormat(0, 5, "Tax Type: "+taxType, "", 1, "L", false, 0, "")
pdf.Ln(2)
pdf.Line(12, pdf.GetY(), 198, pdf.GetY())
pdf.Ln(3)

// ---- Bill To / Ship To ----
billShipY := pdf.GetY()
addressLine := invoice.AddressLine1
if invoice.AddressLine2 != "" {
addressLine += ", " + invoice.AddressLine2
}
cityLine := fmt.Sprintf("%s, %s, %s", invoice.AddressCity, invoice.AddressState, invoice.AddressPincode)

pdf.SetFont("Arial", "B", 9)
pdf.CellFormat(93, 5, "Bill To", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, "Ship To", "", 1, "L", false, 0, "")
pdf.SetFont("Arial", "", 9)
pdf.SetXY(12, billShipY+5)
pdf.CellFormat(93, 5, invoice.CustomerName, "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, invoice.CustomerName, "", 1, "L", false, 0, "")
pdf.SetXY(12, billShipY+10)
pdf.CellFormat(93, 5, addressLine, "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, addressLine, "", 1, "L", false, 0, "")
pdf.SetXY(12, billShipY+15)
pdf.CellFormat(93, 5, cityLine, "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, cityLine, "", 1, "L", false, 0, "")
pdf.Ln(2)
pdf.Line(12, pdf.GetY(), 198, pdf.GetY())
pdf.Ln(3)

// ---- Items table ----
// Column widths widened for HSN (was 10mm, too narrow and caused HSN
// codes like "PRD00393" to wrap onto a second line while every other
// cell in the row kept a fixed 5mm height - this produced the
// mismatched/broken-looking cell borders. SR and Item&Description were
// trimmed slightly to compensate so the table's total width (178mm)
// is unchanged.
colW := []float64{6, 41, 15, 16, 8, 16, 16, 10, 10, 10, 10, 20}
taxCol1Header := "CGST"
taxCol2Header := "S/UT\nGST"
if invoice.IsInterState {
taxCol1Header = "IGST"
taxCol2Header = "-"
}
headers := []string{"SR\nNo", "Item &\nDescription", "Unit\nMRP/RSP", "HSN", "Qty", "Product\nRate", "Taxable\nAmt.", taxCol1Header, taxCol2Header, "GST\n%", "GST\nAmt.", "Total\nAmt."}

// ---- Header row: draw all cells at one uniform height ----
pdf.SetFont("Arial", "B", 6.5)
pdf.SetFillColor(235, 235, 235)
pdf.SetDrawColor(0, 0, 0)
headerLineH := 3.5
headerY := pdf.GetY()
maxHeaderLines := 1
for _, h := range headers {
n := len(strings.Split(h, "\n"))
if n > maxHeaderLines {
maxHeaderLines = n
}
}
headerH := float64(maxHeaderLines) * headerLineH
for i, h := range headers {
x := sumWidths(colW[:i]) + 12
pdf.Rect(x, headerY, colW[i], headerH, "FD")
pdf.SetXY(x, headerY)
pdf.MultiCell(colW[i], headerLineH, h, "", "C", false)
}
pdf.SetXY(12, headerY+headerH)

// ---- Item rows: compute each row's height from its tallest cell
// (after word-wrapping), then draw every cell in that row as a
// uniform-height bordered box so the grid lines stay aligned even
// when a cell (e.g. a long product name or HSN code) wraps to
// multiple lines. ----
pdf.SetFont("Arial", "", 7.5)
rowLineH := 3.6
minRowH := 5.0
taxableTotal := 0.0
gstTotal := 0.0
grandTotal := 0.0
for i, item := range invoice.Items {
lineTotal := item.Price * float64(item.Quantity)
taxableLine := lineTotal - item.GSTAmount
taxableTotal += taxableLine
gstTotal += item.GSTAmount
grandTotal += lineTotal

cgstDisplay := "-"
sgstDisplay := "-"
if invoice.IsInterState {
cgstDisplay = fmt.Sprintf("%.2f", item.GSTAmount)
} else {
half := item.GSTAmount / 2
cgstDisplay = fmt.Sprintf("%.2f", half)
sgstDisplay = fmt.Sprintf("%.2f", half)
}

hsn := item.HSNCode
if hsn == "" {
hsn = "-"
}
values := []string{
fmt.Sprintf("%d", i+1),
item.ProductName,
fmt.Sprintf("%.2f", item.Price),
hsn,
fmt.Sprintf("%d", item.Quantity),
fmt.Sprintf("%.2f", item.Price),
fmt.Sprintf("%.2f", taxableLine),
cgstDisplay,
sgstDisplay,
fmt.Sprintf("%.1f%%", item.GSTPercent),
fmt.Sprintf("%.2f", item.GSTAmount),
fmt.Sprintf("%.2f", lineTotal),
}
aligns := []string{"C", "L", "R", "C", "C", "R", "R", "R", "R", "C", "R", "R"}

// Work out how many lines each cell needs at its column width, and
// use the tallest one as the row height for every cell.
maxLines := 1
for j, v := range values {
splitLines := pdf.SplitLines([]byte(v), colW[j])
if len(splitLines) > maxLines {
maxLines = len(splitLines)
}
}
rowH := float64(maxLines) * rowLineH
if rowH < minRowH {
rowH = minRowH
}

// Page-break check: if this row won't fit before the bottom margin,
// start a fresh page and re-draw the header there.
if pdf.GetY()+rowH > 275 {
pdf.AddPage()
newHeaderY := pdf.GetY()
for hi, h := range headers {
x := sumWidths(colW[:hi]) + 12
pdf.SetFont("Arial", "B", 6.5)
pdf.Rect(x, newHeaderY, colW[hi], headerH, "FD")
pdf.SetXY(x, newHeaderY)
pdf.MultiCell(colW[hi], headerLineH, h, "", "C", false)
}
pdf.SetXY(12, newHeaderY+headerH)
pdf.SetFont("Arial", "", 7.5)
}

rowY := pdf.GetY()
for j, v := range values {
x := sumWidths(colW[:j]) + 12
pdf.Rect(x, rowY, colW[j], rowH, "D")
pdf.SetXY(x, rowY)
pdf.MultiCell(colW[j], rowLineH, v, "", aligns[j], false)
}
pdf.SetXY(12, rowY+rowH)
}

// Item Total row inside the table
totalRowY := pdf.GetY()
pdf.SetFont("Arial", "B", 7.5)
pdf.SetXY(sumWidths(colW[:6])+12, totalRowY)
pdf.CellFormat(colW[6], 5, fmt.Sprintf("%.2f", taxableTotal), "1", 0, "R", false, 0, "")
pdf.CellFormat(colW[7], 5, "", "1", 0, "C", false, 0, "")
pdf.CellFormat(colW[8], 5, "", "1", 0, "C", false, 0, "")
pdf.CellFormat(colW[9], 5, "", "1", 0, "C", false, 0, "")
pdf.CellFormat(colW[10], 5, fmt.Sprintf("%.2f", gstTotal), "1", 0, "R", false, 0, "")
pdf.CellFormat(colW[11], 5, fmt.Sprintf("%.2f", grandTotal), "1", 1, "R", false, 0, "")
pdf.SetX(12)
pdf.Ln(4)

// ---- Item Total / Invoice Value ----
pdf.SetFont("Arial", "B", 9)
pdf.CellFormat(150, 5, "Taxable Value", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("%.2f", invoice.TaxableAmount), "", 1, "R", false, 0, "")
if invoice.IsInterState {
pdf.SetFont("Arial", "", 9)
pdf.CellFormat(150, 5, "IGST", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("%.2f", invoice.IGSTAmount), "", 1, "R", false, 0, "")
} else {
pdf.SetFont("Arial", "", 9)
pdf.CellFormat(150, 5, "CGST", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("%.2f", invoice.CGSTAmount), "", 1, "R", false, 0, "")
pdf.CellFormat(150, 5, "SGST", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("%.2f", invoice.SGSTAmount), "", 1, "R", false, 0, "")
}
pdf.SetFont("Arial", "B", 9)
pdf.CellFormat(150, 5, "Item Total (incl. GST)", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("%.2f", invoice.ItemsAmount), "", 1, "R", false, 0, "")
if invoice.DiscountAmount > 0 {
pdf.SetFont("Arial", "", 9)
pdf.CellFormat(150, 5, "Discount", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("-%.2f", invoice.DiscountAmount), "", 1, "R", false, 0, "")
}
if invoice.DeliveryCharge > 0 {
pdf.SetFont("Arial", "", 9)
pdf.CellFormat(150, 5, "Delivery Charge", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("%.2f", invoice.DeliveryCharge), "", 1, "R", false, 0, "")
}
if invoice.PlatformFee > 0 {
pdf.SetFont("Arial", "", 9)
pdf.CellFormat(150, 5, "Platform Fee", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("%.2f", invoice.PlatformFee), "", 1, "R", false, 0, "")
}
if invoice.WalletUsed > 0 {
pdf.SetFont("Arial", "", 9)
pdf.CellFormat(150, 5, "Wallet Used", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 5, fmt.Sprintf("-%.2f", invoice.WalletUsed), "", 1, "R", false, 0, "")
}
pdf.SetFont("Arial", "B", 10)
pdf.CellFormat(150, 6, "Invoice Value", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 6, fmt.Sprintf("%.2f", invoice.TotalAmount), "", 1, "R", false, 0, "")
pdf.Ln(3)

pdf.SetFont("Arial", "", 7.5)
pdf.CellFormat(0, 4, "Whether GST is payable on reverse-charge - No.", "", 1, "L", false, 0, "")
pdf.CellFormat(0, 4, "For IMEI / Serial number information, please refer to packaging / warranty slip.", "", 1, "L", false, 0, "")
pdf.Ln(3)
pdf.Line(12, pdf.GetY(), 198, pdf.GetY())
pdf.Ln(3)

// ---- Footer: Order Delivered From / E-commerce Platform Info ----
footerY := pdf.GetY()
pdf.SetFont("Arial", "B", 8)
pdf.CellFormat(93, 4, "Order Delivered From -", "", 0, "L", false, 0, "")
pdf.CellFormat(0, 4, "E-commerce Platform (FBO) Information -", "", 1, "L", false, 0, "")
pdf.SetFont("Arial", "", 8)
pdf.SetXY(12, footerY+4)
pdf.CellFormat(93, 4, warehouse.Name, "", 0, "L", false, 0, "")
pdf.CellFormat(0, 4, cfg.SellerCompanyName, "", 1, "L", false, 0, "")
pdf.SetXY(12, footerY+8)
pdf.MultiCell(90, 4, warehouse.Address, "", "L", false)
pdf.SetXY(105, footerY+8)
pdf.MultiCell(90, 4, cfg.SellerAddress, "", "L", false)
fssaiY := pdf.GetY()
if fssaiY < footerY+16 {
fssaiY = footerY + 16
}
pdf.SetXY(12, fssaiY)
pdf.CellFormat(93, 4, "FSSAI: "+valueOrDash(cfg.SellerFSSAINumber), "", 0, "L", false, 0, "")
pdf.CellFormat(0, 4, "FSSAI Lic. No.: "+valueOrDash(cfg.SellerFSSAINumber), "", 1, "L", false, 0, "")
pdf.SetXY(105, fssaiY+4)
if cfg.SellerEmail != "" {
pdf.CellFormat(0, 4, "Email: "+cfg.SellerEmail, "", 1, "L", false, 0, "")
}

var buf []byte
w := &pdfBufferWriter{}
if err := pdf.Output(w); err != nil {
return nil, err
}
buf = w.buf
return buf, nil
}

func sumWidths(widths []float64) float64 {
total := 0.0
for _, w := range widths {
total += w
}
return total
}

func valueOrDash(v string) string {
if v == "" {
return "-"
}
return v
}

// pdfBufferWriter is a minimal io.Writer that gofpdf writes the finished
// PDF bytes into, so we can hand them back as an in-memory []byte instead
// of writing to disk.
type pdfBufferWriter struct{ buf []byte }

func (w *pdfBufferWriter) Write(p []byte) (int, error) {
w.buf = append(w.buf, p...)
return len(p), nil
}

func servePDF(c *gin.Context, invoice models.Invoice, warehouse models.Warehouse) {
pdfBytes, err := buildInvoicePDF(invoice, warehouse)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate invoice PDF"})
return
}
filename := invoice.InvoiceNumber + ".pdf"
c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// GetMyOrderInvoicePDF godoc
// GET /api/v1/orders/:id/invoice/pdf (protected - order owner only)
func GetMyOrderInvoicePDF(c *gin.Context) {
userID := c.MustGet("user_id").(uint)
orderID := c.Param("id")

var order models.Order
if err := database.DB.Preload("Warehouse").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
return
}
// cancelled-order guard: no invoice is shown for a cancelled order.
if order.Status == models.OrderStatusCancelled {
c.JSON(http.StatusNotFound, gin.H{"error": "No invoice for a cancelled order"})
return
}

var invoice models.Invoice
if err := database.DB.Where("order_id = ?", order.ID).Preload("Items").First(&invoice).Error; err != nil {
generated, genErr := services.GenerateInvoiceIfNotExists(order.ID)
if genErr != nil || generated == nil {
c.JSON(http.StatusNotFound, gin.H{"error": "No invoice found for this order yet"})
return
}
database.DB.Where("order_id = ?", order.ID).Preload("Items").First(&invoice)
}
warehouse := models.Warehouse{}
if order.Warehouse != nil {
warehouse = *order.Warehouse
}
servePDF(c, invoice, warehouse)
}

// GetWarehouseOrderInvoicePDF godoc
// GET /api/v1/warehouse/orders/:id/invoice/pdf (warehouse staff only)
func GetWarehouseOrderInvoicePDF(c *gin.Context) {
warehouseID := c.MustGet("warehouse_id").(uint)
orderID := c.Param("id")

var order models.Order
if err := database.DB.Preload("Warehouse").Where("warehouse_id = ?", warehouseID).First(&order, orderID).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Order not found for your warehouse"})
return
}

var invoice models.Invoice
if err := database.DB.Where("order_id = ?", order.ID).Preload("Items").First(&invoice).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "No invoice found for this order yet"})
return
}
warehouse := models.Warehouse{}
if order.Warehouse != nil {
warehouse = *order.Warehouse
}
servePDF(c, invoice, warehouse)
}

// GetAdminInvoicePDF godoc
// GET /api/v1/admin/invoices/:id/pdf (admin only)
func GetAdminInvoicePDF(c *gin.Context) {
id := c.Param("id")

var invoice models.Invoice
if err := database.DB.Preload("Items").First(&invoice, id).Error; err != nil {
c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
return
}

var order models.Order
database.DB.Preload("Warehouse").First(&order, invoice.OrderID)

warehouse := models.Warehouse{}
if order.Warehouse != nil {
warehouse = *order.Warehouse
}
servePDF(c, invoice, warehouse)
}
