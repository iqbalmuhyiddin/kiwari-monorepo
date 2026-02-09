-- ============================================================
-- Rollback: Recreate PostgreSQL ENUMs from VARCHAR
-- ============================================================

-- 1. Recreate enum types
CREATE TYPE user_role AS ENUM ('OWNER', 'MANAGER', 'CASHIER', 'KITCHEN');
CREATE TYPE order_type AS ENUM ('DINE_IN', 'TAKEAWAY', 'DELIVERY', 'CATERING');
CREATE TYPE order_status AS ENUM ('NEW', 'PREPARING', 'READY', 'COMPLETED', 'CANCELLED');
CREATE TYPE order_item_status AS ENUM ('PENDING', 'PREPARING', 'READY');
CREATE TYPE catering_status AS ENUM ('BOOKED', 'DP_PAID', 'SETTLED', 'CANCELLED');
CREATE TYPE kitchen_station AS ENUM ('GRILL', 'BEVERAGE', 'RICE', 'DESSERT');
CREATE TYPE payment_method AS ENUM ('CASH', 'QRIS', 'TRANSFER');
CREATE TYPE payment_status AS ENUM ('PENDING', 'COMPLETED', 'FAILED');
CREATE TYPE discount_type AS ENUM ('PERCENTAGE', 'FIXED_AMOUNT');

-- 2. Drop CHECK constraints
ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_status;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_catering_status;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_order_type;
ALTER TABLE order_items DROP CONSTRAINT IF EXISTS chk_order_items_status;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_status;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;

-- 3. Drop VARCHAR defaults before converting back (they block the type cast)
ALTER TABLE orders ALTER COLUMN status DROP DEFAULT;
ALTER TABLE order_items ALTER COLUMN status DROP DEFAULT;
ALTER TABLE payments ALTER COLUMN status DROP DEFAULT;

-- 4. Convert columns back to enum types
ALTER TABLE users ALTER COLUMN role TYPE user_role USING role::user_role;
ALTER TABLE products ALTER COLUMN station TYPE kitchen_station USING station::kitchen_station;
ALTER TABLE orders ALTER COLUMN order_type TYPE order_type USING order_type::order_type;
ALTER TABLE orders ALTER COLUMN status TYPE order_status USING status::order_status;
ALTER TABLE orders ALTER COLUMN discount_type TYPE discount_type USING discount_type::discount_type;
ALTER TABLE orders ALTER COLUMN catering_status TYPE catering_status USING catering_status::catering_status;
ALTER TABLE order_items ALTER COLUMN discount_type TYPE discount_type USING discount_type::discount_type;
ALTER TABLE order_items ALTER COLUMN status TYPE order_item_status USING status::order_item_status;
ALTER TABLE order_items ALTER COLUMN station TYPE kitchen_station USING station::kitchen_station;
ALTER TABLE payments ALTER COLUMN payment_method TYPE payment_method USING payment_method::payment_method;
ALTER TABLE payments ALTER COLUMN status TYPE payment_status USING status::payment_status;

-- 5. Re-add defaults as enum values
ALTER TABLE orders ALTER COLUMN status SET DEFAULT 'NEW'::order_status;
ALTER TABLE order_items ALTER COLUMN status SET DEFAULT 'PENDING'::order_item_status;
ALTER TABLE payments ALTER COLUMN status SET DEFAULT 'COMPLETED'::payment_status;
