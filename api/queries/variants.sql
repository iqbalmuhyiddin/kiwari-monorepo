-- Variant Groups

-- name: ListVariantGroupsByProduct :many
SELECT * FROM variant_groups WHERE product_id = $1 AND is_active = true ORDER BY sort_order, name;

-- name: GetVariantGroup :one
SELECT * FROM variant_groups WHERE id = $1 AND product_id = $2 AND is_active = true;

-- name: CreateVariantGroup :one
INSERT INTO variant_groups (product_id, name, is_required, sort_order)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateVariantGroup :one
UPDATE variant_groups SET name = $1, is_required = $2, sort_order = $3
WHERE id = $4 AND product_id = $5 AND is_active = true RETURNING *;

-- name: SoftDeleteVariantGroup :one
UPDATE variant_groups SET is_active = false WHERE id = $1 AND product_id = $2 AND is_active = true RETURNING id;

-- Variants

-- name: ListVariantsByGroup :many
SELECT * FROM variants WHERE variant_group_id = $1 AND is_active = true ORDER BY sort_order, name;

-- name: CreateVariant :one
INSERT INTO variants (variant_group_id, name, price_adjustment, sort_order)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateVariant :one
UPDATE variants SET name = $1, price_adjustment = $2, sort_order = $3
WHERE id = $4 AND variant_group_id = $5 AND is_active = true RETURNING *;

-- name: SoftDeleteVariant :one
UPDATE variants SET is_active = false WHERE id = $1 AND variant_group_id = $2 AND is_active = true RETURNING id;
