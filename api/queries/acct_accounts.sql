-- name: ListAcctAccounts :many
SELECT * FROM acct_accounts WHERE is_active = true ORDER BY account_code;

-- name: GetAcctAccount :one
SELECT * FROM acct_accounts WHERE id = $1 AND is_active = true;

-- name: CreateAcctAccount :one
INSERT INTO acct_accounts (account_code, account_name, account_type, line_type)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateAcctAccount :one
UPDATE acct_accounts
SET account_name = $2, account_type = $3, line_type = $4
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: SoftDeleteAcctAccount :one
UPDATE acct_accounts SET is_active = false WHERE id = $1 AND is_active = true RETURNING id;
