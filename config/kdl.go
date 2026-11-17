package config

// Minimal KDL subset parser/serializer, covering exactly what config.kdl
// needs: named nodes with one scalar argument (string/int/bool) and one level
// of children blocks, plus // line comments. Per SPEC.md, a hand-rolled
// subset is preferred over an unmaintained dependency.

import (
	"fmt"
	"strconv"
	"strings"
)

// kdlNode is a parsed node: a name, optional scalar args, optional children.
type kdlNode struct {
	Name     string
	Args     []any // string, int64 or bool
	Children []kdlNode
}

// findChild returns the first child with the given name, or nil.
func (n *kdlNode) findChild(name string) *kdlNode {
	for i := range n.Children {
		if n.Children[i].Name == name {
			return &n.Children[i]
		}
	}
	return nil
}

func (n *kdlNode) str(name, def string) string {
	if c := n.findChild(name); c != nil && len(c.Args) > 0 {
		if s, ok := c.Args[0].(string); ok {
			return s
		}
	}
	return def
}

func (n *kdlNode) boolean(name string, def bool) bool {
	if c := n.findChild(name); c != nil && len(c.Args) > 0 {
		if b, ok := c.Args[0].(bool); ok {
			return b
		}
	}
	return def
}

func (n *kdlNode) integer(name string, def int) int {
	if c := n.findChild(name); c != nil && len(c.Args) > 0 {
		if i, ok := c.Args[0].(int64); ok {
			return int(i)
		}
	}
	return def
}

type kdlParser struct {
	lines []string
	pos   int
}

// parseKDL parses a document into a list of top-level nodes.
func parseKDL(src string) ([]kdlNode, error) {
	p := &kdlParser{lines: strings.Split(src, "\n")}
	nodes, err := p.parseNodes(0)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.lines) {
		return nil, fmt.Errorf("line %d: unexpected %q", p.pos+1, strings.TrimSpace(p.lines[p.pos]))
	}
	return nodes, nil
}

// parseNodes consumes nodes until EOF or a closing brace (left unconsumed at
// depth 0, consumed at depth > 0).
func (p *kdlParser) parseNodes(depth int) ([]kdlNode, error) {
	var nodes []kdlNode
	for p.pos < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.pos])
		if i := strings.Index(line, "//"); i >= 0 && !strings.Contains(line[:i], `"`) {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			p.pos++
			continue
		}
		if line == "}" {
			if depth == 0 {
				return nil, fmt.Errorf("line %d: unmatched }", p.pos+1)
			}
			p.pos++
			return nodes, nil
		}

		openBlock := false
		if strings.HasSuffix(line, "{") {
			openBlock = true
			line = strings.TrimSpace(strings.TrimSuffix(line, "{"))
		}
		node, err := parseNodeLine(line, p.pos+1)
		if err != nil {
			return nil, err
		}
		p.pos++
		if openBlock {
			children, err := p.parseNodes(depth + 1)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}
		nodes = append(nodes, node)
	}
	if depth > 0 {
		return nil, fmt.Errorf("unexpected end of file: missing }")
	}
	return nodes, nil
}

// parseNodeLine parses `name arg arg ...` where args are quoted strings,
// integers, or true/false.
func parseNodeLine(line string, lineno int) (kdlNode, error) {
	fields, err := splitKDLFields(line, lineno)
	if err != nil {
		return kdlNode{}, err
	}
	if len(fields) == 0 {
		return kdlNode{}, fmt.Errorf("line %d: empty node", lineno)
	}
	node := kdlNode{Name: fields[0]}
	for _, f := range fields[1:] {
		switch {
		case strings.HasPrefix(f, `"`):
			s, err := strconv.Unquote(f)
			if err != nil {
				return kdlNode{}, fmt.Errorf("line %d: bad string %s", lineno, f)
			}
			node.Args = append(node.Args, s)
		case f == "true" || f == "#true":
			node.Args = append(node.Args, true)
		case f == "false" || f == "#false":
			node.Args = append(node.Args, false)
		default:
			i, err := strconv.ParseInt(f, 10, 64)
			if err != nil {
				return kdlNode{}, fmt.Errorf("line %d: unsupported value %q", lineno, f)
			}
			node.Args = append(node.Args, i)
		}
	}
	return node, nil
}

// splitKDLFields splits on whitespace, keeping quoted strings intact.
func splitKDLFields(line string, lineno int) ([]string, error) {
	var fields []string
	var cur strings.Builder
	inString := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inString:
			cur.WriteByte(c)
			if c == '\\' && i+1 < len(line) {
				i++
				cur.WriteByte(line[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			cur.WriteByte(c)
			inString = true
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if inString {
		return nil, fmt.Errorf("line %d: unterminated string", lineno)
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields, nil
}

// kdlValue renders a scalar as KDL source.
func kdlValue(v any) string {
	switch t := v.(type) {
	case string:
		return strconv.Quote(t)
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return strconv.Quote(fmt.Sprint(t))
	}
}
