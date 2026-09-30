package models

import "time"

// MismatchItem is a detected inconsistency between two financial records
// (order vs payment, payment vs ledger, etc). It is never persisted itself -
// each request to the Mismatch Center re-runs the detection queries live
// against Orders/Payments/LedgerEntry/BankTransaction, so results always
// reflect current data. IsDismissed is populated by joining against
// MismatchDismissal at read time.
type MismatchItem struct {
CheckType      string    `json:"check_type"`  // order_payment_amount / sale_not_posted / refund_ledger_mismatch / bank_unmatched / duplicate_payment_ref / refund_exceeds_payment
Severity       string    `json:"severity"`     // low / medium / high
EntityType     string    `json:"entity_type"`  // order / payment / bank_transaction
EntityID       uint      `json:"entity_id"`
OrderID        *uint     `json:"order_id,omitempty"`
Description    string    `json:"description"`
DetectedAmount float64   `json:"detected_amount,omitempty"`
DetectedAt     time.Time `json:"detected_at"`
IsDismissed    bool      `json:"is_dismissed"`
}

// MismatchDismissal records that a finance user has reviewed and
// acknowledged a specific detected mismatch (e.g. a known/explained
// discrepancy), so it stops surfacing as an open item. Keyed on
// (CheckType, EntityID) rather than a foreign key, since EntityID can
// point at different tables depending on CheckType.
type MismatchDismissal struct {
ID            uint      `gorm:"primaryKey" json:"id"`
CheckType     string    `gorm:"not null;index:idx_mismatch_dismissal_lookup,priority:1" json:"check_type"`
EntityID      uint      `gorm:"not null;index:idx_mismatch_dismissal_lookup,priority:2" json:"entity_id"`
Reason        string    `json:"reason"`
DismissedByID uint      `gorm:"not null" json:"dismissed_by_id"`
DismissedAt   time.Time `json:"dismissed_at"`
}

// MismatchDismissRequest is the body for POST /admin/finance/mismatch-center/dismiss
type MismatchDismissRequest struct {
CheckType string `json:"check_type" binding:"required"`
EntityID  uint   `json:"entity_id" binding:"required"`
Reason    string `json:"reason" binding:"required"`
}