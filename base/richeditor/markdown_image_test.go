package richeditor_test

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/base/richeditor"
)

func TestMarkdownInlineImages(t *testing.T) {
	exts := append(richeditor.StarterKit(richeditor.StarterOptions{}), richeditor.Extension{Name: "inlineImage",
		Nodes: []richeditor.NodeSpec{{Name: "inlineImage", Inline: true, Group: "inline", Atom: true,
			Attrs: []richeditor.AttrSpec{{Name: "src"}, {Name: "alt", HasDefault: true}, {Name: "title", HasDefault: true}}}}})
	const source = "see ![pic](a.png 'a \"quote\"') here\n\n![solo](b.png)\n\ncaption\n![pic](c.png)\n\nsee [![pic](d.png \"the \\\"title\\\"\")](https://x) here\n"
	s, err := richeditor.NewSession(richeditor.SessionConfig{Extensions: exts, Markdown: source})
	if err != nil {
		t.Fatal(err)
	}
	doc := s.State().Doc
	if got := doc.Child(1).Type.Name; got != "image" {
		t.Fatalf("standalone image became %s", got)
	}
	var sources []string
	doc.Descendants(func(n *richeditor.Node, _ int, _ *richeditor.Node, _ int) bool {
		if n.Type.Name == "inlineImage" {
			sources = append(sources, n.AttrStr("src"))
		}
		return true
	})
	if got := strings.Join(sources, ","); got != "a.png,c.png,d.png" {
		t.Fatalf("inline image sources = %q", got)
	}
	got := s.Markdown()
	for _, want := range []string{`see ![pic](a.png "a \"quote\"") here`, `see [![pic](d.png "the \"title\"")](https://x) here`, "caption ![pic](c.png)", "\n\n![solo](b.png)\n\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("Markdown %q omits %q", got, want)
		}
	}
}
