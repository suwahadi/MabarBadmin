package seo

import (
	"strings"
	"testing"
)

func TestTitle(t *testing.T) {
	if got := Title("{title} | {site}", "FAQ", "MabarBadmin"); got != "FAQ | MabarBadmin" {
		t.Fatalf("got %q", got)
	}
	if got := Title("{title} | {site}", "", "MabarBadmin"); got != "MabarBadmin" {
		t.Fatalf("got %q", got)
	}
}

func TestFirstAndTruncate(t *testing.T) {
	if First("", "  ", "b", "c") != "b" {
		t.Fatal("First")
	}
	long := strings.Repeat("kata ", 60)
	if got := Truncate(long, 160); len([]rune(got)) > 161 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
}

func TestJSONLDEscapes(t *testing.T) {
	got := string(JSONLD(FAQPage([]QA{{Question: "</script><script>", Answer: "a & b"}})))
	if strings.Contains(got, "</script>") || strings.Contains(got, "&") && !strings.Contains(got, `&`) {
		t.Fatalf("unsafe json-ld: %s", got)
	}
	if !strings.Contains(string(JSONLD(SportsEvent(Event{Name: "X", Price: 60000, Available: true}))), `"priceCurrency":"IDR"`) {
		t.Fatal("event offer missing")
	}
	if !strings.Contains(string(JSONLD(BreadcrumbList([]Crumb{{"Beranda", "/"}}))), `"position":1`) {
		t.Fatal("breadcrumb")
	}
}
