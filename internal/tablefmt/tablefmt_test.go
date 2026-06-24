package tablefmt_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/tttpeng/grove/internal/tablefmt"
)

func TestColumnsAllRowsSameWidth(t *testing.T) {
	rows := [][]string{
		{"分支", "仓库数", "描述"},
		{"feat/x", "2", "修复登录"},
		{"feature/long-branch", "10", "—"},
	}
	lines := tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Right, tablefmt.Left})
	if len(lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3", len(lines))
	}
	for _, l := range lines {
		if strings.Contains(l, "\t") {
			t.Errorf("line must not contain tab: %q", l)
		}
	}
}

func TestColumnsLeftAlignSameStart(t *testing.T) {
	rows := [][]string{
		{"feat/x", "修复"},
		{"feature/long", "对齐"},
	}
	lines := tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left})
	starts := make([]int, len(lines))
	for i, l := range lines {
		idx := strings.Index(l, rows[i][1])
		if idx < 0 {
			t.Fatalf("row %d missing second column value %q in %q", i, rows[i][1], l)
		}
		starts[i] = lipgloss.Width(l[:idx])
	}
	if starts[0] != starts[1] {
		t.Errorf("second column starts differ: %d vs %d (%q / %q)", starts[0], starts[1], lines[0], lines[1])
	}
	wantStart := lipgloss.Width("feature/long") + 1
	if starts[0] != wantStart {
		t.Errorf("second column start = %d, want %d", starts[0], wantStart)
	}
}

func TestColumnsRightAlignNumbers(t *testing.T) {
	rows := [][]string{
		{"a", "2", "x"},
		{"b", "100", "y"},
	}
	lines := tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Right, tablefmt.Left})
	if !strings.Contains(lines[0], "  2 ") {
		t.Errorf("right-aligned narrow value should be padded on the left: %q", lines[0])
	}
	if !strings.Contains(lines[1], "100 ") {
		t.Errorf("widest value sets column width: %q", lines[1])
	}
}

func TestColumnsCJKWidth(t *testing.T) {
	rows := [][]string{
		{"中文描述", "x"},
		{"ab", "y"},
	}
	lines := tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left})
	s0 := lipgloss.Width(lines[0][:strings.Index(lines[0], "x")])
	s1 := lipgloss.Width(lines[1][:strings.Index(lines[1], "y")])
	if s0 != s1 {
		t.Errorf("CJK column start differs: %d vs %d (%q / %q)", s0, s1, lines[0], lines[1])
	}
}

func TestColumnsNoTrailingPadOnLastColumn(t *testing.T) {
	rows := [][]string{
		{"a", "desc"},
		{"bb", "d"},
	}
	lines := tablefmt.Columns(rows, []tablefmt.Align{tablefmt.Left, tablefmt.Left})
	for _, l := range lines {
		if strings.HasSuffix(l, " ") {
			t.Errorf("last column should not be right-padded: %q", l)
		}
	}
}
