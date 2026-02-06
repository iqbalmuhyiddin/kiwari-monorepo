-- Modifier Groups

-- name: ListModifierGroupsByProduct :many
SELECT * FROM modifier_groups WHERE product_id = $1 AND is_active = true ORDER BY sort_order, name;

-- name: GetModifierGroup :one
SELECT * FROM modifier_groups WHERE id = $1 AND product_id = $2 AND is_active = true;

-- name: CreateModifierGroup :one
INSERT INTO modifier_groups (product_id, name, min_select, max_select, sort_order)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: UpdateModifierGroup :one
UPDATE modifier_groups SET name = $1, min_select = $2, max_select = $3, sort_order = $4
WHERE id = $5 AND product_id = $6 AND is_active = true RETURNING *;

-- name: SoftDeleteModifierGroup :one
UPDATE modifier_groups SET is_active = false WHERE id = $1 AND product_id = $2 AND is_active = true RETURNING id;

-- Modifiers

-- name: ListModifiersByGroup :many
SELECT * FROM modifiers WHERE modifier_group_id = $1 AND is_active = true ORDER BY sort_order, name;

-- name: CreateModifier :one
INSERT INTO modifiers (modifier_group_id, name, price, sort_order)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateModifier :one
UPDATE modifiers SET name = $1, price = $2, sort_order = $3
WHERE id = $4 AND modifier_group_id = $5 AND is_active = true RETURNING *;

-- name: SoftDeleteModifier :one
UPDATE modifiers SET is_active = false WHERE id = $1 AND modifier_group_id = $2 AND is_active = true RETURNING id;
