package jarch

// A small DOM view that reproduces what the original pipeline got from
// `pup 'div#content json{}'`: element nodes only, attributes HTML-escaped, and
// a per-node `text` made of the node's own (direct) trimmed text children,
// HTML-escaped and joined with single spaces.

import (
	"html"
	"strings"

	nethtml "golang.org/x/net/html"
)

type node struct {
	n        *nethtml.Node
	children []*node
	text     string
	hasText  bool
}

func wrap(n *nethtml.Node) *node {
	w := &node{n: n}
	var texts []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case nethtml.ElementNode:
			w.children = append(w.children, wrap(c))
		case nethtml.TextNode:
			if t := strings.TrimSpace(c.Data); t != "" {
				texts = append(texts, html.EscapeString(t))
			}
		}
	}
	w.text = strings.Join(texts, " ")
	w.hasText = len(texts) > 0
	return w
}

// parseContent parses a page and returns div#content (nil if absent).
func parseContent(page string) *node {
	doc, err := nethtml.Parse(strings.NewReader(page))
	if err != nil {
		return nil
	}
	var find func(*nethtml.Node) *nethtml.Node
	find = func(n *nethtml.Node) *nethtml.Node {
		if n.Type == nethtml.ElementNode && n.Data == "div" && rawAttr(n, "id") == "content" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if f := find(c); f != nil {
				return f
			}
		}
		return nil
	}
	if c := find(doc); c != nil {
		return wrap(c)
	}
	return nil
}

func rawAttr(n *nethtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// attr returns the attribute as pup emitted it (HTML-escaped).
func (w *node) attr(key string) string {
	if w == nil {
		return ""
	}
	return html.EscapeString(rawAttr(w.n, key))
}

func (w *node) tag() string {
	if w == nil {
		return ""
	}
	return w.n.Data
}

// all returns w and its descendants matching f, in document order.
func (w *node) all(f func(*node) bool) []*node {
	var out []*node
	var walk func(*node)
	walk = func(x *node) {
		if f(x) {
			out = append(out, x)
		}
		for _, c := range x.children {
			walk(c)
		}
	}
	if w != nil {
		walk(w)
	}
	return out
}

func (w *node) first(f func(*node) bool) *node {
	if r := w.all(f); len(r) > 0 {
		return r[0]
	}
	return nil
}

func byClass(c string) func(*node) bool { return func(x *node) bool { return x.attr("class") == c } }
func byID(id string) func(*node) bool   { return func(x *node) bool { return x.attr("id") == id } }
func byTag(t string) func(*node) bool   { return func(x *node) bool { return x.tag() == t } }

// rtext flattens all text under w: each node's own text in pre-order
// (parent before children), joined, decoded and whitespace-collapsed.
func (w *node) rtext() string {
	var parts []string
	for _, x := range w.all(func(*node) bool { return true }) {
		if x.hasText {
			parts = append(parts, x.text)
		}
	}
	return trim(wsRunRE.ReplaceAllString(unescape(strings.Join(parts, " ")), " "))
}
