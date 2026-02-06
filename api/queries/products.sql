-- name: ListProductsByOutlet :many
SELECT * FROM products WHERE outlet_id = $1 AND is_active = true ORDER BY name;

-- name: GetProduct :one
SELECT * FROM products WHERE id = $1 AND outlet_id = $2 AND is_active = true;

-- name: CreateProduct :one
INSERT INTO products (outlet_id, category_id, name, description, base_price, image_url, station, preparation_time, is_combo)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateProduct :one
UPDATE products SET category_id = $1, name = $2, description = $3, base_price = $4, image_url = $5, station = $6, preparation_time = $7, is_combo = $8, updated_at = now()
WHERE id = $9 AND outlet_id = $10 AND is_active = true
RETURNING *;

-- name: SoftDeleteProduct :one
UPDATE products SET is_active = false, updated_at = now() WHERE id = $1 AND outlet_id = $2 AND is_active = true RETURNING id;
