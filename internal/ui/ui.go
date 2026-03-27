package ui

import (
	"fmt"
	"sort"
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
	ClassA   lipgloss.Style
	ClassB   lipgloss.Style
	ClassN   lipgloss.Style
	Attack   lipgloss.Style
	Skill    lipgloss.Style
	Ability  lipgloss.Style
	Enemy    lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		Title:    lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Bold(true),
		Subtitle: lipgloss.NewStyle().Foreground(lipgloss.Color("189")).Bold(true),
		Panel:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("74")).Padding(1, 2),
		PanelAlt: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("237")).Padding(1, 1),
		Selected: lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("221")).Bold(true),
		Normal:   lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
		Muted:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Good:     lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Bold(true),
		Bad:      lipgloss.NewStyle().Foreground(lipgloss.Color("217")).Bold(true),
		Accent:   lipgloss.NewStyle().Foreground(lipgloss.Color("122")).Bold(true),
		Chip:     lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("238")).Padding(0, 1),
		ClassA:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		ClassB:   lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
		ClassN:   lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Bold(true),
		Attack:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		Skill:    lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
		Ability:  lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Bold(true),
		Enemy:    lipgloss.NewStyle().Foreground(lipgloss.Color("209")).Bold(true),
	}
}

func styleForClass(theme Theme, classID string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(classID)) {
	case "vanguard":
		return theme.ClassA
	case "arcanist":
		return theme.ClassB
	default:
		return theme.ClassN
	}
}

func styleForCardKind(theme Theme, kind string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "attack":
		return theme.Attack
	case "skill":
		return theme.Skill
	default:
		return theme.Ability
	}
}

func styleForNodeKind(theme Theme, kind engine.NodeKind) lipgloss.Style {
	switch kind {
	case engine.NodeElite, engine.NodeBoss:
		return theme.Bad
	case engine.NodeShop:
		return theme.Skill
	case engine.NodeEvent:
		return theme.Accent
	case engine.NodeRest:
		return theme.ClassN
	default:
		return theme.Good
	}
}

func styleForNodeKindLabel(theme Theme, kind string) lipgloss.Style {
	switch strings.TrimSpace(kind) {
	case "Boss", "绮捐嫳":
		return theme.Bad
	case "鍟嗗簵":
		return theme.Skill
	case "浜嬩欢":
		return theme.Accent
	case "绡濈伀":
		return theme.ClassN
	case "鎬墿":
		return theme.Good
	default:
		return theme.Muted
	}
}

func primaryCardKind(tags []string) string {
	for _, tag := range tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "attack":
			return "attack"
		case "skill":
			return "skill"
		case "power", "ability", "spell":
			return "ability"
		}
	}
	return "ability"
}

func styledClassName(theme Theme, name string, classID string) string {
	if strings.TrimSpace(name) == "" {
		return name
	}
	return styleForClass(theme, classID).Render(name)
}

func styledEnemyName(theme Theme, name string) string {
	if strings.TrimSpace(name) == "" {
		return name
	}
	return theme.Enemy.Render(name)
}

func styledCardName(theme Theme, name string, kind string) string {
	if strings.TrimSpace(name) == "" {
		return name
	}
	return styleForCardKind(theme, kind).Render(name)
}

func cardKindLabel(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "attack":
		return "ATK"
	case "skill":
		return "SKL"
	default:
		return "ABL"
	}
}

func renderCardKindChip(theme Theme, kind string) string {
	return styleForCardKind(theme, kind).Render("[" + cardKindLabel(kind) + "]")
}

type styledToken struct {
	token  string
	styled string
}

func renderInlineChips(theme Theme, values []string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, theme.Chip.Render(value))
	}
	return strings.Join(parts, " ")
}

func renderBadgeChips(theme Theme, badges []string) string {
	return renderInlineChips(theme, badges)
}

func styleLogTurnPrefix(theme Theme, text string) string {
	prefix, rest, ok := strings.Cut(text, " ")
	if !ok {
		return text
	}
	if len(prefix) < 2 || prefix[0] != 'T' {
		return text
	}
	return theme.Accent.Render(prefix) + " " + rest
}

func applyStyledTokens(text string, tokens []styledToken) string {
	if strings.TrimSpace(text) == "" || len(tokens) == 0 {
		return text
	}
	sorted := append([]styledToken{}, tokens...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return len([]rune(sorted[i].token)) > len([]rune(sorted[j].token))
	})
	out := text
	for _, token := range sorted {
		if strings.TrimSpace(token.token) == "" || token.styled == "" {
			continue
		}
		out = strings.ReplaceAll(out, token.token, token.styled)
	}
	return out
}

func singlePlayerCombatLogTokens(theme Theme, lib *content.Library, run *engine.RunState, combat *engine.CombatState) []styledToken {
	tokens := []styledToken{}
	seen := map[string]struct{}{}
	addToken := func(raw string, styled string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		tokens = append(tokens, styledToken{token: raw, styled: styled})
	}
	if run != nil {
		addToken(run.Player.Name, styledClassName(theme, run.Player.Name, run.Player.ClassID))
	}
	if combat != nil {
		for idx, actor := range engine.PartyMembersView(combat) {
			classID := ""
			if idx < len(combat.SeatPlayers) {
				classID = combat.SeatPlayers[idx].ClassID
			}
			addToken(actor.Name, styledClassName(theme, actor.Name, classID))
		}
		for _, enemy := range combat.Enemies {
			addToken(enemy.Name, styledEnemyName(theme, enemy.Name))
		}
	}
	if lib != nil {
		for _, card := range lib.CardList() {
			addToken(card.Name, styledCardName(theme, card.Name, primaryCardKind(card.Tags)))
		}
	}
	return tokens
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
		line := fmt.Sprintf("%d. %s", i+1, styleForNodeKind(theme, node.Kind).Render(describeNode(node)))
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render("方向键或 hjkl 选择，回车进入，q 退出"))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderMapTreeOverlay(theme Theme, run *engine.RunState, currentNode engine.Node, width int, height int) string {
	if run == nil {
		return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(theme.Muted.Render("当前没有进行中的地图。"))
	}
	layers := make([][]mapTreeNodeView, 0, len(run.Map.Floors))
	for _, floor := range run.Map.Floors {
		layer := make([]mapTreeNodeView, 0, len(floor))
		for _, node := range floor {
			layer = append(layer, mapTreeNodeView{ID: node.ID, Floor: node.Floor, Index: node.Index, Kind: engine.NodeKindName(node.Kind), Edges: append([]string{}, node.Edges...)})
		}
		layers = append(layers, layer)
	}
	return renderMapTreeOverlayPanel(theme, fmt.Sprintf("地图总览 Act %d", run.Act), mapOverlayPosition(currentNode, run.CurrentFloor), run.Reachable, layers, currentNode.ID, run.CurrentFloor, width, height)
}

