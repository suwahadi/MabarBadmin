package seo

import (
	"encoding/json"
	"html/template"
	"strings"
)

type Meta struct {
	Title       string
	Description string
	Keywords    string
	Canonical   string
	Image       string
	ImageAlt    string
	Type        string
	NoIndex     bool
	JSONLD      []template.JS
}

// Title applies the admin's pattern, e.g. "{title} | {site}"; the home page uses the site name alone.
func Title(pattern, title, site string) string {
	if strings.TrimSpace(title) == "" || title == site {
		return site
	}
	if pattern == "" {
		pattern = "{title} | {site}"
	}
	return strings.NewReplacer("{title}", title, "{site}", site).Replace(pattern)
}

// First returns the first non-empty value, following the entity → automatic → default order.
func First(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// Truncate shortens descriptions to about 160 characters on a word boundary.
func Truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	cut := string(r[:n])
	if i := strings.LastIndex(cut, " "); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, ",.;: ") + "…"
}

// JSONLD marshals structured data; encoding/json escapes <, >, and & so the result is safe in a script tag.
func JSONLD(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

type Map = map[string]any

func Organization(name, url, logo string, sameAs []string) Map {
	m := Map{"@context": "https://schema.org", "@type": "Organization", "name": name, "url": url}
	if logo != "" {
		m["logo"] = logo
	}
	if len(sameAs) > 0 {
		m["sameAs"] = sameAs
	}
	return m
}

func WebSite(name, url string) Map {
	return Map{"@context": "https://schema.org", "@type": "WebSite", "name": name, "url": url, "inLanguage": "id-ID"}
}

type Event struct {
	Name, Description, URL, Image  string
	Start, End                     string
	VenueName, VenueAddress        string
	Price                          int64
	Available                      bool
	Cancelled                      bool
	OrganizerName, OrganizerURL    string
	RemainingCapacity, MaxCapacity int
}

func SportsEvent(e Event) Map {
	status := "https://schema.org/EventScheduled"
	if e.Cancelled {
		status = "https://schema.org/EventCancelled"
	}
	availability := "https://schema.org/InStock"
	if !e.Available {
		availability = "https://schema.org/SoldOut"
	}
	m := Map{
		"@context":                  "https://schema.org",
		"@type":                     "SportsEvent",
		"name":                      e.Name,
		"description":               e.Description,
		"url":                       e.URL,
		"startDate":                 e.Start,
		"endDate":                   e.End,
		"eventStatus":               status,
		"eventAttendanceMode":       "https://schema.org/OfflineEventAttendanceMode",
		"sport":                     "Badminton",
		"maximumAttendeeCapacity":   e.MaxCapacity,
		"remainingAttendeeCapacity": e.RemainingCapacity,
		"location": Map{"@type": "Place", "name": e.VenueName,
			"address": Map{"@type": "PostalAddress", "streetAddress": e.VenueAddress, "addressCountry": "ID"}},
		"offers":    Map{"@type": "Offer", "price": e.Price, "priceCurrency": "IDR", "availability": availability, "url": e.URL},
		"organizer": Map{"@type": "Organization", "name": e.OrganizerName, "url": e.OrganizerURL},
	}
	if e.Image != "" {
		m["image"] = []string{e.Image}
	}
	return m
}

type QA struct{ Question, Answer string }

func FAQPage(items []QA) Map {
	entities := make([]Map, len(items))
	for i, it := range items {
		entities[i] = Map{"@type": "Question", "name": it.Question,
			"acceptedAnswer": Map{"@type": "Answer", "text": it.Answer}}
	}
	return Map{"@context": "https://schema.org", "@type": "FAQPage", "mainEntity": entities}
}

type Crumb struct{ Name, URL string }

func BreadcrumbList(crumbs []Crumb) Map {
	items := make([]Map, len(crumbs))
	for i, c := range crumbs {
		items[i] = Map{"@type": "ListItem", "position": i + 1, "name": c.Name, "item": c.URL}
	}
	return Map{"@context": "https://schema.org", "@type": "BreadcrumbList", "itemListElement": items}
}
