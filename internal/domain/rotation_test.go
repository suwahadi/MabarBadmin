package domain

import (
	"math/rand/v2"
	"testing"
)

func players(levels ...string) []Player {
	out := make([]Player, len(levels))
	for i, l := range levels {
		out[i] = Player{ID: int64(i + 1), Level: l}
	}
	return out
}

func TestNextRoundFillsCourtsAndSitsOutRest(t *testing.T) {
	ps := players(LevelAdvanced, LevelAdvanced, LevelIntermediate, LevelIntermediate, LevelBeginner,
		LevelBeginner, LevelBeginner, LevelIntermediate, LevelAdvanced, LevelBeginner)
	r := rand.New(rand.NewPCG(7, 7))
	round := NextRound(ps, 2, nil, r)
	if len(round) != 2 {
		t.Fatalf("want 2 matches, got %d", len(round))
	}
	seen := map[int64]bool{}
	for _, m := range round {
		for _, id := range m.Players() {
			if seen[id] {
				t.Fatalf("player %d plays twice", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 8 {
		t.Fatalf("want 8 players, got %d", len(seen))
	}

	sitting := []int64{}
	for _, p := range ps {
		if !seen[p.ID] {
			sitting = append(sitting, p.ID)
		}
	}
	next := NextRound(ps, 2, [][]Match{round}, r)
	inNext := map[int64]bool{}
	for _, m := range next {
		for _, id := range m.Players() {
			inNext[id] = true
		}
	}
	for _, id := range sitting {
		if !inNext[id] {
			t.Fatalf("player %d sat out and must play next round", id)
		}
	}
}

func TestNextRoundBalancesTeams(t *testing.T) {
	ps := players(LevelAdvanced, LevelAdvanced, LevelBeginner, LevelBeginner)
	m := NextRound(ps, 1, nil, rand.New(rand.NewPCG(1, 1)))
	if len(m) != 1 {
		t.Fatal("want one match")
	}
	w := func(id int64) int { return LevelWeight(ps[id-1].Level) }
	a := w(m[0].TeamA[0]) + w(m[0].TeamA[1])
	b := w(m[0].TeamB[0]) + w(m[0].TeamB[1])
	if a != b {
		t.Fatalf("unbalanced teams %d vs %d", a, b)
	}
}

func TestNextRoundAvoidsPreviousPartner(t *testing.T) {
	ps := players(LevelIntermediate, LevelIntermediate, LevelIntermediate, LevelIntermediate)
	r := rand.New(rand.NewPCG(3, 3))
	first := NextRound(ps, 1, nil, r)
	second := NextRound(ps, 1, [][]Match{first}, r)
	prev := map[[2]int64]bool{
		pairKey(first[0].TeamA[0], first[0].TeamA[1]): true,
		pairKey(first[0].TeamB[0], first[0].TeamB[1]): true,
	}
	if prev[pairKey(second[0].TeamA[0], second[0].TeamA[1])] || prev[pairKey(second[0].TeamB[0], second[0].TeamB[1])] {
		t.Fatalf("partners repeated: %v then %v", first, second)
	}
}

func TestNextRoundNotEnoughPlayers(t *testing.T) {
	if NextRound(players(LevelBeginner, LevelBeginner, LevelBeginner), 2, nil, rand.New(rand.NewPCG(1, 1))) != nil {
		t.Fatal("fewer than 4 players must produce no matches")
	}
	if got := NextRound(players(LevelBeginner, LevelBeginner, LevelBeginner, LevelBeginner, LevelBeginner), 3, nil, rand.New(rand.NewPCG(1, 1))); len(got) != 1 {
		t.Fatalf("5 players on 3 courts must give 1 match, got %d", len(got))
	}
}
