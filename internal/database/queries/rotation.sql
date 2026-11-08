-- name: CreateRotationRound :one
INSERT INTO rotation_rounds (session_id, round_number) VALUES ($1, $2) RETURNING *;

-- name: CreateRotationMatch :exec
INSERT INTO rotation_matches (round_id, court_number, team_a_player1_id, team_a_player2_id, team_b_player1_id, team_b_player2_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListRotationRounds :many
SELECT * FROM rotation_rounds WHERE session_id = $1 ORDER BY round_number;

-- name: ListRotationMatches :many
SELECT m.*, rr.round_number FROM rotation_matches m
JOIN rotation_rounds rr ON rr.id = m.round_id
WHERE rr.session_id = $1
ORDER BY rr.round_number, m.court_number;

-- name: DeleteLastRotationRound :exec
DELETE FROM rotation_rounds
WHERE id = (SELECT rr.id FROM rotation_rounds rr WHERE rr.session_id = $1 ORDER BY rr.round_number DESC LIMIT 1);
