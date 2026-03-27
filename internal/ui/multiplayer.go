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
		return "Room"
	}
	if strings.TrimSpace(snapshot.PhaseTitle) != "" {
		return snapshot.PhaseTitle
	}
	if strings.TrimSpace(snapshot.Phase) != "" {
		phase := strings.ToLower(snapshot.Phase)
		if len(phase) == 0 {
			return "Room"
		}
		return strings.ToUpper(phase[:1]) + phase[1:]
	}
	return "Room"
}

func multiplayerPhaseMetaLines(theme Theme, snapshot *netplay.Snapshot) []string {
	if snapshot == nil {
		return nil
	}
	parts := []string{theme.Chip.Render("Phase " + multiplayerPhaseLabel(snapshot))}
	if strings.TrimSpace(snapshot.RoomAddr) != "" {
		if host, port := roomHostPort(snapshot.RoomAddr); host != "" {
			parts = append(parts, theme.Chip.Render("IP "+host))
			if port != "" {
				parts = append(parts, theme.Chip.Render("Port "+port))
			}
		}
	}
	if snapshot.Seat > 0 {
		parts = append(parts, theme.Chip.Render(fmt.Sprintf("Seat %d", snapshot.Seat)))
	}
	if snapshot.SelfID != "" && snapshot.SelfID == snapshot.HostID {
		parts = append(parts, theme.Good.Render("Host"))
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
	parts = append(parts, styledMultiplayerOfferName(theme, kind, category, name), "|", fmt.Sprintf("%d gold", price))
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
		lines = append(lines, "", theme.Accent.Render("Tips"))
		for _, tip := range tips {
			for _, line := range wrapLine("- "+tip, contentWidth) {
				lines = append(lines, theme.Muted.Render(line))
			}
		}
	}
	lines = append(lines, "", theme.Accent.Render("Current Setup"))
	for i, item := range items {
		line := truncateASCII(item, contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render(strings.Join([]string{"Enter confirms current selection", "Left/Right switch class or toggle", "Esc goes back", "Ctrl+C quits"}, " | ")))
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
	return strings.Join([]string{theme.Title.Render("Multiplayer Room"), focusBar, keyBar, body, inputPanel}, "\n\n")
}

func RenderMultiplayerMapTreeOverlay(theme Theme, snapshot *netplay.Snapshot, width int, height int) string {
	if snapshot == nil {
		return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(theme.Muted.Render("No shared map is available right now."))
	}
	mapData := snapshot.Map
	if mapData == nil {
		mapData = snapshot.SharedMap
	}
	if mapData == nil {
		return theme.Panel.Width(fitPanelWidth(width, 72, 4)).Render(theme.Muted.Render("No shared map is available right now."))
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
	position := fmt.Sprintf("Shared position: next floor %d", mapData.NextFloor)
	return renderMapTreeOverlayPanel(theme, fmt.Sprintf("Shared Map Overlay Act %d", mapData.Act), position, reachableIDs, layers, mapData.CurrentNodeID, mapData.CurrentFloor, width, height)
}

func renderMultiplayerFocusBar(theme Theme, snapshot *netplay.Snapshot, actionsFocused bool, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	title := theme.Accent.Render("Focus")
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
		appendItem("Combat Ops", combatState.OperationsFocused)
		appendItem("Combat Inspect", combatState.InspectFocused)
		appendItem("Chat Input", combatState.ChatFocused)
		return items
	}
	if usesMultiplayerStructuredSidebar(snapshot) {
		appendItem("Phase Ops", !combatState.ChatFocused)
		appendItem("Chat Input", combatState.ChatFocused)
		return items
	}
	appendItem("Quick Actions", actionsFocused)
	appendItem("Chat Input", !actionsFocused)
	return items
}

func renderMultiplayerKeyBar(theme Theme, snapshot *netplay.Snapshot, actionsFocused bool, combatState MultiplayerCombatState, width int) string {
	hints := multiplayerKeyHints(snapshot, actionsFocused, combatState)
	if len(hints) == 0 {
		return ""
	}
	contentWidth := panelContentWidth(width)
	lines := []string{theme.Accent.Render("Keys")}
	for _, line := range wrapLine(strings.Join(hints, " | "), contentWidth) {
		lines = append(lines, theme.Muted.Render(line))
	}
	return theme.PanelAlt.Width(max(24, width)).Render(strings.Join(lines, "\n"))
}

func multiplayerKeyHints(snapshot *netplay.Snapshot, actionsFocused bool, combatState MultiplayerCombatState) []string {
	if snapshot != nil && snapshot.Combat != nil {
		if combatState.InspectFocused {
			return []string{"up/down/left/right: switch inspect panes", "[/]: page top summary", "./,: page logs", "Tab: switch to chat", "Enter: keep current focus"}
		}
		if combatState.ChatFocused {
			return []string{"type text", "Enter: send command", "Tab: back to combat ops"}
		}
		return []string{"up/down: select hand or potion", "left/right: cycle target", "[/]: page top summary", "Enter: confirm", "z: hand/potion", "e: end turn", "Tab: inspect/chat"}
	}
	if usesMultiplayerStructuredSidebar(snapshot) {
		if combatState.ChatFocused {
			return []string{"type text", "Enter: send command", "Tab: back to phase ops"}
		}
		return []string{"up/down: phase selection", "Enter: confirm", "Tab: switch to chat", "Esc/s/l: back or skip"}
	}
	if actionsFocused {
		return []string{"up/down: quick actions", "Enter: use action", "Tab: switch to chat"}
	}
	return []string{"type text", "Enter: send command", "Tab: back to quick actions"}
}

func renderMultiplayerMain(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	if snapshot == nil {
		return strings.Join([]string{
			theme.Accent.Render("Connecting to room"),
			theme.Muted.Render("Room details, phase state, and party information will appear here once connected."),
		}, "\n")
	}
	lines := []string{theme.Accent.Render(snapshotTitle(snapshot))}
	lines = append(lines, theme.Subtitle.Render(fmt.Sprintf("Room %s | Phase %s", snapshot.RoomAddr, multiplayerPhaseLabel(snapshot))))
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
		for _, line := range wrapLine("Notice: "+snapshot.Banner, contentWidth) {
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
		for _, line := range wrapLine("Your role: "+snapshot.RoleNote, contentWidth) {
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
	lines := []string{theme.Subtitle.Render("Identity")}
	if identity := multiplayerIdentityLine(theme, snapshot); identity != "" {
		for _, line := range wrapLine(identity, width) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if control := multiplayerControlSummary(snapshot); control != "" {
		for _, line := range wrapLine("Control: "+control, width) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	return lines
}

func multiplayerIdentityLine(theme Theme, snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	role := "Player"
	if snapshot.SelfID != "" && snapshot.SelfID == snapshot.HostID {
		role = "Host"
	}
	seat := snapshot.Seat
	name := "Current Player"
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
		parts = append(parts, fmt.Sprintf("Seat %d", seat))
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
			return "Host can configure the room and start the run."
		}
		return "Players can set class, ready up, and chat while waiting for the host."
	case snapshot.Map != nil:
		if snapshot.ControlLabel == "Route vote submitted" {
			return "Your route vote is locked. Waiting for the remaining connected seats."
		}
		return "Every connected seat votes on the next route before the room resolves it."
	case snapshot.Combat != nil:
		return "Every connected seat can play cards, use potions, and vote to end the turn."
	case snapshot.Reward != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Personal reward choice" {
			return "Resolve your own reward choice, then wait for the other seats."
		}
		return "Your reward is done. Waiting for the other seats to finish."
	case snapshot.Event != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Personal event choice" {
			return "Resolve your own event choice. The shared map advances after everyone finishes."
		}
		return "Your event is done. Waiting for the other seats to finish."
	case snapshot.Shop != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Personal shop" {
			return "You control your own shop purchases and when you leave the shop."
		}
		return "Your shop phase is done. Waiting for the other seats to leave their shops."
	case snapshot.Rest != nil:
		if isHost {
			return "Host chooses the campfire action."
		}
		return "You can suggest a campfire action, but the host confirms it."
	case snapshot.Equipment != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Equipment replacement" {
			return "You decide whether to replace the current equipment."
		}
		return "Waiting for another seat to finish equipment replacement."
	case snapshot.Deck != nil:
		if snapshot.ControlLabel == "" || snapshot.ControlLabel == "Deck action" {
			return "You are resolving the current deck action."
		}
		return "Waiting for another seat to finish a deck action."
	case snapshot.Summary != nil:
		if isHost {
			return "Host decides whether to start a new run or close the room."
		}
		return "Waiting for the host to decide the next step."
	default:
		if snapshot.ControlLabel != "" {
			return snapshot.ControlLabel
		}
		return ""
	}
}

