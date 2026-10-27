package handlers

import (
    "errors"
    "net/http"

    "gorm.io/gorm"
    "gorm.io/gorm/clause"

    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
)

// BUG-12: a payroll record that has been paid, or that already has ledger
// entries, is part of the audit trail and must not be hard-deleted (Payroll
// has no soft-delete column, so the row would be gone for good and its
// PAYROLL-<id> journal entries would be orphaned). The check and the delete
// run in one transaction with a row lock.
func deletePayrollGuarded(id string) (int, string) {
    status := http.StatusInternalServerError
    msg := "Failed to delete payroll record"

    err := database.DB.Transaction(func(tx *gorm.DB) error {
        var p models.Payroll
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, id).Error; err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                status, msg = http.StatusNotFound, "Payroll record not found"
            }
            return err
        }
        if p.Status == "paid" {
            status, msg = http.StatusConflict, "Paid payroll records cannot be deleted"
            return errors.New(msg)
        }

        // Safety net: even if the status is not "paid", never delete a record
        // that already has ledger entries.
        var n int64
        if err := tx.Model(&models.LedgerEntry{}).
            Where("reference_type = ? AND reference_id = ?", "payroll", p.ID).
            Count(&n).Error; err != nil {
            return err
        }
        if n > 0 {
            status, msg = http.StatusConflict, "Payroll record has ledger entries and cannot be deleted"
            return errors.New(msg)
        }

        return tx.Delete(&models.Payroll{}, p.ID).Error
    })
    if err != nil {
        return status, msg
    }
    return http.StatusOK, ""
}