package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/engine"
)

func RenderShopGrouped(theme Theme, run *engine.RunState, shop engine.ShopState, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 84, 4)
	contentWidth := panelContentWidth(panelWidth)
	selected = clampSelection(selected, len(shop.Offers))
	window := listPageWindow(len(shop.Offers), selected, pagedListSize)

	lines := []string{
		theme.Title.Render("商店"),
		theme.Subtitle.Render(fmt.Sprintf("金币 %d | %s", run.Player.Gold, listPageSummary(len(shop.Offers), selected))),
		"",
	}

	lastKind := ""
	for i := window.Start; i < window.End; i++ {
		offer := shop.Offers[i]
		if offer.Kind != lastKind {
			if lastKind != "" {
				lines = append(lines, "")
			}
			lines = append(lines, theme.Accent.Render(shopKindLabel(offer.Kind)))
			lastKind = offer.Kind
		}
		line := truncateASCII(fmt.Sprintf("%d. %s [%d] - %s", i+1, offer.Name, offer.Price, offer.Description), contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}

	lines = append(lines, "", theme.Muted.Render("每页最多显示 10 项；回车购买，l 离开"))
	if len(shop.Log) > 0 {
		lines = append(lines, "", theme.Good.Render(strings.Join(shop.Log, " ")))
	}
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func shopKindLabel(kind string) string {
	switch kind {
	case "card":
		return "卡牌"
	case "relic":
		return "遗物"
	case "equipment":
		return "装备"
	case "potion":
		return "药水"
	case "remove":
		return "卡牌移除服务"
	case "heal":
		return "补给服务"
	default:
		return kind
	}
}
