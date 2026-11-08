package domain

import (
	"sort"
	"time"
)

// Seat is the allocation view of an active registration.
type Seat struct {
	ID       int64
	Status   string
	Paid     bool
	QueuedAt time.Time
	PaidAt   time.Time
}

type Move struct {
	ID int64
	To string
}

type AllocateOptions struct {
	// DeadlinePassed applies an explicit per-session payment deadline: unpaid seats lose their slot
	// and only paid participants may be promoted.
	DeadlinePassed bool
	// Hold keeps these registrations out of promotion during this run, e.g. right after an admin
	// moved someone to the waiting list.
	Hold map[int64]bool
}

// Allocate computes which active registrations must move between registered and waitlisted so
// the session follows the quota, waiting list order, and paid-priority rules (PRD S4, rules 1-3).
func Allocate(capacity int, seats []Seat, opts AllocateOptions) []Move {
	var reg, wl []Seat
	original := map[int64]string{}
	for _, s := range seats {
		original[s.ID] = s.Status
		switch s.Status {
		case StatusRegistered:
			reg = append(reg, s)
		case StatusWaitlisted:
			wl = append(wl, s)
		}
	}

	if opts.DeadlinePassed {
		kept := reg[:0:0]
		for _, s := range reg {
			if s.Paid {
				kept = append(kept, s)
			} else {
				wl = append(wl, s)
			}
		}
		reg = kept
	}

	for len(reg) > capacity && capacity >= 0 {
		i := demotionCandidate(reg, true)
		if i < 0 {
			i = demotionCandidate(reg, false)
		}
		wl = append(wl, reg[i])
		reg = append(reg[:i], reg[i+1:]...)
	}

	eligible := func(s Seat) bool {
		if opts.Hold[s.ID] {
			return false
		}
		return s.Paid || !opts.DeadlinePassed
	}

	SortWaitlist(wl)
	for len(reg) < capacity {
		i := firstIndex(wl, eligible)
		if i < 0 {
			break
		}
		reg = append(reg, wl[i])
		wl = append(wl[:i], wl[i+1:]...)
	}

	for {
		pi := firstIndex(wl, func(s Seat) bool { return s.Paid && eligible(s) })
		if pi < 0 {
			break
		}
		di := demotionCandidate(reg, true)
		if di < 0 {
			break
		}
		promoted, demoted := wl[pi], reg[di]
		wl = append(wl[:pi], wl[pi+1:]...)
		reg = append(reg[:di], reg[di+1:]...)
		reg = append(reg, promoted)
		wl = append(wl, demoted)
		SortWaitlist(wl)
	}

	var moves []Move
	for _, s := range reg {
		if original[s.ID] != StatusRegistered {
			moves = append(moves, Move{ID: s.ID, To: StatusRegistered})
		}
	}
	for _, s := range wl {
		if original[s.ID] != StatusWaitlisted {
			moves = append(moves, Move{ID: s.ID, To: StatusWaitlisted})
		}
	}
	sort.Slice(moves, func(i, j int) bool { return moves[i].ID < moves[j].ID })
	return moves
}

// SortWaitlist orders the waiting list: paid first by payment time, then unpaid by queue time.
func SortWaitlist(wl []Seat) {
	sort.SliceStable(wl, func(i, j int) bool {
		a, b := wl[i], wl[j]
		if a.Paid != b.Paid {
			return a.Paid
		}
		if a.Paid && !a.PaidAt.Equal(b.PaidAt) {
			return a.PaidAt.Before(b.PaidAt)
		}
		if !a.QueuedAt.Equal(b.QueuedAt) {
			return a.QueuedAt.Before(b.QueuedAt)
		}
		return a.ID < b.ID
	})
}

// demotionCandidate picks the seat that loses its slot first: among unpaid (or paid when
// unpaidOnly is false) the one queued last.
func demotionCandidate(reg []Seat, unpaidOnly bool) int {
	best := -1
	for i, s := range reg {
		if unpaidOnly && s.Paid {
			continue
		}
		if best < 0 || later(s, reg[best], unpaidOnly) {
			best = i
		}
	}
	return best
}

func later(a, b Seat, byQueue bool) bool {
	ta, tb := a.QueuedAt, b.QueuedAt
	if !byQueue {
		ta, tb = a.PaidAt, b.PaidAt
	}
	if !ta.Equal(tb) {
		return ta.After(tb)
	}
	return a.ID > b.ID
}

func firstIndex(s []Seat, ok func(Seat) bool) int {
	for i, v := range s {
		if ok(v) {
			return i
		}
	}
	return -1
}
