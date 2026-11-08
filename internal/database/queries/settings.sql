-- name: ListSettings :many
SELECT * FROM settings;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: InsertSettingIfMissing :exec
INSERT INTO settings (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO NOTHING;

-- name: GetSetting :one
SELECT * FROM settings WHERE key = $1;
