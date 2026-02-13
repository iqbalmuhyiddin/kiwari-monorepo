-- name: ListAcctSalesDailySummaries :many
SELECT * FROM acct_sales_daily_summaries
WHERE
    (sqlc.narg('start_date')::date IS NULL OR sales_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR sales_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
ORDER BY sales_date DESC, channel, payment_method
LIMIT $1 OFFSET $2;

-- name: GetAcctSalesDailySummary :one
SELECT * FROM acct_sales_daily_summaries WHERE id = $1;

-- name: CreateAcctSalesDailySummary :one
INSERT INTO acct_sales_daily_summaries (
    sales_date, channel, payment_method,
    gross_sales, discount_amount, net_sales,
    cash_account_id, outlet_id, source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateAcctSalesDailySummary :one
UPDATE acct_sales_daily_summaries
SET channel = $2, payment_method = $3,
    gross_sales = $4, discount_amount = $5, net_sales = $6,
    cash_account_id = $7
WHERE id = $1 AND source = 'manual' AND posted_at IS NULL
RETURNING *;

-- name: DeleteAcctSalesDailySummary :exec
DELETE FROM acct_sales_daily_summaries
WHERE id = $1 AND source = 'manual' AND posted_at IS NULL;

-- name: UpsertAcctSalesDailySummary :one
-- Used by POS sync — upserts by the UNIQUE(sales_date, channel, payment_method, outlet_id) constraint.
-- Only updates if not yet posted.
INSERT INTO acct_sales_daily_summaries (
    sales_date, channel, payment_method,
    gross_sales, discount_amount, net_sales,
    cash_account_id, outlet_id, source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pos')
ON CONFLICT (sales_date, channel, payment_method, outlet_id)
DO UPDATE SET
    gross_sales = EXCLUDED.gross_sales,
    discount_amount = EXCLUDED.discount_amount,
    net_sales = EXCLUDED.net_sales,
    cash_account_id = EXCLUDED.cash_account_id
WHERE acct_sales_daily_summaries.posted_at IS NULL
RETURNING *;

-- name: MarkSalesSummariesPosted :exec
-- Batch-mark summaries as posted for a given date + outlet.
UPDATE acct_sales_daily_summaries
SET posted_at = now()
WHERE sales_date = $1
    AND (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
    AND posted_at IS NULL;

-- name: ListUnpostedSalesSummaries :many
-- Get unposted summaries for a specific date + optional outlet for posting.
SELECT * FROM acct_sales_daily_summaries
WHERE sales_date = $1
    AND (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
    AND posted_at IS NULL
ORDER BY channel, payment_method;

-- name: AggregatePOSSales :many
-- Cross-domain query: aggregates completed POS orders by date, order_type, payment_method.
-- Handler maps order_type → channel name and payment_method → display name.
SELECT
    o.completed_at::date AS sales_date,
    o.order_type,
    p.payment_method,
    SUM(p.amount)::text AS total_amount
FROM orders o
JOIN payments p ON p.order_id = o.id
WHERE o.status = 'COMPLETED'
    AND p.status = 'COMPLETED'
    AND o.outlet_id = $1
    AND o.completed_at::date >= $2::date
    AND o.completed_at::date <= $3::date
GROUP BY o.completed_at::date, o.order_type, p.payment_method
ORDER BY 1, 2, 3;
