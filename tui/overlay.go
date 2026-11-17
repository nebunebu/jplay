package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR sequences so a backdrop can be uniformly dimmed.
func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// placeOverlayDim splices modal over a plain-text (ANSI-free) backdrop,
// centered in width×height. Backdrop fragments are wrapped in the given dim
// style so the board reads as ~15% opacity behind the modal, per the design
// doc.
func placeOverlayDim(backdrop, modal string, width, height int, dim lipgloss.Style) string {
	bgLines := strings.Split(backdrop, "\n")
	modalLines := strings.Split(modal, "\n")
	mw := lipgloss.Width(modal)
	x := (width - mw) / 2
	if x < 0 {
		x = 0
	}
	y := (height - len(modalLines)) / 2
	if y < 0 {
		y = 0
	}

	rows := len(bgLines)
	if y+len(modalLines) > rows {
		rows = y + len(modalLines)
	}
	out := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		bg := ""
		if i < len(bgLines) {
			bg = bgLines[i]
		}
		if i < y || i >= y+len(modalLines) {
			out = append(out, dim.Render(bg))
			continue
		}
		ml := modalLines[i-y]
		mlw := lipgloss.Width(ml)
		// Pad the backdrop line so left/right fragments line up.
		if lipgloss.Width(bg) < x+mlw {
			bg += strings.Repeat(" ", x+mlw-lipgloss.Width(bg))
		}
		left := ansi.Truncate(bg, x, "")
		right := ansi.TruncateLeft(bg, x+mlw, "")
		out = append(out, dim.Render(left)+ml+dim.Render(right))
	}
	return strings.Join(out, "\n")
}
