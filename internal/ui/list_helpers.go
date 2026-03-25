package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

const pagedListSize = 10

type listPage struct {
	Start      int
	End        int
	Page       int
	TotalPages int
}

func viewportWidth(width, fallback int) int {
	if width > 0 {
		return max(32, width)
	}
	return max(32, fallback)
}

func fitPanelWidth(width, fallback, margin int) int {
	return max(24, viewportWidth(width, fallback)-margin)
}

func splitAdaptiveColumns(totalWidth, preferredLeft, minLeft, minRight, gap int) (int, int, bool) {
	if totalWidth <= 0 {
		return minLeft, minRight, false
	}
	if totalWidth < minLeft+minRight+gap {
		return totalWidth, totalWidth, true
	}
	left := preferredLeft
	if left < minLeft {
		left = minLeft
	}
	if left > totalWidth-minRight-gap {
		left = totalWidth - minRight - gap
	}
	right := totalWidth - left - gap
	if right < minRight {
		right = minRight
		left = totalWidth - right - gap
	}
	return left, right, false
}

func panelContentWidth(panelWidth int) int {
	return max(12, panelWidth-6)
}

func indexedListLine(index int, item string, width int) string {
	return truncateASCII(fmt.Sprintf("%d. %s", index+1, item), width)
}

func listPageWindow(length, selected, pageSize int) listPage {
	if pageSize <= 0 {
		pageSize = pagedListSize
	}
	if length <= 0 {
		return listPage{Page: 1, TotalPages: 1}
	}
	selected = clampSelection(selected, length)
	page := selected / pageSize
	totalPages := (length + pageSize - 1) / pageSize
	start := page * pageSize
	end := min(length, start+pageSize)
	return listPage{
		Start:      start,
		End:        end,
		Page:       page + 1,
		TotalPages: totalPages,
	}
}

func listPageSummary(length, selected int) string {
	window := listPageWindow(length, selected, pagedListSize)
	return fmt.Sprintf("第 %d/%d 页 · 共 %d 项", window.Page, window.TotalPages, length)
}

func truncateASCII(line string, width int) string {
	const ellipsis = "..."
	if width <= len(ellipsis) || lipgloss.Width(line) <= width {
		return line
	}
	runes := []rune(line)
	for len(runes) > 0 && lipgloss.Width(string(runes)+ellipsis) > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}
