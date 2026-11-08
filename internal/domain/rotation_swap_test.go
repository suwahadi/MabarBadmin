package domain

import (
	"errors"
	"testing"
)

func round() []Match {
	return []Match{
		{Court: 1, TeamA: [2]int64{1, 2}, TeamB: [2]int64{3, 4}},
		{Court: 2, TeamA: [2]int64{5, 6}, TeamB: [2]int64{7, 8}},
	}
}

func TestSwapAcrossCourts(t *testing.T) {
	orig := round()
	got, err := SwapInRound(orig, nil, 2, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].TeamA != [2]int64{1, 7} || got[1].TeamB != [2]int64{2, 8} {
		t.Fatalf("got %+v", got)
	}
	if orig[0].TeamA != [2]int64{1, 2} {
		t.Fatal("input must not be modified")
	}
}

func TestSwapWithinCourtChangesPartners(t *testing.T) {
	got, err := SwapInRound(round(), nil, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].TeamA != [2]int64{1, 3} || got[0].TeamB != [2]int64{2, 4} {
		t.Fatalf("got %+v", got)
	}
}

func TestSwapWithWaitingPlayer(t *testing.T) {
	got, err := SwapInRound(round(), []int64{9}, 9, 6)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].TeamA != [2]int64{5, 9} {
		t.Fatalf("got %+v", got)
	}
	got, err = SwapInRound(round(), []int64{9}, 4, 9)
	if err != nil || got[0].TeamB != [2]int64{3, 9} {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestSwapErrors(t *testing.T) {
	cases := []struct {
		a, b int64
		want error
	}{
		{1, 1, ErrSwapSamePlayer},
		{9, 10, ErrSwapBothWaiting},
		{1, 99, ErrSwapUnknownPlayer},
		{99, 1, ErrSwapUnknownPlayer},
	}
	for _, c := range cases {
		if _, err := SwapInRound(round(), []int64{9, 10}, c.a, c.b); !errors.Is(err, c.want) {
			t.Errorf("swap %d,%d: got %v want %v", c.a, c.b, err, c.want)
		}
	}
}
