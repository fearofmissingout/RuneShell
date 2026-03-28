package netplay

import (
	"fmt"
	"sort"
	"strings"

	"cmdcards/internal/engine"
	"cmdcards/internal/i18n"
)

func (s *server) joinNoticeLocked(id string) string {
	player := s.players[id]
	if player == nil {
		return ""
	}
	return fmt.Sprintf("已加入座位 %d，职业 %s。当前阶段：%s。可用本地命令：help、status、log、quit。", s.playerSeatIndexLocked(id)+1, player.ClassID, phaseDisplayName(s.phase))
}

func (s *server) reconnectNoticeLocked(id string) string {
	player := s.players[id]
	if player == nil {
		return ""
	}
	seat := s.playerSeatIndexLocked(id) + 1
	if s.restoredFromSave && id == s.hostID {
		return fmt.Sprintf("已恢复保存的房间。你重新接管了座位 %d，并回到了%s。请提醒其他玩家使用相同名字重连。", seat, phaseDisplayName(s.phase))
	}
	return fmt.Sprintf("你已在%s阶段重新加入座位 %d。保存的职业是 %s。", phaseDisplayName(s.phase), seat, player.ClassID)
}

func (s *server) reconnectResumeLocked(id string) []string {
	player := s.players[id]
	if player == nil {
		return nil
	}
	lines := []string{
		fmt.Sprintf("已恢复阶段: %s", phaseDisplayName(s.phase)),
		fmt.Sprintf("你重新接管了座位 %d，职业 %s。", s.playerSeatIndexLocked(id)+1, player.ClassID),
	}
	lines = append(lines, s.phaseResumeLinesLocked(id)...)
	if len(s.offlineSeatSummariesLocked()) > 0 {
		lines = append(lines, fmt.Sprintf("仍保留的离线席位: %d。", len(s.offlineSeatSummariesLocked())))
	}
	return lines
}

func (s *server) offlineSeatSummariesLocked() []string {
	seats := []string{}
	for index, id := range s.order {
		player := s.players[id]
		if player == nil || player.Connected {
			continue
		}
		seats = append(seats, formatSeatPlayerFor(i18n.DefaultLanguage, index+1, player.Name, player.ClassID))
	}
	return seats
}

func (s *server) waitingOnForLocked(selfID string) []string {
	lang := s.playerLanguageLocked(selfID)
	waiting := []string{}
	switch s.phase {
	case phaseLobby:
		for index, id := range s.order {
			player := s.players[id]
			if player == nil || !player.Connected || player.Ready {
				continue
			}
			waiting = append(waiting, formatSeatPlayerFor(lang, index+1, player.Name, player.ClassID))
		}
	case phaseCombat:
		if s.combat == nil {
			return nil
		}
		for index := 0; index < len(s.combat.Coop.EndTurnVotes) && index < len(s.order); index++ {
			player := s.players[s.order[index]]
			if player == nil || !player.Connected || s.combat.Coop.EndTurnVotes[index] {
				continue
			}
			waiting = append(waiting, formatSeatPlayerFor(lang, index+1, player.Name, player.ClassID))
		}
	case phaseMap:
		for index, id := range s.order {
			player := s.players[id]
			if player == nil || !player.Connected {
				continue
			}
			if state := s.seatStateLocked(id); state != nil && state.MapVote > 0 {
				continue
			}
			waiting = append(waiting, formatSeatPlayerFor(lang, index+1, player.Name, player.ClassID))
		}
	}
	return waiting
}

func (s *server) waitingOnLocked() []string {
	return s.waitingOnForLocked("")
}

func (s *server) seatStatusLocked(selfID string) []string {
	lang := s.playerLanguageLocked(selfID)
	statuses := make([]string, 0, len(s.order))
	for index, id := range s.order {
		player := s.players[id]
		if player == nil {
			continue
		}
		label := formatSeatPlayerFor(lang, index+1, player.Name, player.ClassID)
		if id == s.hostID {
			label += " " + textForLanguage(lang, "net.tag.host", nil)
		}
		if id == selfID {
			label += " " + textForLanguage(lang, "net.tag.you", nil)
		}
		statuses = append(statuses, fmt.Sprintf("%s: %s", label, s.seatPhaseStateLocked(selfID, index, player)))
	}
	return statuses
}

