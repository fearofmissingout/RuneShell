package ui

import (
	"fmt"
	"net"
	"strings"

	"cmdcards/internal/engine"
	"cmdcards/internal/netplay"

	"github.com/charmbracelet/lipgloss"
)

const MultiplayerCombatInspectPaneCount = 5

type MultiplayerCombatState struct {
	Enabled           bool
	OperationsFocused bool
	InspectFocused    bool
	ChatFocused       bool
	ModeLabel         string
	Phase             string
	SelectedCard      int
	SelectedPotion    int
	SelectedIndex     int
	SelectionLabel    string
	InspectPane       int
	TopPage           int
	InspectLogPage    int
	TargetKind        string
	TargetIndex       int
	TargetLabel       string
}

func multiplayerCombatLogTokens(theme Theme, snapshot *netplay.Snapshot) []styledToken {
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
	if snapshot == nil {
		return tokens
	}
	for _, player := range snapshot.Players {
		addToken(player.Name, styledClassName(theme, player.Name, player.ClassID))
	}
	if snapshot.Combat != nil {
		for _, actor := range snapshot.Combat.Party {
			addToken(actor.Name, styledClassName(theme, actor.Name, actor.ClassID))
		}
		for _, enemy := range snapshot.Combat.Enemies {
			addToken(enemy.Name, styledEnemyName(theme, enemy.Name))
		}
		for _, card := range snapshot.Combat.Hand {
			addToken(card.Name, styledCardName(theme, card.Name, card.Kind))
		}
	}
	return tokens
}

func appendStyledEntries(lines *[]string, theme Theme, contentWidth int, entries []string, tokens []styledToken, style lipgloss.Style) {
	for _, item := range entries {
		styled := applyStyledTokens("- "+item, tokens)
		for _, line := range wrapLine(styled, contentWidth) {
			*lines = append(*lines, style.Render(line))
		}
	}
}

func multiplayerRecentLogCount(contentWidth int) int {
	switch {
	case contentWidth < 38:
		return 4
	case contentWidth < 54:
		return 6
	default:
		return 8
	}
}

func multiplayerPhaseLabel(snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return "房间"
	}
	if strings.TrimSpace(snapshot.PhaseTitle) != "" {
		return localizedMultiplayerPhaseLabel(snapshot.PhaseTitle)
	}
	if strings.TrimSpace(snapshot.Phase) != "" {
		phase := localizedMultiplayerPhaseLabel(snapshot.Phase)
		if len(phase) == 0 {
			return "房间"
		}
		return phase
	}
	return "房间"
}

func multiplayerPhaseMetaLines(theme Theme, snapshot *netplay.Snapshot) []string {
	if snapshot == nil {
		return nil
	}
	parts := []string{theme.Chip.Render("阶段 " + multiplayerPhaseLabel(snapshot))}
	if strings.TrimSpace(snapshot.RoomAddr) != "" {
		if host, port := roomHostPort(snapshot.RoomAddr); host != "" {
			parts = append(parts, theme.Chip.Render("地址 "+host))
			if port != "" {
				parts = append(parts, theme.Chip.Render("端口 "+port))
			}
		}
	}
	if snapshot.Seat > 0 {
		parts = append(parts, theme.Chip.Render(fmt.Sprintf("座位 %d", snapshot.Seat)))
	}
	if snapshot.SelfID != "" && snapshot.SelfID == snapshot.HostID {
		parts = append(parts, theme.Good.Render("房主"))
	}
	if len(parts) == 0 {
		return nil
	}
	return []string{lipgloss.JoinHorizontal(lipgloss.Left, parts...)}
}

func styledMultiplayerOfferName(theme Theme, kind string, category string, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "card":
		return styleForCardKind(theme, category).Render(name)
	case "potion":
		return theme.Good.Render(name)
	case "relic":
		return theme.Accent.Render(name)
	case "equipment":
		return theme.Selected.Render(name)
	default:
		return theme.Normal.Render(name)
	}
}

func styledMultiplayerChoice(theme Theme, label string, active bool) string {
	if active {
		return theme.Selected.Render(label)
	}
	return theme.Normal.Render(label)
}

func multiplayerToneChip(theme Theme, label string, tone string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	switch tone {
	case "good":
		return theme.Good.Copy().Padding(0, 1).Render(label)
	case "bad":
		return theme.Bad.Copy().Padding(0, 1).Render(label)
	case "accent":
		return theme.Accent.Copy().Padding(0, 1).Render(label)
	case "selected":
		return theme.Selected.Copy().Padding(0, 1).Render(label)
	case "muted":
		return theme.Muted.Copy().Padding(0, 1).Render(label)
	default:
		return theme.Chip.Copy().Render(label)
	}
}

func multiplayerStatusTone(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "acting", "done", "connected":
		return "good"
	case "offline", "blocked":
		return "bad"
	case "waiting", "not ready":
		return "muted"
	default:
		return "accent"
	}
}

func multiplayerCardChoiceLine(theme Theme, index int, kind string, name string, summary string, badges []string) string {
	parts := []string{fmt.Sprintf("%d.", index), renderCardKindChip(theme, kind), styledCardName(theme, name, kind)}
	if strings.TrimSpace(summary) != "" {
		parts = append(parts, "|", summary)
	}
	if chips := renderBadgeChips(theme, badges); chips != "" {
		parts = append(parts, "|", chips)
	}
	return strings.Join(parts, " ")
}

func multiplayerChoiceLine(theme Theme, index int, label string, description string, badges []string) string {
	parts := []string{fmt.Sprintf("%d.", index), label}
	if strings.TrimSpace(description) != "" {
		parts = append(parts, "|", description)
	}
	if chips := renderBadgeChips(theme, badges); chips != "" {
		parts = append(parts, "|", chips)
	}
	return strings.Join(parts, " ")
}

func multiplayerOfferLine(theme Theme, index int, kind string, category string, name string, price int, description string, badges []string) string {
	parts := []string{fmt.Sprintf("%d.", index)}
	if strings.TrimSpace(kind) != "" {
		parts = append(parts, multiplayerToneChip(theme, strings.ToUpper(kind), "accent"))
	}
	if strings.TrimSpace(category) != "" {
		parts = append(parts, multiplayerToneChip(theme, strings.ToUpper(category), "muted"))
	}
	parts = append(parts, styledMultiplayerOfferName(theme, kind, category, name), "|", fmt.Sprintf("%d 金币", price))
	if strings.TrimSpace(description) != "" {
		parts = append(parts, "|", description)
	}
	if chips := renderBadgeChips(theme, badges); chips != "" {
		parts = append(parts, "|", chips)
	}
	return strings.Join(parts, " ")
}

