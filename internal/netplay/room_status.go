package netplay

import (
	"fmt"
	"sort"
	"strings"

	"cmdcards/internal/engine"
)

func (s *server) joinNoticeLocked(id string) string {
	player := s.players[id]
	if player == nil {
		return ""
	}
	return fmt.Sprintf("Joined seat %d as %s. Current phase: %s. Local commands: help, status, log, quit.", s.playerSeatIndexLocked(id)+1, player.ClassID, phaseDisplayName(s.phase))
}

func (s *server) reconnectNoticeLocked(id string) string {
	player := s.players[id]
	if player == nil {
		return ""
	}
	seat := s.playerSeatIndexLocked(id) + 1
	if s.restoredFromSave && id == s.hostID {
		return fmt.Sprintf("Saved room restored. You reclaimed seat %d and resumed %s. Ask players to rejoin with the same names.", seat, phaseDisplayName(s.phase))
	}
	return fmt.Sprintf("Rejoined seat %d during %s. Your saved class is %s.", seat, phaseDisplayName(s.phase), player.ClassID)
}

func (s *server) reconnectResumeLocked(id string) []string {
	player := s.players[id]
	if player == nil {
		return nil
	}
	lines := []string{
		fmt.Sprintf("Recovered phase: %s", phaseDisplayName(s.phase)),
		fmt.Sprintf("You reclaimed seat %d as %s.", s.playerSeatIndexLocked(id)+1, player.ClassID),
	}
	lines = append(lines, s.phaseResumeLinesLocked(id)...)
	if len(s.offlineSeatSummariesLocked()) > 0 {
		lines = append(lines, fmt.Sprintf("Offline reserved seats remaining: %d.", len(s.offlineSeatSummariesLocked())))
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
		seats = append(seats, formatSeatPlayer(index+1, player.Name, player.ClassID))
	}
	return seats
}

func (s *server) waitingOnLocked() []string {
	waiting := []string{}
	switch s.phase {
	case phaseLobby:
		for index, id := range s.order {
			player := s.players[id]
			if player == nil || !player.Connected || player.Ready {
				continue
			}
			waiting = append(waiting, formatSeatPlayer(index+1, player.Name, player.ClassID))
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
			waiting = append(waiting, formatSeatPlayer(index+1, player.Name, player.ClassID))
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
			waiting = append(waiting, formatSeatPlayer(index+1, player.Name, player.ClassID))
		}
	}
	return waiting
}

func (s *server) seatStatusLocked(selfID string) []string {
	statuses := make([]string, 0, len(s.order))
	for index, id := range s.order {
		player := s.players[id]
		if player == nil {
			continue
		}
		label := formatSeatPlayer(index+1, player.Name, player.ClassID)
		if id == s.hostID {
			label += " [host]"
		}
		if id == selfID {
			label += " [you]"
		}
		statuses = append(statuses, fmt.Sprintf("%s: %s", label, s.seatPhaseStateLocked(index, player)))
	}
	return statuses
}

