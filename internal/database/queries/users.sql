-- name: CreateUser :one
INSERT INTO users (phone, name, password_hash, role, level)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET name = $2,
    level = $3,
    refund_bank_name = $4,
    refund_account_number = $5,
    refund_account_name = $6,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, must_change_password = $3, updated_at = now()
WHERE id = $1;

-- name: UpdateUserAdmin :exec
UPDATE users
SET name = $2, level = $3, is_active = $4, updated_at = now()
WHERE id = $1;

-- name: ListMembers :many
SELECT u.*,
       (SELECT count(*) FROM registrations r WHERE r.user_id = u.id AND r.status = 'attended')::bigint AS attended_count,
       (SELECT count(*) FROM registrations r WHERE r.user_id = u.id AND r.status = 'no_show')::bigint AS no_show_count
FROM users u
WHERE (sqlc.arg(query)::text = '' OR u.name ILIKE '%' || sqlc.arg(query)::text || '%' OR u.phone LIKE '%' || sqlc.arg(query)::text || '%')
ORDER BY u.role, u.name
LIMIT 200;

-- name: CountAdmins :one
SELECT count(*) FROM users WHERE role = 'admin';

-- name: LinkGuestRegistrations :exec
UPDATE registrations SET user_id = $1, updated_at = now()
WHERE phone = $2 AND user_id IS NULL;

-- name: CreateAuthSession :exec
INSERT INTO auth_sessions (token_hash, user_id, expires_at, user_agent)
VALUES ($1, $2, $3, $4);

-- name: GetAuthSessionUser :one
SELECT u.* FROM auth_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now() AND u.is_active;

-- name: DeleteAuthSession :exec
DELETE FROM auth_sessions WHERE token_hash = $1;

-- name: DeleteUserAuthSessions :exec
DELETE FROM auth_sessions WHERE user_id = $1;

-- name: DeleteExpiredAuthSessions :exec
DELETE FROM auth_sessions WHERE expires_at < now();
