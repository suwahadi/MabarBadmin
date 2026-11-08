package domain

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func at(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

func seat(id int64, status string, queued int, paidAt int) Seat {
	s := Seat{ID: id, Status: status, QueuedAt: at(queued)}
	if paidAt >= 0 {
		s.Paid = true
		s.PaidAt = at(paidAt)
	}
	return s
}

func apply(seats []Seat, moves []Move) map[int64]string {
	out := map[int64]string{}
	for _, s := range seats {
		out[s.ID] = s.Status
	}
	for _, m := range moves {
		out[m.ID] = m.To
	}
	return out
}

func TestAllocateFillsFreeSlotsInOrder(t *testing.T) {
	seats := []Seat{
		seat(1, StatusWaitlisted, 1, -1),
		seat(2, StatusWaitlisted, 2, 50),
		seat(3, StatusWaitlisted, 3, -1),
	}
	got := apply(seats, Allocate(2, seats, AllocateOptions{}))
	if got[2] != StatusRegistered || got[1] != StatusRegistered || got[3] != StatusWaitlisted {
		t.Fatalf("paid first then earliest unpaid should be promoted, got %v", got)
	}
}

func TestAllocateSwapsPaidWaitlistWithLatestUnpaid(t *testing.T) {
	seats := []Seat{
		seat(1, StatusRegistered, 1, -1),
		seat(2, StatusRegistered, 2, 10),
		seat(3, StatusRegistered, 3, -1),
		seat(4, StatusWaitlisted, 4, 20),
		seat(5, StatusWaitlisted, 5, 30),
		seat(6, StatusWaitlisted, 6, -1),
	}
	got := apply(seats, Allocate(3, seats, AllocateOptions{}))
	want := map[int64]string{1: StatusWaitlisted, 2: StatusRegistered, 3: StatusWaitlisted, 4: StatusRegistered, 5: StatusRegistered, 6: StatusWaitlisted}
	for id, st := range want {
		if got[id] != st {
			t.Errorf("seat %d: got %s want %s", id, got[id], st)
		}
	}
}

func TestAllocateStopsWhenAllRegisteredPaid(t *testing.T) {
	seats := []Seat{
		seat(1, StatusRegistered, 1, 5),
		seat(2, StatusRegistered, 2, 6),
		seat(3, StatusWaitlisted, 3, 7),
	}
	if moves := Allocate(2, seats, AllocateOptions{}); len(moves) != 0 {
		t.Fatalf("expected no moves, got %v", moves)
	}
}

func TestAllocateNoMovesWhenUnderQuotaAndNoWaitlist(t *testing.T) {
	seats := []Seat{seat(1, StatusRegistered, 1, -1)}
	if moves := Allocate(4, seats, AllocateOptions{}); len(moves) != 0 {
		t.Fatalf("expected no moves, got %v", moves)
	}
}

func TestAllocateCapacityReductionDemotesUnpaidThenPaid(t *testing.T) {
	seats := []Seat{
		seat(1, StatusRegistered, 1, 10),
		seat(2, StatusRegistered, 2, -1),
		seat(3, StatusRegistered, 3, 5),
	}
	got := apply(seats, Allocate(1, seats, AllocateOptions{}))
	if got[2] != StatusWaitlisted || got[1] != StatusWaitlisted || got[3] != StatusRegistered {
		t.Fatalf("got %v", got)
	}
}

func TestAllocateDeadlinePassedOnlyPaidKeepSlots(t *testing.T) {
	seats := []Seat{
		seat(1, StatusRegistered, 1, -1),
		seat(2, StatusRegistered, 2, 3),
		seat(3, StatusWaitlisted, 3, -1),
		seat(4, StatusWaitlisted, 4, 9),
	}
	got := apply(seats, Allocate(3, seats, AllocateOptions{DeadlinePassed: true}))
	want := map[int64]string{1: StatusWaitlisted, 2: StatusRegistered, 3: StatusWaitlisted, 4: StatusRegistered}
	for id, st := range want {
		if got[id] != st {
			t.Errorf("seat %d: got %s want %s", id, got[id], st)
		}
	}
}

func TestAllocateHoldSkipsPromotion(t *testing.T) {
	seats := []Seat{
		seat(1, StatusWaitlisted, 1, -1),
		seat(2, StatusWaitlisted, 2, -1),
	}
	got := apply(seats, Allocate(1, seats, AllocateOptions{Hold: map[int64]bool{1: true}}))
	if got[1] != StatusWaitlisted || got[2] != StatusRegistered {
		t.Fatalf("got %v", got)
	}
}

func TestSortWaitlist(t *testing.T) {
	wl := []Seat{
		seat(1, StatusWaitlisted, 1, -1),
		seat(2, StatusWaitlisted, 2, 40),
		seat(3, StatusWaitlisted, 0, -1),
		seat(4, StatusWaitlisted, 5, 30),
	}
	SortWaitlist(wl)
	order := []int64{wl[0].ID, wl[1].ID, wl[2].ID, wl[3].ID}
	want := []int64{4, 2, 3, 1}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v want %v", order, want)
		}
	}
}
