-- name: GetOrderForUpdate :one
SELECT * FROM orders WHERE id = $1 AND outlet_id = $2 FOR NO KEY UPDATE;

-- name: CreatePayment :one
INSERT INTO payments (
    order_id, payment_method, amount, status,
    reference_number, amount_received, change_amount, processed_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: SumPaymentsByOrder :one
SELECT COALESCE(SUM(amount), 0)::decimal(12,2) AS total_paid
FROM payments
WHERE order_id = $1 AND status = 'COMPLETED';

-- name: UpdateCateringStatus :one
UPDATE orders SET catering_status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CompleteOrder :one
UPDATE orders SET status = 'COMPLETED', completed_at = now(), updated_at = now()
WHERE id = $1 AND status != 'CANCELLED'
RETURNING *;
