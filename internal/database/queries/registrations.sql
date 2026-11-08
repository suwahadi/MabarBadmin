-- name: CreateRegistration :one
INSERT INTO registrations (session_id, user_id, display_name, phone, level, status, payment_status, amount_due,
                           unique_code, ticket_token_hash, registered_at, queued_at, paid_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetRegistration :one
SELECT * FROM registrations WHERE id = $1;

-- name: GetRegistrationByTicket :one
SELECT * FROM registrations WHERE ticket_token_hash = $1;

-- name: GetRegistrationBySessionPhone :one
SELECT * FROM registrations WHERE session_id = $1 AND phone = $2;

-- name: ListSessionRegistrations :many
SELECT * FROM registrations WHERE session_id = $1 ORDER BY registered_at, id;

-- name: ListSessionUniqueCodes :many
SELECT unique_code FROM registrations WHERE session_id = $1 AND unique_code > 0;

-- name: UpdateRegistrationPlacement :exec
UPDATE registrations SET status = $2, queued_at = $3, updated_at = now() WHERE id = $1;

-- name: UpdateRegistrationStatus :exec
UPDATE registrations SET status = $2, updated_at = now() WHERE id = $1;

-- name: UpdatePaymentStatus :exec
UPDATE registrations SET payment_status = $2, paid_at = $3, updated_at = now() WHERE id = $1;

-- name: CancelRegistration :exec
UPDATE registrations SET status = 'cancelled', payment_status = $2, cancelled_at = now(), updated_at = now() WHERE id = $1;

-- name: MarkTransferred :exec
UPDATE registrations SET status = 'transferred', transferred_to_id = $2, cancelled_at = now(), updated_at = now() WHERE id = $1;

-- name: UpdateRegistrationLevel :exec
UPDATE registrations SET level = $2, updated_at = now() WHERE id = $1;

-- name: SetTicketToken :exec
UPDATE registrations SET ticket_token_hash = $2, updated_at = now() WHERE id = $1;

-- name: DeleteRegistration :exec
DELETE FROM registrations WHERE id = $1;

-- name: ListUserRegistrations :many
SELECT r.*, s.slug AS session_slug, s.title AS session_title, s.starts_at AS session_starts_at,
       s.ends_at AS session_ends_at, s.status AS session_status, s.refund_deadline_at, v.name AS venue_name
FROM registrations r
JOIN sessions s ON s.id = r.session_id
JOIN venues v ON v.id = s.venue_id
WHERE r.user_id = $1
ORDER BY s.starts_at DESC
LIMIT 100;

-- name: MarkUnattendedNoShow :exec
UPDATE registrations SET status = 'no_show', updated_at = now()
WHERE session_id = $1 AND status = 'registered';

-- name: ListPaidWaitlisted :many
SELECT * FROM registrations WHERE session_id = $1 AND status = 'waitlisted' AND payment_status = 'paid';

-- name: ListPaidInSession :many
SELECT * FROM registrations
WHERE session_id = $1 AND payment_status = 'paid' AND status IN ('registered', 'waitlisted', 'attended', 'no_show');

-- name: ListSessionCountsForUser :one
SELECT count(*) FILTER (WHERE r.status = 'attended')::bigint AS attended,
       count(*) FILTER (WHERE r.status = 'no_show')::bigint AS no_show
FROM registrations r WHERE r.user_id = $1;
