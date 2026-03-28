package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/i18n"

	"github.com/charmbracelet/lipgloss"
)

func RenderProgression(theme Theme, lib *content.Library, profile engine.Profile, tab, selected, width int) string {
	totalWidth := max(30, viewportWidth(width, 100)-2)
	leftWidth, rightWidth, stacked := splitFramedAdaptiveColumns(totalWidth, totalWidth/3, 24, 24, 1, 4)
	listWidth := panelContentWidth(leftWidth)
	detailWidth := panelContentWidth(rightWidth)

	tabs := renderSimpleTabs(theme, []string{
		theme.Text("progression.tab.perks"),
		theme.Text("progression.tab.equipment"),
		theme.Text("progression.tab.loadout"),
		theme.Text("progression.tab.settings"),
	}, tab)
	items, details := progressionContent(theme, lib, profile, tab)
	selected = clampSelection(selected, len(items))
	window := listPageWindow(len(items), selected, pagedListSize)

	listLines := []string{theme.Title.Render(theme.Text("progression.title"))}
	for _, line := range wrapLine(theme.Textf("progression.subtitle", i18n.Args{
		"currency": profile.MetaCurrency,
		"page":     listPageSummary(len(items), selected),
	}), listWidth) {
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

	detailLines := []string{theme.Accent.Render(theme.Text("progression.details")), ""}
	if len(details) > 0 {
		for _, line := range wrapDetailLines(details[selected], detailWidth) {
			detailLines = append(detailLines, line)
		}
	}
	detailLines = append(detailLines, "", theme.Muted.Render(theme.Text("progression.footer")))
	detailPanel := theme.PanelAlt.Width(rightWidth).Render(strings.Join(detailLines, "\n"))

	if stacked {
		return lipgloss.JoinVertical(lipgloss.Left, listPanel, detailPanel)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, " ", detailPanel)
}

func progressionContent(theme Theme, lib *content.Library, profile engine.Profile, tab int) ([]string, []string) {
	switch tab % 4 {
	case 0:
		return progressionPerkContent(theme, profile)
	case 1:
		return progressionEquipmentContent(theme, lib, profile)
	case 2:
		return progressionLoadoutContent(theme, lib, profile)
	default:
		return progressionSettingsContent(theme, profile)
	}
}

func progressionPerkContent(theme Theme, profile engine.Profile) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, perk := range engine.ProgressionPerks() {
		value := profile.Perks[perk.ID]
		level := engine.PerkLevel(perk, value)
		cost, canUpgrade := engine.NextPerkCost(profile, perk.ID)
		costText := theme.Text("progression.perk.maxed")
		if canUpgrade {
			costText = theme.Textf("progression.perk.next_cost", i18n.Args{"cost": cost})
		}
		items = append(items, theme.Textf("progression.perk.item", i18n.Args{
			"name":  perk.Name,
			"level": level,
			"max":   perk.MaxLevel,
			"value": value,
			"cost":  costText,
		}))

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
				theme.Textf("progression.perk.next_purchase", i18n.Args{"cost": cost}),
				theme.Textf("progression.perk.next_value", i18n.Args{"value": value + perk.Step}),
			)
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func progressionEquipmentContent(theme Theme, lib *content.Library, profile engine.Profile) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, equipment := range lib.EquipmentList() {
		if equipment.Rarity == "starter" {
			continue
		}
		status := theme.Text("progression.equipment.locked")
		if engine.IsEquipmentUnlocked(profile, equipment.ID) {
			status = theme.Text("progression.equipment.unlocked")
		}
		cost := engine.EquipmentUnlockCost(equipment.Rarity)
		items = append(items, theme.Textf("progression.equipment.item", i18n.Args{
			"name":   equipment.Name,
			"slot":   engine.EquipmentSlotName(equipment.Slot),
			"status": status,
			"cost":   cost,
		}))
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
			theme.Textf("progression.equipment.unlock_cost", i18n.Args{"cost": cost}),
		}
		details = append(details, strings.Join(lines, "\n"))
	}
	return items, details
}

func progressionLoadoutContent(theme Theme, lib *content.Library, profile engine.Profile) ([]string, []string) {
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
			currentName := theme.Text("progression.loadout.empty")
			if currentID != "" {
				currentName = lib.Equipments[currentID].Name
			}
			items = append(items, theme.Textf("progression.loadout.item", i18n.Args{
				"class": class.Name,
				"slot":  engine.EquipmentSlotName(slot),
				"name":  currentName,
			}))

			options := []string{}
			if slot == "accessory" {
				options = append(options, theme.Text("progression.loadout.empty"))
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
				theme.Text("progression.loadout.hint"),
			}
			details = append(details, strings.Join(lines, "\n"))
		}
	}
	return items, details
}

func progressionSettingsContent(theme Theme, profile engine.Profile) ([]string, []string) {
	items := []string{}
	details := []string{}
	for _, lang := range i18n.SupportedLanguages() {
		suffix := ""
		if profile.Language == lang {
			suffix = theme.Text("progression.settings.current")
		}
		name := theme.Text("language." + lang)
		items = append(items, theme.Textf("progression.settings.item", i18n.Args{
			"name":   name,
			"suffix": suffix,
		}))
		details = append(details, strings.Join([]string{
			themeLine(theme.Text("progression.settings.current_label"), theme.Text("language."+profile.Language)),
			themeLine(theme.Text("progression.settings.available"), name),
			"",
			theme.Text("progression.settings.hint"),
		}, "\n"))
	}
	return items, details
}
