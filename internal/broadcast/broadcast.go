package broadcast

import (
	"fmt"
	"strings"
)

type Line struct {
	Name string
	Paid bool
}

type Data struct {
	AppName      string
	Title        string
	Date         string
	TimeRange    string
	Venue        string
	Address      string
	MapsURL      string
	CourtCount   int
	Capacity     int
	Price        string
	PointRule    string
	Level        string
	Shuttlecock  string
	Notes        string
	SessionURL   string
	PaymentInfo  string
	Participants []Line
	Waitlist     []Line
	Social       []string
}

// PaidMark is the money emoji the WhatsApp group used for paid participants; WhatsApp cannot show SVG icons.
const PaidMark = "💰"

// Render fills the admin's template. The participant list always shows numbered seats up to the
// quota so the group can see open slots, matching the old manual list.
func Render(tpl string, d Data) string {
	var list strings.Builder
	n := max(d.Capacity, len(d.Participants))
	for i := range n {
		fmt.Fprintf(&list, "%d. ", i+1)
		if i < len(d.Participants) {
			p := d.Participants[i]
			list.WriteString(p.Name)
			if p.Paid {
				list.WriteString(" " + PaidMark)
			}
		}
		if i < n-1 {
			list.WriteByte('\n')
		}
	}
	var wl strings.Builder
	if len(d.Waitlist) > 0 {
		wl.WriteString("\n*Waiting list*:\n")
		for i, p := range d.Waitlist {
			fmt.Fprintf(&wl, "%d. %s", i+1, p.Name)
			if p.Paid {
				wl.WriteString(" " + PaidMark)
			}
			wl.WriteByte('\n')
		}
	}
	notes := strings.TrimSpace(d.Notes)
	if notes == "" {
		notes = "-"
	}
	r := strings.NewReplacer(
		"{app_name}", d.AppName,
		"{title}", d.Title,
		"{date}", d.Date,
		"{time_range}", d.TimeRange,
		"{venue}", d.Venue,
		"{address}", d.Address,
		"{maps_url}", d.MapsURL,
		"{court_count}", fmt.Sprint(d.CourtCount),
		"{capacity}", fmt.Sprint(d.Capacity),
		"{registered_count}", fmt.Sprint(len(d.Participants)),
		"{price}", d.Price,
		"{point_rule}", d.PointRule,
		"{level}", d.Level,
		"{shuttlecock}", d.Shuttlecock,
		"{participants}", list.String(),
		"{waitlist}", wl.String(),
		"{payment_info}", d.PaymentInfo,
		"{notes}", notes,
		"{session_url}", d.SessionURL,
		"{social}", strings.Join(d.Social, "\n"),
	)
	out := r.Replace(tpl)
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}
