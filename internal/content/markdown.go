package content

import (
	"bytes"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	md = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithHardWraps()),
	)
	policy = func() *bluemonday.Policy {
		p := bluemonday.UGCPolicy()
		p.RequireNoFollowOnLinks(false)
		p.AddTargetBlankToFullyQualifiedLinks(true)
		return p
	}()
)

// Render converts admin Markdown to HTML and sanitizes it, so stored content cannot inject scripts.
func Render(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(src))
	}
	return template.HTML(policy.SanitizeBytes(buf.Bytes()))
}

// PlainText strips all markup, used for meta descriptions and JSON-LD answers.
func PlainText(src string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return src
	}
	return bluemonday.StrictPolicy().Sanitize(buf.String())
}