func RenderMultiplayerSetup(theme Theme, title, subtitle string, items []string, selected int, tips []string, width int) string {
	panelWidth := fitPanelWidth(width, 84, 4)
	contentWidth := panelContentWidth(panelWidth)
	selected = clampSelection(selected, len(items))

	lines := []string{theme.Title.Render(title)}
	for _, line := range wrapLine(subtitle, contentWidth) {
		lines = append(lines, theme.Subtitle.Render(line))
	}
	if len(tips) > 0 {
		lines = append(lines, "", theme.Accent.Render(theme.Text("multiplayer.setup.tips")))
		for _, tip := range tips {
			for _, line := range wrapLine("- "+tip, contentWidth) {
				lines = append(lines, theme.Muted.Render(line))
			}
		}
	}
	lines = append(lines, "", theme.Accent.Render(theme.Text("multiplayer.setup.current")))
	for i, item := range items {
		line := truncateASCII(item, contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render(strings.Join([]string{
		theme.Text("multiplayer.setup.footer.confirm"),
		theme.Text("multiplayer.setup.footer.switch"),
		theme.Text("multiplayer.setup.footer.back"),
		theme.Text("multiplayer.setup.footer.quit"),
	}, " | ")))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderMultiplayerRoom(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, input string, combatState MultiplayerCombatState, width int, height int) string {
	totalWidth := max(36, viewportWidth(width, 100)-2)
	leftWidth, rightWidth, stacked := splitFramedAdaptiveColumns(totalWidth, totalWidth*11/16, 40, 24, 1, 4)
	if totalWidth < 104 {
		leftWidth, rightWidth, stacked = totalWidth, totalWidth, true
	}
	focusBar := renderMultiplayerFocusBar(theme, snapshot, actionsFocused, combatState, totalWidth)
	keyBar := renderMultiplayerKeyBar(theme, snapshot, actionsFocused, combatState, totalWidth)
	left := theme.PanelAlt.Width(leftWidth).Render(renderMultiplayerMain(theme, snapshot, combatState, leftWidth))
	right := theme.PanelAlt.Width(rightWidth).Render(renderMultiplayerSidebar(theme, snapshot, actions, selectedAction, actionsFocused, combatState, rightWidth))
	body := lipJoinPanels(stacked, left, right)
	inputPanel := theme.Panel.Width(totalWidth).Render(renderMultiplayerInput(theme, input, combatState, actionsFocused, totalWidth, height))
	return strings.Join([]string{theme.Title.Render("\u591a\u4eba\u623f\u95f4"), focusBar, keyBar, body, inputPanel}, "\n\n")
}

func RenderMultiplayerMapTreeOverlay(theme Theme, snapshot *netplay.Snapshot, width int, height int) string {
	if snapshot == nil {
		return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(theme.Muted.Render("\u5f53\u524d\u6ca1\u6709\u53ef\u7528\u7684\u5171\u4eab\u5730\u56fe\u3002"))
	}
	mapData := snapshot.Map
	if mapData == nil {
		mapData = snapshot.SharedMap
	}
	if mapData == nil {
		return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(theme.Muted.Render("\u5f53\u524d\u6ca1\u6709\u53ef\u7528\u7684\u5171\u4eab\u5730\u56fe\u3002"))
	}
	layers := make([][]mapTreeNodeView, 0, len(mapData.Graph))
	for _, floor := range mapData.Graph {
		layer := make([]mapTreeNodeView, 0, len(floor))
		for _, node := range floor {
			layer = append(layer, mapTreeNodeView{ID: node.ID, Floor: node.Floor, Index: node.Index, Kind: engine.NodeKindName(engine.NodeKind(node.Kind)), Edges: append([]string{}, node.Edges...)})
		}
		layers = append(layers, layer)
	}
	reachableIDs := make([]string, 0, len(mapData.Reachable))
	for _, node := range mapData.Reachable {
		reachableIDs = append(reachableIDs, node.ID)
	}
	position := fmt.Sprintf("\u5171\u4eab\u4f4d\u7f6e: \u4e0b\u4e00\u5c42 %d", mapData.NextFloor)
	return renderMapTreeOverlayPanel(theme, fmt.Sprintf("\u5171\u4eab\u5730\u56fe Act %d", mapData.Act), position, reachableIDs, layers, mapData.CurrentNodeID, mapData.CurrentFloor, width, height)
}

func renderMultiplayerFocusBar(theme Theme, snapshot *netplay.Snapshot, actionsFocused bool, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	title := theme.Accent.Render("\u7126\u70b9")
	parts := []string{title}
	for _, item := range multiplayerFocusItems(theme, snapshot, actionsFocused, combatState) {
		parts = append(parts, item)
	}
	line := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	if lipgloss.Width(line) > contentWidth {
		line = lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	return theme.Panel.Width(max(24, width)).Render(line)
}

func multiplayerFocusItems(theme Theme, snapshot *netplay.Snapshot, actionsFocused bool, combatState MultiplayerCombatState) []string {
	items := []string{}
	appendItem := func(label string, active bool) {
		if active {
			items = append(items, theme.Selected.Render(label))
			return
		}
		items = append(items, theme.Chip.Render(label))
	}
	if snapshot != nil && snapshot.Combat != nil {
		appendItem("\u6218\u6597\u64cd\u4f5c", combatState.OperationsFocused)
		appendItem("\u6218\u6597\u68c0\u89c6", combatState.InspectFocused)
		appendItem("\u804a\u5929\u8f93\u5165", combatState.ChatFocused)
		return items
	}
	if usesMultiplayerStructuredSidebar(snapshot) {
		appendItem("\u9636\u6bb5\u64cd\u4f5c", !combatState.ChatFocused)
		appendItem("\u804a\u5929\u8f93\u5165", combatState.ChatFocused)
		return items
	}
	appendItem("\u5feb\u901f\u64cd\u4f5c", actionsFocused)
	appendItem("\u804a\u5929\u8f93\u5165", !actionsFocused)
	return items
}

func renderMultiplayerKeyBar(theme Theme, snapshot *netplay.Snapshot, actionsFocused bool, combatState MultiplayerCombatState, width int) string {
	hints := multiplayerKeyHints(snapshot, actionsFocused, combatState)
	if len(hints) == 0 {
		return ""
	}
	contentWidth := panelContentWidth(width)
	lines := []string{theme.Accent.Render("\u6309\u952e")}
	for _, line := range wrapLine(strings.Join(hints, " | "), contentWidth) {
		lines = append(lines, theme.Muted.Render(line))
	}
	return theme.PanelAlt.Width(max(24, width)).Render(strings.Join(lines, "\n"))
}

func multiplayerKeyHints(snapshot *netplay.Snapshot, actionsFocused bool, combatState MultiplayerCombatState) []string {
	if snapshot != nil && snapshot.Combat != nil {
		if combatState.InspectFocused {
			return []string{"\u4e0a/\u4e0b/\u5de6/\u53f3: \u5207\u6362\u68c0\u89c6\u9875", "[/]: \u9876\u90e8\u6458\u8981\u7ffb\u9875", "./,: \u65e5\u5fd7\u7ffb\u9875", "Tab: \u5207\u5230\u804a\u5929", "Enter: \u4fdd\u6301\u5f53\u524d\u7126\u70b9"}
		}
		if combatState.ChatFocused {
			return []string{"\u8f93\u5165\u6587\u5b57", "Enter: \u53d1\u9001\u547d\u4ee4", "Tab: \u8fd4\u56de\u6218\u6597\u64cd\u4f5c"}
		}
		return []string{"\u4e0a/\u4e0b: \u9009\u62e9\u624b\u724c\u6216\u836f\u6c34", "\u5de6/\u53f3: \u5207\u6362\u76ee\u6807", "[/]: \u9876\u90e8\u6458\u8981\u7ffb\u9875", "Enter: \u786e\u8ba4", "z: \u624b\u724c/\u836f\u6c34", "e: \u7ed3\u675f\u56de\u5408", "Tab: \u68c0\u89c6/\u804a\u5929"}
	}
	if usesMultiplayerStructuredSidebar(snapshot) {
		if combatState.ChatFocused {
			return []string{"\u8f93\u5165\u6587\u5b57", "Enter: \u53d1\u9001\u547d\u4ee4", "Tab: \u8fd4\u56de\u9636\u6bb5\u64cd\u4f5c"}
		}
		return []string{"\u4e0a/\u4e0b: \u9636\u6bb5\u9009\u62e9", "Enter: \u786e\u8ba4", "Tab: \u5207\u5230\u804a\u5929", "Esc/s/l: \u8fd4\u56de\u6216\u8df3\u8fc7"}
	}
	if actionsFocused {
		return []string{"\u4e0a/\u4e0b: \u5feb\u901f\u64cd\u4f5c", "Enter: \u6267\u884c\u52a8\u4f5c", "Tab: \u5207\u5230\u804a\u5929"}
	}
	return []string{"\u8f93\u5165\u6587\u5b57", "Enter: \u53d1\u9001\u547d\u4ee4", "Tab: \u8fd4\u56de\u5feb\u901f\u64cd\u4f5c"}
}

func renderMultiplayerMain(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	if snapshot == nil {
		return strings.Join([]string{
			theme.Accent.Render("正在连接房间"),
			theme.Muted.Render("连接成功后，这里会显示房间信息、阶段状态和队伍概况。"),
		}, "\n")
	}
	lines := []string{theme.Accent.Render(snapshotTitle(snapshot))}
	lines = append(lines, theme.Subtitle.Render(fmt.Sprintf("房间 %s | 阶段 %s", snapshot.RoomAddr, multiplayerPhaseLabel(snapshot))))
	if meta := multiplayerPhaseMetaLines(theme, snapshot); len(meta) > 0 {
		lines = append(lines, "")
		lines = append(lines, meta...)
	}
	identityLines := renderMultiplayerIdentity(theme, snapshot, contentWidth)
	if len(identityLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, identityLines...)
	}
	if snapshot.Banner != "" {
		lines = append(lines, "")
		for _, line := range wrapLine("提示: "+snapshot.Banner, contentWidth) {
			lines = append(lines, theme.Good.Render(line))
		}
	}
	if snapshot.PhaseHint != "" {
		for _, line := range wrapLine(snapshot.PhaseHint, contentWidth) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if snapshot.RoleNote != "" {
		lines = append(lines, "")
		for _, line := range wrapLine("你的职责: "+snapshot.RoleNote, contentWidth) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	lines = append(lines, "")
	lines = append(lines, renderMultiplayerPhaseLines(theme, snapshot, combatState, contentWidth)...)
	return strings.Join(lines, "\n")
}

func renderMultiplayerIdentity(theme Theme, snapshot *netplay.Snapshot, width int) []string {
	if snapshot == nil {
		return nil
	}
	lines := []string{theme.Subtitle.Render("身份")}
	if identity := multiplayerIdentityLine(theme, snapshot); identity != "" {
		for _, line := range wrapLine(identity, width) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if control := multiplayerControlSummary(snapshot); control != "" {
		for _, line := range wrapLine("控制说明: "+control, width) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	return lines
}

func multiplayerIdentityLine(theme Theme, snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	role := "\u73a9\u5bb6"
	if snapshot.SelfID != "" && snapshot.SelfID == snapshot.HostID {
		role = "\u623f\u4e3b"
	}
	seat := snapshot.Seat
	name := "\u5f53\u524d\u73a9\u5bb6"
	classID := ""
	for _, player := range snapshot.Players {
		if player.ID != snapshot.SelfID {
			continue
		}
		if player.Seat > 0 {
			seat = player.Seat
		}
		if strings.TrimSpace(player.Name) != "" {
			name = player.Name
		}
		classID = player.ClassID
		break
	}
	parts := []string{}
	if seat > 0 {
		parts = append(parts, fmt.Sprintf("\u5ea7\u4f4d %d", seat))
	}
	parts = append(parts, styledClassName(theme, name, classID))
	if classID != "" {
		parts = append(parts, "["+classID+"]")
	}
	parts = append(parts, role)
	return strings.Join(parts, " | ")
}

func multiplayerControlSummary(snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	isHost := snapshot.SelfID != "" && snapshot.SelfID == snapshot.HostID
	switch {
	case snapshot.Lobby != nil:
		if isHost {
			return "\u623f\u4e3b\u53ef\u4ee5\u914d\u7f6e\u623f\u95f4\u5e76\u5f00\u59cb\u5192\u9669\u3002"
		}
		return "\u73a9\u5bb6\u53ef\u4ee5\u9009\u804c\u4e1a\u3001\u51c6\u5907\u5e76\u5728\u7b49\u5f85\u623f\u4e3b\u65f6\u804a\u5929\u3002"
	case snapshot.Map != nil:
		if snapshot.ControlLabel == "Route vote submitted" {
			return "\u4f60\u7684\u8def\u7ebf\u6295\u7968\u5df2\u9501\u5b9a\uff0c\u6b63\u5728\u7b49\u5f85\u5176\u4ed6\u5df2\u8fde\u63a5\u5ea7\u4f4d\u3002"
		}
		return "\u6240\u6709\u5df2\u8fde\u63a5\u5ea7\u4f4d\u90fd\u9700\u8981\u5148\u5bf9\u4e0b\u4e00\u6761\u8def\u7ebf\u6295\u7968\uff0c\u7136\u540e\u623f\u95f4\u518d\u7edf\u4e00\u7ed3\u7b97\u3002"
	case snapshot.Combat != nil:
		return "\u6240\u6709\u5df2\u8fde\u63a5\u5ea7\u4f4d\u90fd\u53ef\u4ee5\u51fa\u724c\u3001\u4f7f\u7528\u836f\u6c34\uff0c\u5e76\u6295\u7968\u7ed3\u675f\u56de\u5408\u3002"
	case snapshot.Reward != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Personal reward choice" {
			return "先完成你自己的奖励选择，然后等待其他座位处理完。"
		}
		return "你的奖励已经处理完，正在等待其他座位完成。"
	case snapshot.Event != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Personal event choice" {
			return "先完成你自己的事件选择，所有人结束后共享地图才会继续推进。"
		}
		return "你的事件已经处理完，正在等待其他座位完成。"
	case snapshot.Shop != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Personal shop" {
			return "你可以独立决定商店购买内容，以及何时离开商店。"
		}
		return "你的商店阶段已经结束，正在等待其他座位离开商店。"
	case snapshot.Rest != nil:
		if isHost {
			return "由房主决定篝火行动。"
		}
		return "你可以建议篝火行动，但最终由房主确认。"
	case snapshot.Equipment != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Equipment replacement" {
			return "由你决定是否替换当前装备。"
		}
		return "正在等待其他座位完成装备替换。"
	case snapshot.Deck != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Deck action" {
			return "你正在处理当前牌组操作。"
		}
		return "正在等待其他座位完成牌组操作。"
	case snapshot.Summary != nil:
		if isHost {
			return "由房主决定是开始新一局还是关闭房间。"
		}
		return "正在等待房主决定下一步。"
	default:
		if snapshot.ControlLabel != "" {
			return snapshot.ControlLabel
		}
		return ""
	}
}

func renderLobbyPhaseLines(theme Theme, snapshot *netplay.Snapshot, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "模式: ", fmt.Sprintf("%s | 种子 %d", snapshot.Lobby.Mode, snapshot.Lobby.Seed))
	appendSectionTitle(&lines, theme, "房间成员")
	for _, player := range snapshot.Players {
		statusLabel := "未准备"
		statusTone := "muted"
		if player.Ready {
			statusLabel = "已准备"
			statusTone = "good"
		}
		if !player.Connected {
			statusLabel = "离线"
			statusTone = "bad"
		}
		memberParts := []string{
			fmt.Sprintf("座位 %d", player.Seat),
			styledClassName(theme, player.Name, player.ClassID),
		}
		if strings.TrimSpace(player.ClassID) != "" {
			memberParts = append(memberParts, multiplayerToneChip(theme, strings.ToUpper(player.ClassID), "accent"))
		}
		memberParts = append(memberParts, multiplayerToneChip(theme, strings.ToUpper(statusLabel), statusTone))
		if player.ID == snapshot.HostID {
			memberParts = append(memberParts, multiplayerToneChip(theme, "房主", "accent"))
		}
		if player.ID == snapshot.SelfID {
			memberParts = append(memberParts, multiplayerToneChip(theme, "你", "selected"))
		}
		appendWrappedLine(&lines, theme, width, "- ", strings.Join(memberParts, " "))
	}
	return lines
}

func renderMapPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	appendWrappedLine(&lines, theme, width, "地图: ", fmt.Sprintf("Act %d | 下一层 %d | 金币 %d", snapshot.Map.Act, snapshot.Map.NextFloor, snapshot.Map.Gold))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "当前选择")
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
	}
	appendSectionTitle(&lines, theme, "队伍")
	for _, actor := range snapshot.Map.Party {
		text := combatActorTextClean(styledClassName(theme, actor.Name, actor.ClassID), actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status)
		appendWrappedLine(&lines, theme, width, "- ", text)
	}
	appendSectionTitle(&lines, theme, "可达节点")
	for i, node := range snapshot.Map.Reachable {
		text := fmt.Sprintf("%d. %s %s", node.Index, multiplayerToneChip(theme, strings.ToUpper(node.Kind), "accent"), node.Label)
		if i == combatState.SelectedIndex {
			text = theme.Selected.Render(text)
		}
		appendWrappedLine(&lines, theme, width, "- ", text)
	}
	if len(snapshot.Map.VoteSummary) > 0 {
		appendSectionTitle(&lines, theme, "投票权重")
		for _, item := range snapshot.Map.VoteSummary {
			appendWrappedLine(&lines, theme, width, "- ", applyStyledTokens(item, logTokens))
		}
	}
	if len(snapshot.Map.VoteStatus) > 0 {
		appendSectionTitle(&lines, theme, "各座位投票")
		for _, item := range snapshot.Map.VoteStatus {
			appendWrappedLine(&lines, theme, width, "- ", applyStyledTokens(item, logTokens))
		}
	}
	if len(snapshot.Map.History) > 0 {
		appendSectionTitle(&lines, theme, "最近进度")
		for _, item := range tailText(snapshot.Map.History, multiplayerRecentLogCount(width)) {
			appendWrappedLine(&lines, theme, width, "- ", applyStyledTokens(item, logTokens))
		}
	}
	return lines
}

func renderCombatPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "\u6218\u6597: ", fmt.Sprintf("\u56de\u5408 %d | \u80fd\u91cf %d/%d", snapshot.Combat.Turn, snapshot.Combat.Energy, snapshot.Combat.MaxEnergy))
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	for _, pending := range snapshot.Combat.PendingRepeats {
		lines = append(lines, theme.Selected.Render(truncateASCII("\u8fde\u53d1\u5f85\u673a: "+pending, width)))
	}
	if summary := renderMultiplayerCombatTopSummary(theme, snapshot, combatState, width); summary != "" {
		lines = append(lines, summary)
	}
	if combatState.Enabled {
		appendCompactSectionTitle(&lines, theme, "\u5f53\u524d\u64cd\u4f5c")
		appendStatusChipLine(&lines, theme, width, "\u6a21\u5f0f", combatState.ModeLabel, combatState.OperationsFocused)
		appendStatusChipLine(&lines, theme, width, "\u76ee\u6807", combatState.TargetLabel, combatState.OperationsFocused)
		if combatState.OperationsFocused {
			lines = append(lines, theme.Good.Render("\u5f53\u524d\u805a\u7126\u6218\u6597\u64cd\u4f5c\uff1a\u65b9\u5411\u952e\u548c Enter \u4f1a\u76f4\u63a5\u4f5c\u7528\u4e8e\u5f53\u524d\u6218\u6597\u9009\u9879\u3002"))
		} else if combatState.InspectFocused {
			lines = append(lines, theme.Good.Render("\u5f53\u524d\u805a\u7126\u6218\u6597\u68c0\u89c6\uff1a\u65b9\u5411\u952e\u4f1a\u5207\u6362\u53f3\u4fa7\u68c0\u89c6\u9875\u3002"))
		} else {
			lines = append(lines, theme.Muted.Render("\u5f53\u524d\u805a\u7126\u804a\u5929\u8f93\u5165\u3002\u6309 Tab \u8fd4\u56de\u6218\u6597\u64cd\u4f5c\u6216\u68c0\u89c6\u3002"))
		}
	}
	partyItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Party))
	for _, actor := range snapshot.Combat.Party {
		name := styledClassName(theme, actor.Name, actor.ClassID)
		if snapshot.Seat == actor.Index {
			name += " [自己]"
		}
		selected := combatState.TargetKind == "ally" && combatState.TargetIndex == actor.Index
		if selected {
			name += " [目标]"
		}
		partyItems = append(partyItems, selectableSectionItem{
			Text:     combatActorTextClean(name, actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status),
			Selected: selected,
		})
	}
	appendCompactCombatSelectionSection(&lines, theme, width, "\u53cb\u65b9", "\u5f53\u524d\u6ca1\u6709\u53ef\u89c1\u53cb\u65b9\u3002", partyItems, true)
	enemyItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Enemies))
	for _, enemy := range snapshot.Combat.Enemies {
		selected := combatState.TargetKind == "enemy" && combatState.TargetIndex == enemy.Index
		name := styledEnemyName(theme, enemy.Name)
		if selected {
			name += " [目标]"
		}
		enemyItems = append(enemyItems, selectableSectionItem{
			Text:     combatEnemyTextClean(enemy.Index, name, enemy.HP, enemy.MaxHP, enemy.Block, enemy.Status, enemy.Intent),
			Selected: selected,
		})
	}
	appendCompactCombatSelectionSection(&lines, theme, width, "\u654c\u65b9", "\u5f53\u524d\u6ca1\u6709\u53ef\u89c1\u654c\u4eba\u3002", enemyItems, true)
	if len(snapshot.Combat.Hand) > 0 {
		handItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Hand))
		for i, card := range snapshot.Combat.Hand {
			selected := combatState.ModeLabel == "手牌" && i == combatState.SelectedCard
			name := styledCardName(theme, card.Name, card.Kind)
			if selected {
				name += " [当前]"
			}
			handItems = append(handItems, selectableSectionItem{
				Text:     combatCardText(card.Index, card.Cost, name, card.Summary, card.TargetHint),
				Selected: selected,
			})
		}
		appendCompactCombatSelectionSection(&lines, theme, width, "\u624b\u724c", "\u5f53\u524d\u6ca1\u6709\u53ef\u6253\u51fa\u7684\u624b\u724c\u3002", handItems, true)
	}
	if len(snapshot.Combat.Potions) > 0 {
		potionItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Potions))
		for i, potion := range snapshot.Combat.Potions {
			selected := combatState.ModeLabel == "药水" && i == combatState.SelectedPotion
			text := fmt.Sprintf("%d. %s", i+1, potion)
			if selected {
				text += " [当前]"
			}
			potionItems = append(potionItems, selectableSectionItem{
				Text:     text,
				Selected: selected,
			})
		}
		appendCompactCombatSelectionSection(&lines, theme, width, "\u836f\u6c34", "\u5f53\u524d\u6ca1\u6709\u53ef\u7528\u836f\u6c34\u3002", potionItems, true)
	}
	if len(snapshot.Combat.Logs) > 0 {
		recent := tailText(snapshot.Combat.Logs, multiplayerRecentLogCount(width))
		lines = append(lines, "")
		appendCompactSectionTitle(&lines, theme, "\u6700\u8fd1\u64cd\u4f5c")
		appendStyledEntries(&lines, theme, width, recent, logTokens, theme.Muted)
		if hidden := len(snapshot.Combat.Logs) - len(recent); hidden > 0 {
			lines = append(lines, theme.Muted.Render(truncateASCII(fmt.Sprintf("\u5f53\u524d\u663e\u793a\u6700\u65b0 %d \u6761\uff0c\u8fdb\u5165\u68c0\u89c6 > \u65e5\u5fd7 \u53ef\u67e5\u770b\u5269\u4f59 %d \u6761\u3002", len(recent), hidden), width)))
		}
	}
	return lines
}

func renderRewardPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "奖励: ", fmt.Sprintf("金币 +%d | 来源 %s", snapshot.Reward.Gold, snapshot.Reward.Source))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "当前选择")
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
	}
	for i, card := range snapshot.Reward.Cards {
		text := multiplayerCardChoiceLine(theme, card.Index, card.Kind, card.Name, card.Summary, card.Badges)
		appendSelectableBulletLine(&lines, theme, width, text, i == combatState.SelectedIndex)
	}
	if strings.TrimSpace(snapshot.Reward.Relic) != "" {
		appendSectionTitle(&lines, theme, "遗物")
		appendWrappedLine(&lines, theme, width, "- ", snapshot.Reward.Relic+" "+renderBadgeChips(theme, snapshot.Reward.RelicBadges))
	}
	if strings.TrimSpace(snapshot.Reward.Potion) != "" {
		appendSectionTitle(&lines, theme, "药水")
		appendWrappedLine(&lines, theme, width, "- ", multiplayerToneChip(theme, "POTION", "good")+" "+snapshot.Reward.Potion)
	}
	return lines
}

func renderEventPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "事件: ", snapshot.Event.Name)
	appendWrappedLine(&lines, theme, width, "说明: ", snapshot.Event.Description)
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "当前选择")
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
	}
	appendSectionTitle(&lines, theme, "可选项")
	for i, choice := range snapshot.Event.Choices {
		text := multiplayerChoiceLine(theme, choice.Index, theme.Accent.Render(choice.Label), choice.Description, choice.Badges)
		appendSelectableBulletLine(&lines, theme, width, text, i == combatState.SelectedIndex)
	}
	if chips := renderBadgeChips(theme, snapshot.Event.Badges); chips != "" {
		appendSectionTitle(&lines, theme, "事件特性")
		appendWrappedLine(&lines, theme, width, "- ", chips)
	}
	return lines
}

func renderShopPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "商店: ", fmt.Sprintf("金币 %d", snapshot.Shop.Gold))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "当前选择")
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
	}
	for i, offer := range snapshot.Shop.Offers {
		text := multiplayerOfferLine(theme, offer.Index, offer.Kind, offer.Category, offer.Name, offer.Price, offer.Description, offer.Badges)
		appendSelectableBulletLine(&lines, theme, width, text, i == combatState.SelectedIndex)
	}
	return lines
}

func renderRestPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	if combatState.SelectionLabel != "" {
		lines = append(lines, theme.Subtitle.Render("当前选择"))
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
		lines = append(lines, "")
	}
	lines = append(lines, theme.Subtitle.Render("篝火队伍状态"))
	for _, actor := range snapshot.Rest.Party {
		appendWrappedLine(&lines, theme, width, "- ", combatActorTextClean(styledClassName(theme, actor.Name, actor.ClassID), actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status))
	}
	return lines
}

func renderEquipmentPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "装备选择: ", fmt.Sprintf("%s | 槽位 %s", snapshot.Equipment.CandidateName, snapshot.Equipment.Slot))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "当前选择")
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
	}
	appendWrappedLine(&lines, theme, width, "候选装备: ", theme.Accent.Render(snapshot.Equipment.CandidateName)+" | "+snapshot.Equipment.CandidateDescription)
	if snapshot.Equipment.CurrentName != "" {
		appendWrappedLine(&lines, theme, width, "当前装备: ", theme.Muted.Render(snapshot.Equipment.CurrentName)+" | "+snapshot.Equipment.CurrentDescription)
	}
	return lines
}

func renderDeckPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "牌组操作: ", snapshot.Deck.Title)
	appendWrappedLine(&lines, theme, width, "说明: ", snapshot.Deck.Subtitle)
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "当前选择")
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
	}
	for i, card := range snapshot.Deck.Cards {
		text := multiplayerCardChoiceLine(theme, card.Index, card.Kind, card.Name, card.Summary, card.Badges)
		appendSelectableBulletLine(&lines, theme, width, text, i == combatState.SelectedIndex)
	}
	return lines
}

func renderSummaryPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "结算: ", fmt.Sprintf("%s | %s | Act %d | Floors %d", snapshot.Summary.Result, snapshot.Summary.Mode, snapshot.Summary.Act, snapshot.Summary.Floors))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "当前选择")
		appendWrappedLine(&lines, theme, width, "选项: ", combatState.SelectionLabel)
	}
	for _, actor := range snapshot.Summary.Party {
		appendWrappedLine(&lines, theme, width, "- ", combatActorTextClean(styledClassName(theme, actor.Name, actor.ClassID), actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status))
	}
	return lines
}

func renderMultiplayerSidebar(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	if snapshot == nil {
		return theme.Muted.Render("正在等待房间快照...")
	}
	if snapshot.Combat != nil {
		if !combatState.InspectFocused {
			return renderCombatOperationsSidebar(theme, snapshot, combatState, contentWidth)
		}
		return renderCombatInspectSidebar(theme, snapshot, combatState, contentWidth)
	}
	if usesMultiplayerStructuredSidebar(snapshot) {
		return renderStructuredMultiplayerSidebar(theme, snapshot, combatState, contentWidth)
	}
	return renderDefaultMultiplayerSidebar(theme, snapshot, actions, selectedAction, actionsFocused, contentWidth)
}

func usesMultiplayerStructuredSidebar(snapshot *netplay.Snapshot) bool {
	if snapshot == nil {
		return false
	}
	return snapshot.Map != nil || snapshot.Reward != nil || snapshot.Event != nil || snapshot.Shop != nil || snapshot.Rest != nil || snapshot.Equipment != nil || snapshot.Deck != nil || snapshot.Summary != nil
}

func renderStructuredMultiplayerSidebar(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) string {
	lines := []string{theme.Accent.Render("\u9636\u6bb5\u64cd\u4f5c")}
	if combatState.ChatFocused {
		lines = append(lines, theme.Muted.Render("\u7126\u70b9: \u804a\u5929\u8f93\u5165"))
	} else if len(multiplayerStructuredSidebarOptions(theme, snapshot)) > 0 {
		lines = append(lines, theme.Good.Render("\u7126\u70b9: \u9636\u6bb5\u64cd\u4f5c"))
	} else {
		lines = append(lines, theme.Muted.Render("\u7126\u70b9: \u623f\u95f4\u8be6\u60c5"))
	}
	options := multiplayerStructuredSidebarOptions(theme, snapshot)
	if len(options) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("\u53ef\u7528\u64cd\u4f5c"))
		selected := clampSelection(combatState.SelectedIndex, len(options))
		for i, option := range options {
			line := truncateASCII(fmt.Sprintf("%d. %s", i+1, option), contentWidth)
			switch {
			case i == selected && !combatState.ChatFocused:
				lines = append(lines, theme.Selected.Render(line))
			case i == selected:
				lines = append(lines, theme.Accent.Render(line))
			default:
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	if combatState.SelectionLabel != "" {
		lines = append(lines, "")
		appendStatusChipLine(&lines, theme, contentWidth, "\u5f53\u524d\u9009\u62e9", combatState.SelectionLabel, !combatState.ChatFocused)
	}
	lines = append(lines, "", theme.Subtitle.Render("\u6309\u952e\u63d0\u793a"))
	for _, item := range []string{
		"\u4e0a/\u4e0b: \u5207\u6362\u5f53\u524d\u9636\u6bb5\u9009\u62e9",
		"Enter: \u786e\u8ba4\u5f53\u524d\u9636\u6bb5\u9009\u62e9",
		"Tab: \u5728\u9636\u6bb5\u64cd\u4f5c\u548c\u804a\u5929\u8f93\u5165\u4e4b\u95f4\u5207\u6362",
	} {
		for _, line := range wrapLine("- "+item, contentWidth) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	appendStructuredSidebarDetailSections(&lines, theme, snapshot, contentWidth)
	return strings.Join(lines, "\n")
}

func multiplayerStructuredSidebarOptions(theme Theme, snapshot *netplay.Snapshot) []string {
	if snapshot == nil {
		return nil
	}
	switch {
	case snapshot.Map != nil:
		options := make([]string, 0, len(snapshot.Map.Reachable))
		for _, node := range snapshot.Map.Reachable {
			options = append(options, fmt.Sprintf("\u8282\u70b9 %d: %s", node.Index, node.Label))
		}
		return options
	case snapshot.Reward != nil:
		options := make([]string, 0, len(snapshot.Reward.Cards))
		for _, card := range snapshot.Reward.Cards {
			options = append(options, fmt.Sprintf("\u5956\u52b1\u5361: %s", styledCardName(theme, card.Name, card.Kind)))
		}
		return options
	case snapshot.Event != nil:
		options := make([]string, 0, len(snapshot.Event.Choices))
		for _, choice := range snapshot.Event.Choices {
			options = append(options, fmt.Sprintf("%s | %s", theme.Accent.Render(choice.Label), choice.Description))
		}
		return options
	case snapshot.Shop != nil:
		options := make([]string, 0, len(snapshot.Shop.Offers))
		for _, offer := range snapshot.Shop.Offers {
			options = append(options, fmt.Sprintf("%s | %d \u91d1\u5e01", styledMultiplayerOfferName(theme, offer.Kind, offer.Category, offer.Name), offer.Price))
		}
		return options
	case snapshot.Rest != nil:
		return []string{theme.Good.Render("\u4f11\u606f"), theme.Accent.Render("\u5347\u7ea7")}
	case snapshot.Equipment != nil:
		return []string{theme.Selected.Render("\u6536\u4e0b\u5019\u9009\u88c5\u5907"), theme.Muted.Render("\u8df3\u8fc7\u88c5\u5907")}
	case snapshot.Deck != nil:
		options := make([]string, 0, len(snapshot.Deck.Cards))
		for _, card := range snapshot.Deck.Cards {
			options = append(options, fmt.Sprintf("%s | %s", styledCardName(theme, card.Name, card.Kind), card.Summary))
		}
		if len(options) == 0 {
			return []string{"\u8fd4\u56de\u4e0a\u4e00\u754c\u9762"}
		}
		return options
	case snapshot.Summary != nil:
		return []string{"\u5f00\u59cb\u65b0\u7684\u5192\u9669", "\u5173\u95ed\u623f\u95f4"}
	default:
		return nil
	}
}

func appendStructuredSidebarDetailSections(lines *[]string, theme Theme, snapshot *netplay.Snapshot, contentWidth int) {
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	if len(snapshot.WaitingOn) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("\u7b49\u5f85"))
		for _, item := range snapshot.WaitingOn {
			for _, line := range wrapLine("- "+item, contentWidth) {
				*lines = append(*lines, theme.Bad.Render(line))
			}
		}
	}
	if len(snapshot.Commands) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("\u547d\u4ee4"))
		for _, item := range snapshot.Commands {
			for _, line := range wrapLine("- "+item, contentWidth) {
				*lines = append(*lines, theme.Normal.Render(line))
			}
		}
	}
	if len(snapshot.SeatStatus) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("\u5ea7\u4f4d"))
		appendStyledEntries(lines, theme, contentWidth, snapshot.SeatStatus, logTokens, theme.Normal)
	}
	if len(snapshot.RoomLog) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("\u623f\u95f4\u65e5\u5fd7"))
		appendStyledEntries(lines, theme, contentWidth, tailText(snapshot.RoomLog, multiplayerRecentLogCount(contentWidth)), logTokens, theme.Muted)
	}
	if len(snapshot.ChatLog) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("\u804a\u5929"))
		appendStyledEntries(lines, theme, contentWidth, tailText(snapshot.ChatLog, max(3, multiplayerRecentLogCount(contentWidth)-2)), logTokens, theme.Muted)
	}
}