func renderLobbyPhaseLines(theme Theme, snapshot *netplay.Snapshot, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Mode: ", fmt.Sprintf("%s | Seed %d", snapshot.Lobby.Mode, snapshot.Lobby.Seed))
	appendSectionTitle(&lines, theme, "Room Members")
	for _, player := range snapshot.Players {
		statusLabel := "not ready"
		statusTone := "muted"
		if player.Ready {
			statusLabel = "ready"
			statusTone = "good"
		}
		if !player.Connected {
			statusLabel = "offline"
			statusTone = "bad"
		}
		memberParts := []string{
			fmt.Sprintf("Seat %d", player.Seat),
			styledClassName(theme, player.Name, player.ClassID),
		}
		if strings.TrimSpace(player.ClassID) != "" {
			memberParts = append(memberParts, multiplayerToneChip(theme, strings.ToUpper(player.ClassID), "accent"))
		}
		memberParts = append(memberParts, multiplayerToneChip(theme, strings.ToUpper(statusLabel), statusTone))
		if player.ID == snapshot.HostID {
			memberParts = append(memberParts, multiplayerToneChip(theme, "HOST", "accent"))
		}
		if player.ID == snapshot.SelfID {
			memberParts = append(memberParts, multiplayerToneChip(theme, "YOU", "selected"))
		}
		appendWrappedLine(&lines, theme, width, "- ", strings.Join(memberParts, " "))
	}
	return lines
}

func renderMapPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	appendWrappedLine(&lines, theme, width, "Map: ", fmt.Sprintf("Act %d | Next floor %d | Gold %d", snapshot.Map.Act, snapshot.Map.NextFloor, snapshot.Map.Gold))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "Current Selection")
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
	}
	appendSectionTitle(&lines, theme, "Party")
	for _, actor := range snapshot.Map.Party {
		text := combatActorText(styledClassName(theme, actor.Name, actor.ClassID), actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status)
		appendWrappedLine(&lines, theme, width, "- ", text)
	}
	appendSectionTitle(&lines, theme, "Reachable Nodes")
	for i, node := range snapshot.Map.Reachable {
		text := fmt.Sprintf("%d. %s %s", node.Index, multiplayerToneChip(theme, strings.ToUpper(node.Kind), "accent"), node.Label)
		if i == combatState.SelectedIndex {
			text = theme.Selected.Render(text)
		}
		appendWrappedLine(&lines, theme, width, "- ", text)
	}
	if len(snapshot.Map.VoteSummary) > 0 {
		appendSectionTitle(&lines, theme, "Vote Weight")
		for _, item := range snapshot.Map.VoteSummary {
			appendWrappedLine(&lines, theme, width, "- ", applyStyledTokens(item, logTokens))
		}
	}
	if len(snapshot.Map.VoteStatus) > 0 {
		appendSectionTitle(&lines, theme, "Seat Votes")
		for _, item := range snapshot.Map.VoteStatus {
			appendWrappedLine(&lines, theme, width, "- ", applyStyledTokens(item, logTokens))
		}
	}
	if len(snapshot.Map.History) > 0 {
		appendSectionTitle(&lines, theme, "Recent Progress")
		for _, item := range tailText(snapshot.Map.History, multiplayerRecentLogCount(width)) {
			appendWrappedLine(&lines, theme, width, "- ", applyStyledTokens(item, logTokens))
		}
	}
	return lines
}

func renderCombatPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Combat: ", fmt.Sprintf("Turn %d | Energy %d/%d", snapshot.Combat.Turn, snapshot.Combat.Energy, snapshot.Combat.MaxEnergy))
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	for _, pending := range snapshot.Combat.PendingRepeats {
		lines = append(lines, theme.Selected.Render(truncateASCII("Pending repeat: "+pending, width)))
	}
	if summary := renderMultiplayerCombatTopSummary(theme, snapshot, combatState, width); summary != "" {
		lines = append(lines, summary)
	}
	if combatState.Enabled {
		appendCompactSectionTitle(&lines, theme, "Current Action")
		appendStatusChipLine(&lines, theme, width, "Mode", combatState.ModeLabel, combatState.OperationsFocused)
		appendStatusChipLine(&lines, theme, width, "Target", combatState.TargetLabel, combatState.OperationsFocused)
		if combatState.OperationsFocused {
			lines = append(lines, theme.Good.Render("Combat controls are focused: arrows and Enter act on the current combat choice."))
		} else if combatState.InspectFocused {
			lines = append(lines, theme.Good.Render("Combat inspect is focused: arrows switch inspect panes on the right sidebar."))
		} else {
			lines = append(lines, theme.Muted.Render("Chat input is focused. Press Tab to return to combat controls or inspect."))
		}
	}
	partyItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Party))
	for _, actor := range snapshot.Combat.Party {
		name := styledClassName(theme, actor.Name, actor.ClassID)
		if snapshot.Seat == actor.Index {
			name += " [you]"
		}
		selected := combatState.TargetKind == "ally" && combatState.TargetIndex == actor.Index
		if selected {
			name += " [target]"
		}
		partyItems = append(partyItems, selectableSectionItem{
			Text:     combatActorText(name, actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status),
			Selected: selected,
		})
	}
	appendCompactCombatSelectionSection(&lines, theme, width, "Allies", "No visible allies right now.", partyItems, true)
	enemyItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Enemies))
	for _, enemy := range snapshot.Combat.Enemies {
		selected := combatState.TargetKind == "enemy" && combatState.TargetIndex == enemy.Index
		name := styledEnemyName(theme, enemy.Name)
		if selected {
			name += " [target]"
		}
		enemyItems = append(enemyItems, selectableSectionItem{
			Text:     combatEnemyText(enemy.Index, name, enemy.HP, enemy.MaxHP, enemy.Block, enemy.Status, enemy.Intent),
			Selected: selected,
		})
	}
	appendCompactCombatSelectionSection(&lines, theme, width, "Enemies", "No visible enemies right now.", enemyItems, true)
	if len(snapshot.Combat.Hand) > 0 {
		handItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Hand))
		for i, card := range snapshot.Combat.Hand {
			selected := combatState.ModeLabel == "手牌" && i == combatState.SelectedCard
			name := styledCardName(theme, card.Name, card.Kind)
			if selected {
				name += " [current]"
			}
			handItems = append(handItems, selectableSectionItem{
				Text:     combatCardText(card.Index, card.Cost, name, card.Summary, card.TargetHint),
				Selected: selected,
			})
		}
		appendCompactCombatSelectionSection(&lines, theme, width, "Hand", "No playable cards right now.", handItems, true)
	}
	if len(snapshot.Combat.Potions) > 0 {
		potionItems := make([]selectableSectionItem, 0, len(snapshot.Combat.Potions))
		for i, potion := range snapshot.Combat.Potions {
			selected := combatState.ModeLabel == "药水" && i == combatState.SelectedPotion
			text := fmt.Sprintf("%d. %s", i+1, potion)
			if selected {
				text += " [current]"
			}
			potionItems = append(potionItems, selectableSectionItem{
				Text:     text,
				Selected: selected,
			})
		}
		appendCompactCombatSelectionSection(&lines, theme, width, "Potions", "No potions available right now.", potionItems, true)
	}
	if len(snapshot.Combat.Logs) > 0 {
		recent := tailText(snapshot.Combat.Logs, multiplayerRecentLogCount(width))
		lines = append(lines, "")
		appendCompactSectionTitle(&lines, theme, "Recent Actions")
		appendStyledEntries(&lines, theme, width, recent, logTokens, theme.Muted)
		if hidden := len(snapshot.Combat.Logs) - len(recent); hidden > 0 {
			lines = append(lines, theme.Muted.Render(truncateASCII(fmt.Sprintf("Showing latest %d. Open Inspect > Logs for %d more.", len(recent), hidden), width)))
		}
	}
	return lines
}

func renderRewardPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Reward: ", fmt.Sprintf("Gold +%d | Source %s", snapshot.Reward.Gold, snapshot.Reward.Source))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "Current Selection")
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
	}
	for i, card := range snapshot.Reward.Cards {
		text := multiplayerCardChoiceLine(theme, card.Index, card.Kind, card.Name, card.Summary, card.Badges)
		appendSelectableBulletLine(&lines, theme, width, text, i == combatState.SelectedIndex)
	}
	if strings.TrimSpace(snapshot.Reward.Relic) != "" {
		appendSectionTitle(&lines, theme, "Relic")
		appendWrappedLine(&lines, theme, width, "- ", snapshot.Reward.Relic+" "+renderBadgeChips(theme, snapshot.Reward.RelicBadges))
	}
	if strings.TrimSpace(snapshot.Reward.Potion) != "" {
		appendSectionTitle(&lines, theme, "Potion")
		appendWrappedLine(&lines, theme, width, "- ", multiplayerToneChip(theme, "POTION", "good")+" "+snapshot.Reward.Potion)
	}
	return lines
}

func renderEventPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Event: ", snapshot.Event.Name)
	appendWrappedLine(&lines, theme, width, "Details: ", snapshot.Event.Description)
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "Current Selection")
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
	}
	appendSectionTitle(&lines, theme, "Choices")
	for i, choice := range snapshot.Event.Choices {
		text := multiplayerChoiceLine(theme, choice.Index, theme.Accent.Render(choice.Label), choice.Description, choice.Badges)
		appendSelectableBulletLine(&lines, theme, width, text, i == combatState.SelectedIndex)
	}
	if chips := renderBadgeChips(theme, snapshot.Event.Badges); chips != "" {
		appendSectionTitle(&lines, theme, "Event Traits")
		appendWrappedLine(&lines, theme, width, "- ", chips)
	}
	return lines
}

func renderShopPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Shop: ", fmt.Sprintf("Gold %d", snapshot.Shop.Gold))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "Current Selection")
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
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
		lines = append(lines, theme.Subtitle.Render("Current Selection"))
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
		lines = append(lines, "")
	}
	lines = append(lines, theme.Subtitle.Render("Campfire Party State"))
	for _, actor := range snapshot.Rest.Party {
		appendWrappedLine(&lines, theme, width, "- ", combatActorText(styledClassName(theme, actor.Name, actor.ClassID), actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status))
	}
	return lines
}

func renderEquipmentPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Equipment Choice: ", fmt.Sprintf("%s | Slot %s", snapshot.Equipment.CandidateName, snapshot.Equipment.Slot))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "Current Selection")
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
	}
	appendWrappedLine(&lines, theme, width, "Candidate: ", theme.Accent.Render(snapshot.Equipment.CandidateName)+" | "+snapshot.Equipment.CandidateDescription)
	if snapshot.Equipment.CurrentName != "" {
		appendWrappedLine(&lines, theme, width, "Current: ", theme.Muted.Render(snapshot.Equipment.CurrentName)+" | "+snapshot.Equipment.CurrentDescription)
	}
	return lines
}

func renderDeckPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Deck Action: ", snapshot.Deck.Title)
	appendWrappedLine(&lines, theme, width, "Details: ", snapshot.Deck.Subtitle)
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "Current Selection")
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
	}
	for i, card := range snapshot.Deck.Cards {
		text := multiplayerCardChoiceLine(theme, card.Index, card.Kind, card.Name, card.Summary, card.Badges)
		appendSelectableBulletLine(&lines, theme, width, text, i == combatState.SelectedIndex)
	}
	return lines
}

func renderSummaryPhaseLines(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) []string {
	lines := []string{}
	appendWrappedLine(&lines, theme, width, "Summary: ", fmt.Sprintf("%s | %s | Act %d | Floors %d", snapshot.Summary.Result, snapshot.Summary.Mode, snapshot.Summary.Act, snapshot.Summary.Floors))
	if combatState.SelectionLabel != "" {
		appendSectionTitle(&lines, theme, "Current Selection")
		appendWrappedLine(&lines, theme, width, "Choice: ", combatState.SelectionLabel)
	}
	for _, actor := range snapshot.Summary.Party {
		appendWrappedLine(&lines, theme, width, "- ", combatActorText(styledClassName(theme, actor.Name, actor.ClassID), actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status))
	}
	return lines
}

func renderMultiplayerSidebar(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	if snapshot == nil {
		return theme.Muted.Render("Waiting for the room snapshot...")
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
	lines := []string{theme.Accent.Render("Phase Controls")}
	if combatState.ChatFocused {
		lines = append(lines, theme.Muted.Render("Focus: chat input"))
	} else if len(multiplayerStructuredSidebarOptions(theme, snapshot)) > 0 {
		lines = append(lines, theme.Good.Render("Focus: phase controls"))
	} else {
		lines = append(lines, theme.Muted.Render("Focus: room details"))
	}
	options := multiplayerStructuredSidebarOptions(theme, snapshot)
	if len(options) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Available Actions"))
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
		appendStatusChipLine(&lines, theme, contentWidth, "Selection", combatState.SelectionLabel, !combatState.ChatFocused)
	}
	lines = append(lines, "", theme.Subtitle.Render("Key Hints"))
	for _, item := range []string{
		"Up/Down: change the current phase selection",
		"Enter: confirm the current phase selection",
		"Tab: switch between phase controls and chat input",
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
			options = append(options, fmt.Sprintf("Node %d: %s", node.Index, node.Label))
		}
		return options
	case snapshot.Reward != nil:
		options := make([]string, 0, len(snapshot.Reward.Cards))
		for _, card := range snapshot.Reward.Cards {
			options = append(options, fmt.Sprintf("Reward card: %s", styledCardName(theme, card.Name, card.Kind)))
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
			options = append(options, fmt.Sprintf("%s | %d gold", styledMultiplayerOfferName(theme, offer.Kind, offer.Category, offer.Name), offer.Price))
		}
		return options
	case snapshot.Rest != nil:
		return []string{theme.Good.Render("Rest"), theme.Accent.Render("Upgrade")}
	case snapshot.Equipment != nil:
		return []string{theme.Selected.Render("Take candidate equipment"), theme.Muted.Render("Skip equipment")}
	case snapshot.Deck != nil:
		options := make([]string, 0, len(snapshot.Deck.Cards))
		for _, card := range snapshot.Deck.Cards {
			options = append(options, fmt.Sprintf("%s | %s", styledCardName(theme, card.Name, card.Kind), card.Summary))
		}
		if len(options) == 0 {
			return []string{"Return to previous screen"}
		}
		return options
	case snapshot.Summary != nil:
		return []string{"Start a new run", "Close the room"}
	default:
		return nil
	}
}