func (s *server) seatPhaseStateLocked(selfID string, index int, player *roomPlayer) string {
	lang := s.playerLanguageLocked(selfID)
	if player == nil {
		return textForLanguage(lang, "net.state.empty", nil)
	}
	if !player.Connected {
		if s.phase == phaseCombat {
			return textForLanguage(lang, "net.state.offline_auto_ready", nil)
		}
		return textForLanguage(lang, "net.state.offline_reserved", nil)
	}
	if s.hostTransfer != nil {
		switch player.ID {
		case s.hostTransfer.FromID:
			return textForLanguage(lang, "net.state.transfer_pending", nil)
		case s.hostTransfer.ToID:
			return textForLanguage(lang, "net.state.transfer_confirm", nil)
		}
	}
	switch s.phase {
	case phaseLobby:
		if player.Ready {
			if player.ID == s.hostID {
				return textForLanguage(lang, "net.state.ready_host", nil)
			}
			return textForLanguage(lang, "net.state.ready", nil)
		}
		if player.ID == s.hostID {
			return textForLanguage(lang, "net.state.room_setup", nil)
		}
		return textForLanguage(lang, "net.state.ready_wait", nil)
	case phaseMap:
		if state := s.seatStateLocked(player.ID); state != nil && state.MapVote > 0 {
			return textForLanguage(lang, "net.state.vote_ready", i18n.Args{"choice": s.mapVoteChoiceLabelForLocked(selfID, state.MapVote)})
		}
		return textForLanguage(lang, "net.state.choose_route", nil)
	case phaseCombat:
		if s.combat == nil || index >= len(s.combat.Coop.EndTurnVotes) {
			return textForLanguage(lang, "net.state.acting", nil)
		}
		if s.combat.Coop.EndTurnVotes[index] {
			return textForLanguage(lang, "net.state.ready", nil)
		}
		return textForLanguage(lang, "net.state.acting", nil)
	case phaseReward:
		if state := s.seatStateLocked(player.ID); state != nil && state.Reward != nil && !state.RewardDone {
			return textForLanguage(lang, "net.state.resolve_reward", nil)
		}
		return textForLanguage(lang, "net.state.ready", nil)
	case phaseEvent:
		if state := s.seatStateLocked(player.ID); state != nil && state.Event != nil && !state.EventDone {
			return textForLanguage(lang, "net.state.resolve_event", nil)
		}
		return textForLanguage(lang, "net.state.ready", nil)
	case phaseShop:
		if state := s.seatStateLocked(player.ID); state != nil && state.Shop != nil && !state.ShopDone {
			return textForLanguage(lang, "net.state.shop_decision", nil)
		}
		return textForLanguage(lang, "net.state.ready", nil)
	case phaseRest:
		if player.ID == s.hostID {
			return textForLanguage(lang, "net.state.campfire_choice", nil)
		}
		return textForLanguage(lang, "net.state.host_campfire", nil)
	case phaseEquipment:
		if player.ID == s.flowOwner {
			return textForLanguage(lang, "net.state.equipment_choice", nil)
		}
		return textForLanguage(lang, "net.state.wait_equipment", nil)
	case phaseDeckAction:
		if player.ID == s.flowOwner {
			return textForLanguage(lang, "net.state.deck_action", nil)
		}
		return textForLanguage(lang, "net.state.wait_deck_action", nil)
	case phaseSummary:
		if player.ID == s.hostID {
			return textForLanguage(lang, "net.state.summary_choice", nil)
		}
		return textForLanguage(lang, "net.state.host_summary", nil)
	default:
		return textForLanguage(lang, "net.state.waiting", nil)
	}
}

func (s *server) recoveryActionsLocked(selfID string) []string {
	if selfID != s.hostID || s.phase != phaseLobby {
		return nil
	}
	actions := []string{}
	offline := s.offlineSeatSummariesLocked()
	if len(offline) == 0 {
		return actions
	}
	for index, id := range s.order {
		player := s.players[id]
		if player == nil || player.Connected {
			continue
		}
		actions = append(actions, fmt.Sprintf("drop %d    remove %s from the reserved seat list", index+1, player.Name))
	}
	if len(actions) > 1 {
		actions = append(actions, "drop all   remove every offline reserved seat")
	}
	return actions
}

func (s *server) reconnectCommandsLocked(selfID string) []string {
	if selfID != s.hostID {
		return nil
	}
	commands := []string{}
	for index, id := range s.order {
		player := s.players[id]
		if player == nil || player.Connected {
			continue
		}
		commands = append(commands, fmt.Sprintf("Seat %d %s: go run ./cmd/cmdcards join --addr %s --name %s --class %s",
			index+1, player.Name, s.roomAddr, player.Name, player.ClassID))
	}
	return commands
}

