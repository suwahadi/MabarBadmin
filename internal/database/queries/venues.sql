-- name: ListVenues :many
SELECT * FROM venues ORDER BY name;

-- name: GetVenue :one
SELECT * FROM venues WHERE id = $1;

-- name: CreateVenue :one
INSERT INTO venues (name, address, maps_url) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateVenue :exec
UPDATE venues SET name = $2, address = $3, maps_url = $4 WHERE id = $1;

-- name: DeleteVenue :exec
DELETE FROM venues WHERE id = $1;

-- name: ListTemplates :many
SELECT t.*, v.name AS venue_name
FROM session_templates t JOIN venues v ON v.id = t.venue_id
ORDER BY t.weekday, t.start_time, t.name;

-- name: GetTemplate :one
SELECT * FROM session_templates WHERE id = $1;

-- name: CreateTemplate :one
INSERT INTO session_templates (name, venue_id, weekday, start_time, end_time, court_count, capacity, price,
                               point_rule, level_label, shuttlecock_included, unique_code_enabled, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: UpdateTemplate :exec
UPDATE session_templates
SET name = $2, venue_id = $3, weekday = $4, start_time = $5, end_time = $6, court_count = $7, capacity = $8,
    price = $9, point_rule = $10, level_label = $11, shuttlecock_included = $12, unique_code_enabled = $13, notes = $14
WHERE id = $1;

-- name: DeleteTemplate :exec
DELETE FROM session_templates WHERE id = $1;
