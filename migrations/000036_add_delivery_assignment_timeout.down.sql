ALTER TABLE orders DROP COLUMN IF EXISTS delivery_assignment_expires_at;
ALTER TABLE orders DROP COLUMN IF EXISTS delivery_attempted_partner_ids;
