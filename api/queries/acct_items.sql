-- name: ListAcctItems :many
SELECT * FROM acct_items WHERE is_active = true ORDER BY item_code;

-- name: GetAcctItem :one
SELECT * FROM acct_items WHERE id = $1 AND is_active = true;

-- name: CreateAcctItem :one
INSERT INTO acct_items (item_code, item_name, item_category, unit, is_inventory, average_price, last_price, for_hpp, keywords)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateAcctItem :one
UPDATE acct_items
SET item_name = $2, item_category = $3, unit = $4, is_inventory = $5,
    average_price = $6, last_price = $7, for_hpp = $8, keywords = $9
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: SoftDeleteAcctItem :one
UPDATE acct_items SET is_active = false WHERE id = $1 AND is_active = true RETURNING id;

-- name: UpdateAcctItemLastPrice :exec
UPDATE acct_items SET last_price = $2 WHERE id = $1;