func appendStructuredSidebarDetailSections(lines *[]string, theme Theme, snapshot *netplay.Snapshot, contentWidth int) {
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	if len(snapshot.WaitingOn) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("Waiting"))
		for _, item := range snapshot.WaitingOn {
			for _, line := range wrapLine("- "+item, contentWidth) {
				*lines = append(*lines, theme.Bad.Render(line))
			}
		}
	}
	if len(snapshot.Commands) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("Commands"))
		for _, item := range snapshot.Commands {
			for _, line := range wrapLine("- "+item, contentWidth) {
				*lines = append(*lines, theme.Normal.Render(line))
			}
		}
	}
	if len(snapshot.SeatStatus) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("Seats"))
		appendStyledEntries(lines, theme, contentWidth, snapshot.SeatStatus, logTokens, theme.Normal)
	}
	if len(snapshot.RoomLog) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("Room Log"))
		appendStyledEntries(lines, theme, contentWidth, tailText(snapshot.RoomLog, multiplayerRecentLogCount(contentWidth)), logTokens, theme.Muted)
	}
	if len(snapshot.ChatLog) > 0 {
		*lines = append(*lines, "", theme.Subtitle.Render("Chat"))
		appendStyledEntries(lines, theme, contentWidth, tailText(snapshot.ChatLog, max(3, multiplayerRecentLogCount(contentWidth)-2)), logTokens, theme.Muted)
	}
}

func renderCombatOperationsSidebar(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) string {
	lines := []string{theme.Accent.Render("Combat Controls")}
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	if combatState.OperationsFocused {
		lines = append(lines, theme.Good.Render("Focus: combat controls"))
	} else if combatState.ChatFocused {
		lines = append(lines, theme.Muted.Render("Focus: chat input"))
	} else {
		lines = append(lines, theme.Muted.Render("Focus: combat inspect"))
	}
	if combatState.ModeLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "Mode", combatState.ModeLabel, combatState.OperationsFocused)
	}
	if combatState.SelectionLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "Selection", combatState.SelectionLabel, combatState.OperationsFocused)
	}
	if combatState.TargetLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "Target", combatState.TargetLabel, combatState.OperationsFocused)
	}
	for _, pending := range snapshot.Combat.PendingRepeats {
		lines = append(lines, theme.Selected.Render(truncateASCII("Pending repeat: "+pending, contentWidth)))
	}
	lines = append(lines, "", theme.Subtitle.Render("Key Hints"))
	for _, item := range []string{
		"Up/Down: choose a card or potion",
		"Left/Right: cycle valid targets",
		"Enter: execute the current action",
		"Z: switch between hand and potions",
		"E: submit end turn",
		"Tab: combat controls / combat inspect / chat input",
	} {
		for _, line := range wrapLine("- "+item, contentWidth) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if len(snapshot.Combat.VoteStatus) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Turn Votes"))
		appendStyledEntries(&lines, theme, contentWidth, snapshot.Combat.VoteStatus, logTokens, theme.Muted)
	}
	if len(snapshot.SeatStatus) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Team Status"))
		appendStyledEntries(&lines, theme, contentWidth, snapshot.SeatStatus, logTokens, theme.Normal)
	}
	if len(snapshot.Combat.Logs) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Recent Actions"))
		recent := tailText(snapshot.Combat.Logs, multiplayerRecentLogCount(contentWidth))
		appendStyledEntries(&lines, theme, contentWidth, recent, logTokens, theme.Muted)
		if hidden := len(snapshot.Combat.Logs) - len(recent); hidden > 0 {
			lines = append(lines, theme.Muted.Render(truncateASCII(fmt.Sprintf("Showing latest %d. Inspect > Logs has %d more.", len(recent), hidden), contentWidth)))
		}
	}
	if len(snapshot.Commands) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Commands"))
		for _, item := range snapshot.Commands {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderCombatInspectSidebar(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) string {
	lines := []string{theme.Accent.Render("Combat Inspect")}
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
	tabs := []string{"Overview", "Piles", "Effects", "Logs", "Votes"}
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
		return append(lines, "", theme.Good.Render("Inspect focus is active: use Left/Right or Up/Down to switch panes, ,/. to page logs, and Tab to move to chat."))
	}
	return append(lines, "", theme.Muted.Render("Press Tab to move focus to inspect and review more combat details."))
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
		appendStatusChipLine(&lines, theme, contentWidth, "Selection", combatState.SelectionLabel, combatState.InspectFocused)
	}
	if combatState.TargetLabel != "" {
		appendStatusChipLine(&lines, theme, contentWidth, "Target", combatState.TargetLabel, combatState.InspectFocused)
	}
	for _, item := range snapshot.Combat.Highlights {
		for _, line := range wrapLine("- "+item, contentWidth) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	for _, pending := range snapshot.Combat.PendingRepeats {
		for _, line := range wrapLine("- Pending repeat: "+pending, contentWidth) {
			lines = append(lines, theme.Selected.Render(line))
		}
	}
	for _, text := range []string{
		fmt.Sprintf("Turn %d | Energy %d/%d", snapshot.Combat.Turn, snapshot.Combat.Energy, snapshot.Combat.MaxEnergy),
		fmt.Sprintf("Deck %d | Draw %d | Discard %d | Exhaust %d", snapshot.Combat.DeckSize, snapshot.Combat.DrawCount, snapshot.Combat.DiscardCount, snapshot.Combat.ExhaustCount),
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
		{title: "Draw Pile", rows: snapshot.Combat.DrawPile},
		{title: "Discard Pile", rows: snapshot.Combat.DiscardPile},
		{title: "Exhaust Pile", rows: snapshot.Combat.ExhaustPile},
	}
	for idx, section := range sections {
		if idx > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, theme.Subtitle.Render(section.title))
		lines = appendCombatInspectEntries(lines, theme, contentWidth, section.rows, section.title+" is empty.", false)
	}
	return lines
}

