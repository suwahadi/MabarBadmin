-- name: CreatePaymentProof :one
INSERT INTO payment_proofs (registration_id, method, media_id, status, amount)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPaymentProof :one
SELECT * FROM payment_proofs WHERE id = $1;

-- name: GetPaymentProofForUpdate :one
SELECT * FROM payment_proofs WHERE id = $1 FOR UPDATE;

-- name: ReviewPaymentProof :exec
UPDATE payment_proofs SET status = $2, review_note = $3, reviewed_at = now() WHERE id = $1;

-- name: ListProofsForRegistration :many
SELECT * FROM payment_proofs WHERE registration_id = $1 ORDER BY uploaded_at DESC;

-- name: ListPaymentQueue :many
SELECT p.*, r.display_name, r.phone, r.amount_due, r.unique_code, r.status AS registration_status,
       r.payment_status, s.id AS session_id, s.title AS session_title, s.starts_at AS session_starts_at,
       m.width AS media_width, m.height AS media_height
FROM payment_proofs p
JOIN registrations r ON r.id = p.registration_id
JOIN sessions s ON s.id = r.session_id
LEFT JOIN media m ON m.id = p.media_id
WHERE p.status = sqlc.arg(status)::text
ORDER BY p.uploaded_at
LIMIT 200;

-- name: CountPendingProofs :one
SELECT count(*) FROM payment_proofs WHERE status = 'pending';

-- name: ListExpiredProofMedia :many
SELECT p.id, p.media_id FROM payment_proofs p
JOIN registrations r ON r.id = p.registration_id
JOIN sessions s ON s.id = r.session_id
WHERE p.media_id IS NOT NULL AND s.status IN ('closed', 'cancelled') AND s.closed_at < sqlc.arg(before)::timestamptz;

-- name: ClearProofMedia :exec
UPDATE payment_proofs SET media_id = NULL WHERE id = $1;

-- name: GetProofByMedia :one
SELECT p.*, r.user_id, r.ticket_token_hash FROM payment_proofs p
JOIN registrations r ON r.id = p.registration_id
WHERE p.media_id = $1;

-- name: CreateRefund :one
INSERT INTO refunds (registration_id, amount, bank_name, account_number, account_name)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (registration_id) DO UPDATE SET amount = EXCLUDED.amount
RETURNING *;

-- name: GetRefund :one
SELECT * FROM refunds WHERE id = $1;

-- name: GetRefundByRegistration :one
SELECT * FROM refunds WHERE registration_id = $1;

-- name: UpdateRefundAccount :exec
UPDATE refunds SET bank_name = $2, account_number = $3, account_name = $4 WHERE registration_id = $1 AND status = 'pending';

-- name: CompleteRefund :exec
UPDATE refunds SET status = 'completed', completed_at = now() WHERE id = $1 AND status = 'pending';

-- name: ListRefunds :many
SELECT f.*, r.display_name, r.phone, r.session_id, s.title AS session_title, s.starts_at AS session_starts_at
FROM refunds f
JOIN registrations r ON r.id = f.registration_id
JOIN sessions s ON s.id = r.session_id
WHERE f.status = sqlc.arg(status)::text
ORDER BY f.created_at
LIMIT 200;

-- name: CountPendingRefunds :one
SELECT count(*) FROM refunds WHERE status = 'pending';
