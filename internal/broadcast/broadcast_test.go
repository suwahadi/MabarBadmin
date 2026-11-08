package broadcast

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	out := Render("{title}\n{participants}\n{waitlist}\n{session_url}\n{notes}", Data{
		Title: "Senin Malam", Capacity: 3, SessionURL: "https://x/sessions/a",
		Participants: []Line{{Name: "Budi", Paid: true}, {Name: "Sari"}},
		Waitlist:     []Line{{Name: "Andi", Paid: true}},
	})
	for _, want := range []string{"1. Budi 💰", "2. Sari\n3. ", "*Waiting list*", "1. Andi 💰", "https://x/sessions/a", "\n-"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\n\n\n") {
		t.Error("triple newline left")
	}
}
