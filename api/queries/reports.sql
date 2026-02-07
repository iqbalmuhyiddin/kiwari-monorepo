-- name: GetDailySales :many
SELECT
    DATE(o.created_at) AS sale_date,
    COUNT(o.id) AS order_count,
    SUM(o.total_amount)::decimal(12,2) AS total_revenue,
    SUM(o.discount_amount)::decimal(12,2) AS total_discount,
    (SUM(o.total_amount) - SUM(o.discount_amount))::decimal(12,2) AS net_revenue
FROM orders o
WHERE o.outlet_id = $1
    AND o.status != 'CANCELLED'
    AND o.created_at >= $2
    AND o.created_at < $3
GROUP BY DATE(o.created_at)
ORDER BY sale_date;

-- name: GetProductSales :many
SELECT
    p.id AS product_id,
    p.name AS product_name,
    SUM(oi.quantity) AS quantity_sold,
    SUM(oi.subtotal)::decimal(12,2) AS total_revenue
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN products p ON p.id = oi.product_id
WHERE o.outlet_id = $1
    AND o.status != 'CANCELLED'
    AND o.created_at >= $2
    AND o.created_at < $3
GROUP BY p.id, p.name
ORDER BY quantity_sold DESC
LIMIT $4;

-- name: GetPaymentSummary :many
SELECT
    p.payment_method,
    COUNT(p.id) AS transaction_count,
    SUM(p.amount)::decimal(12,2) AS total_amount
FROM payments p
JOIN orders o ON o.id = p.order_id
WHERE o.outlet_id = $1
    AND o.status != 'CANCELLED'
    AND p.processed_at >= $2
    AND p.processed_at < $3
GROUP BY p.payment_method
ORDER BY p.payment_method;

-- name: GetHourlySales :many
SELECT
    EXTRACT(HOUR FROM o.created_at)::int AS hour,
    COUNT(o.id) AS order_count,
    SUM(o.total_amount)::decimal(12,2) AS total_revenue
FROM orders o
WHERE o.outlet_id = $1
    AND o.status != 'CANCELLED'
    AND o.created_at >= $2
    AND o.created_at < $3
GROUP BY EXTRACT(HOUR FROM o.created_at)
ORDER BY hour;

-- name: GetOutletComparison :many
SELECT
    outlets.id AS outlet_id,
    outlets.name AS outlet_name,
    COUNT(o.id) AS order_count,
    COALESCE(SUM(o.total_amount), 0)::decimal(12,2) AS total_revenue
FROM outlets
LEFT JOIN orders o ON o.outlet_id = outlets.id
    AND o.status != 'CANCELLED'
    AND o.created_at >= $1
    AND o.created_at < $2
WHERE outlets.is_active = true
GROUP BY outlets.id, outlets.name
ORDER BY total_revenue DESC;