func appendCombatInspectEffectsPane(lines []string, theme Theme, snapshot *netplay.Snapshot, contentWidth int) []string {
	return appendCombatInspectEntries(lines, theme, contentWidth, snapshot.Combat.Effects, "No visible persistent effects right now.", false)
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
		body = append(body, theme.Muted.Render("No combat logs yet."))
	}
	pageCount := fixedRecentPanelPageCount(body, maxLines)
	clampedPage := clampFixedRecentPanelPage(combatState.InspectLogPage, body, maxLines)
	title := "Logs [" + mode + "]"
	if pageCount > 1 {
		currentPage := pageCount - clampedPage
		title += fmt.Sprintf(" [%d/%d]", currentPage, pageCount)
	}
	lines = append(lines, theme.Subtitle.Render(title))
	return append(lines, fixedRecentPanelBodyLines(body, clampedPage, maxLines, theme, contentWidth)...)
}

func multiplayerInspectLogMode(contentWidth int) (string, int) {
	if contentWidth < 42 {
		return "Compact", 6
	}
	if contentWidth < 58 {
		return "Expanded", 9
	}
	return "Expanded", 12
}

func appendCombatInspectVotePane(lines []string, theme Theme, snapshot *netplay.Snapshot, contentWidth int) []string {
	lines = appendStyledInspectEntries(lines, theme, contentWidth, snapshot.Combat.VoteStatus, multiplayerCombatLogTokens(theme, snapshot), "No active vote status right now.", false)
	if len(snapshot.Combat.Potions) == 0 {
		return lines
	}
	lines = append(lines, "", theme.Subtitle.Render("Potions"))
	potions := make([]string, 0, len(snapshot.Combat.Potions))
	for i, potion := range snapshot.Combat.Potions {
		potions = append(potions, fmt.Sprintf("%d. %s", i+1, potion))
	}
	return appendCombatInspectEntries(lines, theme, contentWidth, potions, "", true)
}

func renderDefaultMultiplayerSidebar(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, contentWidth int) string {
	lines := []string{theme.Accent.Render("Room Assistant")}
	logTokens := multiplayerCombatLogTokens(theme, snapshot)
	if len(actions) > 0 {
		selectedAction = clampSelection(selectedAction, len(actions))
		lines = append(lines, theme.Subtitle.Render("Suggested Actions"))
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
		lines = append(lines, "", theme.Muted.Render("Suggested actions mirror the room state. Host-only actions are labeled explicitly, and template commands can be filled directly into the input box."))
	}
	if len(snapshot.Commands) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Commands"))
		for _, item := range snapshot.Commands {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	if len(snapshot.WaitingOn) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Waiting On"))
		for _, item := range snapshot.WaitingOn {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Bad.Render(line))
			}
		}
	}
	if len(snapshot.SeatStatus) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Seat Status"))
		appendStyledEntries(&lines, theme, contentWidth, snapshot.SeatStatus, logTokens, theme.Normal)
	}
	if len(snapshot.ChatLog) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("Recent Chat"))
		appendStyledEntries(&lines, theme, contentWidth, tailText(snapshot.ChatLog, multiplayerRecentLogCount(contentWidth)), logTokens, theme.Muted)
	}
	if len(snapshot.RoomLog) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("System Log"))
		appendStyledEntries(&lines, theme, contentWidth, tailText(snapshot.RoomLog, multiplayerRecentLogCount(contentWidth)), logTokens, theme.Muted)
	}
	return strings.Join(lines, "\n")
}

