package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"

	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Panel    lipgloss.Style
	PanelAlt lipgloss.Style
	Selected lipgloss.Style
	Normal   lipgloss.Style
	Muted    lipgloss.Style
	Good     lipgloss.Style
	Bad      lipgloss.Style
	Accent   lipgloss.Style
	Chip     lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		Title:    lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Bold(true),
		Subtitle: lipgloss.NewStyle().Foreground(lipgloss.Color("189")).Bold(true),
		Panel:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("67")).Padding(1, 2),
		PanelAlt: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("60")).Padding(1, 1),
		Selected: lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("221")).Bold(true),
		Normal:   lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
		Muted:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Good:     lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Bold(true),
		Bad:      lipgloss.NewStyle().Foreground(lipgloss.Color("217")).Bold(true),
		Accent:   lipgloss.NewStyle().Foreground(lipgloss.Color("122")).Bold(true),
		Chip:     lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("24")).Padding(0, 1),
	}
}

func RenderChoiceScreen(theme Theme, title, subtitle string, items []string, selected int, footer string, width int) string {
	panelWidth := fitPanelWidth(width, 72, 4)
	contentWidth := panelContentWidth(panelWidth)
	selected = clampSelection(selected, len(items))
	window := listPageWindow(len(items), selected, pagedListSize)

	lines := []string{theme.Title.Render(title)}
	if subtitle != "" {
		lines = append(lines, theme.Subtitle.Render(subtitle))
	}
	if len(items) > pagedListSize {
		lines = append(lines, theme.Muted.Render(listPageSummary(len(items), selected)))
	}
	lines = append(lines, "")
	for i := window.Start; i < window.End; i++ {
		line := indexedListLine(i, items[i], contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render(footer))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderMap(theme Theme, run *engine.RunState, nodes []engine.Node, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 72, 4)
	lines := []string{
		theme.Title.Render(fmt.Sprintf("第 %d 幕地图", run.Act)),
		theme.Subtitle.Render(fmt.Sprintf("生命 %d/%d  金币 %d  牌组 %d  药水 %d/%d", run.Player.HP, run.Player.MaxHP, run.Player.Gold, len(run.Player.Deck), len(run.Player.Potions), run.Player.PotionCapacity)),
		"",
	}
	for i, node := range nodes {
		line := fmt.Sprintf("%d. %s", i+1, describeNode(node))
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render("方向键或 hjkl 选择，回车进入，q 退出"))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderCombat(theme Theme, lib *content.Library, run *engine.RunState, combat *engine.CombatState, selected int, pane int, target engine.CombatTarget, width int, height int) string {
	viewport := viewportWidth(width, 100)
	totalWidth := max(30, viewport-2)
	contentWidth := panelContentWidth(totalWidth)
	preferredLeft := totalWidth * 2 / 5
	topLeftWidth, topRightWidth, stackedTop := splitAdaptiveColumns(totalWidth, preferredLeft, 28, 28, 1)
	compact := totalWidth < 84

	focusEnemy := combat.Enemy
	if target.Kind == engine.CombatTargetEnemy && target.Index >= 0 && target.Index < len(combat.Enemies) {
		focusEnemy = combat.Enemies[target.Index]
	}

	enemyLines := []string{
		theme.Accent.Render("敌方信息"),
		theme.Title.Render(focusEnemy.Name),
		theme.Normal.Render(fmt.Sprintf("HP %d/%d  Block %d", focusEnemy.HP, focusEnemy.MaxHP, focusEnemy.Block)),
	}
	if status := engine.DescribeStatuses(focusEnemy.Statuses); status != "" {
		enemyLines = append(enemyLines, theme.Normal.Render("状态: "+status))
	} else {
		enemyLines = append(enemyLines, theme.Muted.Render("状态: 无"))
	}
	enemyLines = append(enemyLines, "", theme.Subtitle.Render("当前意图"))
	for _, line := range wrapLine(engine.DescribeIntent(focusEnemy.CurrentIntent), panelContentWidth(topLeftWidth)) {
		enemyLines = append(enemyLines, theme.Normal.Render(line))
	}
	enemyPanel := theme.PanelAlt.Width(topLeftWidth).Render(strings.Join(enemyLines, "\n"))

	infoLines := []string{
		theme.Accent.Render("战斗信息"),
		theme.Subtitle.Render(fmt.Sprintf("回合 %d | 手牌 %d | 能量 %d/%d", combat.Turn, len(combat.Hand), combat.Player.Energy, combat.Player.MaxEnergy)),
		theme.Subtitle.Render(fmt.Sprintf("抽牌堆 %d | 弃牌堆 %d | 消耗堆 %d", len(combat.DrawPile), len(combat.Discard), len(combat.Exhaust))),
		theme.Normal.Render(engine.DescribeCombatTarget(combat, target)),
		renderCombatTabs(theme, pane),
		"",
	}
	for _, line := range engine.CombatInspectLines(lib, run, combat, pane) {
		for _, wrapped := range wrapLine(line, panelContentWidth(topRightWidth)) {
			infoLines = append(infoLines, theme.Normal.Render(wrapped))
		}
	}
	infoPanel := theme.PanelAlt.Width(topRightWidth).Render(strings.Join(infoLines, "\n"))
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, enemyPanel, " ", infoPanel)
	if stackedTop {
		topRow = lipgloss.JoinVertical(lipgloss.Left, enemyPanel, infoPanel)
	}

	friendRow := renderActorStrip(theme, "友方战线", engine.PartyMembersView(combat), target, true, totalWidth)
	enemyRow := renderEnemyStrip(theme, "敌方战线", combat.Enemies, target, totalWidth)

	handLines := []string{
		theme.Accent.Render("手牌"),
		theme.Subtitle.Render("上下选牌，左右切目标，回车打牌，e 结束回合，z 使用首瓶药水，tab 切右侧信息面板"),
		"",
	}
	if len(combat.Hand) == 0 {
		handLines = append(handLines, theme.Muted.Render("当前没有可打出的手牌。"))
	} else {
		for i, card := range combat.Hand {
			label := fmt.Sprintf("%d. [%d] %s", i+1, lib.Cards[card.ID].Cost, engine.CardStateName(lib, card.ID, card.Upgraded))
			desc := engine.CardStateSummary(lib, card.ID, card.Upgraded)
			targetHint := targetHintText(lib, card)
			line := truncateASCII(label+" | "+desc+" | "+targetHint, contentWidth)
			if i == selected {
				handLines = append(handLines, theme.Selected.Render(line))
			} else {
				handLines = append(handLines, theme.Normal.Render(line))
			}
		}
	}
	handPanel := theme.Panel.Width(totalWidth).Render(strings.Join(handLines, "\n"))

	logLines := []string{theme.Accent.Render("战斗日志")}
	if len(combat.Log) == 0 {
		logLines = append(logLines, theme.Muted.Render("暂无日志。"))
	} else {
		visibleLogs := 8
		if height > 0 {
			visibleLogs = min(8, max(3, height/7))
		}
		if compact {
			visibleLogs = min(visibleLogs, 5)
		}
		start := max(0, len(combat.Log)-visibleLogs)
		for _, entry := range combat.Log[start:] {
			for _, wrapped := range wrapLine(fmt.Sprintf("T%d %s", entry.Turn, entry.Text), contentWidth) {
				logLines = append(logLines, theme.Muted.Render(wrapped))
			}
		}
	}
	logPanel := theme.Panel.Width(totalWidth).Render(strings.Join(logLines, "\n"))

	header := []string{
		theme.Title.Render("战斗"),
		theme.Subtitle.Render(fmt.Sprintf("队伍 %d 人 | 敌方 %d 体", 1+len(combat.Allies), len(combat.Enemies))),
		"",
	}
	return strings.Join(append(header, topRow, "", friendRow, "", enemyRow, "", handPanel, "", logPanel), "\n")
}

func RenderReward(theme Theme, lib *content.Library, reward engine.RewardState, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 76, 4)
	contentWidth := panelContentWidth(panelWidth)
	lines := []string{
		theme.Title.Render("战利品"),
		theme.Subtitle.Render(fmt.Sprintf("金币 +%d", reward.Gold)),
		"",
	}
	if reward.RelicID != "" {
		lines = append(lines, theme.Good.Render("遗物: "+lib.Relics[reward.RelicID].Name))
	}
	if reward.EquipmentID != "" {
		lines = append(lines, theme.Good.Render("装备: "+lib.Equipments[reward.EquipmentID].Name+" (选牌后比较)"))
	}
	if reward.PotionID != "" {
		lines = append(lines, theme.Good.Render("药水: "+lib.Potions[reward.PotionID].Name))
	}
	lines = append(lines, "", theme.Accent.Render("选一张卡加入牌组"))
	for i, card := range reward.CardChoices {
		line := truncateASCII(fmt.Sprintf("%d. %s | %s", i+1, card.Name, engine.DescribeEffects(lib, card.Effects)), contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render("回车确认，s 跳过"))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderEquipment(theme Theme, lib *content.Library, offer engine.EquipmentOfferState, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 72, 4)
	contentWidth := panelContentWidth(panelWidth)
	candidate := lib.Equipments[offer.EquipmentID]
	lines := []string{
		theme.Title.Render("装备对比"),
		theme.Subtitle.Render(fmt.Sprintf("%s | %s", equipmentSourceLabel(offer.Source, offer.Price), engine.EquipmentSlotName(offer.Slot))),
		"",
		theme.Good.Render("新装备: " + candidate.Name + " [" + equipmentRarityLabel(candidate.Rarity) + "]"),
		theme.Muted.Render(fmt.Sprintf("估值 %d", offer.CandidateScore)),
		"",
	}
	for _, line := range wrapLine(candidate.Description, contentWidth) {
		lines = append(lines, theme.Normal.Render(line))
	}
	lines = append(lines, "")

	if offer.CurrentEquipmentID != "" {
		current := lib.Equipments[offer.CurrentEquipmentID]
		lines = append(lines, theme.Normal.Render("当前装备: "+current.Name+" ["+equipmentRarityLabel(current.Rarity)+"]"))
		for _, line := range wrapLine(current.Description, contentWidth) {
			lines = append(lines, theme.Normal.Render(line))
		}
		lines = append(lines, theme.Muted.Render(fmt.Sprintf("估值 %d", offer.CurrentScore)))
		delta := offer.CandidateScore - offer.CurrentScore
		if delta >= 0 {
			lines = append(lines, "", theme.Good.Render(fmt.Sprintf("预估提升 %+d", delta)))
		} else {
			lines = append(lines, "", theme.Bad.Render(fmt.Sprintf("预估下降 %d", delta)))
		}
	} else {
		lines = append(lines, theme.Muted.Render("当前槽位为空"))
	}

	options := equipmentOptions(offer)
	lines = append(lines, "")
	for i, option := range options {
		line := fmt.Sprintf("%d. %s", i+1, option)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}

	lines = append(lines, "", theme.Muted.Render("回车确认，esc 返回"))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderDeckAction(theme Theme, lib *content.Library, run *engine.RunState, title, subtitle string, indexes []int, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 84, 4)
	contentWidth := panelContentWidth(panelWidth)
	selected = clampSelection(selected, len(indexes))
	window := listPageWindow(len(indexes), selected, pagedListSize)

	lines := []string{theme.Title.Render(title)}
	if subtitle != "" {
		lines = append(lines, theme.Subtitle.Render(subtitle))
	}
	if len(indexes) > pagedListSize {
		lines = append(lines, theme.Muted.Render(listPageSummary(len(indexes), selected)))
	}
	lines = append(lines, "")
	if len(indexes) == 0 {
		lines = append(lines, theme.Muted.Render("当前没有可操作的卡牌。"))
	} else {
		for i := window.Start; i < window.End; i++ {
			deckIndex := indexes[i]
			card := run.Player.Deck[deckIndex]
			base := truncateASCII(fmt.Sprintf("%d. %s | %s", i+1, engine.CardStateName(lib, card.CardID, card.Upgraded), engine.CardStateSummary(lib, card.CardID, card.Upgraded)), contentWidth)
			if i == selected {
				lines = append(lines, theme.Selected.Render(base))
			} else {
				lines = append(lines, theme.Normal.Render(base))
			}
			def := lib.Cards[card.CardID]
			if !card.Upgraded && len(def.UpgradeEffects) > 0 {
				lines = append(lines, theme.Muted.Render(truncateASCII("    升级后: "+engine.DescribeEffects(lib, def.UpgradeEffects), contentWidth)))
			}
		}
	}
	lines = append(lines, "", theme.Muted.Render("回车确认，esc 返回"))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderEvent(theme Theme, state engine.EventState, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 72, 4)
	contentWidth := panelContentWidth(panelWidth)
	lines := []string{
		theme.Title.Render(state.Event.Name),
		"",
	}
	for _, line := range wrapLine(state.Event.Description, contentWidth) {
		lines = append(lines, theme.Subtitle.Render(line))
	}
	lines = append(lines, "")
	for i, choice := range state.Event.Choices {
		line := truncateASCII(fmt.Sprintf("%d. %s - %s", i+1, choice.Label, choice.Description), contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if len(state.Log) > 0 {
		lines = append(lines, "", theme.Good.Render(strings.Join(state.Log, " ")))
	}
	lines = append(lines, "", theme.Muted.Render("回车确认"))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderShop(theme Theme, run *engine.RunState, shop engine.ShopState, selected int, width int) string {
	lines := []string{
		theme.Title.Render("商店"),
		theme.Subtitle.Render(fmt.Sprintf("金币 %d", run.Player.Gold)),
		"",
	}
	for i, offer := range shop.Offers {
		line := fmt.Sprintf("%d. %s [%d] - %s", i+1, offer.Name, offer.Price, offer.Description)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render("回车购买或进入服务，l 离开"))
	if len(shop.Log) > 0 {
		lines = append(lines, "", theme.Good.Render(strings.Join(shop.Log, " ")))
	}
	return theme.Panel.Width(fitPanelWidth(width, 76, 4)).Render(strings.Join(lines, "\n"))
}

func RenderRest(theme Theme, run *engine.RunState, selected int, log []string, width int) string {
	options := []string{
		"休息: 回复约 33% 最大生命",
		"锻造: 选择一张可升级卡牌",
	}
	lines := []string{
		theme.Title.Render("篝火"),
		theme.Subtitle.Render(fmt.Sprintf("生命 %d/%d", run.Player.HP, run.Player.MaxHP)),
		"",
	}
	for i, option := range options {
		line := fmt.Sprintf("%d. %s", i+1, option)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if len(log) > 0 {
		lines = append(lines, "", theme.Good.Render(strings.Join(log, " ")))
	}
	lines = append(lines, "", theme.Muted.Render("回车确认"))
	return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(strings.Join(lines, "\n"))
}

func RenderProfile(theme Theme, profile engine.Profile, width int) string {
	lines := []string{
		theme.Title.Render("档案"),
		theme.Subtitle.Render(fmt.Sprintf("局外货币 %d", profile.MetaCurrency)),
		"",
		theme.Normal.Render("已解锁职业: " + strings.Join(profile.UnlockedClasses, ", ")),
		theme.Normal.Render(fmt.Sprintf("永久加成: 生命 +%d, 金币 +%d, 额外药水位 +%d", profile.Perks["bonus_max_hp"], profile.Perks["bonus_start_gold"], profile.Perks["extra_potion_slot"])),
		"",
		theme.Muted.Render("任意键返回"),
	}
	return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(strings.Join(lines, "\n"))
}

func RenderSummary(theme Theme, run *engine.RunState, width int) string {
	title := "冒险结束"
	statusStyle := theme.Good
	if run.Status == engine.RunStatusLost {
		statusStyle = theme.Bad
		title = "冒险失败"
	}
	lines := []string{
		theme.Title.Render(title),
		statusStyle.Render(fmt.Sprintf("结果: %s", run.Status)),
		theme.Normal.Render(fmt.Sprintf("到达第 %d 幕，清理层数 %d", run.Act, run.Stats.ClearedFloors)),
		theme.Normal.Render(fmt.Sprintf("战斗胜利 %d，精英 %d，Boss %d", run.Stats.CombatsWon, run.Stats.ElitesWon, run.Stats.BossesWon)),
		theme.Normal.Render(fmt.Sprintf("最终生命 %d/%d，金币 %d，牌组 %d", run.Player.HP, run.Player.MaxHP, run.Player.Gold, len(run.Player.Deck))),
		"",
		theme.Muted.Render("按 q 退出"),
	}
	return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(strings.Join(lines, "\n"))
}

func renderActorStrip(theme Theme, title string, actors []engine.CombatActor, target engine.CombatTarget, friendly bool, width int) string {
	if len(actors) == 0 {
		return theme.PanelAlt.Width(width).Render(theme.Muted.Render(title + "为空"))
	}
	contentWidth := panelContentWidth(width)
	cols := adaptiveStripColumns(contentWidth, len(actors))
	cellWidth := adaptiveStripCellWidth(contentWidth, cols)
	parts := make([]string, 0, len(actors))
	for i, actor := range actors {
		selected := friendly && target.Kind == engine.CombatTargetAlly && target.Index == i
		lines := []string{}
		lines = append(lines, wrapLine(fmt.Sprintf("P%d %s", i+1, actor.Name), cellWidth-2)...)
		lines = append(lines, fmt.Sprintf("%d/%d HP", actor.HP, actor.MaxHP))
		lines = append(lines, fmt.Sprintf("能量 %d/%d", actor.Energy, actor.MaxEnergy))
		lines = append(lines, fmt.Sprintf("格挡 %d", actor.Block))
		if status := engine.DescribeStatuses(actor.Statuses); status != "" {
			lines = append(lines, wrapLine(status, cellWidth-2)...)
		}
		content := strings.Join(lines, "\n")
		style := theme.PanelAlt.Width(cellWidth)
		if selected {
			style = style.BorderForeground(lipgloss.Color("221"))
		}
		parts = append(parts, style.Render(content))
	}
	body := joinStripRows(parts, cols)
	return theme.PanelAlt.Width(width).Render(theme.Accent.Render(title) + "\n" + body)
}

func renderEnemyStrip(theme Theme, title string, enemies []engine.CombatEnemy, target engine.CombatTarget, width int) string {
	if len(enemies) == 0 {
		return theme.PanelAlt.Width(width).Render(theme.Muted.Render(title + "为空"))
	}
	contentWidth := panelContentWidth(width)
	cols := adaptiveStripColumns(contentWidth, len(enemies))
	cellWidth := adaptiveStripCellWidth(contentWidth, cols)
	parts := make([]string, 0, len(enemies))
	for i, enemy := range enemies {
		selected := target.Kind == engine.CombatTargetEnemy && target.Index == i
		lines := []string{}
		lines = append(lines, wrapLine(fmt.Sprintf("M%d %s", i+1, enemy.Name), cellWidth-2)...)
		lines = append(lines, fmt.Sprintf("%d/%d HP", enemy.HP, enemy.MaxHP))
		lines = append(lines, fmt.Sprintf("格挡 %d", enemy.Block))
		if status := engine.DescribeStatuses(enemy.Statuses); status != "" {
			lines = append(lines, wrapLine(status, cellWidth-2)...)
		}
		content := strings.Join(lines, "\n")
		style := theme.PanelAlt.Width(cellWidth)
		if selected {
			style = style.BorderForeground(lipgloss.Color("221"))
		}
		parts = append(parts, style.Render(content))
	}
	body := joinStripRows(parts, cols)
	return theme.PanelAlt.Width(width).Render(theme.Accent.Render(title) + "\n" + body)
}

func adaptiveStripColumns(contentWidth, itemCount int) int {
	if itemCount <= 1 {
		return 1
	}
	switch {
	case contentWidth < 40:
		return 1
	case contentWidth < 72:
		return min(2, itemCount)
	case contentWidth < 108:
		return min(3, itemCount)
	default:
		return min(4, itemCount)
	}
}

func adaptiveStripCellWidth(contentWidth, cols int) int {
	if cols <= 1 {
		return max(18, contentWidth)
	}
	return max(18, (contentWidth-(cols-1))/cols)
}

func joinStripRows(parts []string, cols int) string {
	if len(parts) == 0 {
		return ""
	}
	if cols <= 1 {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	rows := []string{}
	for start := 0; start < len(parts); start += cols {
		end := min(len(parts), start+cols)
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, parts[start:end]...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func renderCombatTabs(theme Theme, selected int) string {
	parts := make([]string, 0, engine.CombatInspectPaneCount)
	for i := 0; i < engine.CombatInspectPaneCount; i++ {
		label := fmt.Sprintf("%d.%s", i+1, engine.CombatInspectPaneName(i))
		if i == selected%engine.CombatInspectPaneCount {
			parts = append(parts, theme.Selected.Render(label))
		} else {
			parts = append(parts, theme.Chip.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func targetHintText(lib *content.Library, card engine.RuntimeCard) string {
	switch engine.CardTargetKindForCard(lib, card) {
	case engine.CombatTargetEnemy:
		return "单体敌方"
	case engine.CombatTargetEnemies:
		return "全体敌方"
	case engine.CombatTargetAlly:
		return "单体友方"
	case engine.CombatTargetAllies:
		return "全体友方"
	default:
		return "无需选目标"
	}
}

func equipmentOptions(offer engine.EquipmentOfferState) []string {
	if offer.Source == "shop" && offer.CurrentEquipmentID != "" {
		return []string{"购买并替换当前装备", "保留当前，取消购买"}
	}
	if offer.Source == "shop" {
		return []string{"购买并装备", "取消购买"}
	}
	if offer.CurrentEquipmentID != "" {
		return []string{"装备新装备并替换当前装备", "保留当前装备"}
	}
	return []string{"装备新装备", "跳过"}
}

func equipmentSourceLabel(source string, price int) string {
	switch source {
	case "shop":
		return fmt.Sprintf("商店 | %d 金币", price)
	case "reward":
		return "战利品"
	case "event":
		return "事件"
	default:
		return source
	}
}

func equipmentRarityLabel(rarity string) string {
	switch rarity {
	case "starter":
		return "初始"
	case "common":
		return "普通"
	case "uncommon":
		return "优秀"
	case "rare":
		return "稀有"
	case "legendary":
		return "传奇"
	default:
		return strings.ToUpper(rarity)
	}
}

func describeNode(node engine.Node) string {
	return fmt.Sprintf("A%d F%d %s", node.Act, node.Floor, engine.NodeKindName(node.Kind))
}

func wrapLine(line string, width int) []string {
	if width <= 0 || lipgloss.Width(line) <= width {
		return []string{line}
	}
	words := strings.Fields(line)
	if len(words) <= 1 {
		return []string{truncate(line, width)}
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		next := word
		if current != "" {
			next = current + " " + word
		}
		if lipgloss.Width(next) > width && current != "" {
			lines = append(lines, current)
			current = word
			continue
		}
		current = next
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func truncate(line string, width int) string {
	if width <= 1 || lipgloss.Width(line) <= width {
		return line
	}
	runes := []rune(line)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"...") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
