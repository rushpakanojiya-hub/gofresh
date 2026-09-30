package database
import (
	"log"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)
var DB *gorm.DB
// ConnectDatabase opens a connection to PostgreSQL and stores it in DB.
func ConnectDatabase(cfg *config.Config) {
	// Only log every SQL query in dev mode. In production this slows down
	// every request and floods the logs.
	logLevel := logger.Info
	if cfg.GinMode == gin.ReleaseMode {
		logLevel = logger.Warn
	}
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	// Connection pool tuning - prevents stale/hanging connections on hosted
	// Postgres (common cause of slow requests after idle periods on Render).
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)
	DB = db
	log.Println("Database connected successfully")
}
// AutoMigrate creates/updates all tables based on the models.
// This covers: Users, OTPs, Categories, Products, Inventory, Cart,
// CartItems, Addresses, Orders, OrderItems, Payments.
func AutoMigrate() {
	err := DB.AutoMigrate(
		&models.User{},
		&models.OTP{},
		&models.Category{},
		&models.Product{},
		&models.Inventory{},
		&models.Cart{},
		&models.CartItem{},
		&models.Address{},
		&models.Order{},
		&models.OrderItem{},
		&models.Payment{},
		&models.Coupon{},
		&models.OrderCoupon{},
		&models.Notification{},
		&models.DeviceToken{},
		&models.Wishlist{},
		&models.Review{},
		&models.DeliveryPartner{},
		&models.AuditLog{},
		&models.Warehouse{},
		&models.WarehouseStaff{},
		&models.StockTransfer{},
		&models.Wallet{},
		&models.WalletTransaction{},
		&models.ReturnRequest{},
		&models.ReturnRequestItem{},
		&models.CartReservation{},
		&models.Setting{},
		&models.Offer{},
		&models.Banner{},
		&models.DeliveryZone{},
		&models.SupportTicket{},
		&models.SupportMessage{},
		&models.PickingTask{},
		&models.PickingTaskItem{},
		&models.PackingTask{},
		&models.OrderHandover{},
		&models.WarehouseZone{},
		&models.WarehouseRack{},
		&models.WarehouseBin{},
		&models.StockMovement{},
		&models.WarehouseException{},
		&models.SubstitutionRequest{},
		&models.WarehouseAuditLog{},
		&models.Receiving{},
		&models.Batch{},
		&models.Invoice{},
		&models.InvoiceItem{},
                &models.Expense{},
                &models.Payroll{},
                &models.Vendor{},
                &models.VendorBill{},
                &models.Account{},
                &models.LedgerEntry{},
                &models.BankTransaction{},
		&models.WarehouseNotification{},
            &models.CreditNote{},
            &models.CreditNoteItem{},
            &models.DebitNote{},
            &models.VendorBankChangeRequest{},
            &models.PendingJournalEntry{},
		&models.MismatchDismissal{},
            &models.RiderCODDeposit{},
            &models.RiderPayout{},
&models.DeliveryNotification{},
&models.WalletTopup{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}
	if err := DB.Exec(`ALTER TABLE warehouses ADD COLUMN IF NOT EXISTS service_area geometry(Polygon, 4326)`).Error; err != nil {
		log.Fatalf("Failed to add service_area column: %v", err)
	}
	if err := DB.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS gst_percent DOUBLE PRECISION NOT NULL DEFAULT 0`).Error; err != nil {
		log.Fatalf("Failed to add gst_percent column: %v", err)
	}
	seedDefaultSettings()
seedChartOfAccounts()
	log.Println("Database migration completed")
}
// EnsureProductionSchemaPatches applies small, idempotent schema patches
// that are needed even in production (GIN_MODE=release), where the full
// AutoMigrate() is skipped in favor of versioned migrations. This lets a
// column land immediately on deploy without requiring someone to run the
// migrate CLI by hand first - safe because ADD COLUMN IF NOT EXISTS is a
// no-op once the versioned migration has actually been applied.
func EnsureProductionSchemaPatches() {
if err := DB.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS gst_percent DOUBLE PRECISION NOT NULL DEFAULT 0`).Error; err != nil {
log.Fatalf("Failed to add gst_percent column: %v", err)
}
if err := DB.Exec(`ALTER TABLE orders ADD COLUMN IF NOT EXISTS platform_fee DOUBLE PRECISION NOT NULL DEFAULT 0`).Error; err != nil {
log.Fatalf("Failed to add platform_fee column to orders: %v", err)
}
if err := DB.Exec(`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS platform_fee DOUBLE PRECISION NOT NULL DEFAULT 0`).Error; err != nil {
log.Fatalf("Failed to add platform_fee column to invoices: %v", err)
}
if err := DB.Exec(`ALTER TABLE delivery_partners ADD COLUMN IF NOT EXISTS is_online BOOLEAN NOT NULL DEFAULT false`).Error; err != nil {
log.Fatalf("Failed to add is_online column to delivery_partners: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS subcategories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category_id BIGINT NOT NULL REFERENCES categories(id),
    image_url TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
)`).Error; err != nil {
log.Fatalf("Failed to create subcategories table: %v", err)
}
if err := DB.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS subcategory_id BIGINT REFERENCES subcategories(id)`).Error; err != nil {
log.Fatalf("Failed to add subcategory_id column to products: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS expenses (
id BIGSERIAL PRIMARY KEY,
amount DOUBLE PRECISION NOT NULL,
category VARCHAR(30) NOT NULL,
expense_date TIMESTAMPTZ NOT NULL,
warehouse_id BIGINT NULL REFERENCES warehouses(id),
note TEXT,
receipt_url TEXT,
added_by_id BIGINT NOT NULL REFERENCES users(id),
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`).Error; err != nil {
log.Fatalf("Failed to create expenses table: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category)`).Error; err != nil {
log.Fatalf("Failed to create expenses category index: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_expenses_expense_date ON expenses(expense_date)`).Error; err != nil {
log.Fatalf("Failed to create expenses date index: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_expenses_warehouse_id ON expenses(warehouse_id)`).Error; err != nil {
log.Fatalf("Failed to create expenses warehouse index: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS payrolls (
id BIGSERIAL PRIMARY KEY,
staff_id BIGINT NOT NULL REFERENCES warehouse_staffs(id),
amount DOUBLE PRECISION NOT NULL,
month INTEGER NOT NULL,
year INTEGER NOT NULL,
status VARCHAR(20) NOT NULL DEFAULT 'pending',
payment_method VARCHAR(20),
note TEXT,
paid_by_id BIGINT NULL REFERENCES users(id),
paid_at TIMESTAMPTZ NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`).Error; err != nil {
log.Fatalf("Failed to create payrolls table: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_payrolls_staff_id ON payrolls(staff_id)`).Error; err != nil {
log.Fatalf("Failed to create payrolls staff index: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_payrolls_status ON payrolls(status)`).Error; err != nil {
log.Fatalf("Failed to create payrolls status index: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_payrolls_month_year ON payrolls(month, year)`).Error; err != nil {
log.Fatalf("Failed to create payrolls month_year index: %v", err)
}
if err := DB.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS cost_price DOUBLE PRECISION NOT NULL DEFAULT 0`).Error; err != nil {
log.Fatalf("Failed to add cost_price column to products: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS vendors (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    contact_name VARCHAR(255),
    phone VARCHAR(20),
    email VARCHAR(255),
    gstin VARCHAR(20),
    address TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS vendor_bills (
    id BIGSERIAL PRIMARY KEY,
    vendor_id BIGINT NOT NULL REFERENCES vendors(id),
    bill_number VARCHAR(100),
    amount DOUBLE PRECISION NOT NULL,
    amount_paid DOUBLE PRECISION NOT NULL DEFAULT 0,
    bill_date TIMESTAMPTZ NOT NULL,
    due_date TIMESTAMPTZ NULL,
    note TEXT,
    created_by_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_vendor_bills_vendor_id ON vendor_bills(vendor_id);
CREATE TABLE IF NOT EXISTS accounts (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_accounts_type ON accounts(type);
CREATE TABLE IF NOT EXISTS ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    transaction_ref VARCHAR(64) NOT NULL,
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    type VARCHAR(10) NOT NULL,
    amount DOUBLE PRECISION NOT NULL,
    description TEXT,
    reference_type VARCHAR(50),
    reference_id BIGINT,
    entry_date TIMESTAMPTZ NOT NULL,
    created_by_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_transaction_ref ON ledger_entries(transaction_ref);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_account_id ON ledger_entries(account_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_entry_date ON ledger_entries(entry_date);
CREATE TABLE IF NOT EXISTS bank_transactions (
    id BIGSERIAL PRIMARY KEY,
    transaction_date TIMESTAMPTZ NOT NULL,
    description TEXT,
    amount DOUBLE PRECISION NOT NULL,
    reference_number VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'unmatched',
    matched_type VARCHAR(50),
    matched_id BIGINT,
    matched_at TIMESTAMPTZ NULL,
    matched_by_id BIGINT NULL REFERENCES users(id),
    note TEXT,
    created_by_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_bank_transactions_status ON bank_transactions(status);
CREATE INDEX IF NOT EXISTS idx_bank_transactions_transaction_date ON bank_transactions(transaction_date);`).Error; err != nil {
log.Fatalf("Failed to create accounting module tables: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendor_bills ADD COLUMN IF NOT EXISTS gst_amount DOUBLE PRECISION NOT NULL DEFAULT 0`).Error; err != nil {
log.Fatalf("Failed to add gst_amount column to vendor_bills: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendor_bills ADD COLUMN IF NOT EXISTS hold_status VARCHAR(20) NOT NULL DEFAULT ''`).Error; err != nil {
log.Fatalf("Failed to add hold_status column to vendor_bills: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendor_bills ADD COLUMN IF NOT EXISTS hold_reason TEXT`).Error; err != nil {
log.Fatalf("Failed to add hold_reason column to vendor_bills: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendor_bills ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ`).Error; err != nil {
log.Fatalf("Failed to add voided_at column to vendor_bills: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendor_bills ADD COLUMN IF NOT EXISTS void_reason TEXT`).Error; err != nil {
log.Fatalf("Failed to add void_reason column to vendor_bills: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendor_bills ADD COLUMN IF NOT EXISTS voided_by_id BIGINT NULL REFERENCES users(id)`).Error; err != nil {
log.Fatalf("Failed to add voided_by_id column to vendor_bills: %v", err)
}
if err := DB.Exec(`ALTER TABLE bank_transactions ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ`).Error; err != nil {
log.Fatalf("Failed to add voided_at column to bank_transactions: %v", err)
}
if err := DB.Exec(`ALTER TABLE bank_transactions ADD COLUMN IF NOT EXISTS void_reason TEXT`).Error; err != nil {
log.Fatalf("Failed to add void_reason column to bank_transactions: %v", err)
}
if err := DB.Exec(`ALTER TABLE bank_transactions ADD COLUMN IF NOT EXISTS voided_by_id BIGINT NULL REFERENCES users(id)`).Error; err != nil {
log.Fatalf("Failed to add voided_by_id column to bank_transactions: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS credit_notes (
    id BIGSERIAL PRIMARY KEY,
    credit_note_number VARCHAR(50) UNIQUE NOT NULL,
    invoice_id BIGINT NOT NULL REFERENCES invoices(id),
    order_id BIGINT NOT NULL REFERENCES orders(id),
    return_request_id BIGINT NULL REFERENCES return_requests(id),
    customer_name VARCHAR(255),
    customer_phone VARCHAR(20),
    reason TEXT,
    taxable_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    cgst_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    sgst_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    igst_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    issued_at TIMESTAMPTZ NOT NULL,
    created_by_id BIGINT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_credit_notes_invoice_id ON credit_notes(invoice_id);
CREATE INDEX IF NOT EXISTS idx_credit_notes_order_id ON credit_notes(order_id);
CREATE INDEX IF NOT EXISTS idx_credit_notes_return_request_id ON credit_notes(return_request_id);
CREATE TABLE IF NOT EXISTS credit_note_items (
    id BIGSERIAL PRIMARY KEY,
    credit_note_id BIGINT NOT NULL REFERENCES credit_notes(id),
    product_id BIGINT,
    product_name VARCHAR(255),
    quantity INTEGER NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    gst_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    gst_amount DOUBLE PRECISION NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_credit_note_items_credit_note_id ON credit_note_items(credit_note_id);
CREATE TABLE IF NOT EXISTS debit_notes (
    id BIGSERIAL PRIMARY KEY,
    debit_note_number VARCHAR(50) UNIQUE NOT NULL,
    vendor_bill_id BIGINT NOT NULL REFERENCES vendor_bills(id),
    vendor_id BIGINT NOT NULL REFERENCES vendors(id),
    reason TEXT NOT NULL,
    amount DOUBLE PRECISION NOT NULL,
    gst_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    issued_at TIMESTAMPTZ NOT NULL,
    created_by_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_debit_notes_vendor_bill_id ON debit_notes(vendor_bill_id);
CREATE INDEX IF NOT EXISTS idx_debit_notes_vendor_id ON debit_notes(vendor_id);`).Error; err != nil {
log.Fatalf("Failed to create credit/debit note tables: %v", err)
}
if err := DB.Exec(`ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_status VARCHAR(20)`).Error; err != nil {
log.Fatalf("Failed to add delivery_status column to orders: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_delivery_status ON orders(delivery_status)`).Error; err != nil {
log.Fatalf("Failed to create delivery_status index on orders: %v", err)
}
if err := DB.Exec(`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS approval_status VARCHAR(20) NOT NULL DEFAULT 'draft'`).Error; err != nil {
log.Fatalf("Failed to add approval_status column to expenses: %v", err)
}
if err := DB.Exec(`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS approved_by_id BIGINT NULL REFERENCES users(id)`).Error; err != nil {
log.Fatalf("Failed to add approved_by_id column to expenses: %v", err)
}
if err := DB.Exec(`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ`).Error; err != nil {
log.Fatalf("Failed to add approved_at column to expenses: %v", err)
}
if err := DB.Exec(`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS rejection_reason TEXT`).Error; err != nil {
log.Fatalf("Failed to add rejection_reason column to expenses: %v", err)
}
if err := DB.Exec(`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ`).Error; err != nil {
log.Fatalf("Failed to add paid_at column to expenses: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_expenses_approval_status ON expenses(approval_status)`).Error; err != nil {
log.Fatalf("Failed to create approval_status index on expenses: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendors ADD COLUMN IF NOT EXISTS bank_account_holder VARCHAR(255) NOT NULL DEFAULT ''`).Error; err != nil {
log.Fatalf("Failed to add bank_account_holder column to vendors: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendors ADD COLUMN IF NOT EXISTS bank_account_number VARCHAR(50) NOT NULL DEFAULT ''`).Error; err != nil {
log.Fatalf("Failed to add bank_account_number column to vendors: %v", err)
}
if err := DB.Exec(`ALTER TABLE vendors ADD COLUMN IF NOT EXISTS bank_ifsc VARCHAR(20) NOT NULL DEFAULT ''`).Error; err != nil {
log.Fatalf("Failed to add bank_ifsc column to vendors: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS vendor_bank_change_requests (
    id BIGSERIAL PRIMARY KEY,
    vendor_id BIGINT NOT NULL REFERENCES vendors(id),
    new_account_holder VARCHAR(255),
    new_account_number VARCHAR(50),
    new_ifsc VARCHAR(20),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    requested_by_id BIGINT NOT NULL REFERENCES users(id),
    approved_by_id BIGINT NULL REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_vendor_bank_change_requests_vendor_id ON vendor_bank_change_requests(vendor_id);
CREATE INDEX IF NOT EXISTS idx_vendor_bank_change_requests_status ON vendor_bank_change_requests(status);`).Error; err != nil {
log.Fatalf("Failed to create vendor_bank_change_requests table: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS pending_journal_entries (
    id BIGSERIAL PRIMARY KEY,
    entry_date VARCHAR(10),
    lines_json TEXT,
    description TEXT,
    total_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    requested_by_id BIGINT NOT NULL REFERENCES users(id),
    approved_by_id BIGINT NULL REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pending_journal_entries_status ON pending_journal_entries(status);`).Error; err != nil {
log.Fatalf("Failed to create pending_journal_entries table: %v", err)
}
if err := DB.Exec(`ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway_fee DOUBLE PRECISION NOT NULL DEFAULT 0`).Error; err != nil {
log.Fatalf("Failed to add gateway_fee column to payments: %v", err)
}
if err := DB.Exec(`ALTER TABLE payments ADD COLUMN IF NOT EXISTS is_settled BOOLEAN NOT NULL DEFAULT false`).Error; err != nil {
log.Fatalf("Failed to add is_settled column to payments: %v", err)
}
if err := DB.Exec(`ALTER TABLE payments ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ`).Error; err != nil {
log.Fatalf("Failed to add settled_at column to payments: %v", err)
}
if err := DB.Exec(`CREATE TABLE IF NOT EXISTS rider_cod_deposits (
    id BIGSERIAL PRIMARY KEY,
    delivery_partner_id BIGINT NOT NULL REFERENCES delivery_partners(id),
    amount DOUBLE PRECISION NOT NULL,
    deposit_date TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    note TEXT,
    verified_by_id BIGINT NULL REFERENCES users(id),
    verified_at TIMESTAMPTZ,
    created_by_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rider_cod_deposits_partner_id ON rider_cod_deposits(delivery_partner_id);
CREATE INDEX IF NOT EXISTS idx_rider_cod_deposits_status ON rider_cod_deposits(status);
CREATE TABLE IF NOT EXISTS rider_payouts (
    id BIGSERIAL PRIMARY KEY,
    delivery_partner_id BIGINT NOT NULL REFERENCES delivery_partners(id),
    period_from TIMESTAMPTZ NOT NULL,
    period_to TIMESTAMPTZ NOT NULL,
    delivered_count INTEGER NOT NULL,
    amount DOUBLE PRECISION NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    approved_by_id BIGINT NULL REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_by_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rider_payouts_partner_id ON rider_payouts(delivery_partner_id);
CREATE INDEX IF NOT EXISTS idx_rider_payouts_status ON rider_payouts(status);`).Error; err != nil {
log.Fatalf("Failed to create rider_cod_deposits/rider_payouts tables: %v", err)
}
seedDefaultSettings()
seedChartOfAccounts()
if err := DB.Exec(`ALTER TABLE ledger_entries ALTER COLUMN created_by_id DROP NOT NULL`).Error; err != nil {
log.Fatalf("Failed to drop NOT NULL on ledger_entries.created_by_id: %v", err)
}
if err := DB.Exec(`ALTER TABLE addresses ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT false`).Error; err != nil {
log.Fatalf("Failed to add is_deleted column to addresses: %v", err)
}
if err := DB.Exec(`CREATE INDEX IF NOT EXISTS idx_addresses_is_deleted ON addresses(is_deleted)`).Error; err != nil {
log.Fatalf("Failed to create addresses is_deleted index: %v", err)
}
	_ = DB.Exec(`CREATE SEQUENCE IF NOT EXISTS public.po_number_seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1`)
	_ = DB.Exec(`ALTER TABLE otps ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0`)
	_ = DB.Exec(`CREATE TABLE IF NOT EXISTS wallet_topups (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id),
		amount DOUBLE PRECISION NOT NULL,
		razorpay_order_id VARCHAR(255) NOT NULL,
		razorpay_payment_id VARCHAR(255),
		razorpay_signature VARCHAR(255),
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	_ = DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_product ON cart_items(cart_id, product_id)`)
	_ = DB.Exec(`ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivery_proof_url TEXT`)
	_ = DB.Exec(`ALTER TABLE return_requests ADD COLUMN IF NOT EXISTS rejection_reason TEXT`)
	_ = DB.Exec(`ALTER TABLE return_requests ADD COLUMN IF NOT EXISTS image_url TEXT`)

    RunMISMigration()
}
// EnsureInvoiceSchemaPatches applies the invoice-related idempotent schema
// patches once at startup, instead of running them inside every
// GenerateInvoiceIfNotExists transaction (which took an ACCESS EXCLUSIVE
// lock on orders/invoices on every call and deadlocked against
// AutoAssignDeliveryPartner row locks).
func EnsureInvoiceSchemaPatches() {
if err := DB.Exec("CREATE SEQUENCE IF NOT EXISTS invoice_number_seq START 1").Error; err != nil {
log.Fatalf("Failed to ensure invoice sequence exists: %v", err)
}
invoiceColumnStatements := []string{
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS discount_amount DOUBLE PRECISION NOT NULL DEFAULT 0`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS address_line1 VARCHAR(255) NOT NULL DEFAULT ''`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS address_line2 VARCHAR(255) NOT NULL DEFAULT ''`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS address_city VARCHAR(100) NOT NULL DEFAULT ''`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS address_state VARCHAR(100) NOT NULL DEFAULT ''`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS address_pincode VARCHAR(20) NOT NULL DEFAULT ''`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS payment_reference VARCHAR(100) NOT NULL DEFAULT ''`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS is_inter_state BOOLEAN NOT NULL DEFAULT false`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS taxable_amount DOUBLE PRECISION NOT NULL DEFAULT 0`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS cgst_amount DOUBLE PRECISION NOT NULL DEFAULT 0`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS sgst_amount DOUBLE PRECISION NOT NULL DEFAULT 0`,
`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS igst_amount DOUBLE PRECISION NOT NULL DEFAULT 0`,
`ALTER TABLE invoice_items ADD COLUMN IF NOT EXISTS sku VARCHAR(100) NOT NULL DEFAULT ''`,
`ALTER TABLE invoice_items ADD COLUMN IF NOT EXISTS gst_percent DOUBLE PRECISION NOT NULL DEFAULT 0`,
`ALTER TABLE invoice_items ADD COLUMN IF NOT EXISTS hsn_code VARCHAR(20) NOT NULL DEFAULT ''`,
`ALTER TABLE products ADD COLUMN IF NOT EXISTS hsn_code VARCHAR(20) NOT NULL DEFAULT ''`,
`ALTER TABLE invoice_items ADD COLUMN IF NOT EXISTS gst_amount DOUBLE PRECISION NOT NULL DEFAULT 0`,
}
for _, stmt := range invoiceColumnStatements {
if err := DB.Exec(stmt).Error; err != nil {
log.Fatalf("Failed to ensure invoice columns exist: %v", err)
}
}
}

