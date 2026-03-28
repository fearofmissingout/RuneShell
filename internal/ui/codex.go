package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/i18n"

	"github.com/charmbracelet/lipgloss"
)

func RenderCodex(theme Theme, lib *content.Library, tab, selected, width int) string {
	totalWidth := max(30, viewportWidth(width, 100)-2)
	leftWidth, rightWidth, stacked := splitFramedAdaptiveColumns(totalWidth, totalWidth/3, 24, 24, 1, 4)
	listWidth := panelContentWidth(leftWidth)
	detailWidth := panelContentWidth(rightWidth)

	tabs := renderSimpleTabs(theme, []string{theme.Text("menu.codex"), theme.Text("codex.relics"), theme.Text("codex.equipment")}, tab)
	items, details := codexContent(theme, lib, tab)
	selected = clampSelection(selected, len(items))
	window := listPageWindow(len(items), selected, pagedListSize)

	currentTab := codexTabName(theme, tab)
	countSummary := theme.Textf("codex.count_summary", i18n.Args{
		"cards":      len(lib.CardList()),
		"relics":     len(lib.RelicList()),
		"equipments": len(lib.EquipmentList()),
	})
	currentSummary := theme.Textf("codex.current_summary", i18n.Args{
		"tab":   currentTab,
		"count": len(items),
		"pages": window.TotalPages,
	})

	listLines := []string{
		theme.Accent.Render(theme.Text("codex.list")),
		theme.Muted.Render(theme.Textf("codex.list_summary", i18n.Args{"page": listPageSummary(theme, len(items), selected)})),
		"",
	}
	for i := window.Start; i < window.End; i++ {
		line := indexedListLine(i, items[i], listWidth)
		if i == selected {
			listLines = append(listLines, theme.Selected.Render(line))
		} else {
			listLines = append(listLines, theme.Normal.Render(line))
		}
	}
	listPanel := theme.PanelAlt.Width(leftWidth).Render(strings.Join(listLines, "\n"))

	detailLines := []string{
		theme.Accent.Render(theme.Text("progression.details")),
		"",
	}
	if len(items) > 0 {
		for _, line := range wrapLine(items[selected], detailWidth) {
			detailLines = append(detailLines, theme.Subtitle.Render(line))
		}
		detailLines = append(detailLines, "")
		for _, line := range wrapDetailLines(details[selected], detailWidth) {
			detailLines = append(detailLines, line)
		}
	}
	detailPanel := theme.PanelAlt.Width(rightWidth).Render(strings.Join(detailLines, "\n"))

	header := []string{theme.Title.Render(theme.Text("menu.codex"))}
	for _, line := range wrapLine(currentSummary, totalWidth) {
		header = append(header, theme.Subtitle.Render(line))
	}
	for _, line := range wrapLine(countSummary, totalWidth) {
		header = append(header, theme.Muted.Render(line))
	}
	header = append(header, tabs, "")
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, " ", detailPanel)
	if stacked {
		body = lipgloss.JoinVertical(lipgloss.Left, listPanel, detailPanel)
	}
	return strings.Join(append(header, body), "\n")
}

func codexContent(theme Theme, lib *content.Library, tab int) ([]string, []string) {
	switch tab % 3 {
	case 0:
		return codexCards(theme, lib)
	case 1:
		return codexRelics(theme, lib)
	default:
		return codexEquipments(theme, lib)
	}
}

func codexCards(theme Theme, lib *content.Library) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, card := range lib.CardList() {
		className := theme.Text("codex.neutral")
		if card.ClassID != "neutral" {
			className = lib.Classes[card.ClassID].Name
		}
		items = append(items, theme.Textf("codex.card_item", i18n.Args{
			"name":  card.Name,
			"class": className,
			"cost":  card.Cost,
		}))

		lines := []string{
			codexThemeLine(theme, "codex.field.name", card.Name),
			codexThemeLine(theme, "codex.field.class", className),
			codexThemeLine(theme, "codex.field.rarity", card.Rarity),
			codexThemeLine(theme, "codex.field.cost", fmt.Sprintf("%d", card.Cost)),
		}
		if len(card.Tags) > 0 {
			lines = append(lines, codexThemeLine(theme, "codex.field.tags", strings.Join(card.Tags, ", ")))
		}
		lines = append(lines, "", theme.Text("codex.effects"), engine.DescribeEffects(lib, card.Effects))
		if len(card.UpgradeEffects) > 0 {
			lines = append(lines, "", theme.Text("codex.upgrade"), engine.DescribeEffects(lib, card.UpgradeEffects))
		}
		if card.Description != "" {
			lines = append(lines, "", theme.Text("codex.description"), card.Description)
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func codexRelics(theme Theme, lib *content.Library) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, relic := range lib.RelicList() {
		items = append(items, theme.Textf("codex.relic_item", i18n.Args{"name": relic.Name, "rarity": relic.Rarity}))
		lines := []string{
			codexThemeLine(theme, "codex.field.name", relic.Name),
			codexThemeLine(theme, "codex.field.rarity", relic.Rarity),
			"",
			theme.Text("codex.description"),
			relic.Description,
			"",
			theme.Text("codex.effects"),
			engine.DescribeEffects(lib, relic.Effects),
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func codexEquipments(theme Theme, lib *content.Library) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, equipment := range lib.EquipmentList() {
		items = append(items, theme.Textf("codex.equipment_item", i18n.Args{
			"name":   equipment.Name,
			"slot":   engine.EquipmentSlotName(equipment.Slot),
			"rarity": equipment.Rarity,
		}))
		lines := []string{
			codexThemeLine(theme, "codex.field.name", equipment.Name),
			codexThemeLine(theme, "codex.field.slot", engine.EquipmentSlotName(equipment.Slot)),
			codexThemeLine(theme, "codex.field.rarity", equipment.Rarity),
			"",
			theme.Text("codex.description"),
			equipment.Description,
			"",
			theme.Text("codex.effects"),
			engine.DescribeEffects(lib, equipment.Effects),
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func codexTabName(theme Theme, tab int) string {
	switch tab % 3 {
	case 0:
		return theme.Text("codex.cards")
	case 1:
		return theme.Text("codex.relics")
	default:
		return theme.Text("codex.equipment")
	}
}

func renderSimpleTabs(theme Theme, labels []string, selected int) string {
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		if i == selected%len(labels) {
			parts = append(parts, theme.Selected.Render(label))
		} else {
			parts = append(parts, theme.Chip.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func clampSelection(selected, length int) int {
	if length == 0 {
		return 0
	}
	if selected < 0 {
		return 0
	}
	if selected >= length {
		return length - 1
	}
	return selected
}

func wrapDetailLines(block string, width int) []string {
	lines := []string{}
	for _, line := range strings.Split(block, "\n") {
		lines = append(lines, wrapLine(line, width)...)
	}
	return lines
}

func codexThemeLine(theme Theme, labelKey, value string) string {
	return theme.Text(labelKey) + " | " + value
}
