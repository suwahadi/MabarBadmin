package domain

import "errors"

var (
	ErrSwapSamePlayer    = errors.New("pilih dua pemain yang berbeda")
	ErrSwapBothWaiting   = errors.New("kedua pemain sedang menunggu, pilih minimal satu yang sedang main")
	ErrSwapUnknownPlayer = errors.New("pemain tidak ada di ronde ini atau tidak hadir")
)

// SwapInRound exchanges two players in a round. Both may be on court (any court, team, or
// position), or one may be waiting, in which case the waiting player takes the other's place.
// waiting lists the present players who are not in any match of the round.
func SwapInRound(matches []Match, waiting []int64, a, b int64) ([]Match, error) {
	if a == b {
		return nil, ErrSwapSamePlayer
	}
	out := make([]Match, len(matches))
	copy(out, matches)
	slot := func(id int64) *int64 {
		for i := range out {
			for j := range out[i].TeamA {
				if out[i].TeamA[j] == id {
					return &out[i].TeamA[j]
				}
			}
			for j := range out[i].TeamB {
				if out[i].TeamB[j] == id {
					return &out[i].TeamB[j]
				}
			}
		}
		return nil
	}
	isWaiting := func(id int64) bool {
		for _, w := range waiting {
			if w == id {
				return true
			}
		}
		return false
	}
	sa, sb := slot(a), slot(b)
	switch {
	case sa != nil && sb != nil:
		*sa, *sb = b, a
	case sa != nil && isWaiting(b):
		*sa = b
	case sb != nil && isWaiting(a):
		*sb = a
	case isWaiting(a) && isWaiting(b):
		return nil, ErrSwapBothWaiting
	default:
		return nil, ErrSwapUnknownPlayer
	}
	return out, nil
}
