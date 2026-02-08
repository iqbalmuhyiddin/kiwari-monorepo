-- =============================================================================
-- Kiwari POS — Seed Data
-- =============================================================================
-- Realistic Nasi Bakar menu with customers, orders, and payments.
-- Idempotent: safe to re-run (TRUNCATE CASCADE first).
--
-- Usage:  psql "$DATABASE_URL" -f api/seed/seed.sql
-- =============================================================================

BEGIN;

-- Clear everything (order matters for FKs, CASCADE handles the rest)
TRUNCATE TABLE payments, order_item_modifiers, order_items, orders,
               combo_items, customers, modifiers, modifier_groups,
               variants, variant_groups, products, categories,
               users, outlets
CASCADE;

-- =============================================================================
-- 1. OUTLET
-- =============================================================================
INSERT INTO outlets (id, name, address, phone) VALUES
  ('17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Kiwari Nasi Bakar', 'Jl. Dago No. 123, Bandung', '081234567890');

-- =============================================================================
-- 2. USERS (password: password123)
-- =============================================================================
INSERT INTO users (id, outlet_id, email, hashed_password, full_name, role, pin) VALUES
  ('a0000000-0000-4000-a000-000000000001', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'admin@kiwari.com',   '$2a$10$4dSuZG2oJOFIjtp8mU6H9eYNuykf.tm5g2eUNN4bIKx8C8xQX6UbC', 'Iqbal Muhyiddin', 'OWNER',   '1234'),
  ('a0000000-0000-4000-a000-000000000002', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'siti@kiwari.com',    '$2a$10$4dSuZG2oJOFIjtp8mU6H9eYNuykf.tm5g2eUNN4bIKx8C8xQX6UbC', 'Siti Rahayu',     'MANAGER', '5678'),
  ('a0000000-0000-4000-a000-000000000003', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'budi@kiwari.com',    '$2a$10$4dSuZG2oJOFIjtp8mU6H9eYNuykf.tm5g2eUNN4bIKx8C8xQX6UbC', 'Budi Santoso',    'CASHIER', '1111'),
  ('a0000000-0000-4000-a000-000000000004', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'dewi@kiwari.com',    '$2a$10$4dSuZG2oJOFIjtp8mU6H9eYNuykf.tm5g2eUNN4bIKx8C8xQX6UbC', 'Dewi Lestari',    'KITCHEN', '2222');

