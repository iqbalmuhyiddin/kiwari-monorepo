-- name: ListAcctCashAccounts :many
SELECT * FROM acct_cash_accounts WHERE is_active = true ORDER BY cash_account_code;

-- name: GetAcctCashAccount :one
SELECT * FROM acct_cash_accounts WHERE id = $1 AND is_active = true;

-- name: CreateAcctCashAccount :one
INSERT INTO acct_cash_accounts (cash_account_code, cash_account_name, bank_name, ownership)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateAcctCashAccount :one
UPDATE acct_cash_accounts
SET cash_account_name = $2, bank_name = $3, ownership = $4
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: SoftDeleteAcctCashAccount :one
UPDATE acct_cash_accounts SET is_active = false WHERE id = $1 AND is_active = true RETURNING id;
