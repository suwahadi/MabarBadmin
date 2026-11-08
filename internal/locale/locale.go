package locale

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

var weekdays = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

var months = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus",
	"September", "Oktober", "November", "Desember"}

func WeekdayName(d time.Weekday) string { return weekdays[d] }

func MonthName(m time.Month) string { return months[m-1] }

func FormatThousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s[i : i+3])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func FormatRupiah(n int64) string {
	if n < 0 {
		return "-Rp " + FormatThousands(-n)
	}
	return "Rp " + FormatThousands(n)
}

// ZoneLabel maps Indonesian IANA zones to their local abbreviation; other zones fall back to Go's.
func ZoneLabel(t time.Time) string {
	_, offset := t.Zone()
	switch offset {
	case 7 * 3600:
		return "WIB"
	case 8 * 3600:
		return "WITA"
	case 9 * 3600:
		return "WIT"
	}
	name, _ := t.Zone()
	return name
}

func FormatDate(t time.Time, loc *time.Location) string {
	t = t.In(loc)
	return fmt.Sprintf("%s, %d %s %d", weekdays[t.Weekday()], t.Day(), months[t.Month()-1], t.Year())
}

func FormatDateTime(t time.Time, loc *time.Location) string {
	t = t.In(loc)
	return fmt.Sprintf("%s %s %s", FormatDate(t, loc), t.Format("15:04"), ZoneLabel(t))
}

func FormatShortDate(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("02/01/2006")
}

func FormatTime(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("15:04")
}

// FormatTimeRange renders "21:00–24:00 WIB (3 jam)"; an end at local midnight is shown as 24:00.
func FormatTimeRange(start, end time.Time, loc *time.Location) string {
	s, e := start.In(loc), end.In(loc)
	endText := e.Format("15:04")
	if endText == "00:00" && e.After(s) {
		endText = "24:00"
	}
	return fmt.Sprintf("%s–%s %s (%s)", s.Format("15:04"), endText, ZoneLabel(s), FormatDuration(end.Sub(start)))
}

func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%d jam %d menit", h, m)
	case h > 0:
		return fmt.Sprintf("%d jam", h)
	default:
		return fmt.Sprintf("%d menit", m)
	}
}

func FormatMonth(t time.Time) string {
	return fmt.Sprintf("%s %d", months[t.Month()-1], t.Year())
}