func (s *server) seatPhaseStateLocked(index int, player *roomPlayer) string {
	if player == nil {
		return "empty"
	}
	if !player.Connected {
		if s.phase == phaseCombat {
			return "offline-auto-ready"
		}
		return "offline-reserved"
	}
	if s.hostTransfer != nil {
		switch player.ID {
		case s.hostTransfer.FromID:
			return "acting: transfer pending"
		case s.hostTransfer.ToID:
			return "acting: confirm host transfer"
		}
	}
	switch s.phase {
	case phaseLobby:
		if player.Ready {
			if player.ID == s.hostID {
				return "ready-host"
			}
			return "ready"
		}
		if player.ID == s.hostID {
			return "acting: room setup"
		}
		return "waiting: ready up"
	case phaseMap:
		if state := s.seatStateLocked(player.ID); state != nil && state.MapVote > 0 {
			return "ready: " + s.mapVoteChoiceLabelLocked(state.MapVote)
		}
		return "acting: choose route"
	case phaseCombat:
		if s.combat == nil || index >= len(s.combat.Coop.EndTurnVotes) {
			return "acting"
		}
		if s.combat.Coop.EndTurnVotes[index] {
			return "ready"
		}
		return "acting"
	case phaseReward:
		if state := s.seatStateLocked(player.ID); state != nil && state.Reward != nil && !state.RewardDone {
			return "acting: resolve reward"
		}
		return "ready"
	case phaseEvent:
		if state := s.seatStateLocked(player.ID); state != nil && state.Event != nil && !state.EventDone {
			return "acting: resolve event"
		}
		return "ready"
	case phaseShop:
		if state := s.seatStateLocked(player.ID); state != nil && state.Shop != nil && !state.ShopDone {
			return "acting: shop decision"
		}
		return "ready"
	case phaseRest:
		if player.ID == s.hostID {
			return "acting: campfire choice"
		}
		return "waiting: host campfire"
	case phaseEquipment:
		if player.ID == s.flowOwner {
			return "acting: equipment choice"
		}
		return "waiting: seat equipment"
	case phaseDeckAction:
		if player.ID == s.flowOwner {
			return "acting: deck action"
		}
		return "waiting: seat deck action"
	case phaseSummary:
		if player.ID == s.hostID {
			return "acting: next run or abandon"
		}
		return "waiting: host summary"
	default:
		return "waiting"
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
	summary = append(summary, fmt.Sprintf("Recovered phase: %s", phaseDisplayName(s.phase)))
	summary = append(summary, s.phaseResumeLinesLocked(selfID)...)
	if len(s.offlineSeatSummariesLocked()) > 0 {
		summary = append(summary, fmt.Sprintf("Offline reserved seats: %d.", len(s.offlineSeatSummariesLocked())))
	}
	if len(s.waitingOnLocked()) > 0 {
		summary = append(summary, fmt.Sprintf("Currently waiting on %d connected seat(s).", len(s.waitingOnLocked())))
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

func phaseDisplayName(phase string) string {
	switch phase {
	case phaseLobby:
		return "LAN Lobby"
	case phaseMap:
		return "Shared Map"
	case phaseCombat:
		return "Shared Combat"
	case phaseReward:
		return "Reward"
	case phaseEvent:
		return "Event"
	case phaseShop:
		return "Shop"
	case phaseRest:
		return "Campfire"
	case phaseEquipment:
		return "Equipment Choice"
	case phaseDeckAction:
		return "Deck Action"
	case phaseSummary:
		return "Run Summary"
	default:
		return phase
	}
}

func buildVoteStatus(order []string, players map[string]*roomPlayer, votes []bool) []string {
	status := make([]string, 0, len(votes))
	for index := 0; index < len(votes) && index < len(order); index++ {
		player := players[order[index]]
		if player == nil {
			continue
		}
		state := "waiting"
		if !player.Connected {
			state = "offline-auto-ready"
		} else if votes[index] {
			state = "ready"
		}
		status = append(status, fmt.Sprintf("%s: %s", formatSeatPlayer(index+1, player.Name, player.ClassID), state))
	}
	return status
}

func (s *server) mapVoteChoiceLabelLocked(vote int) string {
	if s.run == nil {
		return fmt.Sprintf("route %d", vote)
	}
	reachable := engine.ReachableNodes(s.run)
	if vote <= 0 || vote > len(reachable) {
		return fmt.Sprintf("route %d", vote)
	}
	node := reachable[vote-1]
	return fmt.Sprintf("route %d -> A%d F%d %s", vote, node.Act, node.Floor, engine.NodeKindName(node.Kind))
}

func (s *server) mapVoteStatusLocked(selfID string, reachable []engine.Node) []string {
	statuses := make([]string, 0, len(s.order))
	for index, id := range s.order {
		player := s.players[id]
		if player == nil {
			continue
		}
		label := formatSeatPlayer(index+1, player.Name, player.ClassID)
		if id == s.hostID {
			label += " [host]"
		}
		if id == selfID {
			label += " [you]"
		}
		switch {
		case !player.Connected:
			statuses = append(statuses, fmt.Sprintf("%s: offline-reserved", label))
		case s.seatStateLocked(id).MapVote > 0:
			statuses = append(statuses, fmt.Sprintf("%s: %s", label, describeMapVoteChoice(reachable, s.seatStateLocked(id).MapVote)))
		default:
			statuses = append(statuses, fmt.Sprintf("%s: choosing route", label))
		}
	}
	return statuses
}

func (s *server) mapVoteSummaryLocked(reachable []engine.Node) []string {
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
		lines = append(lines, fmt.Sprintf("%s: %d/%d", describeMapVoteChoice(reachable, index), counts[index], total))
	}
	return lines
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

func describeMapVoteChoice(reachable []engine.Node, vote int) string {
	if vote <= 0 || vote > len(reachable) {
		return fmt.Sprintf("route %d", vote)
	}
	node := reachable[vote-1]
	return fmt.Sprintf("route %d -> A%d F%d %s", vote, node.Act, node.Floor, engine.NodeKindName(node.Kind))
}

func formatSeatPlayer(seat int, name, classID string) string {
	if classID == "" {
		return fmt.Sprintf("Seat %d %s", seat, name)
	}
	return fmt.Sprintf("Seat %d %s [%s]", seat, name, classID)
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
	case strings.Contains(hint, "single enemy"):
		return fmt.Sprintf("play %d enemy 1", card.Index)
	case strings.Contains(hint, "single ally"):
		return fmt.Sprintf("play %d ally 1", card.Index)
	default:
		return fmt.Sprintf("play %d", card.Index)
	}
}
