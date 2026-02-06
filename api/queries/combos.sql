-- Combo Items

-- name: ListComboItemsByCombo :many
SELECT * FROM combo_items WHERE combo_id = $1 ORDER BY sort_order, id;

-- name: GetComboItem :one
SELECT * FROM combo_items WHERE id = $1 AND combo_id = $2;

-- name: CreateComboItem :one
INSERT INTO combo_items (combo_id, product_id, quantity, sort_order)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: DeleteComboItem :execrows
DELETE FROM combo_items WHERE id = $1 AND combo_id = $2;
