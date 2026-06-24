package tablefmt

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Align int

const (
	Left Align = iota
	Right
)

func Columns(rows [][]string, aligns []Align) []string {
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	widths := make([]int, cols)
	for _, r := range rows {
		for i, cell := range r {
			if w := lipgloss.Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		var b strings.Builder
		last := len(r) - 1
		for i, cell := range r {
			if i > 0 {
				b.WriteByte(' ')
			}
			pad := widths[i] - lipgloss.Width(cell)
			if pad < 0 {
				pad = 0
			}
			switch {
			case alignOf(aligns, i) == Right:
				b.WriteString(strings.Repeat(" ", pad))
				b.WriteString(cell)
			case i == last:
				b.WriteString(cell)
			default:
				b.WriteString(cell)
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}

func alignOf(aligns []Align, i int) Align {
	if i < len(aligns) {
		return aligns[i]
	}
	return Left
}
