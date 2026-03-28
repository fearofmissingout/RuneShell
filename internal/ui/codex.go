package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"

	"github.com/charmbracelet/lipgloss"
)

func RenderCodex(theme Theme, lib *content.Library, tab, selected, width int) string {
	totalWidth := max(30, viewportWidth(width, 100)-2)
	leftWidth, rightWidth, stacked := splitFramedAdaptiveColumns(totalWidth, totalWidth/3, 24, 24, 1, 4)
	listWidth := panelContentWidth(leftWidth)
	detailWidth := panelContentWidth(rightWidth)

	tabs := renderSimpleTabs(theme, []string{theme.Text("menu.codex"), theme.Text("codex.relics"), theme.Text("codex.equipment")}, tab)
	items, details := codexContent(lib, tab)
	selected = clampSelection(selected, len(items))
	window := listPageWindow(len(items), selected, pagedListSize)

	currentTab := codexTabName(tab)
	countSummary := fmt.Sprintf("卡牌 %d 项 | 遗物 %d 项 | 装备 %d 项", len(lib.CardList()), len(lib.RelicList()), len(lib.EquipmentList()))
	currentSummary := fmt.Sprintf("当前分类：%s，共 %d 项 / %d 页。1/2/3 或 Tab 切换分类，←/→、PgUp/PgDn、[/] 翻页。", currentTab, len(items), window.TotalPages)

	listLines := []string{
		theme.Accent.Render("内容列表"),
		theme.Muted.Render(fmt.Sprintf("%s，长文本在左侧截断。", listPageSummary(len(items), selected))),
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
		theme.Accent.Render("详情"),
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

func codexContent(lib *content.Library, tab int) ([]string, []string) {
	switch tab % 3 {
	case 0:
		return codexCards(lib)
	case 1:
		return codexRelics(lib)
	default:
		return codexEquipments(lib)
	}
}

func codexCards(lib *content.Library) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, card := range lib.CardList() {
		className := "中立"
		if card.ClassID != "neutral" {
			className = lib.Classes[card.ClassID].Name
		}
		items = append(items, fmt.Sprintf("%s [%s | %d费]", card.Name, className, card.Cost))

		lines := []string{
			themeLine("名称", card.Name),
			themeLine("职业", className),
			themeLine("稀有度", card.Rarity),
			themeLine("费用", fmt.Sprintf("%d", card.Cost)),
		}
		if len(card.Tags) > 0 {
			lines = append(lines, themeLine("标签", strings.Join(card.Tags, ", ")))
		}
		lines = append(lines, "", "效果", engine.DescribeEffects(lib, card.Effects))
		if len(card.UpgradeEffects) > 0 {
			lines = append(lines, "", "升级后", engine.DescribeEffects(lib, card.UpgradeEffects))
		}
		if card.Description != "" {
			lines = append(lines, "", "说明", card.Description)
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func codexRelics(lib *content.Library) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, relic := range lib.RelicList() {
		items = append(items, fmt.Sprintf("%s [%s]", relic.Name, relic.Rarity))
		lines := []string{
			themeLine("名称", relic.Name),
			themeLine("稀有度", relic.Rarity),
			"",
			"描述",
			relic.Description,
			"",
			"效果",
			engine.DescribeEffects(lib, relic.Effects),
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func codexEquipments(lib *content.Library) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, equipment := range lib.EquipmentList() {
		items = append(items, fmt.Sprintf("%s [%s | %s]", equipment.Name, engine.EquipmentSlotName(equipment.Slot), equipment.Rarity))
		lines := []string{
			themeLine("名称", equipment.Name),
			themeLine("槽位", engine.EquipmentSlotName(equipment.Slot)),
			themeLine("稀有度", equipment.Rarity),
			"",
			"描述",
			equipment.Description,
			"",
			"效果",
			engine.DescribeEffects(lib, equipment.Effects),
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func codexTabName(tab int) string {
	switch tab % 3 {
	case 0:
		return "卡牌"
	case 1:
		return "遗物"
	default:
		return "装备"
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

func themeLine(label, value string) string {
	return label + " | " + value
}
