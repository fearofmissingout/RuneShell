package ui

import (
	"fmt"
	"strings"

	"cmdcards/internal/netplay"

	"github.com/charmbracelet/lipgloss"
)

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
	TargetKind        string
	TargetIndex       int
	TargetLabel       string
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
		lines = append(lines, "", theme.Accent.Render("使用提示"))
		for _, tip := range tips {
			for _, line := range wrapLine("- "+tip, contentWidth) {
				lines = append(lines, theme.Muted.Render(line))
			}
		}
	}
	lines = append(lines, "", theme.Accent.Render("当前配置"))
	for i, item := range items {
		line := truncateASCII(item, contentWidth)
		if i == selected {
			lines = append(lines, theme.Selected.Render(line))
		} else {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	lines = append(lines, "", theme.Muted.Render(strings.Join([]string{"回车确认当前项", "左右切换职业/开关", "Esc 返回", "Ctrl+C 退出"}, "，")))
	return theme.Panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func RenderMultiplayerRoom(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, input string, combatState MultiplayerCombatState, width int, height int) string {
	totalWidth := max(36, viewportWidth(width, 100)-2)
	leftWidth, rightWidth, stacked := splitAdaptiveColumns(totalWidth, totalWidth*3/5, 36, 28, 1)
	left := theme.PanelAlt.Width(leftWidth).Render(renderMultiplayerMain(theme, snapshot, combatState, leftWidth))
	right := theme.PanelAlt.Width(rightWidth).Render(renderMultiplayerSidebar(theme, snapshot, actions, selectedAction, actionsFocused, combatState, rightWidth))
	body := lipJoinPanels(stacked, left, right)
	inputPanel := theme.Panel.Width(totalWidth).Render(renderMultiplayerInput(theme, input, combatState, actionsFocused, totalWidth, height))
	return strings.Join([]string{theme.Title.Render("多人房间"), body, inputPanel}, "\n\n")
}

func renderMultiplayerMain(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	if snapshot == nil {
		return strings.Join([]string{
			theme.Accent.Render("正在连接房间"),
			theme.Muted.Render("连接成功后，这里会显示当前阶段、房间状态、队伍信息和可执行动作。"),
		}, "\n")
	}
	lines := []string{theme.Accent.Render(snapshotTitle(snapshot))}
	lines = append(lines, theme.Subtitle.Render(fmt.Sprintf("房间 %s | 阶段 %s", snapshot.RoomAddr, snapshot.PhaseTitle)))
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
	lines := []string{theme.Subtitle.Render("当前身份")}
	if identity := multiplayerIdentityLine(snapshot); identity != "" {
		for _, line := range wrapLine(identity, width) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if control := multiplayerControlSummary(snapshot); control != "" {
		for _, line := range wrapLine("当前控制: "+control, width) {
			lines = append(lines, theme.Muted.Render(line))
		}
	}
	return lines
}

func multiplayerIdentityLine(snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	role := "队员"
	if snapshot.SelfID != "" && snapshot.SelfID == snapshot.HostID {
		role = "房主"
	}
	seat := snapshot.Seat
	name := "当前玩家"
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
	parts = append(parts, name)
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
			return "可调整房间配置、清理离线座位并开始本局。"
		}
		return "可切换职业、准备和聊天，开局由房主确认。"
	case snapshot.Map != nil:
		if isHost {
			return "由你决定队伍前往哪个地图节点。"
		}
		return "当前只能建议路线和聊天，实际选路由房主执行。"
	case snapshot.Combat != nil:
		return "所有在线座位都能出牌、用药水和结束回合投票。"
	case snapshot.Reward != nil:
		if isHost {
			return "由你决定领取哪张奖励卡，或是否跳过奖励。"
		}
		return "当前只能建议奖励选择，最终决定由房主执行。"
	case snapshot.Event != nil:
		if isHost {
			return "由你决定事件选项。"
		}
		return "当前只能建议事件选项，最终决定由房主执行。"
	case snapshot.Shop != nil:
		if isHost {
			return "由你决定购买哪件商品以及何时离开商店。"
		}
		return "当前只能建议购买内容，购物与离店由房主执行。"
	case snapshot.Rest != nil:
		if isHost {
			return "由你决定营火休息还是强化。"
		}
		return "当前只能建议营火选择，最终决定由房主执行。"
	case snapshot.Equipment != nil:
		if isHost {
			return "由你决定是否装备当前候选物品。"
		}
		return "当前只能建议是否装备，最终决定由房主执行。"
	case snapshot.Deck != nil:
		if isHost {
			return "由你完成当前牌组操作。"
		}
		return "当前只能建议牌组操作，最终决定由房主执行。"
	case snapshot.Summary != nil:
		if isHost {
			return "由你决定开始下一局还是结束房间。"
		}
		return "当前等待房主决定下一局或结束房间。"
	default:
		if snapshot.ControlLabel != "" {
			return snapshot.ControlLabel
		}
		return ""
	}
}

func renderMultiplayerSidebar(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, combatState MultiplayerCombatState, width int) string {
	contentWidth := panelContentWidth(width)
	if snapshot == nil {
		return theme.Muted.Render("等待服务器发送房间快照...")
	}
	if snapshot.Combat != nil {
		return renderCombatInspectSidebar(theme, snapshot, combatState, contentWidth)
	}
	return renderDefaultMultiplayerSidebar(theme, snapshot, actions, selectedAction, actionsFocused, contentWidth)
}

func renderCombatInspectSidebar(theme Theme, snapshot *netplay.Snapshot, combatState MultiplayerCombatState, contentWidth int) string {
	lines := []string{theme.Accent.Render("战斗检视")}
	tabs := []string{"概览", "日志", "投票"}
	current := clampSelection(combatState.InspectPane, len(tabs))
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
	if combatState.InspectFocused {
		lines = append(lines, "", theme.Good.Render("检视焦点已激活：左右或上下切换检视页，Tab 切到聊天。"))
	} else {
		lines = append(lines, "", theme.Muted.Render("Tab 可切到检视焦点查看更多战斗细节。"))
	}
	if snapshot.Combat == nil {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "")
	switch current {
	case 0:
		if combatState.SelectionLabel != "" {
			for _, line := range wrapLine("当前选择: "+combatState.SelectionLabel, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
		if combatState.TargetLabel != "" {
			for _, line := range wrapLine("当前目标: "+combatState.TargetLabel, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
		for _, item := range snapshot.Combat.Highlights {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Muted.Render(line))
			}
		}
		for _, line := range wrapLine(fmt.Sprintf("回合 %d | 能量 %d/%d", snapshot.Combat.Turn, snapshot.Combat.Energy, snapshot.Combat.MaxEnergy), contentWidth) {
			lines = append(lines, theme.Muted.Render(line))
		}
	case 1:
		entries := tailText(snapshot.Combat.Logs, 8)
		if len(entries) == 0 {
			lines = append(lines, theme.Muted.Render("暂无战斗日志。"))
		}
		for _, item := range entries {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Muted.Render(line))
			}
		}
	case 2:
		entries := snapshot.Combat.VoteStatus
		if len(entries) == 0 {
			entries = []string{"当前没有投票状态。"}
		}
		for _, item := range entries {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
		if len(snapshot.Combat.Potions) > 0 {
			lines = append(lines, "", theme.Subtitle.Render("药水库存"))
			for i, potion := range snapshot.Combat.Potions {
				for _, line := range wrapLine(fmt.Sprintf("- %d. %s", i+1, potion), contentWidth) {
					lines = append(lines, theme.Muted.Render(line))
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderDefaultMultiplayerSidebar(theme Theme, snapshot *netplay.Snapshot, actions []string, selectedAction int, actionsFocused bool, contentWidth int) string {
	lines := []string{theme.Accent.Render("房间助手")}
	if len(actions) > 0 {
		selectedAction = clampSelection(selectedAction, len(actions))
		lines = append(lines, theme.Subtitle.Render("建议操作"))
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
		lines = append(lines, "", theme.Muted.Render("这里优先显示中文动作说明；“房主操作/仅房主”会直接标明权限，带参数的模板会自动回填到底部输入框。"))
	}
	if len(snapshot.Commands) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("协议命令参考"))
		for _, item := range snapshot.Commands {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	if len(snapshot.WaitingOn) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("当前等待"))
		for _, item := range snapshot.WaitingOn {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Bad.Render(line))
			}
		}
	}
	if len(snapshot.SeatStatus) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("座位状态"))
		for _, item := range snapshot.SeatStatus {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Normal.Render(line))
			}
		}
	}
	if len(snapshot.ChatLog) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("最近聊天"))
		for _, item := range tailText(snapshot.ChatLog, 4) {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Muted.Render(line))
			}
		}
	}
	if len(snapshot.RoomLog) > 0 {
		lines = append(lines, "", theme.Subtitle.Render("系统日志"))
		for _, item := range tailText(snapshot.RoomLog, 4) {
			for _, line := range wrapLine("- "+item, contentWidth) {
				lines = append(lines, theme.Muted.Render(line))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderMultiplayerInput(theme Theme, input string, combatState MultiplayerCombatState, actionsFocused bool, width int, _ int) string {
	contentWidth := panelContentWidth(width)
	value := strings.TrimSpace(input)
	if value == "" {
		value = "例如: ready / chat 大家好 / node 1 / play 1 enemy 1"
	}
	title := "输入联机指令"
	if actionsFocused {
		if combatState.Enabled {
			title = "输入联机指令（当前焦点在战斗操作）"
		} else {
			title = "输入联机指令（当前焦点在建议操作）"
		}
	}
	helpText := "Tab 在建议操作和输入框之间切换，回车发送当前输入或执行选中操作，Esc 离开房间。"
	if combatState.Enabled {
		helpText = "战斗时：上下选择手牌或药水，左右切目标，回车执行，z 在手牌/药水间切换，e 结束回合，Tab 在操作区/检视区/聊天框之间切换。"
	} else if combatState.SelectionLabel != "" {
		helpText = "非战斗阶段：上下选择当前选项，回车确认，必要时可用 s/esc/l 执行跳过、返回或离开，Tab 切到聊天输入框。"
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
	lines := []string{}
	appendWrapped := func(prefix, text string) {
		for _, line := range wrapLine(prefix+text, width) {
			lines = append(lines, theme.Normal.Render(line))
		}
	}
	if snapshot.Lobby != nil {
		appendWrapped("模式: ", fmt.Sprintf("%s | 种子 %d", snapshot.Lobby.Mode, snapshot.Lobby.Seed))
		lines = append(lines, "", theme.Subtitle.Render("房间成员"))
		for _, player := range snapshot.Players {
			status := "未准备"
			if player.Ready {
				status = "已准备"
			}
			if !player.Connected {
				status = "离线保留"
			}
			appendWrapped("- ", fmt.Sprintf("Seat %d %s [%s] %s", player.Seat, player.Name, player.ClassID, status))
		}
		return lines
	}
	if snapshot.Map != nil {
		appendWrapped("地图: ", fmt.Sprintf("Act %d | 下一层 %d | 金币 %d", snapshot.Map.Act, snapshot.Map.NextFloor, snapshot.Map.Gold))
		if combatState.SelectionLabel != "" {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
		}
		lines = append(lines, "", theme.Subtitle.Render("队伍"))
		for _, actor := range snapshot.Map.Party {
			appendWrapped("- ", actorText(actor.Name, actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status))
		}
		lines = append(lines, "", theme.Subtitle.Render("可前往节点"))
		for i, node := range snapshot.Map.Reachable {
			text := fmt.Sprintf("%d. %s", node.Index, node.Label)
			if i == combatState.SelectedIndex {
				lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
			} else {
				appendWrapped("- ", text)
			}
		}
		return lines
	}
	if snapshot.Combat != nil {
		appendWrapped("战斗: ", fmt.Sprintf("回合 %d | 主能量 %d/%d", snapshot.Combat.Turn, snapshot.Combat.Energy, snapshot.Combat.MaxEnergy))
		if combatState.Enabled {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("模式: ", combatState.ModeLabel)
			appendWrapped("目标: ", combatState.TargetLabel)
			if combatState.OperationsFocused {
				lines = append(lines, theme.Good.Render("操作焦点已激活：方向键和回车直接用于战斗操作。"))
			} else {
				lines = append(lines, theme.Muted.Render("当前焦点在聊天输入框；按 Tab 返回战斗操作。"))
			}
		}
		lines = append(lines, "", theme.Subtitle.Render("友方"))
		for _, actor := range snapshot.Combat.Party {
			line := actorText(actor.Name, actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status)
			if combatState.TargetKind == "ally" && combatState.TargetIndex == actor.Index {
				lines = append(lines, theme.Selected.Render(truncateASCII("- "+line, width)))
			} else {
				appendWrapped("- ", line)
			}
		}
		lines = append(lines, "", theme.Subtitle.Render("敌方"))
		for _, enemy := range snapshot.Combat.Enemies {
			text := fmt.Sprintf("%d. %s HP %d/%d | 格挡 %d | 意图 %s", enemy.Index, enemy.Name, enemy.HP, enemy.MaxHP, enemy.Block, enemy.Intent)
			if enemy.Status != "" {
				text += " | " + enemy.Status
			}
			if combatState.TargetKind == "enemy" && combatState.TargetIndex == enemy.Index {
				lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
			} else {
				appendWrapped("- ", text)
			}
		}
		if len(snapshot.Combat.Hand) > 0 {
			lines = append(lines, "", theme.Subtitle.Render("手牌"))
			for i, card := range snapshot.Combat.Hand {
				text := fmt.Sprintf("%d. [%d] %s | %s | %s", card.Index, card.Cost, card.Name, card.Summary, card.TargetHint)
				if combatState.ModeLabel == "手牌" && i == combatState.SelectedCard {
					lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
				} else {
					appendWrapped("- ", text)
				}
			}
		}
		if len(snapshot.Combat.Potions) > 0 {
			lines = append(lines, "", theme.Subtitle.Render("药水"))
			for i, potion := range snapshot.Combat.Potions {
				text := fmt.Sprintf("%d. %s", i+1, potion)
				if combatState.ModeLabel == "药水" && i == combatState.SelectedPotion {
					lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
				} else {
					appendWrapped("- ", text)
				}
			}
		}
		return lines
	}
	if snapshot.Reward != nil {
		appendWrapped("奖励: ", fmt.Sprintf("金币 +%d | 来源 %s", snapshot.Reward.Gold, snapshot.Reward.Source))
		if combatState.SelectionLabel != "" {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
		}
		for i, card := range snapshot.Reward.Cards {
			text := fmt.Sprintf("%d. %s | %s", card.Index, card.Name, card.Summary)
			if i == combatState.SelectedIndex {
				lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
			} else {
				appendWrapped("- ", text)
			}
		}
		return lines
	}
	if snapshot.Event != nil {
		appendWrapped("事件: ", snapshot.Event.Name)
		appendWrapped("说明: ", snapshot.Event.Description)
		if combatState.SelectionLabel != "" {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
		}
		lines = append(lines, "", theme.Subtitle.Render("可选项"))
		for i, choice := range snapshot.Event.Choices {
			text := fmt.Sprintf("%d. %s | %s", choice.Index, choice.Label, choice.Description)
			if i == combatState.SelectedIndex {
				lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
			} else {
				appendWrapped("- ", text)
			}
		}
		return lines
	}
	if snapshot.Shop != nil {
		appendWrapped("商店: ", fmt.Sprintf("金币 %d", snapshot.Shop.Gold))
		if combatState.SelectionLabel != "" {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
		}
		for i, offer := range snapshot.Shop.Offers {
			text := fmt.Sprintf("%d. %s | %d 金币 | %s", offer.Index, offer.Name, offer.Price, offer.Description)
			if i == combatState.SelectedIndex {
				lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
			} else {
				appendWrapped("- ", text)
			}
		}
		return lines
	}
	if snapshot.Rest != nil {
		if combatState.SelectionLabel != "" {
			lines = append(lines, theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
			lines = append(lines, "")
		}
		lines = append(lines, theme.Subtitle.Render("营火队伍状态"))
		for _, actor := range snapshot.Rest.Party {
			appendWrapped("- ", actorText(actor.Name, actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status))
		}
		return lines
	}
	if snapshot.Equipment != nil {
		appendWrapped("装备选择: ", fmt.Sprintf("%s | 槽位 %s", snapshot.Equipment.CandidateName, snapshot.Equipment.Slot))
		if combatState.SelectionLabel != "" {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
		}
		appendWrapped("候选说明: ", snapshot.Equipment.CandidateDescription)
		if snapshot.Equipment.CurrentName != "" {
			appendWrapped("当前装备: ", snapshot.Equipment.CurrentName+" | "+snapshot.Equipment.CurrentDescription)
		}
		return lines
	}
	if snapshot.Deck != nil {
		appendWrapped("牌组操作: ", snapshot.Deck.Title)
		appendWrapped("说明: ", snapshot.Deck.Subtitle)
		if combatState.SelectionLabel != "" {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
		}
		for i, card := range snapshot.Deck.Cards {
			text := fmt.Sprintf("%d. %s | %s", card.Index, card.Name, card.Summary)
			if i == combatState.SelectedIndex {
				lines = append(lines, theme.Selected.Render(truncateASCII("- "+text, width)))
			} else {
				appendWrapped("- ", text)
			}
		}
		return lines
	}
	if snapshot.Summary != nil {
		appendWrapped("结算: ", fmt.Sprintf("%s | %s | Act %d | Floors %d", snapshot.Summary.Result, snapshot.Summary.Mode, snapshot.Summary.Act, snapshot.Summary.Floors))
		if combatState.SelectionLabel != "" {
			lines = append(lines, "", theme.Subtitle.Render("当前操作"))
			appendWrapped("选择: ", combatState.SelectionLabel)
		}
		for _, actor := range snapshot.Summary.Party {
			appendWrapped("- ", actorText(actor.Name, actor.HP, actor.MaxHP, actor.Energy, actor.MaxEnergy, actor.Block, actor.Status))
		}
	}
	return lines
}

func actorText(name string, hp int, maxHP int, energy int, maxEnergy int, block int, status string) string {
	line := fmt.Sprintf("%s HP %d/%d | 格挡 %d", name, hp, maxHP, block)
	if maxEnergy > 0 {
		line += fmt.Sprintf(" | 能量 %d/%d", energy, maxEnergy)
	}
	if status != "" {
		line += " | " + status
	}
	return line
}

func snapshotTitle(snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return "多人房间"
	}
	if snapshot.PhaseTitle != "" {
		return snapshot.PhaseTitle
	}
	return "多人房间"
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
