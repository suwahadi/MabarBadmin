-- name: Leaderboard :many
SELECT u.id AS user_id, u.name,
       count(*) FILTER (WHERE r.status = 'attended')::bigint AS attended,
       count(*) FILTER (WHERE r.status = 'no_show')::bigint AS no_show,
       max(s.starts_at) FILTER (WHERE r.status = 'attended')::timestamptz AS last_played
FROM registrations r
JOIN users u ON u.id = r.user_id
JOIN sessions s ON s.id = r.session_id
WHERE r.status IN ('attended', 'no_show')
  AND s.starts_at >= sqlc.arg(since)::timestamptz
  AND s.starts_at < sqlc.arg(until)::timestamptz
GROUP BY u.id, u.name
HAVING count(*) FILTER (WHERE r.status = 'attended') > 0
ORDER BY attended DESC, no_show ASC, u.name
LIMIT sqlc.arg(max_rows);

-- name: SessionFinanceRows :many
SELECT s.id, s.slug, s.title, s.starts_at, s.status, s.court_cost, s.shuttlecock_cost, s.other_cost,
       coalesce((SELECT sum(r.amount_due) FROM registrations r
                 WHERE r.session_id = s.id AND r.status <> 'transferred'
                   AND r.payment_status IN ('paid', 'forfeited', 'refund_pending', 'refunded')), 0)::bigint AS income,
       coalesce((SELECT sum(f.amount) FROM refunds f JOIN registrations r ON r.id = f.registration_id
                 WHERE r.session_id = s.id), 0)::bigint AS refunds,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status = 'attended')::bigint AS attended,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status IN ('registered', 'attended', 'no_show'))::bigint AS filled,
       s.capacity
FROM sessions s
WHERE s.status <> 'draft'
  AND s.starts_at >= sqlc.arg(since)::timestamptz
  AND s.starts_at < sqlc.arg(until)::timestamptz
ORDER BY s.starts_at;

-- name: CountUnpaidNearest :one
SELECT count(*) FROM registrations r
WHERE r.session_id = (SELECT s.id FROM sessions s WHERE s.status = 'published' AND s.ends_at > now() ORDER BY s.starts_at LIMIT 1)
  AND r.status = 'registered' AND r.payment_status <> 'paid';

-- name: RecentAttendance :many
SELECT s.id, s.title, s.starts_at,
       (SELECT count(*) FROM registrations r WHERE r.session_id = s.id AND r.status = 'attended')::bigint AS attended
FROM sessions s
WHERE s.status = 'closed'
ORDER BY s.starts_at DESC
LIMIT 8;

-- name: CreateActivityLog :exec
INSERT INTO activity_logs (actor_type, actor_id, entity_type, entity_id, action, data)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListActivityForEntity :many
SELECT * FROM activity_logs WHERE entity_type = $1 AND entity_id = $2 ORDER BY created_at DESC LIMIT 100;

-- name: ListSessionActivity :many
SELECT a.* FROM activity_logs a
WHERE (a.entity_type = 'session' AND a.entity_id = sqlc.arg(session_id)::bigint)
   OR (a.entity_type = 'registration' AND a.entity_id IN (SELECT r.id FROM registrations r WHERE r.session_id = sqlc.arg(session_id)::bigint))
ORDER BY a.created_at DESC
LIMIT 100;
