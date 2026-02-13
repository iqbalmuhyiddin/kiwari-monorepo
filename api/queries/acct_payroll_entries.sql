-- name: ListAcctPayrollEntries :many
SELECT * FROM acct_payroll_entries
WHERE
    (sqlc.narg('start_date')::date IS NULL OR payroll_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR payroll_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id')) AND
    (sqlc.narg('period_type')::text IS NULL OR period_type = sqlc.narg('period_type'))
ORDER BY payroll_date DESC, created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAcctPayrollEntry :one
SELECT * FROM acct_payroll_entries WHERE id = $1;

-- name: CreateAcctPayrollEntry :one
INSERT INTO acct_payroll_entries (
    payroll_date, period_type, period_ref, employee_name,
    gross_pay, payment_method, cash_account_id, outlet_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateAcctPayrollEntry :one
UPDATE acct_payroll_entries
SET payroll_date = $2, period_type = $3, period_ref = $4,
    employee_name = $5, gross_pay = $6, payment_method = $7,
    cash_account_id = $8, outlet_id = $9
WHERE id = $1 AND posted_at IS NULL
RETURNING *;

-- name: DeleteAcctPayrollEntry :exec
DELETE FROM acct_payroll_entries WHERE id = $1 AND posted_at IS NULL;

-- name: ListUnpostedPayrollEntries :many
SELECT * FROM acct_payroll_entries
WHERE id = ANY($1::uuid[])
    AND posted_at IS NULL
ORDER BY employee_name;

-- name: MarkPayrollEntriesPosted :exec
UPDATE acct_payroll_entries
SET posted_at = now()
WHERE id = ANY($1::uuid[]) AND posted_at IS NULL;
