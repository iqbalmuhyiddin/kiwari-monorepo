-- name: ListCustomersByOutlet :many
SELECT * FROM customers
WHERE outlet_id = $1 AND is_active = true
  AND (sqlc.narg('search')::text IS NULL OR phone LIKE '%' || sqlc.narg('search')::text || '%' OR name ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY name
LIMIT $2 OFFSET $3;

-- name: GetCustomer :one
SELECT * FROM customers WHERE id = $1 AND outlet_id = $2 AND is_active = true;

-- name: CreateCustomer :one
INSERT INTO customers (outlet_id, name, phone, email, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateCustomer :one
UPDATE customers SET name = $2, phone = $3, email = $4, notes = $5, updated_at = now()
WHERE id = $1 AND outlet_id = $6 AND is_active = true
RETURNING *;

-- name: SoftDeleteCustomer :one
UPDATE customers SET is_active = false, updated_at = now()
WHERE id = $1 AND outlet_id = $2 AND is_active = true
RETURNING id;

-- name: GetCustomerStats :one
SELECT
    COUNT(DISTINCT o.id) AS total_orders,
    COALESCE(SUM(o.total_amount), 0)::decimal(12,2) AS total_spend,
    COALESCE(AVG(o.total_amount), 0)::decimal(12,2) AS avg_ticket
FROM orders o
WHERE o.customer_id = $1 AND o.outlet_id = $2 AND o.status != 'CANCELLED';

-- name: GetCustomerTopItems :many
SELECT p.id AS product_id, p.name AS product_name,
    SUM(oi.quantity) AS total_qty,
    SUM(oi.subtotal)::decimal(12,2) AS total_revenue
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN products p ON p.id = oi.product_id
WHERE o.customer_id = $1 AND o.outlet_id = $2 AND o.status != 'CANCELLED'
GROUP BY p.id, p.name
ORDER BY total_qty DESC
LIMIT 5;

-- name: ListCustomerOrders :many
SELECT * FROM orders
WHERE customer_id = $1 AND outlet_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