func RenderStatsOverlay(theme Theme, title string, combatLines []string, runLines []string, width int, height int) string {
	panelWidth := max(48, viewportWidth(width, 92)-2)
	contentWidth := panelContentWidth(panelWidth)
	lines := []string{theme.Title.Render(title)}
	lines = append(lines, theme.Subtitle.Render("K / Esc closes"))
	lines = append(lines, "")
	lines = append(lines, theme.Accent.Render("Current Combat"))
	if len(combatLines) == 0 {
		lines = append(lines, theme.Muted.Render("No combat is active right now."))
	} else {
		for _, item := range combatLines {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	lines = append(lines, "")
	lines = append(lines, theme.Accent.Render("Whole Run"))
	if len(runLines) == 0 {
		lines = append(lines, theme.Muted.Render("No statistics are available yet."))
	} else {
		for _, item := range runLines {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	if height > 0 {
		lines = fixedPanelLines(lines, max(12, height-6), theme, contentWidth)
	}
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

type mapTreeNodeView struct {
	ID    string
	Floor int
	Index int
	Kind  string
	Edges []string
}

func renderMapTreeOverlayPanel(theme Theme, title string, position string, reachableIDs []string, layers [][]mapTreeNodeView, currentNodeID string, currentFloor int, width int, height int) string {
	panelWidth := max(48, viewportWidth(width, 116)-2)
	contentWidth := panelContentWidth(panelWidth)
	reachableSet := map[string]struct{}{}
	for _, id := range reachableIDs {
		reachableSet[id] = struct{}{}
	}
	lines := []string{theme.Title.Render(title)}
	lines = append(lines, theme.Subtitle.Render(position))
	if len(reachableIDs) > 0 {
		parts := make([]string, 0, len(reachableIDs))
		for _, layer := range layers {
			for _, node := range layer {
				if _, ok := reachableSet[node.ID]; ok {
					parts = append(parts, fmt.Sprintf("F%d-%d %s", node.Floor, node.Index+1, node.Kind))
				}
			}
		}
		for _, line := range wrapLine("当前可达: "+strings.Join(parts, " | "), contentWidth) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	lines = append(lines, "", theme.Accent.Render("树状地图"))
	for floorIndex, layer := range layers {
		lines = append(lines, theme.Subtitle.Render(fmt.Sprintf("第%02d层", floorIndex+1)))
		for idx, node := range layer {
			branch := "├─"
			if idx == len(layer)-1 {
				branch = "└─"
			}
			marker := "[未到]"
			style := theme.Normal
			switch {
			case currentNodeID != "" && currentNodeID == node.ID:
				marker = "[当前]"
				style = theme.Selected
			case containsString(reachableIDs, node.ID):
				marker = "[可达]"
				style = theme.Good
			case node.Floor <= currentFloor:
				marker = "[已过]"
				style = theme.Accent
			default:
				style = theme.Muted
			}
			text := fmt.Sprintf("%s %s %d. %s%s", branch, marker, node.Index+1, styleForNodeKindLabel(theme, node.Kind).Render(node.Kind), compactHighlightedNodeEdges(node.Edges, reachableSet, currentNodeID))
			for _, line := range wrapLine(text, contentWidth) {
				lines = append(lines, style.Render(line))
			}
		}
	}
	lines = append(lines, "", theme.Muted.Render("按 M 或 Esc 返回当前界面"))
	if height > 0 {
		maxLines := max(12, height-6)
		lines = fixedPanelLines(lines, maxLines, theme, contentWidth)
	}
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func mapOverlayPosition(currentNode engine.Node, currentFloor int) string {
	position := fmt.Sprintf("当前位置: 第 %d 层后，下一层为第 %d 层", currentFloor, min(15, currentFloor+1))
	if currentNode.ID != "" {
		position = fmt.Sprintf("当前位置: 第 %d 层 %s", currentNode.Floor, engine.NodeKindName(currentNode.Kind))
	}
	return position
}

func compactHighlightedNodeEdges(edges []string, reachableSet map[string]struct{}, currentNodeID string) string {
	if len(edges) == 0 {
		return ""
	}
	parts := make([]string, 0, len(edges))
	highlighted := false
	for _, edge := range edges {
		label := compactNodeID(edge)
		if edge == currentNodeID {
			label = "[当前:" + label + "]"
			highlighted = true
		} else if _, ok := reachableSet[edge]; ok {
			label = "[可达:" + label + "]"
			highlighted = true
		}
		parts = append(parts, label)
	}
	arrow := " -> "
	if highlighted {
		arrow = " => "
	}
	return arrow + strings.Join(parts, ", ")
}

func compactNodeID(id string) string {
	parts := strings.Split(id, "-")
	floor := "?"
	index := "?"
	for _, part := range parts {
		if strings.HasPrefix(part, "f") {
			floor = strings.TrimPrefix(part, "f")
		}
		if strings.HasPrefix(part, "n") {
			index = strings.TrimPrefix(part, "n")
		}
	}
	if strings.Contains(id, "boss") {
		return "Boss"
	}
	return fmt.Sprintf("F%s-%s", floor, index)
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func RenderCombat(theme Theme, lib *content.Library, run *engine.RunState, combat *engine.CombatState, selected int, selectedPotion int, potionMode bool, pane int, topPage int, logPage int, target engine.CombatTarget, width int, height int) string {
	viewport := viewportWidth(width, 100)
	totalWidth := max(30, viewport-2)
	seat := engine.CombatSeatView(combat, 0)
	hand := combat.Hand
	drawPile := combat.DrawPile
	discardPile := combat.Discard
	exhaustPile := combat.Exhaust
	if seat != nil {
		hand = seat.Hand
		drawPile = seat.DrawPile
		discardPile = seat.Discard
		exhaustPile = seat.Exhaust
	}
	energy := combat.Player.Energy
	maxEnergy := combat.Player.MaxEnergy
	if actor := engine.ActorForSeat(combat, 0); actor != nil {
		energy = actor.Energy
		maxEnergy = actor.MaxEnergy
	}

	focusEnemy := combat.Enemy
	if target.Kind == engine.CombatTargetEnemy && target.Index >= 0 && target.Index < len(combat.Enemies) {
		focusEnemy = combat.Enemies[target.Index]
	}
	topRow := renderCombatTopOverviewRow(theme, lib, run, combat, focusEnemy, target, pane, topPage, totalWidth, height, combat.Turn, len(hand), energy, maxEnergy, len(drawPile), len(discardPile), len(exhaustPile), selectedPotion, potionMode)
	keyBar := renderSinglePlayerCombatKeyBar(theme, totalWidth)

	friendRow := renderActorStrip(theme, "友方战线", engine.PartyMembersView(combat), target, true, totalWidth)
	enemyRow := renderEnemyStrip(theme, "敌方战线", combat.Enemies, target, totalWidth)

	handPreferredWidth := totalWidth * 5 / 8
	if len(hand) >= 5 {
		handPreferredWidth = totalWidth * 2 / 3
	}
	bottomLeftWidth, bottomRightWidth, stackedBottom := splitCombatBottomPanelWidths(totalWidth, handPreferredWidth)
	logMode := combatLogMode(totalWidth, height, len(hand), stackedBottom)
	handContentWidth := panelContentWidth(bottomLeftWidth)
	logContentWidth := panelContentWidth(bottomRightWidth)
	handItems := make([]selectableSectionItem, 0, len(hand))
	for i, card := range hand {
		cardDef := lib.Cards[card.ID]
		handItems = append(handItems, selectableSectionItem{
			Text:     combatCardText(i+1, cardDef.Cost, styledCardName(theme, engine.CardStateName(lib, card.ID, card.Upgraded), primaryCardKind(cardDef.Tags)), engine.RuntimeCardStateSummary(lib, card), targetHintText(lib, card)),
			Selected: !potionMode && i == selected,
		})
	}
	potionItems := make([]selectableSectionItem, 0)
	potions := []string{}
	if seat != nil {
		potions = append(potions, seat.Potions...)
	} else if run != nil {
		potions = append(potions, run.Player.Potions...)
	}
	for i, potionID := range potions {
		potion := lib.Potions[potionID]
		label := fmt.Sprintf("%d. %s | %s", i+1, potion.Name, potion.Description)
		if potionMode {
			label = fmt.Sprintf("[%d] %s | %s", i+1, potion.Name, potion.Description)
		}
		potionItems = append(potionItems, selectableSectionItem{
			Text:     label,
			Selected: potionMode && i == selectedPotion,
		})
	}
	handTitle := "手牌"
	handHint := "上下选择当前模式项目，左右切目标，回车执行，z 切换手牌/药水，e 结束回合，tab 切右侧信息面板"
	if potionMode {
		handTitle = "手牌 [药水模式]"
	}
	handPanel := renderCombatActionPanel(theme, bottomLeftWidth, handContentWidth, handTitle, handHint, handItems, potionItems)
	logPanel := renderCombatLogPanelStyled(theme, combat.Log, logPage, bottomRightWidth, logContentWidth, height, logMode, singlePlayerCombatLogTokens(theme, lib, run, combat))
	bottomRow := renderCombatTopSummaryPanels(handPanel, logPanel, stackedBottom)

	header := []string{
		theme.Title.Render("战斗"),
		theme.Subtitle.Render(fmt.Sprintf("队伍 %d 人 | 敌方 %d 体", 1+len(combat.Allies), len(combat.Enemies))),
	}
	return strings.Join(append(header, "", keyBar, topRow, friendRow, enemyRow, bottomRow), "\n")
}

func renderSinglePlayerCombatKeyBar(theme Theme, width int) string {
	hints := []string{"上下: 选当前模式项", "左右: 切目标", "[/]: 顶栏翻页", ",/.: 日志翻页", "Enter: 执行当前项", "1-6: 切检视页", "z: 切换手牌/药水", "e: 结束回合", "Tab: 切右侧信息"}
	contentWidth := panelContentWidth(width)
	lines := []string{theme.Accent.Render("当前按键")}
	for _, line := range wrapLine(strings.Join(hints, " | "), contentWidth) {
		lines = append(lines, theme.Muted.Render(line))
	}
	return theme.PanelAlt.Width(max(24, width)).Render(strings.Join(lines, "\n"))
}

func RenderPotionReplace(theme Theme, lib *content.Library, player engine.PlayerState, pendingPotionID string, selected int, width int) string {
	panelWidth := fitPanelWidth(width, 76, 4)
	contentWidth := panelContentWidth(panelWidth)
	items := make([]string, 0, len(player.Potions)+1)
	for i, potionID := range player.Potions {
		potion := lib.Potions[potionID]
		items = append(items, fmt.Sprintf("替换 %d. %s | %s", i+1, potion.Name, potion.Description))
	}
	items = append(items, "丢弃新药水")
	selected = clampSelection(selected, len(items))
	pending := lib.Potions[pendingPotionID]
	lines := []string{
		theme.Title.Render("药水栏已满"),
		theme.Subtitle.Render(fmt.Sprintf("新获得: %s | %s", pending.Name, pending.Description)),
		theme.Muted.Render(fmt.Sprintf("当前药水 %d/%d", len(player.Potions), engine.EffectivePotionCapacity(lib, player))),
		"",
	}
	for i, item := range items {
		line := truncateASCII(fmt.Sprintf("%d. %s", i+1, item), contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render("上下选择要替换的槽位，回车确认，Esc 直接丢弃"))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
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
		kind := primaryCardKind(card.Tags)
		line := truncateASCII(fmt.Sprintf("%d. %s %s | %s", i+1, renderCardKindChip(theme, kind), styledCardName(theme, card.Name, kind), engine.DescribeEffects(lib, card.Effects)), contentWidth)
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
			kind := primaryCardKind(lib.Cards[card.CardID].Tags)
			base := truncateASCII(fmt.Sprintf("%d. %s %s | %s", i+1, renderCardKindChip(theme, kind), styledCardName(theme, engine.CardStateName(lib, card.CardID, card.Upgraded), kind), engine.DeckCardStateSummary(lib, card)), contentWidth)
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
		line := fmt.Sprintf("%d. %s %s [%d] - %s", i+1, renderInlineChips(theme, []string{strings.ToUpper(strings.TrimSpace(offer.Kind))}), offer.Name, offer.Price, offer.Description)
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
		return theme.PanelAlt.Copy().Padding(0, 1).Width(width).Render(theme.Muted.Render(title + "为空"))
	}
	contentWidth := panelContentWidth(width)
	cols := adaptiveStripColumns(contentWidth, len(actors))
	cellWidth := adaptiveStripCellWidth(contentWidth, cols)
	containerStyle := theme.PanelAlt.Copy().Padding(0, 1)
	cellStyle := theme.PanelAlt.Copy().Padding(0, 1)
	parts := make([]string, 0, len(actors))
	for i, actor := range actors {
		selected := friendly && target.Kind == engine.CombatTargetAlly && target.Index == i
		lines := []string{}
		nameLine := fmt.Sprintf("P%d %s", i+1, actor.Name)
		if selected {
			nameLine += " [目标]"
		}
		lines = append(lines, wrapLine(nameLine, cellWidth-2)...)
		lines = append(lines, wrapLine(combatActorText("", actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, engine.DescribeStatuses(actor.Statuses)), cellWidth-2)...)
		content := strings.Join(lines, "\n")
		style := cellStyle.Width(cellWidth)
		if selected {
			style = style.BorderForeground(lipgloss.Color("221")).Background(lipgloss.Color("236"))
		}
		parts = append(parts, style.Render(content))
	}
	body := joinStripRows(parts, cols)
	return containerStyle.Width(width).Render(theme.Accent.Render(title) + "\n" + body)
}

func renderEnemyStrip(theme Theme, title string, enemies []engine.CombatEnemy, target engine.CombatTarget, width int) string {
	if len(enemies) == 0 {
		return theme.PanelAlt.Copy().Padding(0, 1).Width(width).Render(theme.Muted.Render(title + "为空"))
	}
	contentWidth := panelContentWidth(width)
	cols := adaptiveStripColumns(contentWidth, len(enemies))
	cellWidth := adaptiveStripCellWidth(contentWidth, cols)
	containerStyle := theme.PanelAlt.Copy().Padding(0, 1)
	cellStyle := theme.PanelAlt.Copy().Padding(0, 1)
	parts := make([]string, 0, len(enemies))
	for i, enemy := range enemies {
		selected := target.Kind == engine.CombatTargetEnemy && target.Index == i
		lines := []string{}
		nameLine := fmt.Sprintf("M%d %s", i+1, styledEnemyName(theme, enemy.Name))
		if selected {
			nameLine += " [目标]"
		}
		lines = append(lines, wrapLine(nameLine, cellWidth-2)...)
		lines = append(lines, wrapLine(combatEnemyText(i+1, styledEnemyName(theme, enemy.Name), enemy.HP, enemy.MaxHP, enemy.Block, engine.DescribeStatuses(enemy.Statuses), ""), cellWidth-2)...)
		content := strings.Join(lines, "\n")
		style := cellStyle.Width(cellWidth)
		if selected {
			style = style.BorderForeground(lipgloss.Color("221")).Background(lipgloss.Color("236"))
		}
		parts = append(parts, style.Render(content))
	}
	body := joinStripRows(parts, cols)
	return containerStyle.Width(width).Render(theme.Accent.Render(title) + "\n" + body)
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
		return max(12, contentWidth)
	}
	return max(12, (contentWidth-(cols-1))/cols)
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

func appendWrappedLine(lines *[]string, theme Theme, width int, prefix string, text string) {
	for _, line := range wrapLine(prefix+text, width) {
		*lines = append(*lines, theme.Normal.Render(line))
	}
}

func appendStatusChipLine(lines *[]string, theme Theme, width int, label string, value string, active bool) {
	if strings.TrimSpace(value) == "" {
		return
	}
	labelText := theme.Subtitle.Render(label)
	valueText := theme.Chip.Render(value)
	if active {
		valueText = theme.Selected.Render(value)
	}
	line := lipgloss.JoinHorizontal(lipgloss.Left, labelText, " ", valueText)
	if lipgloss.Width(line) <= width {
		*lines = append(*lines, line)
		return
	}
	*lines = append(*lines, labelText)
	*lines = append(*lines, valueText)
}

func appendSectionTitle(lines *[]string, theme Theme, title string) {
	*lines = append(*lines, "", theme.Subtitle.Render(title))
}

func appendCompactSectionTitle(lines *[]string, theme Theme, title string) {
	*lines = append(*lines, theme.Subtitle.Render(title))
}

func appendSelectableBulletLine(lines *[]string, theme Theme, width int, text string, selected bool) {
	prefix := "- "
	if selected {
		prefix = "> "
	}
	line := truncateASCII(prefix+text, width)
	if selected {
		*lines = append(*lines, theme.Selected.Width(width).Render(line))
		return
	}
	appendWrappedLine(lines, theme, width, "- ", text)
}

func appendSelectableLine(lines *[]string, theme Theme, width int, text string, selected bool) {
	line := truncateASCII(text, width)
	if selected {
		line = truncateASCII("> "+text, width)
	}
	if selected {
		*lines = append(*lines, theme.Selected.Width(width).Render(line))
		return
	}
	*lines = append(*lines, theme.Normal.Render(line))
}

type selectableSectionItem struct {
	Text     string
	Selected bool
}

func appendSelectableSection(lines *[]string, theme Theme, width int, title string, items []selectableSectionItem, bulleted bool) {
	if title != "" {
		appendSectionTitle(lines, theme, title)
	}
	for _, item := range items {
		if bulleted {
			appendSelectableBulletLine(lines, theme, width, item.Text, item.Selected)
			continue
		}
		appendSelectableLine(lines, theme, width, item.Text, item.Selected)
	}
}

func appendCombatSelectionSection(lines *[]string, theme Theme, width int, title string, emptyText string, items []selectableSectionItem, bulleted bool) {
	appendSelectableSection(lines, theme, width, title, items, bulleted)
	if len(items) == 0 && emptyText != "" {
		*lines = append(*lines, theme.Muted.Render(emptyText))
	}
}

func appendCompactCombatSelectionSection(lines *[]string, theme Theme, width int, title string, emptyText string, items []selectableSectionItem, bulleted bool) {
	if title != "" {
		appendCompactSectionTitle(lines, theme, title)
	}
	for _, item := range items {
		if bulleted {
			appendSelectableBulletLine(lines, theme, width, item.Text, item.Selected)
			continue
		}
		appendSelectableLine(lines, theme, width, item.Text, item.Selected)
	}
	if len(items) == 0 && emptyText != "" {
		*lines = append(*lines, theme.Muted.Render(emptyText))
	}
}

func renderCombatSelectionPanel(theme Theme, panelWidth int, contentWidth int, title string, subtitle string, emptyText string, items []selectableSectionItem, bulleted bool) string {
	lines := []string{theme.Accent.Render(title)}
	if subtitle != "" {
		lines = append(lines, theme.Subtitle.Render(subtitle), "")
	}
	appendCombatSelectionSection(&lines, theme, contentWidth, "", emptyText, items, bulleted)
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func renderCombatActionPanel(theme Theme, panelWidth int, contentWidth int, title string, subtitle string, handItems []selectableSectionItem, potionItems []selectableSectionItem) string {
	lines := []string{theme.Accent.Render(title)}
	if subtitle != "" {
		lines = append(lines, theme.Subtitle.Render(subtitle), "")
	}
	appendCompactCombatSelectionSection(&lines, theme, contentWidth, "手牌槽", "当前没有可打出的手牌。", handItems, false)
	lines = append(lines, "")
	appendCompactCombatSelectionSection(&lines, theme, contentWidth, "药水槽", "当前没有可用药水。", potionItems, false)
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func renderCombatSummaryPanel(theme Theme, panelWidth int, title string, subtitle string, detailLines []string, topPage int, maxLines int) string {
	bodyMaxLines := max(1, maxLines-1)
	body := []string{}
	for _, line := range detailLines {
		if strings.TrimSpace(line) == "" {
			body = append(body, "")
			continue
		}
		for _, wrapped := range wrapLine(line, panelContentWidth(panelWidth)) {
			body = append(body, theme.Normal.Render(wrapped))
		}
	}
	pageCount := fixedPanelBodyPageCount(body, max(1, maxLines-2))
	clampedPage := clampFixedPanelBodyPage(topPage, body, max(1, maxLines-2))
	titleLine := title
	if pageCount > 1 {
		titleLine = fmt.Sprintf("%s [%d/%d]", title, clampedPage+1, pageCount)
	}
	lines := []string{theme.Accent.Render(titleLine)}
	if subtitle != "" {
		lines = append(lines, theme.Subtitle.Render(subtitle))
	}
	bodyMaxLines = max(1, maxLines-len(lines))
	clampedPage = clampFixedPanelBodyPage(topPage, body, bodyMaxLines)
	lines = append(lines, fixedPanelBodyPageLines(body, clampedPage, bodyMaxLines, theme, panelContentWidth(panelWidth))...)
	lines = fixedPanelLines(lines, maxLines, theme, panelContentWidth(panelWidth))
	return theme.PanelAlt.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func renderCombatEnemySummaryPanel(theme Theme, name string, hp int, maxHP int, block int, status string, intent string, focusLabel string, panelWidth int, maxLines int) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	enemyLines := []string{
		theme.Accent.Render("敌方信息"),
		theme.Title.Render(styledEnemyName(theme, name)),
		theme.Normal.Render(fmt.Sprintf("HP %d/%d  Block %d", hp, maxHP, block)),
	}
	appendStatusChipLine(&enemyLines, theme, panelContentWidth(panelWidth), "当前目标", focusLabel, true)
	if status != "" {
		enemyLines = append(enemyLines, theme.Normal.Render("状态: "+status))
	} else {
		enemyLines = append(enemyLines, theme.Muted.Render("状态: 无"))
	}
	if intent != "" {
		enemyLines = append(enemyLines, "", theme.Subtitle.Render("当前意图"))
		for _, line := range wrapLine(intent, panelContentWidth(panelWidth)) {
			enemyLines = append(enemyLines, theme.Normal.Render(line))
		}
	}
	enemyLines = fixedPanelLines(enemyLines, maxLines, theme, panelContentWidth(panelWidth))
	return theme.PanelAlt.Width(panelWidth).Render(strings.Join(enemyLines, "\n"))
}

func renderCombatEnemyFocusPanel(theme Theme, combat *engine.CombatState, focusEnemy engine.CombatEnemy, target engine.CombatTarget, panelWidth int, maxLines int) string {
	focusLabel := ""
	if combat != nil && target.Kind == engine.CombatTargetEnemy {
		focusLabel = engine.DescribeCombatTarget(combat, target)
	}
	return renderCombatEnemySummaryPanel(theme, focusEnemy.Name, focusEnemy.HP, focusEnemy.MaxHP, focusEnemy.Block, engine.DescribeStatuses(focusEnemy.Statuses), engine.DescribeIntent(focusEnemy.CurrentIntent), focusLabel, panelWidth, maxLines)
}

func renderCombatSeatStatusPanel(theme Theme, lib *content.Library, combat *engine.CombatState, run *engine.RunState, seatIndex int, target engine.CombatTarget, panelWidth int, maxLines int) string {
	if combat == nil {
		return ""
	}
	actor := engine.ActorForSeat(combat, seatIndex)
	if actor == nil {
		return ""
	}
	classID := ""
	if seatIndex >= 0 && seatIndex < len(combat.SeatPlayers) {
		classID = combat.SeatPlayers[seatIndex].ClassID
	}
	if classID == "" && run != nil {
		classID = run.Player.ClassID
	}
	lines := []string{
		theme.Accent.Render("自身状态"),
		theme.Title.Render(styledClassName(theme, actor.Name, classID)),
		theme.Normal.Render(fmt.Sprintf("HP %d/%d | 格挡 %d | 能量 %d/%d", actor.HP, actor.MaxHP, actor.Block, actor.Energy, actor.MaxEnergy)),
	}
	if status := engine.DescribeStatuses(actor.Statuses); status != "" {
		for _, line := range wrapLine("状态: "+status, panelContentWidth(panelWidth)) {
			lines = append(lines, theme.Normal.Render(line))
		}
	} else {
		lines = append(lines, theme.Muted.Render("状态: 无"))
	}
	if run != nil {
		potionCount := len(run.Player.Potions)
		if seat := engine.CombatSeatView(combat, seatIndex); seat != nil {
			potionCount = len(seat.Potions)
		}
		lines = append(lines, theme.Muted.Render(fmt.Sprintf("金币 %d | 药水 %d/%d | 牌组 %d", run.Player.Gold, potionCount, engine.EffectivePotionCapacity(lib, run.Player), len(run.Player.Deck))))
	}
	for _, pending := range engine.PendingNextCardRepeatDescriptions(combat, seatIndex) {
		lines = append(lines, theme.Selected.Width(panelContentWidth(panelWidth)).Render(truncateASCII("连发待机: "+pending, panelContentWidth(panelWidth))))
	}
	lines = append(lines, "")
	appendStatusChipLine(&lines, theme, panelContentWidth(panelWidth), "当前目标", engine.DescribeCombatTarget(combat, target), true)
	lines = fixedPanelLines(lines, maxLines, theme, panelContentWidth(panelWidth))
	return theme.PanelAlt.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func renderCombatTopSummaryPanels(enemyPanel string, seatInfoPanel string, stacked bool) string {
	if strings.TrimSpace(enemyPanel) == "" {
		return seatInfoPanel
	}
	if strings.TrimSpace(seatInfoPanel) == "" {
		return enemyPanel
	}
	if stacked {
		return lipgloss.JoinVertical(lipgloss.Left, enemyPanel, seatInfoPanel)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, enemyPanel, " ", seatInfoPanel)
}

func renderCombatTopThreePanels(left string, middle string, right string, stacked bool) string {
	parts := []string{}
	for _, panel := range []string{left, middle, right} {
		if strings.TrimSpace(panel) == "" {
			continue
		}
		parts = append(parts, panel)
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if stacked {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	joined := parts[0]
	for i := 1; i < len(parts); i++ {
		joined = lipgloss.JoinHorizontal(lipgloss.Top, joined, " ", parts[i])
	}
	return joined
}

func combatTopPanelLineBudget(height int, stacked bool) int {
	switch {
	case stacked:
		return 8
	case height > 0 && height < 28:
		return 8
	case height > 0 && height >= 40:
		return 11
	default:
		return 9
	}
}

func renderCombatTopOverviewRow(theme Theme, lib *content.Library, run *engine.RunState, combat *engine.CombatState, focusEnemy engine.CombatEnemy, target engine.CombatTarget, pane int, topPage int, totalWidth int, height int, turn int, handCount int, energy int, maxEnergy int, drawCount int, discardCount int, exhaustCount int, selectedPotion int, potionMode bool) string {
	if totalWidth < 72 {
		infoWidth := max(30, totalWidth)
		panelLines := combatTopPanelLineBudget(height, true)
		return renderCombatTopThreePanels(
			renderCombatSeatStatusPanel(theme, lib, combat, run, 0, target, infoWidth, panelLines),
			renderCombatEnemyFocusPanel(theme, combat, focusEnemy, target, infoWidth, panelLines),
			renderCombatSeatInfoPanel(theme, lib, run, combat, 0, pane, topPage, target, infoWidth, panelLines, turn, handCount, energy, maxEnergy, drawCount, discardCount, exhaustCount, selectedPotion, potionMode),
			true,
		)
	}
	leftWidth, middleWidth, rightWidth := splitCombatSummaryPanelWidths(totalWidth)
	panelLines := combatTopPanelLineBudget(height, false)
	leftPanel := renderCombatSeatStatusPanel(theme, lib, combat, run, 0, target, leftWidth, panelLines)
	middlePanel := renderCombatEnemyFocusPanel(theme, combat, focusEnemy, target, middleWidth, panelLines)
	rightPanel := renderCombatSeatInfoPanel(theme, lib, run, combat, 0, pane, topPage, target, rightWidth, panelLines, turn, handCount, energy, maxEnergy, drawCount, discardCount, exhaustCount, selectedPotion, potionMode)
	return renderCombatTopThreePanels(leftPanel, middlePanel, rightPanel, false)
}

func splitCombatSummaryPanelWidths(totalWidth int) (int, int, int) {
	leftWidth, middleWidth, rightWidth, _ := splitFramedThreeColumns(totalWidth, 22, 24, 30, 1, 6)
	return leftWidth, middleWidth, rightWidth
}

func splitCombatBottomPanelWidths(totalWidth int, preferredLeft int) (int, int, bool) {
	return splitFramedAdaptiveColumns(totalWidth, preferredLeft, 30, 24, 1, 4)
}

func renderCombatTopSummaryBlock(theme Theme, focusEnemy engine.CombatEnemy, seatInfoPanel string, enemyPanelWidth int, stacked bool) string {
	enemyPanel := renderCombatEnemyFocusPanel(theme, nil, focusEnemy, engine.CombatTarget{}, enemyPanelWidth, combatTopPanelLineBudget(0, stacked))
	return renderCombatTopSummaryPanels(enemyPanel, seatInfoPanel, stacked)
}

func renderCombatSeatInfoPanel(theme Theme, lib *content.Library, run *engine.RunState, combat *engine.CombatState, seatIndex int, pane int, topPage int, target engine.CombatTarget, panelWidth int, maxLines int, turn int, handCount int, energy int, maxEnergy int, drawCount int, discardCount int, exhaustCount int, selectedPotion int, potionMode bool) string {
	potions := append([]string{}, run.Player.Potions...)
	if seat := engine.CombatSeatView(combat, seatIndex); seat != nil {
		potions = append([]string{}, seat.Potions...)
	}
	potionSummary := fmt.Sprintf("药水 %d/%d", len(potions), engine.EffectivePotionCapacity(lib, run.Player))
	if potionMode && len(potions) > 0 {
		selectedPotion = min(selectedPotion, len(potions)-1)
		potion := lib.Potions[potions[selectedPotion]]
		potionSummary += fmt.Sprintf(" | 当前 %d.%s", selectedPotion+1, potion.Name)
	}
	headerLines := []string{
		theme.Accent.Render("战斗信息"),
		theme.Subtitle.Render(fmt.Sprintf("回合 %d | 手牌 %d | 能量 %d/%d", turn, handCount, energy, maxEnergy)),
		theme.Subtitle.Render(fmt.Sprintf("抽牌堆 %d | 弃牌堆 %d | 消耗堆 %d", drawCount, discardCount, exhaustCount)),
		theme.Subtitle.Render(potionSummary),
		theme.Muted.Render(fmt.Sprintf("检视页: %s", engine.CombatInspectPaneName(pane))),
		renderCombatTabs(theme, pane),
	}
	for _, pending := range engine.PendingNextCardRepeatDescriptions(combat, seatIndex) {
		headerLines = append(headerLines, theme.Selected.Render("连发待机 | "+pending))
	}
	bodyLines := []string{}
	for _, line := range engine.CombatInspectLinesForSeat(lib, run, combat, seatIndex, pane) {
		for _, wrapped := range wrapLine(line, panelContentWidth(panelWidth)) {
			bodyLines = append(bodyLines, theme.Normal.Render(wrapped))
		}
	}
	infoLines := append([]string{}, headerLines...)
	infoLines = append(infoLines, fixedPanelBodyPageLines(bodyLines, topPage, max(1, maxLines-len(headerLines)), theme, panelContentWidth(panelWidth))...)
	infoLines = fixedPanelLines(infoLines, maxLines, theme, panelContentWidth(panelWidth))
	return theme.PanelAlt.Width(panelWidth).Render(strings.Join(infoLines, "\n"))
}

func fixedPanelLines(lines []string, maxLines int, theme Theme, width int) []string {
	if maxLines <= 0 {
		return lines
	}
	if len(lines) > maxLines {
		bodyLines := max(1, maxLines-2)
		pageCount := (len(lines) + bodyLines - 1) / bodyLines
		remaining := max(0, len(lines)-bodyLines)
		lines = append([]string{}, lines[:bodyLines]...)
		lines = append(lines,
			theme.Muted.Render(truncateASCII(fmt.Sprintf("第 1/%d 页", pageCount), width)),
			theme.Muted.Render(truncateASCII(fmt.Sprintf("还有 %d 行...", remaining), width)),
		)
	}
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}

func fixedPanelBodyLines(lines []string, maxLines int, theme Theme, width int) []string {
	return fixedPanelBodyPageLines(lines, 0, maxLines, theme, width)
}

func fixedPanelBodyPageCount(lines []string, maxLines int) int {
	if maxLines <= 0 {
		return 1
	}
	if len(lines) <= maxLines {
		return 1
	}
	bodyLines := max(1, maxLines-2)
	return max(1, (len(lines)+bodyLines-1)/bodyLines)
}

func clampFixedPanelBodyPage(page int, lines []string, maxLines int) int {
	pageCount := fixedPanelBodyPageCount(lines, maxLines)
	return max(0, min(page, pageCount-1))
}

func fixedPanelBodyPageLines(lines []string, page int, maxLines int, theme Theme, width int) []string {
	if maxLines <= 0 {
		return nil
	}
	if len(lines) <= maxLines {
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return lines
	}
	bodyLines := max(1, maxLines-2)
	pageCount := fixedPanelBodyPageCount(lines, maxLines)
	page = clampFixedPanelBodyPage(page, lines, maxLines)
	start := page * bodyLines
	end := min(len(lines), start+bodyLines)
	remaining := max(0, len(lines)-end)
	visible := append([]string{}, lines[start:end]...)
	visible = append(visible,
		theme.Muted.Render(truncateASCII(fmt.Sprintf("第 %d/%d 页", page+1, pageCount), width)),
		theme.Muted.Render(truncateASCII(fmt.Sprintf("还有 %d 行...", remaining), width)),
	)
	for len(visible) < maxLines {
		visible = append(visible, "")
	}
	return visible
}

func combatLogMode(totalWidth int, height int, handCount int, stacked bool) string {
	switch {
	case stacked:
		return "紧凑"
	case totalWidth < 92:
		return "紧凑"
	case handCount >= 5:
		return "紧凑"
	case height > 0 && height < 28:
		return "紧凑"
	default:
		return "展开"
	}
}

func renderCombatLogPanel(theme Theme, entries []engine.CombatLogEntry, page int, panelWidth int, contentWidth int, height int, mode string) string {
	return renderCombatLogPanelStyled(theme, entries, page, panelWidth, contentWidth, height, mode, nil)
}

func renderCombatLogPanelStyled(theme Theme, entries []engine.CombatLogEntry, page int, panelWidth int, contentWidth int, height int, mode string, tokens []styledToken) string {
	title := "战斗日志"
	if mode != "" {
		title += " [" + mode + "]"
	}
	body := []string{}
	if len(entries) == 0 {
		body = append(body, theme.Muted.Render("暂无日志。"))
	} else {
		for _, entry := range entries {
			lineText := styleLogTurnPrefix(theme, applyStyledTokens(fmt.Sprintf("T%d %s", entry.Turn, entry.Text), tokens))
			for _, wrapped := range wrapLine(lineText, contentWidth) {
				body = append(body, theme.Muted.Render(wrapped))
			}
		}
	}
	pageBudget := enhancedCombatLogPanelLineBudget(height, mode)
	pageCount := fixedRecentPanelPageCount(body, pageBudget)
	clampedPage := clampFixedRecentPanelPage(page, body, pageBudget)
	titleLine := title
	if pageCount > 1 {
		currentPage := pageCount - clampedPage
		titleLine = fmt.Sprintf("%s [%d/%d]", title, currentPage, pageCount)
	}
	logLines := []string{theme.Accent.Render(titleLine)}
	logLines = append(logLines, fixedRecentPanelBodyLines(body, clampedPage, pageBudget, theme, contentWidth)...)
	return theme.Panel.Width(panelWidth).Render(strings.Join(logLines, "\n"))
}

func enhancedCombatLogPanelLineBudget(height int, mode string) int {
	if mode != "灞曞紑" {
		switch {
		case height > 0 && height < 28:
			return 5
		case height > 0 && height < 36:
			return 7
		default:
			return 9
		}
	}
	switch mode {
	case "绱у噾":
		switch {
		case height > 0 && height < 28:
			return 5
		case height > 0 && height < 36:
			return 7
		default:
			return 9
		}
	default:
		switch {
		case height > 0 && height < 30:
			return 7
		case height > 0 && height < 40:
			return 10
		default:
			return 14
		}
	}
}

func combatLogPanelLineBudget(height int, mode string) int {
	switch mode {
	case "紧凑":
		if height > 0 && height < 28 {
			return 5
		}
		return 6
	default:
		if height > 0 && height < 30 {
			return 6
		}
		return 8
	}
}

func fixedRecentPanelPageCount(lines []string, maxLines int) int {
	if maxLines <= 0 {
		return 1
	}
	if len(lines) <= maxLines {
		return 1
	}
	bodyLines := max(1, maxLines-2)
	return max(1, (len(lines)+bodyLines-1)/bodyLines)
}

func clampFixedRecentPanelPage(page int, lines []string, maxLines int) int {
	pageCount := fixedRecentPanelPageCount(lines, maxLines)
	return max(0, min(page, pageCount-1))
}

func fixedRecentPanelBodyLines(lines []string, page int, maxLines int, theme Theme, width int) []string {
	if maxLines <= 0 {
		return nil
	}
	if len(lines) <= maxLines {
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return lines
	}
	bodyLines := max(1, maxLines-2)
	pageCount := fixedRecentPanelPageCount(lines, maxLines)
	page = clampFixedRecentPanelPage(page, lines, maxLines)
	absolutePage := pageCount - 1 - page
	start := absolutePage * bodyLines
	end := min(len(lines), start+bodyLines)
	hiddenBefore := start
	hiddenAfter := max(0, len(lines)-end)
	visible := append([]string{}, lines[start:end]...)
	visible = append(visible,
		theme.Muted.Render(truncateASCII(fmt.Sprintf("第 %d/%d 页", absolutePage+1, pageCount), width)),
		theme.Muted.Render(truncateASCII(fmt.Sprintf("前面 %d 行 | 后面 %d 行", hiddenBefore, hiddenAfter), width)),
	)
	for len(visible) < maxLines {
		visible = append(visible, "")
	}
	return visible
}

func combatActorText(name string, hp int, maxHP int, energy int, maxEnergy int, block int, status string) string {
	parts := []string{}
	if strings.TrimSpace(name) != "" {
		parts = append(parts, name)
	}
	parts = append(parts, fmt.Sprintf("HP %d/%d", hp, maxHP))
	parts = append(parts, fmt.Sprintf("格挡 %d", block))
	if maxEnergy > 0 {
		parts = append(parts, fmt.Sprintf("能量 %d/%d", energy, maxEnergy))
	}
	if status != "" {
		parts = append(parts, status)
	}
	return strings.Join(parts, " | ")
}

func combatEnemyText(index int, name string, hp int, maxHP int, block int, status string, intent string) string {
	label := name
	if index > 0 {
		label = fmt.Sprintf("%d. %s", index, name)
	}
	parts := []string{label, fmt.Sprintf("HP %d/%d", hp, maxHP), fmt.Sprintf("格挡 %d", block)}
	if intent != "" {
		parts = append(parts, "意图 "+intent)
	}
	if status != "" {
		parts = append(parts, status)
	}
	return strings.Join(parts, " | ")
}

func combatCardText(index int, cost int, name string, summary string, targetHint string) string {
	parts := []string{fmt.Sprintf("%d. [%d] %s", index, cost, name)}
	if summary != "" {
		parts = append(parts, summary)
	}
	if targetHint != "" {
		parts = append(parts, targetHint)
	}
	return strings.Join(parts, " | ")
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
