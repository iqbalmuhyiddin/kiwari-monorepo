-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND is_active = true;

-- name: GetUserByOutletAndPin :one
SELECT * FROM users WHERE outlet_id = $1 AND pin = $2 AND is_active = true;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND is_active = true;

-- name: ListUsersByOutlet :many
SELECT * FROM users WHERE outlet_id = $1 AND is_active = true ORDER BY full_name;

-- name: CreateUser :one
INSERT INTO users (outlet_id, email, hashed_password, full_name, role, pin)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET email = $1, full_name = $2, role = $3, pin = $4, updated_at = now()
WHERE id = $5 AND outlet_id = $6 AND is_active = true
RETURNING *;

-- name: SoftDeleteUser :one
UPDATE users SET is_active = false, updated_at = now() WHERE id = $1 AND outlet_id = $2 AND is_active = true RETURNING id;
