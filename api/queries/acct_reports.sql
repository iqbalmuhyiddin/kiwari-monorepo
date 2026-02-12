-- name: GetProfitAndLossReport :many
-- Returns rows grouped by month, line type, and account for P&L computation.
-- Handler groups by period, sums SALES/COGS/EXPENSE, computes gross profit/margins.
SELECT
    date_trunc('month', ct.transaction_date)::date AS period,
    ct.line_type,
    ct.account_id,
    a.account_code,
    a.account_name,
    SUM(ct.amount)::text AS total_amount
FROM acct_cash_transactions ct
JOIN acct_accounts a ON a.id = ct.account_id
WHERE
    ct.line_type IN ('SALES', 'COGS', 'EXPENSE') AND
    (sqlc.narg('start_date')::date IS NULL OR ct.transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR ct.transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR ct.outlet_id = sqlc.narg('outlet_id'))
GROUP BY 1, 2, 3, 4, 5
ORDER BY 1, 2, 4;

-- name: GetCashFlowReport :many
-- Returns cash in/out per month per cash account for Cash Flow statement.
-- Cash In = SALES + CAPITAL; Cash Out = INVENTORY + EXPENSE + COGS + DRAWING.
SELECT
    date_trunc('month', ct.transaction_date)::date AS period,
    ca.id AS cash_account_id,
    ca.cash_account_code,
    ca.cash_account_name,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('SALES', 'CAPITAL') THEN ct.amount END), 0)::text AS cash_in,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('INVENTORY', 'EXPENSE', 'COGS', 'DRAWING') THEN ct.amount END), 0)::text AS cash_out
FROM acct_cash_transactions ct
JOIN acct_cash_accounts ca ON ca.id = ct.cash_account_id
WHERE
    ct.cash_account_id IS NOT NULL AND
    (sqlc.narg('start_date')::date IS NULL OR ct.transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR ct.transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR ct.outlet_id = sqlc.narg('outlet_id'))
GROUP BY 1, 2, 3, 4
ORDER BY 1, 3;

-- name: GetCashBalances :many
-- All-time net cash position per cash account (for dashboard cards).
SELECT
    ca.id AS cash_account_id,
    ca.cash_account_code,
    ca.cash_account_name,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('SALES', 'CAPITAL') THEN ct.amount END), 0)::text AS total_in,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('INVENTORY', 'EXPENSE', 'COGS', 'DRAWING') THEN ct.amount END), 0)::text AS total_out
FROM acct_cash_transactions ct
JOIN acct_cash_accounts ca ON ca.id = ct.cash_account_id
WHERE ct.cash_account_id IS NOT NULL
GROUP BY 1, 2, 3
ORDER BY 2;

-- name: GetMonthlyPnlSummary :one
-- Current month P&L totals (for dashboard mini-summary).
SELECT
    COALESCE(SUM(CASE WHEN line_type = 'SALES' THEN amount END), 0)::text AS net_sales,
    COALESCE(SUM(CASE WHEN line_type = 'COGS' THEN amount END), 0)::text AS cogs,
    COALESCE(SUM(CASE WHEN line_type = 'EXPENSE' THEN amount END), 0)::text AS expenses
FROM acct_cash_transactions
WHERE transaction_date >= sqlc.arg('month_start') AND transaction_date < sqlc.arg('month_end');

-- name: GetPendingReimbursementsSummary :one
-- Count + total of Draft + Ready reimbursements (for dashboard badge).
SELECT
    COUNT(*) AS total_count,
    COALESCE(SUM(amount), 0)::text AS total_amount
FROM acct_reimbursement_requests
WHERE status IN ('Draft', 'Ready');
