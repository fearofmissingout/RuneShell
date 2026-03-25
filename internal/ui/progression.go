package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"

	"github.com/charmbracelet/lipgloss"
)

func RenderProgression(theme Theme, lib *content.Library, profile engine.Profile, tab, selected, width int) string {
	totalWidth := max(30, viewportWidth(width, 100)-2)
	leftWidth, rightWidth, stacked := splitAdaptiveColumns(totalWidth, totalWidth/3, 24, 24, 1)
	listWidth := panelContentWidth(leftWidth)
	detailWidth := panelContentWidth(rightWidth)

	tabs := renderSimpleTabs(theme, []string{"成长", "装备解锁", "初始装配"}, tab)
	items, details := progressionContent(lib, profile, tab)
	selected = clampSelection(selected, len(items))
	window := listPageWindow(len(items), selected, pagedListSize)

	listLines := []string{theme.Title.Render("局外养成")}
	for _, line := range wrapLine(fmt.Sprintf("局外货币 %d | %s | tab 切换面板", profile.MetaCurrency, listPageSummary(len(items), selected)), listWidth) {
		listLines = append(listLines, theme.Subtitle.Render(line))
	}
	listLines = append(listLines, tabs, "")
	for i := window.Start; i < window.End; i++ {
		line := indexedListLine(i, items[i], listWidth)
		if i == selected {
			listLines = append(listLines, theme.Selected.Render(line))
		} else {
			listLines = append(listLines, theme.Normal.Render(line))
		}
	}
	listPanel := theme.PanelAlt.Width(leftWidth).Render(strings.Join(listLines, "\n"))

	detailLines := []string{theme.Accent.Render("详情"), ""}
	if len(details) > 0 {
		for _, line := range wrapDetailLines(details[selected], detailWidth) {
			detailLines = append(detailLines, line)
		}
	}
	detailLines = append(detailLines, "", theme.Muted.Render("回车执行当前操作，esc 返回主菜单"))
	detailPanel := theme.PanelAlt.Width(rightWidth).Render(strings.Join(detailLines, "\n"))

	if stacked {
		return lipgloss.JoinVertical(lipgloss.Left, listPanel, detailPanel)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, " ", detailPanel)
}

func progressionContent(lib *content.Library, profile engine.Profile, tab int) ([]string, []string) {
	switch tab % 3 {
	case 0:
		return progressionPerkContent(profile)
	case 1:
		return progressionEquipmentContent(lib, profile)
	default:
		return progressionLoadoutContent(lib, profile)
	}
}

func progressionPerkContent(profile engine.Profile) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, perk := range engine.ProgressionPerks() {
		value := profile.Perks[perk.ID]
		level := engine.PerkLevel(perk, value)
		cost, canUpgrade := engine.NextPerkCost(profile, perk.ID)
		costText := "已满级"
		if canUpgrade {
			costText = fmt.Sprintf("下级消耗 %d", cost)
		}
		items = append(items, fmt.Sprintf("%s Lv.%d/%d | 当前 %d | %s", perk.Name, level, perk.MaxLevel, value, costText))

		lines := []string{
			themeLine("名称", perk.Name),
			themeLine("当前值", fmt.Sprintf("%d", value)),
			themeLine("等级", fmt.Sprintf("%d/%d", level, perk.MaxLevel)),
			"",
			perk.Description,
		}
		if canUpgrade {
			lines = append(lines,
				"",
				fmt.Sprintf("下一次购买会消耗 %d 局外货币。", cost),
				fmt.Sprintf("购买后将提升到 %d。", value+perk.Step),
			)
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func progressionEquipmentContent(lib *content.Library, profile engine.Profile) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, equipment := range lib.EquipmentList() {
		if equipment.Rarity == "starter" {
			continue
		}
		status := "未解锁"
		if engine.IsEquipmentUnlocked(profile, equipment.ID) {
			status = "已解锁"
		}
		cost := engine.EquipmentUnlockCost(equipment.Rarity)
		items = append(items, fmt.Sprintf("%s | %s | %s | 花费 %d", equipment.Name, engine.EquipmentSlotName(equipment.Slot), status, cost))
		lines := []string{
			themeLine("名称", equipment.Name),
			themeLine("槽位", engine.EquipmentSlotName(equipment.Slot)),
			themeLine("稀有度", equipment.Rarity),
			themeLine("状态", status),
			"",
			"描述",
			equipment.Description,
			"",
			"效果",
			engine.DescribeEffects(lib, equipment.Effects),
			"",
			fmt.Sprintf("解锁消耗 %d 局外货币。", cost),
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func progressionLoadoutContent(lib *content.Library, profile engine.Profile) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, class := range lib.ClassList() {
		loadout := engine.EffectiveLoadout(lib, profile, class)
		for _, slot := range []string{"weapon", "armor", "accessory"} {
			currentID := ""
			switch slot {
			case "weapon":
				currentID = loadout.Weapon
			case "armor":
				currentID = loadout.Armor
			case "accessory":
				currentID = loadout.Accessory
			}
			currentName := "空置"
			if currentID != "" {
				currentName = lib.Equipments[currentID].Name
			}
			items = append(items, fmt.Sprintf("%s | %s | %s", class.Name, engine.EquipmentSlotName(slot), currentName))

			options := []string{}
			if slot == "accessory" {
				options = append(options, "空置")
			}
			for _, equipment := range engine.UnlockedEquipmentOptions(lib, profile, slot) {
				options = append(options, equipment.Name)
			}
			lines := []string{
				themeLine("职业", class.Name),
				themeLine("槽位", engine.EquipmentSlotName(slot)),
				themeLine("当前装配", currentName),
				"",
				"可用选项",
				strings.Join(options, " / "),
				"",
				"回车循环切换到下一件已解锁装备。",
			}
			details = append(details, strings.Join(lines, "\n"))
		}
	}
	return items, details
}
