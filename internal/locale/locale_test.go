package locale

import (
	"testing"
	"time"
)

func TestFormatRupiah(t *testing.T) {
	cases := map[int64]string{0: "Rp 0", 500: "Rp 500", 85000: "Rp 85.000", 60017: "Rp 60.017", 1234567: "Rp 1.234.567", -2500: "-Rp 2.500"}
	for in, want := range cases {
		if got := FormatRupiah(in); got != want {
			t.Errorf("FormatRupiah(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatDateTime(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	start := time.Date(2026, 11, 9, 21, 0, 0, 0, loc)
	if got := FormatDateTime(start, loc); got != "Senin, 9 November 2026 21:00 WIB" {
		t.Errorf("got %q", got)
	}
	if got := FormatTimeRange(start, start.Add(3*time.Hour), loc); got != "21:00–24:00 WIB (3 jam)" {
		t.Errorf("got %q", got)
	}
	if got := FormatShortDate(start, loc); got != "09/11/2026" {
		t.Errorf("got %q", got)
	}
	makassar, _ := time.LoadLocation("Asia/Makassar")
	if got := FormatDateTime(start, makassar); got != "Senin, 9 November 2026 22:00 WITA" {
		t.Errorf("got %q", got)
	}
}

func TestPhone(t *testing.T) {
	for _, in := range []string{"0812-3456-7890", "+62 812 3456 7890", "6281234567890", "81234567890"} {
		got, err := NormalizePhone(in)
		if err != nil || got != "6281234567890" {
			t.Errorf("NormalizePhone(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "12345", "021123456", "0712345678"} {
		if _, err := NormalizePhone(in); err == nil {
			t.Errorf("NormalizePhone(%q) should fail", in)
		}
	}
	if got := FormatPhone("6281234567890"); got != "0812-3456-7890" {
		t.Errorf("FormatPhone = %q", got)
	}
	if got := MaskPhone("6281234567890"); got != "0812-xxxx-7890" {
		t.Errorf("MaskPhone = %q", got)
	}
}
