-- name: GetOutlet :one
SELECT * FROM outlets WHERE id = $1 AND is_active = true;

-- name: ListOutlets :many
SELECT * FROM outlets WHERE is_active = true ORDER BY name;

-- name: CreateOutlet :one
INSERT INTO outlets (name, address, phone)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateOutlet :one
UPDATE outlets SET name = $1, address = $2, phone = $3
WHERE id = $4 AND is_active = true
RETURNING *;

-- name: SoftDeleteOutlet :exec
UPDATE outlets SET is_active = false WHERE id = $1;