func renderMultiplayerInput(theme Theme, input string, combatState MultiplayerCombatState, actionsFocused bool, width int, _ int) string {
	contentWidth := panelContentWidth(width)
	value := strings.TrimSpace(input)
	if value == "" {
		value = "Examples: ready / chat hello / node 1 / play 1 enemy 1"
	}
	title := "Multiplayer Command Input"
	if actionsFocused {
		if combatState.Enabled {
			title = "Multiplayer Command Input (combat controls focused)"
		} else if combatState.SelectionLabel != "" {
			title = "Multiplayer Command Input (phase controls focused)"
		} else {
			title = "Multiplayer Command Input (suggested actions focused)"
		}
	}
	helpText := "Tab switches between suggested actions and the input box. Enter sends the current input or executes the selected action. Esc leaves the room."
	if combatState.Enabled {
		helpText = "During combat: Up/Down selects cards or potions, Left/Right changes targets, Enter acts, Z switches hand or potion mode, E ends the turn, and Tab cycles combat controls, inspect, and chat."
	} else if combatState.SelectionLabel != "" {
		helpText = "Outside combat: Up/Down selects the current phase option, Enter confirms it, and S/Esc/L can skip, go back, or leave when allowed. Tab switches between phase controls and chat."
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
		fmt.Sprintf("Draw %d | Discard %d | Exhaust %d", snapshot.Combat.DrawCount, snapshot.Combat.DiscardCount, snapshot.Combat.ExhaustCount),
		fmt.Sprintf("Inspect Pane: %s", engine.CombatInspectPaneName(inspectPane)),
	}
	for _, pending := range snapshot.Combat.PendingRepeats {
		details = append(details, "Pending repeat | "+pending)
	}
	if combatState.SelectionLabel != "" {
		details = append(details, "Selection | "+combatState.SelectionLabel)
	}
	if combatState.TargetLabel != "" {
		details = append(details, "Target | "+combatState.TargetLabel)
	}
	if len(snapshot.Combat.VoteStatus) > 0 {
		details = append(details, "", "Turn Votes:")
		for _, status := range snapshot.Combat.VoteStatus {
			details = append(details, status)
		}
	}
	infoPanel := renderCombatSummaryPanel(theme, rightWidth, "Combat Info", fmt.Sprintf("Turn %d | Hand %d | Energy %d/%d", snapshot.Combat.Turn, len(snapshot.Combat.Hand), snapshot.Combat.Energy, snapshot.Combat.MaxEnergy), details, combatState.TopPage, panelLines)
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
	return fmt.Sprintf("Enemy %d", snapshot.Combat.Enemies[0].Index)
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
		theme.Accent.Render("Seat Status"),
		theme.Title.Render(styledClassName(theme, name, classID)),
		theme.Normal.Render(fmt.Sprintf("HP %d/%d | Block %d | Energy %d/%d", hp, maxHP, block, energy, maxEnergy)),
	}
	if strings.TrimSpace(status) != "" {
		for _, line := range wrapLine("Status: "+status, panelContentWidth(panelWidth)) {
			lines = append(lines, theme.Normal.Render(line))
		}
	} else {
		lines = append(lines, theme.Muted.Render("Status: none"))
	}
	lines = append(lines, theme.Muted.Render(fmt.Sprintf("Potions %d | Deck %d", len(snapshot.Combat.Potions), snapshot.Combat.DeckSize)))
	for _, pending := range snapshot.Combat.PendingRepeats {
		lines = append(lines, theme.Selected.Width(panelContentWidth(panelWidth)).Render(truncateASCII("Pending repeat: "+pending, panelContentWidth(panelWidth))))
	}
	lines = append(lines, "")
	appendStatusChipLine(&lines, theme, panelContentWidth(panelWidth), "Target", combatState.TargetLabel, true)
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
		return "Multiplayer Room"
	}
	if snapshot.PhaseTitle != "" {
		return snapshot.PhaseTitle
	}
	return "Multiplayer Room"
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
