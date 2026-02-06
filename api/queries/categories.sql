-- name: ListCategoriesByOutlet :many
SELECT * FROM categories WHERE outlet_id = $1 AND is_active = true ORDER BY sort_order, name;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1 AND outlet_id = $2 AND is_active = true;

-- name: CreateCategory :one
INSERT INTO categories (outlet_id, name, description, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories SET name = $1, description = $2, sort_order = $3
WHERE id = $4 AND outlet_id = $5 AND is_active = true
RETURNING *;

-- name: SoftDeleteCategory :one
UPDATE categories SET is_active = false WHERE id = $1 AND outlet_id = $2 AND is_active = true RETURNING id;
