package utils

import (
"errors"

"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
"gorm.io/gorm"
"gorm.io/gorm/clause"
)

// GetOrCreateWallet returns the user's wallet, creating one with a zero
// balance if it doesn't exist yet. Must be called within a transaction
// when the caller intends to modify the balance right after, so the row
// gets locked consistently.
func GetOrCreateWallet(tx *gorm.DB, userID uint) (*models.Wallet, error) {
var wallet models.Wallet
err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
Where("user_id = ?", userID).First(&wallet).Error
if err == nil {
return &wallet, nil
}
if !errors.Is(err, gorm.ErrRecordNotFound) {
return nil, err
}
// BUG-36: concurrent first-time creates must not duplicate or fail; insert-if-absent, then re-read under lock.
	newWallet := models.Wallet{UserID: userID, Balance: 0}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&newWallet).Error; err != nil {
		return nil, err
	}
	var locked models.Wallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&locked).Error; err != nil {
		return nil, err
	}
	return &locked, nil
}

// CreditWallet adds amount to the user's wallet balance and writes a ledger
// entry. Must be called within a transaction.
func CreditWallet(tx *gorm.DB, userID uint, amount float64, reason string, refType string, refID *uint, note string) error {
if amount <= 0 {
return errors.New("credit amount must be positive")
}
wallet, err := GetOrCreateWallet(tx, userID)
if err != nil {
return err
}
wallet.Balance += amount
if err := tx.Save(wallet).Error; err != nil {
return err
}
txn := models.WalletTransaction{
WalletID:      wallet.ID,
Type:          models.WalletTxnCredit,
Amount:        amount,
Reason:        reason,
ReferenceType: refType,
ReferenceID:   refID,
BalanceAfter:  wallet.Balance,
Note:          note,
}
return tx.Create(&txn).Error
}

// DebitWallet subtracts amount from the user's wallet balance and writes a
// ledger entry. Fails if the balance would go negative. Must be called
// within a transaction.
func DebitWallet(tx *gorm.DB, userID uint, amount float64, reason string, refType string, refID *uint, note string) error {
if amount <= 0 {
return errors.New("debit amount must be positive")
}
wallet, err := GetOrCreateWallet(tx, userID)
if err != nil {
return err
}
if wallet.Balance < amount {
return errors.New("insufficient wallet balance")
}
wallet.Balance -= amount
if err := tx.Save(wallet).Error; err != nil {
return err
}
txn := models.WalletTransaction{
WalletID:      wallet.ID,
Type:          models.WalletTxnDebit,
Amount:        amount,
Reason:        reason,
ReferenceType: refType,
ReferenceID:   refID,
BalanceAfter:  wallet.Balance,
Note:          note,
}
return tx.Create(&txn).Error
}