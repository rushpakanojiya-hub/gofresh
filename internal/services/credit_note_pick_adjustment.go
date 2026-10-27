package services

import (
    "errors"
    "fmt"
    "math"
    "time"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
    "gorm.io/gorm"
)

// BUG-08: credit notes for warehouse short-picks and cheaper substitutions.
// An issued GST invoice is never edited; the reduction is documented by a
// credit note against it, exactly like a customer return.

const (
    CreditNoteReasonShortPick    = "Short/unavailable items at picking"
    CreditNoteReasonSubstitution = "Price adjustment: cheaper substitute item"
)

// PickAdjustmentLine is one product line being credited.
type PickAdjustmentLine struct {
    ProductID  uint
    Quantity   int
    UnitCredit float64 // GST-inclusive credit per unit, before the coupon ratio
}

// GenerateCreditNoteForPickAdjustment issues a credit note against the order's
// invoice. Call it inside the same transaction that reduces the order amounts
// and credits the wallet. creditTotal is the coupon-adjusted amount actually
// credited. Returns (nil, nil) when the order has no invoice yet: an invoice
// generated later is built from the already-reduced order amounts.
func GenerateCreditNoteForPickAdjustment(tx *gorm.DB, orderID uint, lines []PickAdjustmentLine, creditTotal float64, reason string) (*models.CreditNote, error) {
    if creditTotal <= 0 || len(lines) == 0 {
        return nil, nil
    }

    var invoice models.Invoice
    if err := tx.Where("order_id = ?", orderID).First(&invoice).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, fmt.Errorf("failed to load invoice for order %d: %w", orderID, err)
    }

    if err := tx.Exec("CREATE SEQUENCE IF NOT EXISTS credit_note_number_seq START 1").Error; err != nil {
        return nil, fmt.Errorf("failed to ensure credit note sequence exists: %w", err)
    }
    var seqVal int64
    if err := tx.Raw("SELECT nextval('credit_note_number_seq')").Scan(&seqVal).Error; err != nil {
        return nil, fmt.Errorf("failed to allocate credit note number: %w", err)
    }
    creditNoteNumber := fmt.Sprintf("CN-%d-%06d", time.Now().Year(), seqVal)

    var totalTaxable, totalGST float64
    items := make([]models.CreditNoteItem, 0, len(lines))
    for _, ln := range lines {
        var invoiceItem models.InvoiceItem
        if err := tx.Where("invoice_id = ? AND product_id = ?", invoice.ID, ln.ProductID).First(&invoiceItem).Error; err != nil {
            // Product is not on the invoice snapshot (for example an earlier
            // substitution swapped the order line onto another product): use
            // the product's own GST rate.
            var prod models.Product
            if perr := tx.Unscoped().First(&prod, ln.ProductID).Error; perr == nil {
                invoiceItem = models.InvoiceItem{ProductName: prod.Name, GSTPercent: prod.GSTPercent}
            }
        }
        lineTotal := ln.UnitCredit * float64(ln.Quantity)
        taxableLine := lineTotal
        gstLine := 0.0
        if invoiceItem.GSTPercent > 0 {
            taxableLine = lineTotal / (1 + invoiceItem.GSTPercent/100)
            gstLine = lineTotal - taxableLine
        }
        totalTaxable += taxableLine
        totalGST += gstLine
        items = append(items, models.CreditNoteItem{
            ProductID:   ln.ProductID,
            ProductName: invoiceItem.ProductName,
            Quantity:    ln.Quantity,
            Price:       ln.UnitCredit,
            GSTPercent:  invoiceItem.GSTPercent,
            GSTAmount:   gstLine,
        })
    }

    // Make taxable + GST equal the amount actually credited (coupon-adjusted)
    // so the ledger entry balances.
    if sum := totalTaxable + totalGST; sum > 0 {
        if math.Abs(sum-creditTotal) > 0.01 {
            k := creditTotal / sum
            totalTaxable *= k
            totalGST *= k
            for i := range items {
                items[i].GSTAmount *= k
            }
        }
    } else {
        totalTaxable = creditTotal
    }

    cgst, sgst, igst := 0.0, 0.0, 0.0
    if invoice.IsInterState {
        igst = totalGST
    } else {
        cgst = totalGST / 2
        sgst = totalGST / 2
    }

    note := models.CreditNote{
        CreditNoteNumber: creditNoteNumber,
        InvoiceID:        invoice.ID,
        OrderID:          orderID,
        CustomerName:     invoice.CustomerName,
        CustomerPhone:    invoice.CustomerPhone,
        Reason:           reason,
        TaxableAmount:    totalTaxable,
        CGSTAmount:       cgst,
        SGSTAmount:       sgst,
        IGSTAmount:       igst,
        TotalAmount:      creditTotal,
        IssuedAt:         time.Now(),
        Items:            items,
    }
    if err := tx.Create(&note).Error; err != nil {
        return nil, fmt.Errorf("failed to create credit note: %w", err)
    }

    utils.LogAudit(0, "system", "generate_credit_note", "credit_note", fmt.Sprint(note.ID),
        fmt.Sprintf("order_id=%d total=%.2f reason=%s", orderID, note.TotalAmount, reason))

    return &note, nil
}

