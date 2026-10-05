ALTER TABLE orders ADD COLUMN IF NOT EXISTS collected_via VARCHAR(10);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS upi_status VARCHAR(12);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS upi_verified_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS upi_utr VARCHAR(40);
CREATE INDEX IF NOT EXISTS idx_orders_upi_status ON orders (upi_status) WHERE collected_via = 'upi';