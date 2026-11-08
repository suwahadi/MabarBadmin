package domain

import (
	"math/rand/v2"
	"sort"
)

type Player struct {
	ID    int64
	Level string
}

type Match struct {
	Court int
	TeamA [2]int64
	TeamB [2]int64
}

func (m Match) Players() [4]int64 {
	return [4]int64{m.TeamA[0], m.TeamA[1], m.TeamB[0], m.TeamB[1]}
}

type playerState struct {
	Player
	games      int
	lastPlayed int
	tiebreak   float64
}

// NextRound builds the next rotation round (PRD 5.5). Players are picked by fewest games, then
// longest time sitting, and split into 2 vs 2 matches with the closest level totals while avoiding
// partners from the previous round. Players beyond courts*4 sit out and gain priority next round.
func NextRound(players []Player, courts int, history [][]Match, r *rand.Rand) []Match {
	if courts <= 0 || len(players) < 4 {
		return nil
	}
	states := make([]playerState, len(players))
	index := map[int64]int{}
	for i, p := range players {
		states[i] = playerState{Player: p, lastPlayed: -1, tiebreak: r.Float64()}
		index[p.ID] = i
	}
	for round, matches := range history {
		for _, m := range matches {
			for _, id := range m.Players() {
				if i, ok := index[id]; ok {
					states[i].games++
					states[i].lastPlayed = round
				}
			}
		}
	}
	prevPartners := map[[2]int64]bool{}
	if len(history) > 0 {
		for _, m := range history[len(history)-1] {
			prevPartners[pairKey(m.TeamA[0], m.TeamA[1])] = true
			prevPartners[pairKey(m.TeamB[0], m.TeamB[1])] = true
		}
	}

	sort.SliceStable(states, func(i, j int) bool {
		a, b := states[i], states[j]
		if a.games != b.games {
			return a.games < b.games
		}
		if a.lastPlayed != b.lastPlayed {
			return a.lastPlayed < b.lastPlayed
		}
		return a.tiebreak < b.tiebreak
	})

	active := min(courts, len(states)/4)
	selected := append([]playerState(nil), states[:active*4]...)
	sort.SliceStable(selected, func(i, j int) bool {
		wi, wj := LevelWeight(selected[i].Level), LevelWeight(selected[j].Level)
		if wi != wj {
			return wi > wj
		}
		return selected[i].tiebreak < selected[j].tiebreak
	})

	matches := make([]Match, 0, active)
	for c := 0; c < active; c++ {
		group := selected[c*4 : c*4+4]
		matches = append(matches, bestPairing(group, prevPartners, c+1))
	}
	return matches
}

func bestPairing(g []playerState, prev map[[2]int64]bool, court int) Match {
	options := [3][4]int{{0, 1, 2, 3}, {0, 2, 1, 3}, {0, 3, 1, 2}}
	best, bestScore := Match{}, -1
	for _, o := range options {
		a1, a2, b1, b2 := g[o[0]], g[o[1]], g[o[2]], g[o[3]]
		diff := LevelWeight(a1.Level) + LevelWeight(a2.Level) - LevelWeight(b1.Level) - LevelWeight(b2.Level)
		if diff < 0 {
			diff = -diff
		}
		score := diff * 100
		if prev[pairKey(a1.ID, a2.ID)] {
			score++
		}
		if prev[pairKey(b1.ID, b2.ID)] {
			score++
		}
		if bestScore < 0 || score < bestScore {
			bestScore = score
			best = Match{Court: court, TeamA: [2]int64{a1.ID, a2.ID}, TeamB: [2]int64{b1.ID, b2.ID}}
		}
	}
	return best
}

func pairKey(a, b int64) [2]int64 {
	if a > b {
		a, b = b, a
	}
	return [2]int64{a, b}
}
