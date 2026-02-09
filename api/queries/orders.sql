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

-- name: ListOrders :many
SELECT * FROM orders
WHERE outlet_id = $1
  AND (sqlc.narg('status')::order_status IS NULL OR status = sqlc.narg('status')::order_status)
  AND (sqlc.narg('order_type')::order_type IS NULL OR order_type = sqlc.narg('order_type')::order_type)
  AND (sqlc.narg('start_date')::timestamptz IS NULL OR created_at >= sqlc.narg('start_date')::timestamptz)
  AND (sqlc.narg('end_date')::timestamptz IS NULL OR created_at < sqlc.narg('end_date')::timestamptz + interval '1 day')
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $3,
    completed_at = CASE WHEN $3 = 'COMPLETED' THEN now() ELSE completed_at END,
    updated_at = now()
WHERE id = $1 AND outlet_id = $2 AND status = $4
RETURNING *;

-- name: CancelOrder :one
UPDATE orders SET status = 'CANCELLED', updated_at = now()
WHERE id = $1 AND outlet_id = $2 AND status NOT IN ('COMPLETED', 'CANCELLED')
RETURNING *;

-- name: ListPaymentsByOrder :many
SELECT * FROM payments WHERE order_id = $1 ORDER BY processed_at;

-- name: GetOrderItem :one
SELECT * FROM order_items WHERE id = $1 AND order_id = $2;

-- name: UpdateOrderItem :one
UPDATE order_items SET
    quantity = $3,
    notes = $4,
    discount_amount = $5,
    subtotal = $6
WHERE id = $1 AND order_id = $2
RETURNING *;

-- name: DeleteOrderItem :exec
DELETE FROM order_items WHERE id = $1 AND order_id = $2;

-- name: UpdateOrderItemStatus :one
UPDATE order_items SET
    status = $3
WHERE id = $1 AND order_id = $2
RETURNING *;

-- name: CountOrderItems :one
SELECT COUNT(*) FROM order_items WHERE order_id = $1;

-- name: UpdateOrderTotals :one
UPDATE orders SET
    subtotal = (SELECT COALESCE(SUM(oi.subtotal), 0) FROM order_items oi WHERE oi.order_id = $1),
    discount_amount = CASE
        WHEN discount_type = 'PERCENTAGE' THEN
            (SELECT COALESCE(SUM(oi.subtotal), 0) FROM order_items oi WHERE oi.order_id = $1) * discount_value / 100
        WHEN discount_type = 'FIXED_AMOUNT' THEN LEAST(discount_value, (SELECT COALESCE(SUM(oi.subtotal), 0) FROM order_items oi WHERE oi.order_id = $1))
        ELSE 0
    END,
    total_amount = (SELECT COALESCE(SUM(oi.subtotal), 0) FROM order_items oi WHERE oi.order_id = $1)
        - CASE
            WHEN discount_type = 'PERCENTAGE' THEN
                (SELECT COALESCE(SUM(oi.subtotal), 0) FROM order_items oi WHERE oi.order_id = $1) * discount_value / 100
            WHEN discount_type = 'FIXED_AMOUNT' THEN LEAST(discount_value, (SELECT COALESCE(SUM(oi.subtotal), 0) FROM order_items oi WHERE oi.order_id = $1))
            ELSE 0
        END
        + tax_amount,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListActiveOrders :many
SELECT o.*,
       COALESCE(
         (SELECT SUM(p.amount) FROM payments p WHERE p.order_id = o.id AND p.status = 'COMPLETED'),
         0
       )::decimal(12,2) AS amount_paid
FROM orders o
WHERE o.outlet_id = $1
  AND (
    o.status IN ('NEW', 'PREPARING', 'READY')
    OR (o.order_type = 'CATERING' AND o.catering_status IN ('BOOKED', 'DP_PAID'))
  )
ORDER BY o.created_at DESC
LIMIT $2 OFFSET $3;
