-- ============================================================
-- Remove PostgreSQL ENUMs → VARCHAR + CHECK constraints
-- ============================================================

-- 1. Convert all enum columns to VARCHAR
ALTER TABLE users ALTER COLUMN role TYPE VARCHAR(20) USING role::text;

ALTER TABLE products ALTER COLUMN station TYPE VARCHAR(20) USING station::text;

ALTER TABLE orders ALTER COLUMN order_type TYPE VARCHAR(20) USING order_type::text;
ALTER TABLE orders ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
ALTER TABLE orders ALTER COLUMN discount_type TYPE VARCHAR(20) USING discount_type::text;
ALTER TABLE orders ALTER COLUMN catering_status TYPE VARCHAR(20) USING catering_status::text;

ALTER TABLE order_items ALTER COLUMN discount_type TYPE VARCHAR(20) USING discount_type::text;
ALTER TABLE order_items ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
ALTER TABLE order_items ALTER COLUMN station TYPE VARCHAR(20) USING station::text;

ALTER TABLE payments ALTER COLUMN payment_method TYPE VARCHAR(20) USING payment_method::text;
ALTER TABLE payments ALTER COLUMN status TYPE VARCHAR(20) USING status::text;

-- 2. Drop defaults that reference enum types, then drop enum types
ALTER TABLE orders ALTER COLUMN status DROP DEFAULT;
ALTER TABLE order_items ALTER COLUMN status DROP DEFAULT;
ALTER TABLE payments ALTER COLUMN status DROP DEFAULT;

DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS order_type;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_item_status;
DROP TYPE IF EXISTS catering_status;
DROP TYPE IF EXISTS kitchen_station;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS discount_type;

-- Re-add defaults as plain VARCHAR values
ALTER TABLE orders ALTER COLUMN status SET DEFAULT 'NEW';
ALTER TABLE order_items ALTER COLUMN status SET DEFAULT 'PENDING';
ALTER TABLE payments ALTER COLUMN status SET DEFAULT 'COMPLETED';

-- 3. Add CHECK constraints for Group A (state machines)
ALTER TABLE orders ADD CONSTRAINT chk_orders_status
  CHECK (status IN ('NEW', 'PREPARING', 'READY', 'COMPLETED', 'CANCELLED'));

ALTER TABLE order_items ADD CONSTRAINT chk_order_items_status
  CHECK (status IN ('PENDING', 'PREPARING', 'READY'));

ALTER TABLE orders ADD CONSTRAINT chk_orders_catering_status
  CHECK (catering_status IS NULL OR catering_status IN ('BOOKED', 'DP_PAID', 'SETTLED', 'CANCELLED'));

ALTER TABLE payments ADD CONSTRAINT chk_payments_status
  CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED'));

-- 4. Add CHECK constraints for Group C (borderline)
ALTER TABLE users ADD CONSTRAINT chk_users_role
  CHECK (role IN ('OWNER', 'MANAGER', 'CASHIER', 'KITCHEN'));

ALTER TABLE orders ADD CONSTRAINT chk_orders_order_type
  CHECK (order_type IN ('DINE_IN', 'TAKEAWAY', 'DELIVERY', 'CATERING'));
