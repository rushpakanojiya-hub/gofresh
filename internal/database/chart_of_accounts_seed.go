package database

import "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"

// defaultChartOfAccounts is the standard account list from SRS 12.2.
// Seeded idempotently by unique Code, so re-running on every boot is
// always safe - existing rows (and any admin edits to name/active status)
// are left untouched, only missing codes get inserted. Codes here must
// stay in sync with internal/services/ledger_posting.go, which looks
// accounts up by code, not id.
var defaultChartOfAccounts = []models.Account{
	{Code: "1001", Name: "Cash", Type: "asset"},
	{Code: "1002", Name: "Bank", Type: "asset"},
	{Code: "1003", Name: "PG Receivable", Type: "asset"},
	{Code: "1004", Name: "Inventory", Type: "asset"},
	{Code: "1005", Name: "GST Input Credit (ITC)", Type: "asset"},
	{Code: "2001", Name: "Vendor Payable", Type: "liability"},
	{Code: "2002", Name: "GST Payable", Type: "liability"},
	{Code: "2003", Name: "Rider Payable", Type: "liability"},
	{Code: "2004", Name: "Customer Refund Payable", Type: "liability"},
	{Code: "2005", Name: "Customer Wallet Liability", Type: "liability"},
	{Code: "4001", Name: "Product Sales", Type: "revenue"},
	{Code: "5001", Name: "COGS", Type: "expense"},
	{Code: "5002", Name: "Discount Given", Type: "expense"},
	{Code: "5003", Name: "Operating Expenses", Type: "expense"},
	{Code: "5004", Name: "Rider Delivery Expense", Type: "expense"},
	{Code: "5005", Name: "Gateway Fees", Type: "expense"},
	{Code: "5006", Name: "Promotional Expense", Type: "expense"},
}

func seedChartOfAccounts() {
	for _, acc := range defaultChartOfAccounts {
		var existing models.Account
		if err := DB.Where("code = ?", acc.Code).First(&existing).Error; err != nil {
			acc.IsActive = true
			DB.Create(&acc)
		}
	}
}
