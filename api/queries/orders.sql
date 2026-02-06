-- name: GetNextOrderNumber :one
SELECT COALESCE(MAX(CAST(SPLIT_PART(order_number, '-', 2) AS INT)), 0) + 1 AS next_number
FROM orders
WHERE outlet_id = $1 AND created_at::date = CURRENT_DATE;

-- name: CreateOrder :one
INSERT INTO orders (
    outlet_id, order_number, customer_id, order_type, table_number, notes,
    subtotal, discount_type, discount_value, discount_amount, tax_amount, total_amount,
    catering_date, catering_status, catering_dp_amount,
    delivery_platform, delivery_address, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15,
    $16, $17, $18
) RETURNING *;

-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_id, product_id, variant_id, quantity, unit_price,
    discount_type, discount_value, discount_amount, subtotal,
    notes, station
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11
) RETURNING *;

-- name: CreateOrderItemModifier :one
INSERT INTO order_item_modifiers (
    order_item_id, modifier_id, quantity, unit_price
) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetProductForOrder :one
SELECT id, outlet_id, base_price, station FROM products
WHERE id = $1 AND outlet_id = $2 AND is_active = true;

-- name: GetVariantForOrder :one
SELECT v.id, v.variant_group_id, v.price_adjustment, vg.product_id
FROM variants v
JOIN variant_groups vg ON vg.id = v.variant_group_id
WHERE v.id = $1 AND v.is_active = true AND vg.is_active = true;

-- name: GetModifierForOrder :one
SELECT m.id, m.price, mg.product_id
FROM modifiers m
JOIN modifier_groups mg ON mg.id = m.modifier_group_id
WHERE m.id = $1 AND m.is_active = true AND mg.is_active = true;

-- name: GetOrder :one
SELECT * FROM orders WHERE id = $1 AND outlet_id = $2;

-- name: ListOrderItemsByOrder :many
SELECT * FROM order_items WHERE order_id = $1 ORDER BY id;

-- name: ListOrderItemModifiersByOrderItem :many
SELECT * FROM order_item_modifiers WHERE order_item_id = $1 ORDER BY id;
