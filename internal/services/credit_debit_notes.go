package services

import (
	"fmt"
	"math"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GenerateCreditNoteForReturn issues a Credit Note against the order's
// invoice for one approved return request (SRS 12.17). Called from
// ApproveReturn, inside the same DB transaction that restores stock and
// credits the wallet, so a credit note is never created for a return that
// didn't actually complete. Idempotent per return request.
func GenerateCreditNoteForReturn(tx *gorm.DB, returnReq models.ReturnRequest) (*models.CreditNote, error) {
	var existing models.CreditNote
	if err := tx.Where("return_request_id = ?", returnReq.ID).First(&existing).Error; err == nil {
		return &existing, nil
	}

	var invoice models.Invoice
	if err := tx.Where("order_id = ?", returnReq.OrderID).First(&invoice).Error; err != nil {
		return nil, fmt.Errorf("no invoice found for order %d: %w", returnReq.OrderID, err)
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

	// BUG-02: InvoiceItem.Price is undiscounted, so apply the invoice's coupon ratio.
	cnDiscountRatio := 1.0
	if invoice.ItemsAmount > 0 && invoice.DiscountAmount > 0 {
		cnDiscountRatio = (invoice.ItemsAmount - invoice.DiscountAmount) / invoice.ItemsAmount
		if cnDiscountRatio < 0 {
			cnDiscountRatio = 0
		}
	}
	items := make([]models.CreditNoteItem, 0, len(returnReq.Items))

	for _, ri := range returnReq.Items {
		var invoiceItem models.InvoiceItem
		if err := tx.Where("invoice_id = ? AND product_id = ?", invoice.ID, ri.OrderItem.ProductID).First(&invoiceItem).Error; err != nil {
			// Product on the invoice snapshot not found (shouldn't normally
			// happen) - fall back to zero GST rather than failing the whole
			// credit note, so the note still balances against RefundAmount.
			invoiceItem = models.InvoiceItem{Price: ri.OrderItem.Price, GSTPercent: 0}
		}

		lineTotal := invoiceItem.Price * float64(ri.Quantity) * cnDiscountRatio
		taxableLine := lineTotal
		gstLine := 0.0
		if invoiceItem.GSTPercent > 0 {
			taxableLine = lineTotal / (1 + invoiceItem.GSTPercent/100)
			gstLine = lineTotal - taxableLine
		}
		totalTaxable += taxableLine
		totalGST += gstLine

		items = append(items, models.CreditNoteItem{
			ProductID:   ri.OrderItem.ProductID,
			ProductName: invoiceItem.ProductName,
			Quantity:    ri.Quantity,
			Price:       invoiceItem.Price,
			GSTPercent:  invoiceItem.GSTPercent,
			GSTAmount:   gstLine,
		})
	}

	// BUG-02: make taxable + GST equal the amount actually refunded so the
	// credit note ledger entry balances.
	if sum := totalTaxable + totalGST; sum > 0 && returnReq.RefundAmount > 0 && math.Abs(sum-returnReq.RefundAmount) > 0.01 {
		k := returnReq.RefundAmount / sum
		totalTaxable *= k
		totalGST *= k
		for i := range items {
			items[i].GSTAmount *= k
		}
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
		OrderID:          returnReq.OrderID,
		ReturnRequestID:  &returnReq.ID,
		CustomerName:     invoice.CustomerName,
		CustomerPhone:    invoice.CustomerPhone,
		Reason:           returnReq.Reason,
		TaxableAmount:    totalTaxable,
		CGSTAmount:       cgst,
		SGSTAmount:       sgst,
		IGSTAmount:       igst,
		TotalAmount:      returnReq.RefundAmount,
		IssuedAt:         time.Now(),
		Items:            items,
	}
	if err := tx.Create(&note).Error; err != nil {
		return nil, fmt.Errorf("failed to create credit note: %w", err)
	}

	utils.LogAudit(0, "system", "generate_credit_note", "credit_note", fmt.Sprint(note.ID),
		fmt.Sprintf("return_request_id=%d order_id=%d total=%.2f", returnReq.ID, returnReq.OrderID, note.TotalAmount))

	return &note, nil
}

// PostCreditNoteLedgerEntry records the double-entry ledger lines reversing
// revenue for one credit note: Debit Product Sales + Debit GST Payable,
// Credit Customer Wallet Liability - since ApproveReturn refunds to the
// customer's wallet (utils.CreditWallet), not to bank, the credit side here
// matches that actual money movement rather than a bank/cash account.
func PostCreditNoteLedgerEntry(creditNoteID uint) error {
	var existing models.LedgerEntry
	if err := database.DB.Where("reference_type = ? AND reference_id = ?", "credit_note", creditNoteID).First(&existing).Error; err == nil {
		return nil
	}

	var note models.CreditNote
	if err := database.DB.First(&note, creditNoteID).Error; err != nil {
		return fmt.Errorf("credit note not found: %w", err)
	}

	var sales, walletLiability models.Account
	if err := database.DB.Where("code = ?", "4001").First(&sales).Error; err != nil {
		return fmt.Errorf("chart of accounts missing code 4001 (Product Sales): %w", err)
	}
	if err := database.DB.Where("code = ?", "2005").First(&walletLiability).Error; err != nil {
		return fmt.Errorf("chart of accounts missing code 2005 (Customer Wallet Liability): %w", err)
	}

	totalGST := note.CGSTAmount + note.SGSTAmount + note.IGSTAmount
	transactionRef := fmt.Sprintf("CREDITNOTE-%d", creditNoteID)
    // BUG-03: cost of the returned units (same CostPrice basis as PostSalesLedgerEntry).
    var cnItems []models.CreditNoteItem
    if err := database.DB.Where("credit_note_id = ?", creditNoteID).Find(&cnItems).Error; err != nil {
        return fmt.Errorf("failed to load credit note items: %w", err)
    }
    var totalCOGS float64
    for _, it := range cnItems {
        var prod models.Product
        if err := database.DB.Select("id", "cost_price").First(&prod, it.ProductID).Error; err == nil && prod.CostPrice > 0 {
            totalCOGS += prod.CostPrice * float64(it.Quantity)
        }
    }

	return database.DB.Transaction(func(tx *gorm.DB) error {
		debitSales := models.LedgerEntry{
			TransactionRef: transactionRef,
			AccountID:      sales.ID,
			Type:           "debit",
			Amount:         note.TaxableAmount,
			Description:    fmt.Sprintf("Credit note %s", note.CreditNoteNumber),
			ReferenceType:  "credit_note",
			ReferenceID:    &creditNoteID,
			EntryDate:      note.IssuedAt,
		}
		if err := tx.Create(&debitSales).Error; err != nil {
			return fmt.Errorf("failed to create sales-reversal ledger entry: %w", err)
		}

		if totalGST > 0 {
			var gstPayable models.Account
			if err := tx.Where("code = ?", "2002").First(&gstPayable).Error; err != nil {
				return fmt.Errorf("chart of accounts missing code 2002 (GST Payable): %w", err)
			}
			debitGST := models.LedgerEntry{
				TransactionRef: transactionRef,
				AccountID:      gstPayable.ID,
				Type:           "debit",
				Amount:         totalGST,
				Description:    fmt.Sprintf("Credit note %s", note.CreditNoteNumber),
				ReferenceType:  "credit_note",
				ReferenceID:    &creditNoteID,
				EntryDate:      note.IssuedAt,
			}
			if err := tx.Create(&debitGST).Error; err != nil {
				return fmt.Errorf("failed to create GST-reversal ledger entry: %w", err)
			}
		}

		credit := models.LedgerEntry{
			TransactionRef: transactionRef,
			AccountID:      walletLiability.ID,
			Type:           "credit",
			Amount:         note.TotalAmount,
			Description:    fmt.Sprintf("Credit note %s", note.CreditNoteNumber),
			ReferenceType:  "credit_note",
			ReferenceID:    &creditNoteID,
			EntryDate:      note.IssuedAt,
		}
		if err := tx.Create(&credit).Error; err != nil {
			return fmt.Errorf("failed to create wallet-liability ledger entry: %w", err)
        }

        // BUG-03: returned goods go back to stock, so reverse the COGS booked at sale time.
        if totalCOGS > 0 {
            var inventoryAcc, cogsAcc models.Account
            if err := tx.Where("code = ?", "1004").First(&inventoryAcc).Error; err != nil {
                return fmt.Errorf("chart of accounts missing code 1004 (Inventory): %w", err)
            }
            if err := tx.Where("code = ?", "5001").First(&cogsAcc).Error; err != nil {
                return fmt.Errorf("chart of accounts missing code 5001 (COGS): %w", err)
            }
            for _, l := range []struct {
                acc uint
                typ string
            }{{inventoryAcc.ID, "debit"}, {cogsAcc.ID, "credit"}} {
                e := models.LedgerEntry{
                    TransactionRef: transactionRef,
                    AccountID:      l.acc,
                    Type:           l.typ,
                    Amount:         totalCOGS,
                    Description:    fmt.Sprintf("COGS reversal for credit note %s", note.CreditNoteNumber),
                    ReferenceType:  "credit_note",
                    ReferenceID:    &creditNoteID,
                    EntryDate:      note.IssuedAt,
                }
                if err := tx.Create(&e).Error; err != nil {
                    return fmt.Errorf("failed to create COGS-reversal ledger entry: %w", err)
                }
            }
        }
        return nil
	})
}

// GenerateDebitNote issues a Debit Note against a vendor bill for a
// purchase return, short supply, rate difference, or quality issue (SRS
// 12.18). Always an explicit admin action - there is no automatic trigger
// on the purchase side comparable to a customer return.
func GenerateDebitNote(billID uint, amount, gstAmount float64, reason string, adminID uint) (*models.DebitNote, error) {
	var bill models.VendorBill
	if err := database.DB.First(&bill, billID).Error; err != nil {
		return nil, fmt.Errorf("vendor bill not found: %w", err)
	}

	var note models.DebitNote
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("CREATE SEQUENCE IF NOT EXISTS debit_note_number_seq START 1").Error; err != nil {
			return fmt.Errorf("failed to ensure debit note sequence exists: %w", err)
		}
		var seqVal int64
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Raw("SELECT nextval('debit_note_number_seq')").Scan(&seqVal).Error; err != nil {
			return fmt.Errorf("failed to allocate debit note number: %w", err)
		}
		debitNoteNumber := fmt.Sprintf("DN-%d-%06d", time.Now().Year(), seqVal)

		note = models.DebitNote{
			DebitNoteNumber: debitNoteNumber,
			VendorBillID:    bill.ID,
			VendorID:        bill.VendorID,
			Reason:          reason,
			Amount:          amount,
			GSTAmount:       gstAmount,
			TotalAmount:     amount + gstAmount,
			IssuedAt:        time.Now(),
			CreatedByID:     adminID,
		}
		if err := tx.Create(&note).Error; err != nil {
			return fmt.Errorf("failed to create debit note: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return &note, nil
}

// PostDebitNoteLedgerEntry records the double-entry ledger lines for one
// debit note: Debit Vendor Payable (reduces what's owed), Credit Inventory
// for the goods-value portion and Credit GST ITC for the GST portion
// (reversing the input credit claimed on the original purchase).
func PostDebitNoteLedgerEntry(debitNoteID uint) error {
	var existing models.LedgerEntry
	if err := database.DB.Where("reference_type = ? AND reference_id = ?", "debit_note", debitNoteID).First(&existing).Error; err == nil {
		return nil
	}

	var note models.DebitNote
	if err := database.DB.First(&note, debitNoteID).Error; err != nil {
		return fmt.Errorf("debit note not found: %w", err)
	}

	var vendorPayable, inventory models.Account
	if err := database.DB.Where("code = ?", "2001").First(&vendorPayable).Error; err != nil {
		return fmt.Errorf("chart of accounts missing code 2001 (Vendor Payable): %w", err)
	}
	if err := database.DB.Where("code = ?", "1004").First(&inventory).Error; err != nil {
		return fmt.Errorf("chart of accounts missing code 1004 (Inventory): %w", err)
	}

	transactionRef := fmt.Sprintf("DEBITNOTE-%d", debitNoteID)

	return database.DB.Transaction(func(tx *gorm.DB) error {
		debit := models.LedgerEntry{
			TransactionRef: transactionRef,
			AccountID:      vendorPayable.ID,
			Type:           "debit",
			Amount:         note.TotalAmount,
			Description:    fmt.Sprintf("Debit note %s", note.DebitNoteNumber),
			ReferenceType:  "debit_note",
			ReferenceID:    &debitNoteID,
			EntryDate:      note.IssuedAt,
		}
		if err := tx.Create(&debit).Error; err != nil {
			return fmt.Errorf("failed to create vendor-payable ledger entry: %w", err)
		}

		creditInventory := models.LedgerEntry{
			TransactionRef: transactionRef,
			AccountID:      inventory.ID,
			Type:           "credit",
			Amount:         note.Amount,
			Description:    fmt.Sprintf("Debit note %s", note.DebitNoteNumber),
			ReferenceType:  "debit_note",
			ReferenceID:    &debitNoteID,
			EntryDate:      note.IssuedAt,
		}
		if err := tx.Create(&creditInventory).Error; err != nil {
			return fmt.Errorf("failed to create inventory-reversal ledger entry: %w", err)
		}

		if note.GSTAmount > 0 {
			var gstITC models.Account
			if err := tx.Where("code = ?", "1005").First(&gstITC).Error; err != nil {
				return fmt.Errorf("chart of accounts missing code 1005 (GST Input Credit): %w", err)
			}
			creditGST := models.LedgerEntry{
				TransactionRef: transactionRef,
				AccountID:      gstITC.ID,
				Type:           "credit",
				Amount:         note.GSTAmount,
				Description:    fmt.Sprintf("Debit note %s", note.DebitNoteNumber),
				ReferenceType:  "debit_note",
				ReferenceID:    &debitNoteID,
				EntryDate:      note.IssuedAt,
			}
			if err := tx.Create(&creditGST).Error; err != nil {
				return fmt.Errorf("failed to create GST-ITC-reversal ledger entry: %w", err)
			}
		}
		return nil
	})
}
