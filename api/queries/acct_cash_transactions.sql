-- name: ListAcctCashTransactions :many
SELECT * FROM acct_cash_transactions
WHERE
    (sqlc.narg('start_date')::date IS NULL OR transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('line_type')::text IS NULL OR line_type = sqlc.narg('line_type')) AND
    (sqlc.narg('account_id')::uuid IS NULL OR account_id = sqlc.narg('account_id')) AND
    (sqlc.narg('cash_account_id')::uuid IS NULL OR cash_account_id = sqlc.narg('cash_account_id')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id')) AND
    (sqlc.narg('search')::text IS NULL OR description ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY transaction_date DESC, created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAcctCashTransaction :one
SELECT * FROM acct_cash_transactions WHERE id = $1;

-- name: CreateAcctCashTransaction :one
INSERT INTO acct_cash_transactions (
    transaction_code, transaction_date, item_id, description,
    quantity, unit_price, amount, line_type,
    account_id, cash_account_id, outlet_id, reimbursement_batch_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetNextTransactionCode :one
SELECT COALESCE(MAX(transaction_code), 'PCS000000')::text AS max_code
FROM acct_cash_transactions;

-- name: GetLastItemPrice :one
SELECT unit_price FROM acct_cash_transactions
WHERE item_id = $1
ORDER BY transaction_date DESC, created_at DESC
LIMIT 1;