func renderCombatOperationsSidebar(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) string {
	lines := []string{theme.Accent.Render("\u6218\u6597\u64cd\u4f5c")}
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	if combatState.OperationsFocused {
		lines = append(lines, theme.Good.Render("\u7126\u70b9: \u6218\u6597\u64cd\u4f5c"))
	} else if combatState.ChatFocused {
		lines = append(lines, theme.Muted.Render("\u7126\u70b9: \u804a\u5929\u8f93\u5165"))
	} else {
		lines = append(lines, theme.Muted.Render("\u7126\u70b9: \u6218\u6597\u68c0\u89c6"))
	}
	if combatState.ModeLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "\u6a21\u5f0f", combatState.ModeLabel, combatState.OperationsFocused)
	}
	if combatState.SelectionLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "\u9009\u62e9", combatState.SelectionLabel, combatState.OperationsFocused)
	}
	if combatState.TargetLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "\u76ee\u6807", combatState.TargetLabel, combatState.OperationsFocused)
	}
	for _, pending := range snapshot.Combat.PendingRepeats {
		lines = append(lines, theme.Selected.Render(truncateASCII("连发待机: "+pending, contentWidth)))
	}
	lines = append(lines, "", theme.Subtitle.Render("按键提示"))
	for _, item := range []string{
		"上/下: 选择手牌或药水",
		"左/右: 切换可用目标",
		"Enter: 执行当前操作",
		"Z: 在手牌和药水之间切换",
		"E: 提交结束回合",
		"Tab: 在战斗操作 / 战斗检视 / 聊天输入之间切换",
	} {
		for _, line := range wrapLine("- "+item, contentWidth) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if len(snapshot.Combat.VoteStatus) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("回合投票"))
		appendStyledEntries(&lines, theme, contentWidth, snapshot.Combat.VoteStatus, logTokens, theme.Muted)
	}
	if len(snapshot.SeatStatus) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("队伍状态"))
		appendStyledEntries(&lines, theme, contentWidth, snapshot.SeatStatus, logTokens, theme.Normal)
	}
	if len(snapshot.Combat.Logs) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("最近操作"))
		recent := tailText(snapshot.Combat.Logs, multiplayerRecentLogCount(contentWidth))
		appendStyledEntries(&lines, theme, contentWidth, recent, logTokens, theme.Muted)
		if hidden := len(snapshot.Combat.Logs) - len(recent); hidden > 0 {
			lines = append(lines, theme.Muted.Render(truncateASCII(fmt.Sprintf("当前显示最新 %d 条，进入检视 > 日志 还能看到另外 %d 条。", len(recent), hidden), contentWidth)))
		}
	}
	if len(snapshot.Commands) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("命令"))
		for _, item := range snapshot.Commands {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderCombatInspectSidebar(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) string {
	lines := []string{theme.Accent.Render("战斗检视")}
	current := appendCombatInspectTabs(lines, theme, combatState, contentWidth)
	lines = appendCombatInspectFocusHint(lines, theme, combatState)
	if snapshot.Combat == nil {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "")
	switch current {
	case 0:
		lines = appendCombatInspectOverviewPane(lines, theme, snapshot, combatState, contentWidth)
	case 1:
		lines = appendCombatInspectPilePane(lines, theme, snapshot, contentWidth)
	case 2:
		lines = appendCombatInspectEffectsPane(lines, theme, snapshot, contentWidth)
	case 3:
		lines = appendCombatInspectLogPane(lines, theme, snapshot, combatState, contentWidth)
	case 4:
		lines = appendCombatInspectVotePane(lines, theme, snapshot, contentWidth)
	}
	return strings.Join(lines, "\n")
}

func appendCombatInspectTabs(lines []string, theme Theme, combatState MultiplayerCombatState, contentWidth int) int {
	tabs := []string{"概览", "牌堆", "效果", "日志", "投票"}
	current := clampSelection(combatState.InspectPane, MultiplayerCombatInspectPaneCount)
	for i, tab := range tabs {
		line := truncateASCII(fmt.Sprintf("%d. %s", i+1, tab), contentWidth)
		switch {
		case i == current && combatState.InspectFocused:
			lines = append(lines, theme.Selected.Render(line))
		case i == current:
			lines = append(lines, theme.Accent.Render(line))
		default:
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	return current
}

func appendCombatInspectFocusHint(lines []string, theme Theme, combatState MultiplayerCombatState) []string {
	if combatState.InspectFocused {
		return append(lines, "", theme.Good.Render("当前聚焦战斗检视：用左/右或上/下切换页签，用 ,/. 翻日志页，按 Tab 切到聊天。"))
	}
	return append(lines, "", theme.Muted.Render("按 Tab 把焦点切到检视区，可以查看更多战斗细节。"))
}

func appendCombatInspectEntries(lines []string, theme Theme, contentWidth int, entries []string, empty string, muted bool) []string {
	if len(entries) == 0 && empty != "" {
		entries = []string{empty}
	}
	for _, item := range entries {
		for _, line := range wrapLine("- "+item, contentWidth) {
			if muted {
				lines = append(lines, theme.Muted.Render(line))
				continue
			}
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	return lines
}

func appendStyledInspectEntries(lines []string, theme Theme, contentWidth int, entries []string, tokens []styledToken, empty string, muted bool) []string {
	if len(entries) == 0 && empty != "" {
		entries = []string{empty}
	}
	style := theme.Normal
	if muted {
		style = theme.Muted
	}
	for _, item := range entries {
		styled := applyStyledTokens("- "+item, tokens)
		for _, line := range wrapLine(styled, contentWidth) {
			lines = append(lines, style.Render(line))
		}
	}
	return lines
}

func appendCombatInspectOverviewPane(lines []string, theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) []string {
	if combatState.SelectionLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "当前选择", combatState.SelectionLabel, combatState.InspectFocused)
	}
	if combatState.TargetLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "目标", combatState.TargetLabel, combatState.InspectFocused)
	}
	for _, item := range snapshot.Combat.Highlights {
		for _, line := range wrapLine("- "+item, contentWidth) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	for _, pending := range snapshot.Combat.PendingRepeats {
		for _, line := range wrapLine("- 连发待机: "+pending, contentWidth) {
			lines = append(lines, theme.Selected.Render(line))
		}
	}
	for _, text := range []string{
		fmt.Sprintf("回合 %d | 能量 %d/%d", snapshot.Combat.Turn, snapshot.Combat.Energy, snapshot.Combat.MaxEnergy),
		fmt.Sprintf("牌组 %d | 抽牌堆 %d | 弃牌堆 %d | 消耗堆 %d", snapshot.Combat.DeckSize, snapshot.Combat.DrawCount, snapshot.Combat.DiscardCount, snapshot.Combat.ExhaustCount),
	} {
		for _, line := range wrapLine(text, contentWidth) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	return lines
}

func appendCombatInspectPilePane(lines []string, theme Theme, snapshot *netplay.Snapshot, contentWidth int) []string {
	sections := []struct {
		title string
		rows  []string
	}{
		{title: "抽牌堆", rows: snapshot.Combat.DrawPile},
		{title: "弃牌堆", rows: snapshot.Combat.DiscardPile},
		{title: "消耗堆", rows: snapshot.Combat.ExhaustPile},
	}
	for idx, section := range sections {
		if idx > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, theme.Subtitle.Render(section.title))
		lines = appendCombatInspectEntries(lines, theme, contentWidth, section.rows, section.title+"为空。", false)
	}
	return lines
}

func appendCombatInspectEffectsPane(lines []string, theme Theme, snapshot *netplay.Snapshot, contentWidth int) []string {
	return appendCombatInspectEntries(lines, theme, contentWidth, snapshot.Combat.Effects, "当前没有可见的持续效果。", false)
}

func appendCombatInspectLogPane(lines []string, theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) []string {
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	mode, maxLines := multiplayerInspectLogMode(contentWidth)
	body := []string{}
	for _, item := range snapshot.Combat.Logs {
		styled := applyStyledTokens("- "+item, logTokens)
		for _, line := range wrapLine(styled, contentWidth) {
			body = append(body, theme.Muted.Render(line))
		}
	}
	if len(body) == 0 {
		body = append(body, theme.Muted.Render("当前还没有战斗日志。"))
	}
	pageCount := fixedRecentPanelPageCount(body, maxLines)
	clampedPage := clampFixedRecentPanelPage(combatState.InspectLogPage, body, maxLines)
	title := "日志 [" + mode + "]"
	if pageCount > 1 {
		currentPage := pageCount - clampedPage
		title += fmt.Sprintf(" [%d/%d]", currentPage, pageCount)
	}
	lines = append(lines, theme.Subtitle.Render(title))
	return append(lines, fixedRecentPanelBodyLines(body, clampedPage, maxLines, theme, contentWidth)...)
}

func multiplayerInspectLogMode(contentWidth int) (string, int) {
	if contentWidth < 42 {
		return "紧凑", 6
	}
	if contentWidth < 58 {
		return "展开", 9
	}
	return "展开", 12
}

func appendCombatInspectVotePane(lines []string, theme Theme, snapshot *netplay.Snapshot, contentWidth int) []string {
	lines = appendStyledInspectEntries(lines, theme, contentWidth, snapshot.Combat.VoteStatus, multiplayerCombatLogTokens(theme, snapshot), "当前没有进行中的投票状态。", false)
	if len(snapshot.Combat.Potions) == 0 {
		return lines
	}
	lines = append(lines, "", theme.Subtitle.Render("药水"))
	potions := make([]string, 0, len(snapshot.Combat.Potions))
	for i, potion := range snapshot.Combat.Potions {
		potions = append(potions, fmt.Sprintf("%d. %s", i+1, potion))
	}
	return appendCombatInspectEntries(lines, theme, contentWidth, potions, "", true)
}

func renderDefaultMultiplayerSidebar(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, contentWidth int) string {
	lines := []string{theme.Accent.Render("\u623f\u95f4\u52a9\u624b")}
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	if len(actions) > 0 {
		selectedAction = clampSelection(selectedAction, len(actions))
		lines = append(lines, theme.Subtitle.Render("\u5efa\u8bae\u64cd\u4f5c"))
		for i, item := range actions {
			line := truncateASCII(fmt.Sprintf("%d. %s", i+1, item), contentWidth)
			switch {
			case i == selectedAction && actionsFocused:
				lines = append(lines, theme.Selected.Render(line))
			case i == selectedAction:
				lines = append(lines, theme.Accent.Render(line))
			default:
				lines = append(lines, theme.Normal.Render(line))
			}
		}
		lines = append(lines, "", theme.Muted.Render("\u5efa\u8bae\u64cd\u4f5c\u4f1a\u968f\u623f\u95f4\u72b6\u6001\u53d8\u5316\uff0c\u623f\u4e3b\u72ec\u5360\u64cd\u4f5c\u4f1a\u660e\u786e\u6807\u51fa\uff0c\u6a21\u677f\u547d\u4ee4\u4e5f\u53ef\u4ee5\u76f4\u63a5\u586b\u5165\u8f93\u5165\u6846\u3002"))
	}
	if len(snapshot.Commands) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("\u547d\u4ee4"))
		for _, item := range snapshot.Commands {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	if len(snapshot.WaitingOn) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("\u7b49\u5f85\u4e2d"))
		for _, item := range snapshot.WaitingOn {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Bad.Render(line))
			}
		}
	}
	if len(snapshot.SeatStatus) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("\u5ea7\u4f4d\u72b6\u6001"))
		appendStyledEntries(&lines, theme, contentWidth, snapshot.SeatStatus, logTokens, theme.Normal)
	}
	if len(snapshot.ChatLog) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("\u6700\u8fd1\u804a\u5929"))
		appendStyledEntries(&lines, theme, contentWidth, tailText(snapshot.ChatLog, multiplayerRecentLogCount(contentWidth)), logTokens, theme.Muted)
	}
	if len(snapshot.RoomLog) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("\u7cfb\u7edf\u65e5\u5fd7"))
		appendStyledEntries(&lines, theme, contentWidth, tailText(snapshot.RoomLog, multiplayerRecentLogCount(contentWidth)), logTokens, theme.Muted)
	}
	return strings.Join(lines, "\n")
}