func (s *server) resumeSummaryLocked(selfID string) []string {
	summary := []string{}
	if !s.restoredFromSave {
		return summary
	}
	summary = append(summary, fmt.Sprintf("已恢复阶段: %s", phaseDisplayName(s.phase)))
	summary = append(summary, s.phaseResumeLinesLocked(selfID)...)
	if len(s.offlineSeatSummariesLocked()) > 0 {
		summary = append(summary, fmt.Sprintf("离线保留席位: %d。", len(s.offlineSeatSummariesLocked())))
	}
	if len(s.waitingOnLocked()) > 0 {
		summary = append(summary, fmt.Sprintf("当前仍在等待 %d 个已连接席位。", len(s.waitingOnLocked())))
	}
	return summary
}

func (s *server) firstConnectedHostTransferSeatLocked() int {
	if s.connectedHostTransferSeatCountLocked() == 0 {
		return 0
	}
	for index, id := range s.order {
		if id == s.hostID {
			continue
		}
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		return index + 1
	}
	return 0
}

func (s *server) connectedHostTransferSeatCountLocked() int {
	count := 0
	for _, id := range s.order {
		if id == s.hostID {
			continue
		}
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		count++
	}
	return count
}

func (s *server) chatUnreadLocked(id string) int {
	client := s.clients[id]
	if client == nil {
		return 0
	}
	if client.chatSeen >= len(s.chatLog) {
		return 0
	}
	return len(s.chatLog) - client.chatSeen
}

func (s *server) transferNoteLocked(selfID string) string {
	if s.hostTransfer == nil {
		return ""
	}
	fromSeat := s.playerSeatIndexLocked(s.hostTransfer.FromID) + 1
	toSeat := s.playerSeatIndexLocked(s.hostTransfer.ToID) + 1
	fromName := s.hostTransfer.FromID
	toName := s.hostTransfer.ToID
	if player := s.players[s.hostTransfer.FromID]; player != nil {
		fromName = player.Name
	}
	if player := s.players[s.hostTransfer.ToID]; player != nil {
		toName = player.Name
	}
	switch selfID {
	case s.hostTransfer.FromID:
		return fmt.Sprintf("Host transfer pending: Seat %d %s -> Seat %d %s. Use cancel-host to stop it.", fromSeat, fromName, toSeat, toName)
	case s.hostTransfer.ToID:
		return fmt.Sprintf("Host transfer request pending from Seat %d %s. Use accept-host or deny-host.", fromSeat, fromName)
	default:
		return fmt.Sprintf("Host transfer pending: Seat %d %s -> Seat %d %s.", fromSeat, fromName, toSeat, toName)
	}
}

func (s *server) appendTransferCommandsLocked(selfID string, commands []string) []string {
	if s.hostTransfer != nil {
		switch selfID {
		case s.hostTransfer.FromID:
			return append(commands, "cancel-host")
		case s.hostTransfer.ToID:
			return append(commands, "accept-host", "deny-host")
		default:
			return commands
		}
	}
	if selfID == s.hostID && s.connectedHostTransferSeatCountLocked() > 0 {
		return append(commands, "host <seat>")
	}
	return commands
}

func (s *server) appendTransferExamplesLocked(selfID string, examples []string) []string {
	if s.hostTransfer != nil {
		switch selfID {
		case s.hostTransfer.FromID:
			return append(examples, "cancel-host")
		case s.hostTransfer.ToID:
			return append(examples, "accept-host", "deny-host")
		default:
			return examples
		}
	}
	if selfID == s.hostID {
		if seat := s.firstConnectedHostTransferSeatLocked(); seat > 0 {
			return append(examples, fmt.Sprintf("host %d", seat))
		}
	}
	return examples
}

func buildVoteStatus(lang string, order []string, players map[string]*roomPlayer, votes []bool) []string {
	status := make([]string, 0, len(votes))
	for index := 0; index < len(votes) && index < len(order); index++ {
		player := players[order[index]]
		if player == nil {
			continue
		}
		state := textForLanguage(lang, "net.state.waiting", nil)
		if !player.Connected {
			state = textForLanguage(lang, "net.state.offline_auto_ready", nil)
		} else if votes[index] {
			state = textForLanguage(lang, "net.state.ready", nil)
		}
		status = append(status, fmt.Sprintf("%s: %s", formatSeatPlayerFor(lang, index+1, player.Name, player.ClassID), state))
	}
	return status
}

func (s *server) mapVoteChoiceLabelForLocked(selfID string, vote int) string {
	lang := s.playerLanguageLocked(selfID)
	if s.run == nil {
		return textForLanguage(lang, "net.vote.route_index", i18n.Args{"index": vote})
	}
	reachable := engine.ReachableNodes(s.run)
	if vote <= 0 || vote > len(reachable) {
		return textForLanguage(lang, "net.vote.route_index", i18n.Args{"index": vote})
	}
	node := reachable[vote-1]
	return textForLanguage(lang, "net.vote.route_named", i18n.Args{"index": vote, "act": node.Act, "floor": node.Floor, "kind": engine.NodeKindNameFor(lang, node.Kind)})
}