-- =============================================================================
-- 3. CATEGORIES
-- =============================================================================
INSERT INTO categories (id, outlet_id, name, description, sort_order) VALUES
  ('ca000000-0000-4000-a000-000000000001', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Nasi Bakar',   'Menu utama nasi bakar',       1),
  ('ca000000-0000-4000-a000-000000000002', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Minuman',      'Aneka minuman segar',         2),
  ('ca000000-0000-4000-a000-000000000003', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Camilan',      'Lauk tambahan dan snack',     3),
  ('ca000000-0000-4000-a000-000000000004', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Paket Hemat',  'Paket bundling hemat',        4);

-- =============================================================================
-- 4. PRODUCTS
-- =============================================================================

-- Nasi Bakar (GRILL, 15 min)
INSERT INTO products (id, outlet_id, category_id, name, description, base_price, station, preparation_time) VALUES
  ('d0000000-0000-4000-a000-000000000001', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000001', 'Nasi Bakar Ayam',       'Nasi bakar dengan ayam suwir bumbu rempah',          25000.00, 'GRILL', 15),
  ('d0000000-0000-4000-a000-000000000002', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000001', 'Nasi Bakar Cumi',       'Nasi bakar dengan cumi asin pedas',                  28000.00, 'GRILL', 15),
  ('d0000000-0000-4000-a000-000000000003', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000001', 'Nasi Bakar Iga Sapi',   'Nasi bakar dengan iga sapi bakar empuk',              35000.00, 'GRILL', 15),
  ('d0000000-0000-4000-a000-000000000004', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000001', 'Nasi Bakar Jamur',      'Nasi bakar dengan jamur crispy',                      22000.00, 'GRILL', 15),
  ('d0000000-0000-4000-a000-000000000005', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000001', 'Nasi Bakar Teri Medan', 'Nasi bakar dengan teri medan dan kacang',             23000.00, 'GRILL', 15),
  ('d0000000-0000-4000-a000-000000000006', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000001', 'Nasi Bakar Seafood',    'Nasi bakar dengan campuran udang, cumi, dan kerang',  32000.00, 'GRILL', 15);

-- Minuman (BEVERAGE, 5 min)
INSERT INTO products (id, outlet_id, category_id, name, description, base_price, station, preparation_time) VALUES
  ('d0000000-0000-4000-a000-000000000007', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000002', 'Es Teh Manis',   'Teh manis dingin segar',         5000.00, 'BEVERAGE', 5),
  ('d0000000-0000-4000-a000-000000000008', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000002', 'Es Jeruk',       'Jeruk peras segar dengan es',    8000.00, 'BEVERAGE', 5),
  ('d0000000-0000-4000-a000-000000000009', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000002', 'Kopi Susu',      'Kopi susu gula aren',           12000.00, 'BEVERAGE', 5),
  ('d0000000-0000-4000-a000-00000000000a', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000002', 'Jus Alpukat',    'Jus alpukat segar dengan susu',  15000.00, 'BEVERAGE', 5),
  ('d0000000-0000-4000-a000-00000000000b', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000002', 'Air Mineral',    'Air mineral botol',              4000.00, 'BEVERAGE', 5);

-- Camilan (GRILL/DESSERT, 10 min)
INSERT INTO products (id, outlet_id, category_id, name, description, base_price, station, preparation_time) VALUES
  ('d0000000-0000-4000-a000-00000000000c', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000003', 'Tempe Goreng (5 pcs)',      'Tempe goreng tepung crispy',          8000.00, 'GRILL',   10),
  ('d0000000-0000-4000-a000-00000000000d', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000003', 'Tahu Crispy (5 pcs)',       'Tahu goreng tepung crispy',           8000.00, 'GRILL',   10),
  ('d0000000-0000-4000-a000-00000000000e', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000003', 'Kerupuk Udang',             'Kerupuk udang goreng',                5000.00, 'GRILL',   10),
  ('d0000000-0000-4000-a000-00000000000f', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000003', 'Pisang Bakar Coklat',       'Pisang bakar dengan topping coklat', 12000.00, 'DESSERT', 10);

-- Paket Hemat (combos)
INSERT INTO products (id, outlet_id, category_id, name, description, base_price, station, preparation_time, is_combo) VALUES
  ('d0000000-0000-4000-a000-000000000010', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000004', 'Paket Ayam Komplit', 'Nasi Bakar Ayam + Es Teh Manis',      32000.00, 'GRILL', 15, true),
  ('d0000000-0000-4000-a000-000000000011', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'ca000000-0000-4000-a000-000000000004', 'Paket Iga Spesial',  'Nasi Bakar Iga Sapi + Jus Alpukat',   45000.00, 'GRILL', 15, true);

-- =============================================================================
-- 5. COMBO ITEMS
-- =============================================================================
INSERT INTO combo_items (id, combo_id, product_id, quantity, sort_order) VALUES
  ('cb000000-0000-4000-a000-000000000001', 'd0000000-0000-4000-a000-000000000010', 'd0000000-0000-4000-a000-000000000001', 1, 1),  -- Paket Ayam → NB Ayam
  ('cb000000-0000-4000-a000-000000000002', 'd0000000-0000-4000-a000-000000000010', 'd0000000-0000-4000-a000-000000000007', 1, 2),  -- Paket Ayam → Es Teh
  ('cb000000-0000-4000-a000-000000000003', 'd0000000-0000-4000-a000-000000000011', 'd0000000-0000-4000-a000-000000000003', 1, 1),  -- Paket Iga  → NB Iga
  ('cb000000-0000-4000-a000-000000000004', 'd0000000-0000-4000-a000-000000000011', 'd0000000-0000-4000-a000-00000000000a', 1, 2);  -- Paket Iga  → Jus Alpukat

-- =============================================================================
-- 6. VARIANT GROUPS  (pick one — stored in order_items.variant_id)
-- =============================================================================
-- Ukuran for each Nasi Bakar product
INSERT INTO variant_groups (id, product_id, name, is_required, sort_order) VALUES
  ('e0000000-0000-4000-a000-000000000001', 'd0000000-0000-4000-a000-000000000001', 'Ukuran', true, 1),
  ('e0000000-0000-4000-a000-000000000002', 'd0000000-0000-4000-a000-000000000002', 'Ukuran', true, 1),
  ('e0000000-0000-4000-a000-000000000003', 'd0000000-0000-4000-a000-000000000003', 'Ukuran', true, 1),
  ('e0000000-0000-4000-a000-000000000004', 'd0000000-0000-4000-a000-000000000004', 'Ukuran', true, 1),
  ('e0000000-0000-4000-a000-000000000005', 'd0000000-0000-4000-a000-000000000005', 'Ukuran', true, 1),
  ('e0000000-0000-4000-a000-000000000006', 'd0000000-0000-4000-a000-000000000006', 'Ukuran', true, 1),
  -- Ukuran for drinks with size option
  ('e0000000-0000-4000-a000-000000000007', 'd0000000-0000-4000-a000-000000000009', 'Ukuran', true, 1),  -- Kopi Susu
  ('e0000000-0000-4000-a000-000000000008', 'd0000000-0000-4000-a000-00000000000a', 'Ukuran', true, 1);  -- Jus Alpukat

-- =============================================================================
-- 7. VARIANTS
-- =============================================================================
-- Nasi Bakar: Reguler (+0) / Jumbo (+5000)
INSERT INTO variants (id, variant_group_id, name, price_adjustment, sort_order) VALUES
  -- NB Ayam
  ('f0000000-0000-4000-a000-000000000001', 'e0000000-0000-4000-a000-000000000001', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-000000000002', 'e0000000-0000-4000-a000-000000000001', 'Jumbo',   5000.00, 2),
  -- NB Cumi
  ('f0000000-0000-4000-a000-000000000003', 'e0000000-0000-4000-a000-000000000002', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-000000000004', 'e0000000-0000-4000-a000-000000000002', 'Jumbo',   5000.00, 2),
  -- NB Iga
  ('f0000000-0000-4000-a000-000000000005', 'e0000000-0000-4000-a000-000000000003', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-000000000006', 'e0000000-0000-4000-a000-000000000003', 'Jumbo',   5000.00, 2),
  -- NB Jamur
  ('f0000000-0000-4000-a000-000000000007', 'e0000000-0000-4000-a000-000000000004', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-000000000008', 'e0000000-0000-4000-a000-000000000004', 'Jumbo',   5000.00, 2),
  -- NB Teri
  ('f0000000-0000-4000-a000-000000000009', 'e0000000-0000-4000-a000-000000000005', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-00000000000a', 'e0000000-0000-4000-a000-000000000005', 'Jumbo',   5000.00, 2),
  -- NB Seafood
  ('f0000000-0000-4000-a000-00000000000b', 'e0000000-0000-4000-a000-000000000006', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-00000000000c', 'e0000000-0000-4000-a000-000000000006', 'Jumbo',   5000.00, 2),
  -- Kopi Susu: Reguler / Large
  ('f0000000-0000-4000-a000-00000000000d', 'e0000000-0000-4000-a000-000000000007', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-00000000000e', 'e0000000-0000-4000-a000-000000000007', 'Large',   5000.00, 2),
  -- Jus Alpukat: Reguler / Large
  ('f0000000-0000-4000-a000-00000000000f', 'e0000000-0000-4000-a000-000000000008', 'Reguler', 0.00,    1),
  ('f0000000-0000-4000-a000-000000000010', 'e0000000-0000-4000-a000-000000000008', 'Large',   5000.00, 2);

-- =============================================================================
-- 8. MODIFIER GROUPS
-- =============================================================================

-- Level Pedas for each Nasi Bakar (min=1, max=1 → required single-select)
INSERT INTO modifier_groups (id, product_id, name, min_select, max_select, sort_order) VALUES
  ('b0000000-0000-4000-a000-000000000001', 'd0000000-0000-4000-a000-000000000001', 'Level Pedas', 1, 1, 1),
  ('b0000000-0000-4000-a000-000000000002', 'd0000000-0000-4000-a000-000000000002', 'Level Pedas', 1, 1, 1),
  ('b0000000-0000-4000-a000-000000000003', 'd0000000-0000-4000-a000-000000000003', 'Level Pedas', 1, 1, 1),
  ('b0000000-0000-4000-a000-000000000004', 'd0000000-0000-4000-a000-000000000004', 'Level Pedas', 1, 1, 1),
  ('b0000000-0000-4000-a000-000000000005', 'd0000000-0000-4000-a000-000000000005', 'Level Pedas', 1, 1, 1),
  ('b0000000-0000-4000-a000-000000000006', 'd0000000-0000-4000-a000-000000000006', 'Level Pedas', 1, 1, 1);

-- Tambahan for each Nasi Bakar (min=0, max=3)
INSERT INTO modifier_groups (id, product_id, name, min_select, max_select, sort_order) VALUES
  ('b0000000-0000-4000-a000-000000000007', 'd0000000-0000-4000-a000-000000000001', 'Tambahan', 0, 3, 2),
  ('b0000000-0000-4000-a000-000000000008', 'd0000000-0000-4000-a000-000000000002', 'Tambahan', 0, 3, 2),
  ('b0000000-0000-4000-a000-000000000009', 'd0000000-0000-4000-a000-000000000003', 'Tambahan', 0, 3, 2),
  ('b0000000-0000-4000-a000-00000000000a', 'd0000000-0000-4000-a000-000000000004', 'Tambahan', 0, 3, 2),
  ('b0000000-0000-4000-a000-00000000000b', 'd0000000-0000-4000-a000-000000000005', 'Tambahan', 0, 3, 2),
  ('b0000000-0000-4000-a000-00000000000c', 'd0000000-0000-4000-a000-000000000006', 'Tambahan', 0, 3, 2);

-- Topping for drinks (min=0, max=2)
INSERT INTO modifier_groups (id, product_id, name, min_select, max_select, sort_order) VALUES
  ('b0000000-0000-4000-a000-00000000000d', 'd0000000-0000-4000-a000-000000000007', 'Topping', 0, 2, 1),  -- Es Teh
  ('b0000000-0000-4000-a000-00000000000e', 'd0000000-0000-4000-a000-000000000008', 'Topping', 0, 2, 1),  -- Es Jeruk
  ('b0000000-0000-4000-a000-00000000000f', 'd0000000-0000-4000-a000-00000000000a', 'Topping', 0, 2, 1);  -- Jus Alpukat

-- =============================================================================
-- 9. MODIFIERS
-- =============================================================================

-- Level Pedas options (4 per group × 6 products = 24)
-- Tidak Pedas=0, Sedang=0, Pedas=0, Extra Pedas=2000
INSERT INTO modifiers (id, modifier_group_id, name, price, sort_order) VALUES
  -- NB Ayam
  ('ba000000-0000-4000-a000-000000000001', 'b0000000-0000-4000-a000-000000000001', 'Tidak Pedas',  0.00,    1),
  ('ba000000-0000-4000-a000-000000000002', 'b0000000-0000-4000-a000-000000000001', 'Sedang',       0.00,    2),
  ('ba000000-0000-4000-a000-000000000003', 'b0000000-0000-4000-a000-000000000001', 'Pedas',        0.00,    3),
  ('ba000000-0000-4000-a000-000000000004', 'b0000000-0000-4000-a000-000000000001', 'Extra Pedas',  2000.00, 4),
  -- NB Cumi
  ('ba000000-0000-4000-a000-000000000005', 'b0000000-0000-4000-a000-000000000002', 'Tidak Pedas',  0.00,    1),
  ('ba000000-0000-4000-a000-000000000006', 'b0000000-0000-4000-a000-000000000002', 'Sedang',       0.00,    2),
  ('ba000000-0000-4000-a000-000000000007', 'b0000000-0000-4000-a000-000000000002', 'Pedas',        0.00,    3),
  ('ba000000-0000-4000-a000-000000000008', 'b0000000-0000-4000-a000-000000000002', 'Extra Pedas',  2000.00, 4),
  -- NB Iga
  ('ba000000-0000-4000-a000-000000000009', 'b0000000-0000-4000-a000-000000000003', 'Tidak Pedas',  0.00,    1),
  ('ba000000-0000-4000-a000-00000000000a', 'b0000000-0000-4000-a000-000000000003', 'Sedang',       0.00,    2),
  ('ba000000-0000-4000-a000-00000000000b', 'b0000000-0000-4000-a000-000000000003', 'Pedas',        0.00,    3),
  ('ba000000-0000-4000-a000-00000000000c', 'b0000000-0000-4000-a000-000000000003', 'Extra Pedas',  2000.00, 4),
  -- NB Jamur
  ('ba000000-0000-4000-a000-00000000000d', 'b0000000-0000-4000-a000-000000000004', 'Tidak Pedas',  0.00,    1),
  ('ba000000-0000-4000-a000-00000000000e', 'b0000000-0000-4000-a000-000000000004', 'Sedang',       0.00,    2),
  ('ba000000-0000-4000-a000-00000000000f', 'b0000000-0000-4000-a000-000000000004', 'Pedas',        0.00,    3),
  ('ba000000-0000-4000-a000-000000000010', 'b0000000-0000-4000-a000-000000000004', 'Extra Pedas',  2000.00, 4),
  -- NB Teri
  ('ba000000-0000-4000-a000-000000000011', 'b0000000-0000-4000-a000-000000000005', 'Tidak Pedas',  0.00,    1),
  ('ba000000-0000-4000-a000-000000000012', 'b0000000-0000-4000-a000-000000000005', 'Sedang',       0.00,    2),
  ('ba000000-0000-4000-a000-000000000013', 'b0000000-0000-4000-a000-000000000005', 'Pedas',        0.00,    3),
  ('ba000000-0000-4000-a000-000000000014', 'b0000000-0000-4000-a000-000000000005', 'Extra Pedas',  2000.00, 4),
  -- NB Seafood
  ('ba000000-0000-4000-a000-000000000015', 'b0000000-0000-4000-a000-000000000006', 'Tidak Pedas',  0.00,    1),
  ('ba000000-0000-4000-a000-000000000016', 'b0000000-0000-4000-a000-000000000006', 'Sedang',       0.00,    2),
  ('ba000000-0000-4000-a000-000000000017', 'b0000000-0000-4000-a000-000000000006', 'Pedas',        0.00,    3),
  ('ba000000-0000-4000-a000-000000000018', 'b0000000-0000-4000-a000-000000000006', 'Extra Pedas',  2000.00, 4);

-- Tambahan options (3 per group × 6 products = 18)
INSERT INTO modifiers (id, modifier_group_id, name, price, sort_order) VALUES
  -- NB Ayam
  ('ba000000-0000-4000-a000-000000000019', 'b0000000-0000-4000-a000-000000000007', 'Extra Sambal',     3000.00, 1),
  ('ba000000-0000-4000-a000-00000000001a', 'b0000000-0000-4000-a000-000000000007', 'Telur Ceplok',     4000.00, 2),
  ('ba000000-0000-4000-a000-00000000001b', 'b0000000-0000-4000-a000-000000000007', 'Keju Mozarella',   5000.00, 3),
  -- NB Cumi
  ('ba000000-0000-4000-a000-00000000001c', 'b0000000-0000-4000-a000-000000000008', 'Extra Sambal',     3000.00, 1),
  ('ba000000-0000-4000-a000-00000000001d', 'b0000000-0000-4000-a000-000000000008', 'Telur Ceplok',     4000.00, 2),
  ('ba000000-0000-4000-a000-00000000001e', 'b0000000-0000-4000-a000-000000000008', 'Keju Mozarella',   5000.00, 3),
  -- NB Iga
  ('ba000000-0000-4000-a000-00000000001f', 'b0000000-0000-4000-a000-000000000009', 'Extra Sambal',     3000.00, 1),
  ('ba000000-0000-4000-a000-000000000020', 'b0000000-0000-4000-a000-000000000009', 'Telur Ceplok',     4000.00, 2),
  ('ba000000-0000-4000-a000-000000000021', 'b0000000-0000-4000-a000-000000000009', 'Keju Mozarella',   5000.00, 3),
  -- NB Jamur
  ('ba000000-0000-4000-a000-000000000022', 'b0000000-0000-4000-a000-00000000000a', 'Extra Sambal',     3000.00, 1),
  ('ba000000-0000-4000-a000-000000000023', 'b0000000-0000-4000-a000-00000000000a', 'Telur Ceplok',     4000.00, 2),
  ('ba000000-0000-4000-a000-000000000024', 'b0000000-0000-4000-a000-00000000000a', 'Keju Mozarella',   5000.00, 3),
  -- NB Teri
  ('ba000000-0000-4000-a000-000000000025', 'b0000000-0000-4000-a000-00000000000b', 'Extra Sambal',     3000.00, 1),
  ('ba000000-0000-4000-a000-000000000026', 'b0000000-0000-4000-a000-00000000000b', 'Telur Ceplok',     4000.00, 2),
  ('ba000000-0000-4000-a000-000000000027', 'b0000000-0000-4000-a000-00000000000b', 'Keju Mozarella',   5000.00, 3),
  -- NB Seafood
  ('ba000000-0000-4000-a000-000000000028', 'b0000000-0000-4000-a000-00000000000c', 'Extra Sambal',     3000.00, 1),
  ('ba000000-0000-4000-a000-000000000029', 'b0000000-0000-4000-a000-00000000000c', 'Telur Ceplok',     4000.00, 2),
  ('ba000000-0000-4000-a000-00000000002a', 'b0000000-0000-4000-a000-00000000000c', 'Keju Mozarella',   5000.00, 3);

-- Topping options (2 per group × 3 drinks = 6)
INSERT INTO modifiers (id, modifier_group_id, name, price, sort_order) VALUES
  -- Es Teh
  ('ba000000-0000-4000-a000-00000000002b', 'b0000000-0000-4000-a000-00000000000d', 'Boba',   4000.00, 1),
  ('ba000000-0000-4000-a000-00000000002c', 'b0000000-0000-4000-a000-00000000000d', 'Jelly',  3000.00, 2),
  -- Es Jeruk
  ('ba000000-0000-4000-a000-00000000002d', 'b0000000-0000-4000-a000-00000000000e', 'Boba',   4000.00, 1),
  ('ba000000-0000-4000-a000-00000000002e', 'b0000000-0000-4000-a000-00000000000e', 'Jelly',  3000.00, 2),
  -- Jus Alpukat
  ('ba000000-0000-4000-a000-00000000002f', 'b0000000-0000-4000-a000-00000000000f', 'Boba',   4000.00, 1),
  ('ba000000-0000-4000-a000-000000000030', 'b0000000-0000-4000-a000-00000000000f', 'Jelly',  3000.00, 2);

-- =============================================================================
-- 10. CUSTOMERS
-- =============================================================================
INSERT INTO customers (id, outlet_id, name, phone, email, notes) VALUES
  ('cc000000-0000-4000-a000-000000000001', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Ahmad Fadli',     '081200001111', 'ahmad@gmail.com', 'Pelanggan tetap'),
  ('cc000000-0000-4000-a000-000000000002', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Rina Wati',       '081200002222', 'rina@gmail.com',  'Alergi kacang'),
  ('cc000000-0000-4000-a000-000000000003', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Pak Hendra',      '081200003333', NULL,              'Kantor lantai 3'),
  ('cc000000-0000-4000-a000-000000000004', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Ibu Sari',        '081200004444', 'sari@gmail.com',  'Sering pesan catering'),
  ('cc000000-0000-4000-a000-000000000005', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'Dimas Prasetyo',  '081200005555', NULL,              NULL);

-- =============================================================================
-- 11. ORDERS
-- =============================================================================
-- Timestamps relative to NOW() for fresh-looking data on every seed run.
-- created_by = Cashier (Budi) for most orders.

-- KWR-001: Ahmad, DINE_IN, COMPLETED
-- NB Ayam Jumbo (30000) + Pedas (0) + Extra Sambal (3000) + Es Teh (5000) = 38000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at, completed_at) VALUES
  ('aa000000-0000-4000-a000-000000000001', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-001',
   'cc000000-0000-4000-a000-000000000001', 'DINE_IN', 'COMPLETED', '5', NULL,
   38000.00, 38000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '2 hours 30 minutes');

-- KWR-002: Rina, TAKEAWAY, COMPLETED
-- NB Jamur Reguler (22000) + Tidak Pedas (0) + Air Mineral (4000) = 26000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at, completed_at) VALUES
  ('aa000000-0000-4000-a000-000000000002', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-002',
   'cc000000-0000-4000-a000-000000000002', 'TAKEAWAY', 'COMPLETED', NULL, NULL,
   26000.00, 26000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '2 hours 30 minutes', NOW() - INTERVAL '2 hours');

-- KWR-003: Walk-in, DINE_IN, COMPLETED
-- Paket Ayam Komplit (32000) + Tempe Goreng (8000) = 40000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at, completed_at) VALUES
  ('aa000000-0000-4000-a000-000000000003', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-003',
   NULL, 'DINE_IN', 'COMPLETED', '2', NULL,
   40000.00, 40000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour 30 minutes');

-- KWR-004: Pak Hendra, TAKEAWAY, COMPLETED
-- 2x NB Cumi Reguler (28000×2=56000) + Sedang (0) + 2x Es Jeruk (8000×2=16000) = 72000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at, completed_at) VALUES
  ('aa000000-0000-4000-a000-000000000004', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-004',
   'cc000000-0000-4000-a000-000000000003', 'TAKEAWAY', 'COMPLETED', NULL, 'Pesan buat kantor',
   72000.00, 72000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '1 hour 30 minutes', NOW() - INTERVAL '1 hour');

-- KWR-005: Dimas, DINE_IN, COMPLETED (split payment)
-- NB Iga Jumbo (40000) + Pedas (0) + Telur (4000) + Keju (5000) + Kopi Susu Large (17000) = 66000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at, completed_at) VALUES
  ('aa000000-0000-4000-a000-000000000005', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-005',
   'cc000000-0000-4000-a000-000000000005', 'DINE_IN', 'COMPLETED', '8', NULL,
   66000.00, 66000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '30 minutes');

-- KWR-006: Ibu Sari, CATERING, DP_PAID
-- 20x NB Ayam Reguler (25000×20=500000) + Sedang (0) + 20x Es Teh (5000×20=100000) = 600000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, catering_date, catering_status, catering_dp_amount, created_by, created_at) VALUES
  ('aa000000-0000-4000-a000-000000000006', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-006',
   'cc000000-0000-4000-a000-000000000004', 'CATERING', 'NEW', NULL, 'Acara arisan',
   600000.00, 600000.00,
   NOW() + INTERVAL '7 days', 'DP_PAID', 250000.00,
   'a0000000-0000-4000-a000-000000000002', NOW() - INTERVAL '1 day');

-- KWR-007: Ahmad, DINE_IN, PREPARING
-- NB Seafood Reguler (32000) + Extra Pedas (2000) + Jus Alpukat Reguler (15000) + Boba (4000) = 53000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at) VALUES
  ('aa000000-0000-4000-a000-000000000007', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-007',
   'cc000000-0000-4000-a000-000000000001', 'DINE_IN', 'PREPARING', '5', NULL,
   53000.00, 53000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '20 minutes');

-- KWR-008: Walk-in, TAKEAWAY, NEW
-- NB Teri Reguler (23000) + Sedang (0) + Kerupuk Udang (5000) = 28000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at) VALUES
  ('aa000000-0000-4000-a000-000000000008', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-008',
   NULL, 'TAKEAWAY', 'NEW', NULL, NULL,
   28000.00, 28000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '5 minutes');

-- KWR-009: Rina, DINE_IN, READY
-- Paket Iga Spesial (45000) + Tahu Crispy (8000) = 53000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, created_by, created_at) VALUES
  ('aa000000-0000-4000-a000-000000000009', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-009',
   'cc000000-0000-4000-a000-000000000002', 'DINE_IN', 'READY', '1', NULL,
   53000.00, 53000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '15 minutes');

-- KWR-010: Dimas, DELIVERY, CANCELLED
-- NB Ayam Reguler (25000) + Sedang (0) + Es Jeruk (8000) = 33000
INSERT INTO orders (id, outlet_id, order_number, customer_id, order_type, status, table_number, notes, subtotal, total_amount, delivery_platform, delivery_address, created_by, created_at) VALUES
  ('aa000000-0000-4000-a000-00000000000a', '17fbe5e3-6dea-4a8e-9036-8a59c345e157', 'KWR-010',
   'cc000000-0000-4000-a000-000000000005', 'DELIVERY', 'CANCELLED', NULL, 'Alamat salah, dibatalkan',
   33000.00, 33000.00,
   'GoFood', 'Jl. Dipatiukur No. 50, Bandung',
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '45 minutes');

-- =============================================================================
-- 12. ORDER ITEMS
-- =============================================================================

-- KWR-001: NB Ayam Jumbo + Es Teh
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-000000000001', 'aa000000-0000-4000-a000-000000000001', 'd0000000-0000-4000-a000-000000000001', 'f0000000-0000-4000-a000-000000000002', 1, 30000.00, 30000.00, 'READY', 'GRILL'),
  ('ab000000-0000-4000-a000-000000000002', 'aa000000-0000-4000-a000-000000000001', 'd0000000-0000-4000-a000-000000000007', NULL,                                   1,  5000.00,  5000.00, 'READY', 'BEVERAGE');

-- KWR-002: NB Jamur Reguler + Air Mineral
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-000000000003', 'aa000000-0000-4000-a000-000000000002', 'd0000000-0000-4000-a000-000000000004', 'f0000000-0000-4000-a000-000000000007', 1, 22000.00, 22000.00, 'READY', 'GRILL'),
  ('ab000000-0000-4000-a000-000000000004', 'aa000000-0000-4000-a000-000000000002', 'd0000000-0000-4000-a000-00000000000b', NULL,                                   1,  4000.00,  4000.00, 'READY', 'BEVERAGE');

-- KWR-003: Paket Ayam + Tempe
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-000000000005', 'aa000000-0000-4000-a000-000000000003', 'd0000000-0000-4000-a000-000000000010', NULL, 1, 32000.00, 32000.00, 'READY', 'GRILL'),
  ('ab000000-0000-4000-a000-000000000006', 'aa000000-0000-4000-a000-000000000003', 'd0000000-0000-4000-a000-00000000000c', NULL, 1,  8000.00,  8000.00, 'READY', 'GRILL');

-- KWR-004: 2x NB Cumi Reguler + 2x Es Jeruk
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-000000000007', 'aa000000-0000-4000-a000-000000000004', 'd0000000-0000-4000-a000-000000000002', 'f0000000-0000-4000-a000-000000000003', 2, 28000.00, 56000.00, 'READY', 'GRILL'),
  ('ab000000-0000-4000-a000-000000000008', 'aa000000-0000-4000-a000-000000000004', 'd0000000-0000-4000-a000-000000000008', NULL,                                   2,  8000.00, 16000.00, 'READY', 'BEVERAGE');

-- KWR-005: NB Iga Jumbo + Kopi Susu Large
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-000000000009', 'aa000000-0000-4000-a000-000000000005', 'd0000000-0000-4000-a000-000000000003', 'f0000000-0000-4000-a000-000000000006', 1, 40000.00, 40000.00, 'READY', 'GRILL'),
  ('ab000000-0000-4000-a000-00000000000a', 'aa000000-0000-4000-a000-000000000005', 'd0000000-0000-4000-a000-000000000009', 'f0000000-0000-4000-a000-00000000000e', 1, 17000.00, 17000.00, 'READY', 'BEVERAGE');

-- KWR-006: 20x NB Ayam Reguler + 20x Es Teh (catering)
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-00000000000b', 'aa000000-0000-4000-a000-000000000006', 'd0000000-0000-4000-a000-000000000001', 'f0000000-0000-4000-a000-000000000001', 20, 25000.00, 500000.00, 'PENDING', 'GRILL'),
  ('ab000000-0000-4000-a000-00000000000c', 'aa000000-0000-4000-a000-000000000006', 'd0000000-0000-4000-a000-000000000007', NULL,                                   20,  5000.00, 100000.00, 'PENDING', 'BEVERAGE');

-- KWR-007: NB Seafood Reguler + Jus Alpukat Reguler (preparing)
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-00000000000d', 'aa000000-0000-4000-a000-000000000007', 'd0000000-0000-4000-a000-000000000006', 'f0000000-0000-4000-a000-00000000000b', 1, 32000.00, 32000.00, 'PREPARING', 'GRILL'),
  ('ab000000-0000-4000-a000-00000000000e', 'aa000000-0000-4000-a000-000000000007', 'd0000000-0000-4000-a000-00000000000a', 'f0000000-0000-4000-a000-00000000000f', 1, 15000.00, 15000.00, 'PREPARING', 'BEVERAGE');

-- KWR-008: NB Teri Reguler + Kerupuk Udang (new)
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-00000000000f', 'aa000000-0000-4000-a000-000000000008', 'd0000000-0000-4000-a000-000000000005', 'f0000000-0000-4000-a000-000000000009', 1, 23000.00, 23000.00, 'PENDING', 'GRILL'),
  ('ab000000-0000-4000-a000-000000000010', 'aa000000-0000-4000-a000-000000000008', 'd0000000-0000-4000-a000-00000000000e', NULL,                                   1,  5000.00,  5000.00, 'PENDING', 'GRILL');

-- KWR-009: Paket Iga Spesial + Tahu Crispy (ready)
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-000000000011', 'aa000000-0000-4000-a000-000000000009', 'd0000000-0000-4000-a000-000000000011', NULL, 1, 45000.00, 45000.00, 'READY', 'GRILL'),
  ('ab000000-0000-4000-a000-000000000012', 'aa000000-0000-4000-a000-000000000009', 'd0000000-0000-4000-a000-00000000000d', NULL, 1,  8000.00,  8000.00, 'READY', 'GRILL');

-- KWR-010: NB Ayam Reguler + Es Jeruk (cancelled)
INSERT INTO order_items (id, order_id, product_id, variant_id, quantity, unit_price, subtotal, status, station) VALUES
  ('ab000000-0000-4000-a000-000000000013', 'aa000000-0000-4000-a000-00000000000a', 'd0000000-0000-4000-a000-000000000001', 'f0000000-0000-4000-a000-000000000001', 1, 25000.00, 25000.00, 'PENDING', 'GRILL'),
  ('ab000000-0000-4000-a000-000000000014', 'aa000000-0000-4000-a000-00000000000a', 'd0000000-0000-4000-a000-000000000008', NULL,                                   1,  8000.00,  8000.00, 'PENDING', 'BEVERAGE');

-- =============================================================================
-- 13. ORDER ITEM MODIFIERS
-- =============================================================================

-- KWR-001: NB Ayam → Pedas (0) + Extra Sambal (3000)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-000000000001', 'ab000000-0000-4000-a000-000000000001', 'ba000000-0000-4000-a000-000000000003', 1,    0.00),  -- Pedas
  ('ac000000-0000-4000-a000-000000000002', 'ab000000-0000-4000-a000-000000000001', 'ba000000-0000-4000-a000-000000000019', 1, 3000.00);  -- Extra Sambal

-- KWR-002: NB Jamur → Tidak Pedas (0)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-000000000003', 'ab000000-0000-4000-a000-000000000003', 'ba000000-0000-4000-a000-00000000000d', 1, 0.00);    -- Tidak Pedas

-- KWR-004: NB Cumi → Sedang (0)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-000000000004', 'ab000000-0000-4000-a000-000000000007', 'ba000000-0000-4000-a000-000000000006', 1, 0.00);    -- Sedang

-- KWR-005: NB Iga → Pedas (0) + Telur (4000) + Keju (5000)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-000000000005', 'ab000000-0000-4000-a000-000000000009', 'ba000000-0000-4000-a000-00000000000b', 1,    0.00),  -- Pedas
  ('ac000000-0000-4000-a000-000000000006', 'ab000000-0000-4000-a000-000000000009', 'ba000000-0000-4000-a000-000000000020', 1, 4000.00),  -- Telur Ceplok
  ('ac000000-0000-4000-a000-000000000007', 'ab000000-0000-4000-a000-000000000009', 'ba000000-0000-4000-a000-000000000021', 1, 5000.00);  -- Keju Mozarella

-- KWR-006: NB Ayam (catering) → Sedang (0)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-000000000008', 'ab000000-0000-4000-a000-00000000000b', 'ba000000-0000-4000-a000-000000000002', 1, 0.00);    -- Sedang

-- KWR-007: NB Seafood → Extra Pedas (2000), Jus Alpukat → Boba (4000)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-000000000009', 'ab000000-0000-4000-a000-00000000000d', 'ba000000-0000-4000-a000-000000000018', 1, 2000.00),  -- Extra Pedas
  ('ac000000-0000-4000-a000-00000000000a', 'ab000000-0000-4000-a000-00000000000e', 'ba000000-0000-4000-a000-00000000002f', 1, 4000.00);  -- Boba

-- KWR-008: NB Teri → Sedang (0)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-00000000000b', 'ab000000-0000-4000-a000-00000000000f', 'ba000000-0000-4000-a000-000000000012', 1, 0.00);    -- Sedang

-- KWR-010: NB Ayam → Sedang (0)
INSERT INTO order_item_modifiers (id, order_item_id, modifier_id, quantity, unit_price) VALUES
  ('ac000000-0000-4000-a000-00000000000c', 'ab000000-0000-4000-a000-000000000013', 'ba000000-0000-4000-a000-000000000002', 1, 0.00);    -- Sedang

-- =============================================================================
-- 14. PAYMENTS
-- =============================================================================

-- KWR-001: CASH 38000 (received 40000, change 2000)
INSERT INTO payments (id, order_id, payment_method, amount, status, amount_received, change_amount, processed_by, processed_at) VALUES
  ('ad000000-0000-4000-a000-000000000001', 'aa000000-0000-4000-a000-000000000001', 'CASH', 38000.00, 'COMPLETED', 40000.00, 2000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '2 hours 30 minutes');

-- KWR-002: QRIS 26000
INSERT INTO payments (id, order_id, payment_method, amount, status, reference_number, processed_by, processed_at) VALUES
  ('ad000000-0000-4000-a000-000000000002', 'aa000000-0000-4000-a000-000000000002', 'QRIS', 26000.00, 'COMPLETED', 'QRIS-20260207-001',
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '2 hours');

-- KWR-003: CASH 40000 (received 50000, change 10000)
INSERT INTO payments (id, order_id, payment_method, amount, status, amount_received, change_amount, processed_by, processed_at) VALUES
  ('ad000000-0000-4000-a000-000000000003', 'aa000000-0000-4000-a000-000000000003', 'CASH', 40000.00, 'COMPLETED', 50000.00, 10000.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '1 hour 30 minutes');

-- KWR-004: TRANSFER 72000
INSERT INTO payments (id, order_id, payment_method, amount, status, reference_number, processed_by, processed_at) VALUES
  ('ad000000-0000-4000-a000-000000000004', 'aa000000-0000-4000-a000-000000000004', 'TRANSFER', 72000.00, 'COMPLETED', 'TRF-20260207-001',
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '1 hour');

-- KWR-005: Split — CASH 36000 + QRIS 30000
INSERT INTO payments (id, order_id, payment_method, amount, status, amount_received, change_amount, processed_by, processed_at) VALUES
  ('ad000000-0000-4000-a000-000000000005', 'aa000000-0000-4000-a000-000000000005', 'CASH', 36000.00, 'COMPLETED', 36000.00, 0.00,
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '30 minutes');
INSERT INTO payments (id, order_id, payment_method, amount, status, reference_number, processed_by, processed_at) VALUES
  ('ad000000-0000-4000-a000-000000000006', 'aa000000-0000-4000-a000-000000000005', 'QRIS', 30000.00, 'COMPLETED', 'QRIS-20260207-002',
   'a0000000-0000-4000-a000-000000000003', NOW() - INTERVAL '30 minutes');

-- KWR-006: TRANSFER 250000 (down payment for catering)
INSERT INTO payments (id, order_id, payment_method, amount, status, reference_number, processed_by, processed_at) VALUES
  ('ad000000-0000-4000-a000-000000000007', 'aa000000-0000-4000-a000-000000000006', 'TRANSFER', 250000.00, 'COMPLETED', 'TRF-20260207-002',
   'a0000000-0000-4000-a000-000000000002', NOW() - INTERVAL '1 day');

COMMIT;

-- =============================================================================
-- Done! Seed data loaded successfully.
-- =============================================================================
