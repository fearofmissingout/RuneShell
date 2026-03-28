package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/engine"
	"cmdcards/internal/i18n"
)

func RenderShopGrouped(theme Theme, run *engine.RunState, shop engine.ShopState, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 84, 4)
	contentWidth := panelContentWidth(panelWidth)
	selected = clampSelection(selected, len(shop.Offers))
	window := listPageWindow(len(shop.Offers), selected, pagedListSize)

	lines := []string{
		theme.Title.Render(theme.Text("shop.title")),
		theme.Subtitle.Render(theme.Textf("shop.grouped.subtitle", i18n.Args{
			"gold": run.Player.Gold,
			"page": listPageSummary(theme, len(shop.Offers), selected),
		})),
		"",
	}

	lastKind := ""
	for i := window.Start; i < window.End; i++ {
		offer := shop.Offers[i]
		if offer.Kind != lastKind {
			if lastKind != "" {
				lines = append(lines, "")
			}
			lines = append(lines, theme.Accent.Render(shopKindLabel(theme, offer.Kind)))
			lastKind = offer.Kind
		}
		line := truncateASCII(fmt.Sprintf("%d. %s [%d] - %s", i+1, offer.Name, offer.Price, offer.Description), contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}

	lines = append(lines, "", theme.Muted.Render(theme.Text("shop.grouped.footer")))
	if len(shop.Log) > 0 {
		lines = append(lines, "", theme.Good.Render(strings.Join(shop.Log, " ")))
	}
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func shopKindLabel(theme Theme, kind string) string {
	switch kind {
	case "card":
		return theme.Text("shop.kind.card")
	case "relic":
		return theme.Text("shop.kind.relic")
	case "equipment":
		return theme.Text("shop.kind.equipment")
	case "potion":
		return theme.Text("shop.kind.potion")
	case "remove":
		return theme.Text("shop.kind.remove")
	case "heal":
		return theme.Text("shop.kind.heal")
	default:
		return kind
	}
}