// PostPickAdjustmentCreditNoteLedgerEntry posts the ledger lines for a credit
// note issued by GenerateCreditNoteForPickAdjustment:
// Debit Product Sales + Debit GST Payable, Credit Customer Wallet Liability for
// the part credited back to the wallet, and (COD only) Credit Cash/Bank for the
// part that simply lowers what the customer pays. If the sale itself is not in
// the ledger yet (COD is booked at delivery) this does nothing, and
// PostSalesLedgerEntry posts the note right after it books the sale.
// Idempotent per credit note.
func PostPickAdjustmentCreditNoteLedgerEntry(creditNoteID uint) error {
    var existing models.LedgerEntry
    if err := database.DB.Where("reference_type = ? AND reference_id = ?", "credit_note", creditNoteID).First(&existing).Error; err == nil {
        return nil
    }

    var note models.CreditNote
    if err := database.DB.First(&note, creditNoteID).Error; err != nil {
        return fmt.Errorf("credit note not found: %w", err)
    }

    var sale models.LedgerEntry
    if err := database.DB.Where("reference_type = ? AND reference_id = ?", "sale", note.OrderID).First(&sale).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil
        }
        return err
    }

    var order models.Order
    if err := database.DB.First(&order, note.OrderID).Error; err != nil {
        return fmt.Errorf("order not found: %w", err)
    }
    var invoice models.Invoice
    if err := database.DB.First(&invoice, note.InvoiceID).Error; err != nil {
        return fmt.Errorf("invoice not found: %w", err)
    }

    walletPart := note.TotalAmount
    if order.PaymentMethod == models.PaymentMethodCOD {
        // COD: only the wallet-paid part is credited back to the wallet
        // (order.WalletAmountUsed was reduced by exactly that); the rest only
        // lowers the cash the rider collects. Allocate in credit note order.
        walletReturned := invoice.WalletUsed - order.WalletAmountUsed
        if walletReturned < 0 {
            walletReturned = 0
        }
        var siblings []models.CreditNote
        if err := database.DB.Where("order_id = ? AND return_request_id IS NULL", note.OrderID).Order("id").Find(&siblings).Error; err != nil {
            return fmt.Errorf("failed to load credit notes for order %d: %w", note.OrderID, err)
        }
        walletPart = 0
        for _, s := range siblings {
            alloc := math.Min(s.TotalAmount, walletReturned)
            walletReturned -= alloc
            if s.ID == note.ID {
                walletPart = alloc
                break
            }
        }
    }
    cashPart := note.TotalAmount - walletPart
    if cashPart < 0.005 {
        cashPart = 0
    }

    cashOrBankCode := "1002"
    if order.PaymentMethod == models.PaymentMethodCOD && !(order.CollectedVia != nil && *order.CollectedVia == "upi") {
        cashOrBankCode = "1001"
    }

    // A short pick put the stock back, so reverse the COGS booked at sale time.
    // A substitution returns no stock, so it has no COGS reversal.
    var totalCOGS float64
    if note.Reason == CreditNoteReasonShortPick {
        var cnItems []models.CreditNoteItem
        if err := database.DB.Where("credit_note_id = ?", creditNoteID).Find(&cnItems).Error; err != nil {
            return fmt.Errorf("failed to load credit note items: %w", err)
        }
        for _, it := range cnItems {
            if unit := orderLineUnitCost(database.DB, note.OrderID, it.ProductID); unit > 0 { // BUG-09: snapshot cost from the order line, not live product cost
    totalCOGS += unit * float64(it.Quantity)
            }
        }
    }

    totalGST := note.CGSTAmount + note.SGSTAmount + note.IGSTAmount
    transactionRef := fmt.Sprintf("CREDITNOTE-%d", creditNoteID)
    desc := fmt.Sprintf("Credit note %s", note.CreditNoteNumber)
    cogsDesc := fmt.Sprintf("COGS reversal for credit note %s", note.CreditNoteNumber)

    type line struct {
        code   string
        lType  string
        amount float64
        desc   string
    }
    lines := []line{
        {"4001", "debit", note.TaxableAmount, desc},
        {"2002", "debit", totalGST, desc},
        {"2005", "credit", walletPart, desc},
        {cashOrBankCode, "credit", cashPart, desc},
    }
    if totalCOGS > 0 {
        lines = append(lines, line{"1004", "debit", totalCOGS, cogsDesc}, line{"5001", "credit", totalCOGS, cogsDesc})
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
                ReferenceType:  "credit_note",
                ReferenceID:    &creditNoteID,
                EntryDate:      note.IssuedAt,
            }
            if err := tx.Create(&entry).Error; err != nil {
                return fmt.Errorf("failed to create credit note ledger entry: %w", err)
            }
        }
        return nil
    })
}

// postPendingPickAdjustmentCreditNotes posts credit notes for an order that
// were issued before its sale reached the ledger. Called by PostSalesLedgerEntry.
func postPendingPickAdjustmentCreditNotes(orderID uint) {
    var notes []models.CreditNote
    if err := database.DB.Where("order_id = ? AND return_request_id IS NULL", orderID).Order("id").Find(&notes).Error; err != nil {
        fmt.Printf("ledger: failed to load pending credit notes for order %d: %v\n", orderID, err)
        return
    }
    for _, n := range notes {
        if err := PostPickAdjustmentCreditNoteLedgerEntry(n.ID); err != nil {
            fmt.Printf("ledger: credit note %d for order %d failed to post: %v\n", n.ID, orderID, err)
        }
    }
}