func renderMultiplayerInput(theme Theme, input string, combatState MultiplayerCombatState, actionsFocused bool, width int, _ int) string {
	contentWidth := panelContentWidth(width)
	value := strings.TrimSpace(input)
	if value == "" {
		value = "\u793a\u4f8b: ready / chat \u4f60\u597d / node 1 / play 1 enemy 1"
	}
	title := "\u591a\u4eba\u547d\u4ee4\u8f93\u5165"
	if actionsFocused {
		if combatState.Enabled {
			title = "\u591a\u4eba\u547d\u4ee4\u8f93\u5165 (\u805a\u7126\u6218\u6597\u64cd\u4f5c)"
		} else if combatState.SelectionLabel != "" {
			title = "\u591a\u4eba\u547d\u4ee4\u8f93\u5165 (\u805a\u7126\u9636\u6bb5\u64cd\u4f5c)"
		} else {
			title = "\u591a\u4eba\u547d\u4ee4\u8f93\u5165 (\u805a\u7126\u5efa\u8bae\u64cd\u4f5c)"
		}
	}
	helpText := "Tab \u5728\u5efa\u8bae\u64cd\u4f5c\u548c\u8f93\u5165\u6846\u4e4b\u95f4\u5207\u6362\u3002Enter \u53d1\u9001\u5f53\u524d\u8f93\u5165\u6216\u6267\u884c\u9009\u4e2d\u7684\u64cd\u4f5c\u3002Esc \u79bb\u5f00\u623f\u95f4\u3002"
	if combatState.Enabled {
		helpText = "\u6218\u6597\u4e2d\uff1a\u4e0a/\u4e0b\u9009\u62e9\u624b\u724c\u6216\u836f\u6c34\uff0c\u5de6/\u53f3\u5207\u6362\u76ee\u6807\uff0cEnter \u6267\u884c\uff0cZ \u5207\u6362\u624b\u724c/\u836f\u6c34\u6a21\u5f0f\uff0cE \u7ed3\u675f\u56de\u5408\uff0cTab \u5728\u6218\u6597\u64cd\u4f5c\u3001\u68c0\u89c6\u548c\u804a\u5929\u4e4b\u95f4\u5207\u6362\u3002"
	} else if combatState.SelectionLabel != "" {
		helpText = "\u975e\u6218\u6597\u9636\u6bb5\uff1a\u4e0a/\u4e0b\u9009\u62e9\u5f53\u524d\u9636\u6bb5\u9009\u9879\uff0cEnter \u786e\u8ba4\uff0cS/Esc/L \u53ef\u5728\u5141\u8bb8\u65f6\u7528\u4e8e\u8df3\u8fc7\u3001\u8fd4\u56de\u6216\u79bb\u5f00\uff0cTab \u5728\u9636\u6bb5\u64cd\u4f5c\u548c\u804a\u5929\u4e4b\u95f4\u5207\u6362\u3002"
	}
	lines := []string{
		theme.Accent.Render(title),
		theme.Normal.Render(truncateASCII(value, contentWidth)),
		"",
		theme.Muted.Render(helpText),
	}
	return strings.Join(lines, "\n")
}

func renderMultiplayerPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	if snapshot.Lobby != nil {
		return renderLobbyPhaseLines(theme, snapshot, width)
	}
	if snapshot.Map != nil {
		return renderMapPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Combat != nil {
		return renderCombatPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Reward != nil {
		return renderRewardPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Event != nil {
		return renderEventPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Shop != nil {
		return renderShopPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Rest != nil {
		return renderRestPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Equipment != nil {
		return renderEquipmentPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Deck != nil {
		return renderDeckPhaseLines(theme, snapshot, combatState, width)
	}
	if snapshot.Summary != nil {
		return renderSummaryPhaseLines(theme, snapshot, combatState, width)
	}
	return nil
}

func renderMultiplayerCombatTopSummary(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) string {
	if snapshot == nil || snapshot.Combat == nil {
		return ""
	}
	panelWidth := max(30, width)
	leftWidth, middleWidth, rightWidth, stacked := splitThreeCombatSummaryColumns(panelWidth)
	panelLines := combatTopPanelLineBudget(0, stacked)
	inspectPane := clampSelection(combatState.InspectPane, MultiplayerCombatInspectPaneCount)
	details := []string{
		fmt.Sprintf("\u62bd\u724c\u5806 %d | \u5f03\u724c\u5806 %d | \u6d88\u8017\u5806 %d", snapshot.Combat.DrawCount, snapshot.Combat.DiscardCount, snapshot.Combat.ExhaustCount),
		fmt.Sprintf("\u68c0\u89c6\u9875: %s", localizedCombatInspectPaneName(inspectPane)),
	}
	for _, pending := range snapshot.Combat.PendingRepeats {
		details = append(details, "\u8fde\u53d1\u5f85\u673a | "+pending)
	}
	if combatState.SelectionLabel != "" {
		details = append(details, "\u5f53\u524d\u9009\u62e9 | "+combatState.SelectionLabel)
	}
	if combatState.TargetLabel != "" {
		details = append(details, "\u76ee\u6807 | "+combatState.TargetLabel)
	}
	if len(snapshot.Combat.VoteStatus) > 0 {
		details = append(details, "", "\u56de\u5408\u6295\u7968:")
		for _, status := range snapshot.Combat.VoteStatus {
			details = append(details, status)
		}
	}
	infoPanel := renderCombatSummaryPanel(theme, rightWidth, "\u6218\u6597\u4fe1\u606f", fmt.Sprintf("\u56de\u5408 %d | \u624b\u724c %d | \u80fd\u91cf %d/%d", snapshot.Combat.Turn, len(snapshot.Combat.Hand), snapshot.Combat.Energy, snapshot.Combat.MaxEnergy), details, combatState.TopPage, panelLines)
	name, hp, maxHP, block, status, intent := selectedMultiplayerEnemy(snapshot, combatState)
	seatPanel := renderMultiplayerSeatStatusPanel(theme, snapshot, combatState, leftWidth, panelLines)
	enemyPanel := renderCombatEnemySummaryPanel(theme, name, hp, maxHP, block, status, intent, multiplayerEnemyFocusLabel(snapshot, combatState), middleWidth, panelLines)
	return renderCombatTopThreePanels(seatPanel, enemyPanel, infoPanel, stacked)
}

func multiplayerEnemyFocusLabel(snapshot *netplay.Snapshot, combatState MultiplayerCombatState) string {
	if strings.TrimSpace(combatState.TargetLabel) != "" {
		return combatState.TargetLabel
	}
	if snapshot == nil || snapshot.Combat == nil || len(snapshot.Combat.Enemies) == 0 {
		return ""
	}
	return fmt.Sprintf("\u654c\u65b9 %d", snapshot.Combat.Enemies[0].Index)
}

func splitThreeCombatSummaryColumns(totalWidth int) (int, int, int, bool) {
	leftWidth, middleWidth, rightWidth, stacked := splitFramedThreeColumns(totalWidth, 22, 24, 30, 1, 6)
	return leftWidth, middleWidth, rightWidth, stacked
}

func renderMultiplayerSeatStatusPanel(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, panelWidth int, maxLines int) string {
	if snapshot == nil || snapshot.Combat == nil {
		return ""
	}
	name, classID, hp, maxHP, block, energy, maxEnergy, status, ok := selectedMultiplayerSeatActor(snapshot)
	if !ok {
		return ""
	}
	lines := []string{
		theme.Accent.Render("\u5ea7\u4f4d\u72b6\u6001"),
		theme.Title.Render(styledClassName(theme, name, classID)),
		theme.Normal.Render(fmt.Sprintf("HP %d/%d | \u683c\u6321 %d | \u80fd\u91cf %d/%d", hp, maxHP, block, energy, maxEnergy)),
	}
	if strings.TrimSpace(status) != "" {
		for _, line := range wrapLine("\u72b6\u6001: "+status, panelContentWidth(panelWidth)) {
			lines = append(lines, theme.Normal.Render(line))
		}
	} else {
		lines = append(lines, theme.Muted.Render("\u72b6\u6001: \u65e0"))
	}
	lines = append(lines, theme.Muted.Render(fmt.Sprintf("\u836f\u6c34 %d | \u724c\u7ec4 %d", len(snapshot.Combat.Potions), snapshot.Combat.DeckSize)))
	for _, pending := range snapshot.Combat.PendingRepeats {
		lines = append(lines, theme.Selected.Width(panelContentWidth(panelWidth)).Render(truncateASCII("\u8fde\u53d1\u5f85\u673a: "+pending, panelContentWidth(panelWidth))))
	}
	lines = append(lines, "")
	appendStatusChipLine(&lines, theme, panelContentWidth(panelWidth), "\u76ee\u6807", combatState.TargetLabel, true)
	lines = fixedPanelLines(lines, maxLines, theme, panelContentWidth(panelWidth))
	return theme.PanelAlt.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func selectedMultiplayerSeatActor(snapshot *netplay.Snapshot) (string, string, int, int, int, int, int, string, bool) {
	if snapshot == nil || snapshot.Combat == nil {
		return "", "", 0, 0, 0, 0, 0, "", false
	}
	for _, actor := range snapshot.Combat.Party {
		if actor.Index == snapshot.Seat {
			return actor.Name, actor.ClassID, actor.HP, actor.MaxHP, actor.Block, actor.Energy, actor.MaxEnergy, actor.Status, true
		}
	}
	if len(snapshot.Combat.Party) == 0 {
		return "", "", 0, 0, 0, 0, 0, "", false
	}
	actor := snapshot.Combat.Party[0]
	return actor.Name, actor.ClassID, actor.HP, actor.MaxHP, actor.Block, actor.Energy, actor.MaxEnergy, actor.Status, true
}

func selectedMultiplayerEnemy(snapshot *netplay.Snapshot, combatState MultiplayerCombatState) (string, int, int, int, string, string) {
	if snapshot == nil || snapshot.Combat == nil || len(snapshot.Combat.Enemies) == 0 {
		return "", 0, 0, 0, "", ""
	}
	selected := snapshot.Combat.Enemies[0]
	if combatState.TargetKind == "enemy" {
		for _, enemy := range snapshot.Combat.Enemies {
			if enemy.Index == combatState.TargetIndex {
				selected = enemy
				break
			}
		}
	}
	return selected.Name, selected.HP, selected.MaxHP, selected.Block, selected.Status, selected.Intent
}

func snapshotTitle(snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return "\u591a\u4eba\u623f\u95f4"
	}
	if snapshot.PhaseTitle != "" {
		return localizedMultiplayerPhaseLabel(snapshot.PhaseTitle)
	}
	return "\u591a\u4eba\u623f\u95f4"
}

func localizedMultiplayerPhaseLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "room":
		return "房间"
	case "lobby":
		return "大厅"
	case "map":
		return "地图"
	case "combat":
		return "战斗"
	case "reward":
		return "奖励"
	case "event":
		return "事件"
	case "shop":
		return "商店"
	case "rest":
		return "篝火"
	case "equipment":
		return "装备"
	case "deck":
		return "牌组"
	case "summary":
		return "结算"
	default:
		return raw
	}
}

func roomHostPort(addr string) (string, string) {
	if strings.TrimSpace(addr) == "" {
		return "", ""
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return host, port
}

func tailText(items []string, limit int) []string {
	if len(items) <= limit {
		return items
	}
	return items[len(items)-limit:]
}

func lipJoinPanels(stacked bool, left, right string) string {
	if stacked {
		return lipgloss.JoinVertical(lipgloss.Left, left, right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}
