ALTER TABLE invoices DROP COLUMN IF EXISTS discount_amount;
ALTER TABLE invoices DROP COLUMN IF EXISTS address_line1;
ALTER TABLE invoices DROP COLUMN IF EXISTS address_line2;
ALTER TABLE invoices DROP COLUMN IF EXISTS address_city;
ALTER TABLE invoices DROP COLUMN IF EXISTS address_state;
ALTER TABLE invoices DROP COLUMN IF EXISTS address_pincode;

DROP SEQUENCE IF EXISTS invoice_number_seq;
