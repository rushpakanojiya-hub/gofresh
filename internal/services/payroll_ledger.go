package services

import (
    "fmt"
    "time"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
    "gorm.io/gorm"
)

// PostPayrollLedgerEntry books a paid payroll record (BUG-13):
// Debit Operating Expenses (5003), Credit Cash (1001) if paid in cash, else Bank (1002).
// Idempotent per payroll (reference_type="payroll"). No-op unless the record is paid with amount > 0.
func PostPayrollLedgerEntry(payrollID uint) error {
    var existing models.LedgerEntry
    if err := database.DB.Where("reference_type = ? AND reference_id = ?", "payroll", payrollID).First(&existing).Error; err == nil {
        return nil
    }
    var p models.Payroll
    if err := database.DB.First(&p, payrollID).Error; err != nil {
        return fmt.Errorf("payroll not found: %w", err)
    }
    if p.Status != "paid" || p.Amount <= 0 {
        return nil
    }
    payCode := "1002"
    if p.PaymentMethod == "cash" {
        payCode = "1001"
    }
    var opex, pay models.Account
    if err := database.DB.Where("code = ?", "5003").First(&opex).Error; err != nil {
        return fmt.Errorf("chart of accounts missing code 5003 (Operating Expenses): %w", err)
    }
    if err := database.DB.Where("code = ?", payCode).First(&pay).Error; err != nil {
        return fmt.Errorf("chart of accounts missing code %s: %w", payCode, err)
    }
    entryDate := time.Now()
    if p.PaidAt != nil {
        entryDate = *p.PaidAt
    }
    ref := fmt.Sprintf("PAYROLL-%d", payrollID)
    desc := fmt.Sprintf("Payroll for staff #%d, %02d/%d", p.StaffID, p.Month, p.Year)
    return database.DB.Transaction(func(tx *gorm.DB) error {
        for _, l := range []struct {
            acc uint
            typ string
        }{{opex.ID, "debit"}, {pay.ID, "credit"}} {
            e := models.LedgerEntry{
                TransactionRef: ref,
                AccountID:      l.acc,
                Type:           l.typ,
                Amount:         p.Amount,
                Description:    desc,
                ReferenceType:  "payroll",
                ReferenceID:    &payrollID,
                EntryDate:      entryDate,
                CreatedByID:    p.PaidByID,
            }
            if err := tx.Create(&e).Error; err != nil {
                return fmt.Errorf("failed to create payroll ledger entry: %w", err)
            }
        }
        return nil
    })
}