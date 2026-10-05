DROP INDEX IF EXISTS idx_orders_upi_status;
ALTER TABLE orders DROP COLUMN IF EXISTS upi_utr;
ALTER TABLE orders DROP COLUMN IF EXISTS upi_verified_at;
ALTER TABLE orders DROP COLUMN IF EXISTS upi_status;
ALTER TABLE orders DROP COLUMN IF EXISTS collected_via;