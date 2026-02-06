-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND is_active = true;

-- name: GetUserByOutletAndPin :one
SELECT * FROM users WHERE outlet_id = $1 AND pin = $2 AND is_active = true;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND is_active = true;
