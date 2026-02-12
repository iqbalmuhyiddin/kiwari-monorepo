-- name: ListAcctReimbursementRequests :many
SELECT * FROM acct_reimbursement_requests
WHERE
    (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')) AND
    (sqlc.narg('requester')::text IS NULL OR requester = sqlc.narg('requester')) AND
    (sqlc.narg('batch_id')::text IS NULL OR batch_id = sqlc.narg('batch_id')) AND
    (sqlc.narg('start_date')::date IS NULL OR expense_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR expense_date <= sqlc.narg('end_date'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAcctReimbursementRequest :one
SELECT * FROM acct_reimbursement_requests WHERE id = $1;

-- name: CreateAcctReimbursementRequest :one
INSERT INTO acct_reimbursement_requests (
    expense_date, item_id, description, qty, unit_price, amount,
    line_type, account_id, status, requester, receipt_link
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateAcctReimbursementRequest :one
UPDATE acct_reimbursement_requests
SET expense_date = $2, item_id = $3, description = $4, qty = $5, unit_price = $6,
    amount = $7, line_type = $8, account_id = $9, status = $10, receipt_link = $11
WHERE id = $1 AND status != 'Posted'
RETURNING *;

-- name: DeleteAcctReimbursementRequest :one
DELETE FROM acct_reimbursement_requests
WHERE id = $1 AND status = 'Draft'
RETURNING id;

-- name: AssignReimbursementBatch :exec
UPDATE acct_reimbursement_requests
SET batch_id = $1, status = 'Ready'
WHERE id = $2 AND status = 'Draft';

-- name: ListReimbursementsByBatch :many
SELECT * FROM acct_reimbursement_requests
WHERE batch_id = $1
ORDER BY created_at;

-- name: PostReimbursementBatch :exec
UPDATE acct_reimbursement_requests
SET status = 'Posted', posted_at = now()
WHERE batch_id = $1 AND status = 'Ready';

-- name: CheckBatchPosted :one
SELECT EXISTS(
    SELECT 1 FROM acct_reimbursement_requests
    WHERE batch_id = $1 AND status = 'Posted'
)::boolean AS is_posted;

-- name: GetNextBatchCode :one
SELECT COALESCE(MAX(batch_id), 'RMB000')::text AS max_code
FROM acct_reimbursement_requests
WHERE batch_id IS NOT NULL;
