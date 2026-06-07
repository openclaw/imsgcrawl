package cli

import (
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"golang.org/x/sys/unix"
)

const (
	defaultTextTableWidth = 100
	minTextTableWidth     = 72
	textTableGap          = "  "
)

type textColumn struct {
	header string
	width  int
	wrap   bool
}

func textOutputWidth(w io.Writer) int {
	if width := terminalOutputWidth(w); width > 0 {
		return normalizeTextTableWidth(width)
	}
	raw := strings.TrimSpace(os.Getenv("COLUMNS"))
	if raw == "" {
		return defaultTextTableWidth
	}
	width, err := strconv.Atoi(raw)
	if err != nil {
		return defaultTextTableWidth
	}
	return normalizeTextTableWidth(width)
}

func terminalOutputWidth(w io.Writer) int {
	file, ok := w.(*os.File)
	if !ok {
		return 0
	}
	size, err := unix.IoctlGetWinsize(int(file.Fd()), unix.TIOCGWINSZ)
	if err != nil || size == nil {
		return 0
	}
	return int(size.Col)
}

func normalizeTextTableWidth(width int) int {
	if width < minTextTableWidth {
		return minTextTableWidth
	}
	return width
}

func renderTextTable(w io.Writer, columns []textColumn, rows [][]string) error {
	header := make([]string, 0, len(columns))
	for _, column := range columns {
		header = append(header, column.header)
	}
	if err := renderTextRow(w, columns, header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := renderTextRow(w, columns, row); err != nil {
			return err
		}
	}
	return nil
}

func renderTextRow(w io.Writer, columns []textColumn, row []string) error {
	cells := make([][]string, len(columns))
	height := 1
	for i, column := range columns {
		value := ""
		if i < len(row) {
			value = row[i]
		}
		if column.wrap {
			cells[i] = wrapCell(value, column.width)
		} else {
			cells[i] = []string{truncateCell(value, column.width)}
		}
		if len(cells[i]) > height {
			height = len(cells[i])
		}
	}
	for line := 0; line < height; line++ {
		for i, column := range columns {
			value := ""
			if line < len(cells[i]) {
				value = cells[i][line]
			}
			if i == len(columns)-1 {
				if _, err := io.WriteString(w, value); err != nil {
					return err
				}
			} else if _, err := io.WriteString(w, padCell(value, column.width)); err != nil {
				return err
			}
			if i < len(columns)-1 {
				if _, err := io.WriteString(w, textTableGap); err != nil {
					return err
				}
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func wrapCell(value string, width int) []string {
	value = normalizeCellText(value)
	if strings.TrimSpace(value) == "" {
		return []string{"(empty)"}
	}
	value = strings.TrimRight(value, "\n")
	out := []string{}
	for _, line := range strings.Split(value, "\n") {
		out = append(out, wrapLine(line, width)...)
	}
	return out
}

func wrapLine(line string, width int) []string {
	line = strings.ReplaceAll(line, "\t", "    ")
	if line == "" || width <= 0 {
		return []string{line}
	}
	out := []string{}
	for runewidth.StringWidth(line) > width {
		partEnd, nextStart := splitLineAtWidth(line, width)
		part := strings.TrimRightFunc(line[:partEnd], unicode.IsSpace)
		if part == "" {
			part = line[:nextStart]
		}
		out = append(out, part)
		line = strings.TrimLeftFunc(line[nextStart:], unicode.IsSpace)
		if line == "" {
			return out
		}
	}
	return append(out, line)
}

func splitLineAtWidth(line string, width int) (partEnd int, nextStart int) {
	cellWidth := 0
	lastSpaceStart := -1
	lastSpaceEnd := -1
	for index, r := range line {
		runeWidth := runewidth.RuneWidth(r)
		if unicode.IsSpace(r) {
			lastSpaceStart = index
			lastSpaceEnd = index + len(string(r))
		}
		if cellWidth+runeWidth > width {
			if lastSpaceStart > 0 {
				return lastSpaceStart, lastSpaceEnd
			}
			if index == 0 {
				end := index + len(string(r))
				return end, end
			}
			return index, index
		}
		cellWidth += runeWidth
	}
	return len(line), len(line)
}

func truncateCell(value string, width int) string {
	value = compactCellText(value)
	if value == "" {
		return "-"
	}
	if width <= 0 || runewidth.StringWidth(value) <= width {
		return value
	}
	tail := "..."
	tailWidth := runewidth.StringWidth(tail)
	if width <= tailWidth {
		return strings.Repeat(".", width)
	}
	limit := width - tailWidth
	var b strings.Builder
	cellWidth := 0
	for _, r := range value {
		runeWidth := runewidth.RuneWidth(r)
		if cellWidth+runeWidth > limit {
			break
		}
		b.WriteRune(r)
		cellWidth += runeWidth
	}
	return strings.TrimRightFunc(b.String(), unicode.IsSpace) + tail
}

func padCell(value string, width int) string {
	cellWidth := runewidth.StringWidth(value)
	if cellWidth >= width {
		return value
	}
	return value + strings.Repeat(" ", width-cellWidth)
}

func compactCellText(value string) string {
	return strings.Join(strings.Fields(normalizeCellText(value)), " ")
}

func normalizeCellText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func tableRows(count int) [][]string {
	return make([][]string, 0, count)
}

func textColumnWidth(totalWidth int, fixedColumns ...int) int {
	fixed := 0
	for _, width := range fixedColumns {
		fixed += width
	}
	gaps := len(fixedColumns) * len(textTableGap)
	width := totalWidth - fixed - gaps
	if width < 16 {
		return 16
	}
	return width
}
