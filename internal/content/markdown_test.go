package content

import (
	"strings"
	"testing"
)

func TestRenderSanitizes(t *testing.T) {
	out := string(Render("# Judul\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1)) **tebal**"))
	if strings.Contains(out, "<script") || strings.Contains(out, "javascript:") {
		t.Fatalf("unsafe output: %s", out)
	}
	if !strings.Contains(out, "<h1") || !strings.Contains(out, "<strong>tebal</strong>") {
		t.Fatalf("markdown not rendered: %s", out)
	}
	if got := PlainText("**a** _b_"); strings.TrimSpace(got) != "a b" {
		t.Fatalf("plain %q", got)
	}
}
