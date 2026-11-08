package locale

import (
	"errors"
	"strings"
)

var ErrInvalidPhone = errors.New("nomor WA tidak valid")

// NormalizePhone converts 08xx, +628xx, 628xx, and 8xx forms to 628xx.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case strings.HasPrefix(d, "62"):
	case strings.HasPrefix(d, "0"):
		d = "62" + d[1:]
	case strings.HasPrefix(d, "8"):
		d = "62" + d
	default:
		return "", ErrInvalidPhone
	}
	if !strings.HasPrefix(d, "628") || len(d) < 10 || len(d) > 15 {
		return "", ErrInvalidPhone
	}
	return d, nil
}

// FormatPhone renders 6281234567890 as 0812-3456-7890.
func FormatPhone(p string) string {
	if !strings.HasPrefix(p, "62") {
		return p
	}
	local := "0" + p[2:]
	if len(local) <= 8 {
		return local
	}
	return local[:4] + "-" + local[4:8] + "-" + local[8:]
}

// MaskPhone hides the middle group for public pages: 0812-xxxx-7890.
func MaskPhone(p string) string {
	f := FormatPhone(p)
	parts := strings.Split(f, "-")
	if len(parts) != 3 {
		return f
	}
	return parts[0] + "-" + strings.Repeat("x", len(parts[1])) + "-" + parts[2]
}