func (s *server) mapVoteChoiceLabelLocked(vote int) string {
	if s.run == nil {
		return textForLanguage(i18n.DefaultLanguage, "net.vote.route_index", i18n.Args{"index": vote})
	}
	return describeMapVoteChoice(engine.ReachableNodes(s.run), vote)
}

func (s *server) mapVoteStatusLocked(selfID string, reachable []engine.Node) []string {
	lang := s.playerLanguageLocked(selfID)
	statuses := make([]string, 0, len(s.order))
	for index, id := range s.order {
		player := s.players[id]
		if player == nil {
			continue
		}
		label := formatSeatPlayerFor(lang, index+1, player.Name, player.ClassID)
		if id == s.hostID {
			label += " " + textForLanguage(lang, "net.tag.host", nil)
		}
		if id == selfID {
			label += " " + textForLanguage(lang, "net.tag.you", nil)
		}
		switch {
		case !player.Connected:
			statuses = append(statuses, fmt.Sprintf("%s: %s", label, textForLanguage(lang, "net.state.offline_reserved", nil)))
		case s.seatStateLocked(id).MapVote > 0:
			statuses = append(statuses, fmt.Sprintf("%s: %s", label, describeMapVoteChoiceFor(lang, reachable, s.seatStateLocked(id).MapVote)))
		default:
			statuses = append(statuses, fmt.Sprintf("%s: %s", label, textForLanguage(lang, "net.state.choose_route", nil)))
		}
	}
	return statuses
}

func (s *server) mapVoteSummaryForLocked(selfID string, reachable []engine.Node) []string {
	lang := s.playerLanguageLocked(selfID)
	counts := map[int]int{}
	total := 0
	for _, id := range s.connectedSeatIDsLocked() {
		vote := s.seatStateLocked(id).MapVote
		if vote <= 0 {
			continue
		}
		counts[vote]++
		total++
	}
	if total == 0 {
		return nil
	}
	indexes := make([]int, 0, len(counts))
	for index := range counts {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	lines := make([]string, 0, len(indexes))
	for _, index := range indexes {
		lines = append(lines, fmt.Sprintf("%s: %d/%d", describeMapVoteChoiceFor(lang, reachable, index), counts[index], total))
	}
	return lines
}

func (s *server) mapVoteSummaryLocked(reachable []engine.Node) []string {
	return s.mapVoteSummaryForLocked("", reachable)
}

func (s *server) mapVoteOddsLocked(vote int) string {
	total := 0
	selected := 0
	for _, id := range s.connectedSeatIDsLocked() {
		current := s.seatStateLocked(id).MapVote
		if current <= 0 {
			continue
		}
		total++
		if current == vote {
			selected++
		}
	}
	if total == 0 || selected == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", selected, total)
}

func describeMapVoteChoiceFor(lang string, reachable []engine.Node, vote int) string {
	if vote <= 0 || vote > len(reachable) {
		return textForLanguage(lang, "net.vote.route_index", i18n.Args{"index": vote})
	}
	node := reachable[vote-1]
	return textForLanguage(lang, "net.vote.route_named", i18n.Args{"index": vote, "act": node.Act, "floor": node.Floor, "kind": engine.NodeKindNameFor(lang, node.Kind)})
}

func describeMapVoteChoice(reachable []engine.Node, vote int) string {
	return describeMapVoteChoiceFor(i18n.DefaultLanguage, reachable, vote)
}

func formatSeatPlayerFor(lang string, seat int, name, classID string) string {
	if classID == "" {
		return textForLanguage(lang, "net.seat.named", i18n.Args{"seat": seat, "name": name})
	}
	return textForLanguage(lang, "net.seat.named_class", i18n.Args{"seat": seat, "name": name, "class": classID})
}

func formatSeatPlayer(seat int, name, classID string) string {
	return formatSeatPlayerFor(i18n.DefaultLanguage, seat, name, classID)
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func combatCommandExample(card cardSnapshot) string {
	hint := strings.ToLower(card.TargetHint)
	switch {
	case strings.Contains(hint, "single enemy"), strings.Contains(hint, "单体敌方"):
		return fmt.Sprintf("play %d enemy 1", card.Index)
	case strings.Contains(hint, "single ally"), strings.Contains(hint, "单体友方"):
		return fmt.Sprintf("play %d ally 1", card.Index)
	default:
		return fmt.Sprintf("play %d", card.Index)
	}
}
