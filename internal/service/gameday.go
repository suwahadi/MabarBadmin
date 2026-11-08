package service

import (
	"context"
	"math/rand/v2"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/domain"
)

type RotationRound struct {
	Number  int
	Matches []domain.Match
}

// Rotation returns all rounds of a session in order.
func Rotation(ctx context.Context, q *sqlc.Queries, sessionID int64) ([]RotationRound, error) {
	rows, err := q.ListRotationMatches(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	var rounds []RotationRound
	for _, m := range rows {
		if len(rounds) == 0 || rounds[len(rounds)-1].Number != int(m.RoundNumber) {
			rounds = append(rounds, RotationRound{Number: int(m.RoundNumber)})
		}
		r := &rounds[len(rounds)-1]
		r.Matches = append(r.Matches, domain.Match{
			Court: int(m.CourtNumber),
			TeamA: [2]int64{m.TeamAPlayer1ID, m.TeamAPlayer2ID},
			TeamB: [2]int64{m.TeamBPlayer1ID, m.TeamBPlayer2ID},
		})
	}
	return rounds, nil
}

// NextRotationRound creates the next round from participants marked present; when nobody has
// been checked in yet it uses every slot holder.
func (s *Service) NextRotationRound(ctx context.Context, a Actor, sessionID int64, courts int) error {
	if courts < 1 || courts > 50 {
		return userErr("Jumlah lapangan aktif tidak valid.")
	}
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, err := q.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return notFound(err)
		}
		if sess.Status != domain.SessionPublished {
			return userErr("Rotasi hanya untuk sesi yang sedang berjalan.")
		}
		pool, err := rotationPool(ctx, q, sessionID)
		if err != nil {
			return err
		}
		if len(pool) < 4 {
			return userErr("Butuh minimal 4 pemain hadir untuk membuat rotasi.")
		}
		rounds, err := Rotation(ctx, q, sessionID)
		if err != nil {
			return err
		}
		history := make([][]domain.Match, len(rounds))
		for i, r := range rounds {
			history[i] = r.Matches
		}
		var matches []domain.Match
		s.withRand(func(r *rand.Rand) { matches = domain.NextRound(pool, courts, history, r) })
		round, err := q.CreateRotationRound(ctx, sqlc.CreateRotationRoundParams{SessionID: sessionID, RoundNumber: int32(len(rounds) + 1)})
		if err != nil {
			return err
		}
		for _, m := range matches {
			if err := q.CreateRotationMatch(ctx, sqlc.CreateRotationMatchParams{
				RoundID: round.ID, CourtNumber: int32(m.Court),
				TeamAPlayer1ID: m.TeamA[0], TeamAPlayer2ID: m.TeamA[1], TeamBPlayer1ID: m.TeamB[0], TeamBPlayer2ID: m.TeamB[1],
			}); err != nil {
				return err
			}
		}
		return logActivity(ctx, q, a, "session", sessionID, "rotation_round_created", map[string]any{"round": round.RoundNumber, "courts": len(matches)})
	})
}

func (s *Service) UndoRotationRound(ctx context.Context, a Actor, sessionID int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.GetSessionForUpdate(ctx, sessionID); err != nil {
			return notFound(err)
		}
		if err := q.DeleteLastRotationRound(ctx, sessionID); err != nil {
			return err
		}
		return logActivity(ctx, q, a, "session", sessionID, "rotation_round_removed", nil)
	})
}

// rotationPool returns the players eligible for rotation: those checked in as present, or every
// slot holder while nobody has been checked in yet.
func rotationPool(ctx context.Context, q *sqlc.Queries, sessionID int64) ([]domain.Player, error) {
	regs, err := q.ListSessionRegistrations(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	var present, holders []domain.Player
	for _, r := range regs {
		p := domain.Player{ID: r.ID, Level: r.Level}
		switch r.Status {
		case domain.StatusAttended:
			present = append(present, p)
			holders = append(holders, p)
		case domain.StatusRegistered:
			holders = append(holders, p)
		}
	}
	if len(present) > 0 {
		return present, nil
	}
	return holders, nil
}

// SwapRotationPlayers lets the admin correct the latest round by hand: two players on court
// trade places, or a waiting player takes a court spot. Earlier rounds stay as played because
// they drive the "fewest games, longest wait" priority of the next round.
func (s *Service) SwapRotationPlayers(ctx context.Context, a Actor, sessionID, playerA, playerB int64) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		sess, err := q.GetSessionForUpdate(ctx, sessionID)
		if err != nil {
			return notFound(err)
		}
		if sess.Status != domain.SessionPublished {
			return userErr("Rotasi hanya untuk sesi yang sedang berjalan.")
		}
		rows, err := q.ListRotationMatches(ctx, sessionID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return userErr("Belum ada ronde untuk diubah.")
		}
		last := rows[len(rows)-1].RoundNumber
		var ids []int64
		var matches []domain.Match
		playing := map[int64]bool{}
		for _, m := range rows {
			if m.RoundNumber != last {
				continue
			}
			ids = append(ids, m.ID)
			dm := domain.Match{Court: int(m.CourtNumber), TeamA: [2]int64{m.TeamAPlayer1ID, m.TeamAPlayer2ID}, TeamB: [2]int64{m.TeamBPlayer1ID, m.TeamBPlayer2ID}}
			for _, id := range dm.Players() {
				playing[id] = true
			}
			matches = append(matches, dm)
		}
		pool, err := rotationPool(ctx, q, sessionID)
		if err != nil {
			return err
		}
		var waiting []int64
		for _, p := range pool {
			if !playing[p.ID] {
				waiting = append(waiting, p.ID)
			}
		}
		updated, err := domain.SwapInRound(matches, waiting, playerA, playerB)
		if err != nil {
			return userErr(capitalize(err.Error()) + ".")
		}
		for i, m := range updated {
			if m == matches[i] {
				continue
			}
			if err := q.UpdateRotationMatchPlayers(ctx, sqlc.UpdateRotationMatchPlayersParams{
				ID: ids[i], TeamAPlayer1ID: m.TeamA[0], TeamAPlayer2ID: m.TeamA[1], TeamBPlayer1ID: m.TeamB[0], TeamBPlayer2ID: m.TeamB[1],
			}); err != nil {
				return err
			}
		}
		return logActivity(ctx, q, a, "session", sessionID, "rotation_swapped", map[string]any{"round": last, "player_a": playerA, "player_b": playerB})
	})
}
