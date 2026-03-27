package netplay

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
)

const (
	maxPlayers = 4
	maxChatLog = 48

	phaseLobby      = "lobby"
	phaseMap        = "map"
	phaseCombat     = "combat"
	phaseReward     = "reward"
	phaseEvent      = "event"
	phaseShop       = "shop"
	phaseRest       = "rest"
	phaseEquipment  = "equipment"
	phaseDeckAction = "deck_action"
	phaseSummary    = "summary"
)

func formatChannelEntry(source, actor, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if hasFormattedChannelPrefix(text) {
		return text
	}
	if actor != "" {
		return fmt.Sprintf("%s %s | %s: %s", time.Now().Format("15:04"), source, actor, text)
	}
	return fmt.Sprintf("%s %s | %s", time.Now().Format("15:04"), source, text)
}

func hasFormattedChannelPrefix(line string) bool {
	return hasTimePrefix(line) && strings.Contains(line[6:], " | ")
}

func hasTimePrefix(line string) bool {
	if len(line) < 6 {
		return false
	}
	return line[2] == ':' && line[5] == ' '
}

func displayChannelEntry(source, line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	if hasFormattedChannelPrefix(trimmed) {
		return trimmed
	}
	if hasTimePrefix(trimmed) {
		return trimmed[:6] + source + " | " + trimmed[6:]
	}
	return source + " | " + trimmed
}

func renderChannelSection(section, heading, source string, lines []string) {
	renderDivider(section)
	fmt.Println(heading)
	if len(lines) == 0 {
		fmt.Println("- (empty)")
		return
	}
	for _, line := range lines {
		rendered := displayChannelEntry(source, line)
		if rendered == "" {
			continue
		}
		fmt.Println("-", rendered)
	}
}

func (s *server) appendRoomLogLocked(text string) {
	entry := formatChannelEntry("System", "", text)
	if entry == "" {
		return
	}
	s.roomLog = append(s.roomLog, entry)
}

func RunHost(lib *content.Library, port int, name, classID string, forceNew bool) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	savePath, err := defaultRoomSavePath()
	if err != nil {
		return err
	}
	if forceNew {
		if err := clearSavedRoom(savePath); err != nil {
			return err
		}
	}
	srv, restored, err := loadServerFromSavePath(lib, addr, savePath)
	if err != nil {
		return err
	}
	if restored {
		srv.restoredFromSave = true
		if hostPlayer, ok := srv.players[srv.hostID]; ok && !strings.EqualFold(hostPlayer.Name, name) {
			_ = srv.listener.Close()
			return fmt.Errorf("saved room belongs to host %q; use the same name or pass --new", hostPlayer.Name)
		}
		srv.roomLog = append(srv.roomLog, "Saved room restored. Waiting for players to reconnect.")
	}
	_ = srv.persistLocked()
	go srv.serve()
	clientErr := runClientWithRetry(fmt.Sprintf("127.0.0.1:%d", port), name, classID, 3*time.Second)

	srv.mu.Lock()
	srv.shutdownLocked(fmt.Sprintf("%s ended the host session. Room saved for restore.", name))
	srv.mu.Unlock()
	return clientErr
}

func RunJoin(lib *content.Library, addr, name, classID string) error {
	_ = lib
	return runClient(addr, name, classID)
}

func newServer(lib *content.Library, addr string) (*server, error) {
	return newServerWithSavePath(lib, addr, "")
}

func dialTCPWithRetry(addr string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	dialStep := 250 * time.Millisecond
	if timeout > 0 && timeout < dialStep {
		dialStep = timeout
	}
	for {
		conn, err := net.DialTimeout("tcp", addr, dialStep)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *server) handleConn(conn net.Conn) {
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var helloMsg message
	if err := dec.Decode(&helloMsg); err != nil {
		_ = conn.Close()
		return
	}
	if helloMsg.Type != "hello" || helloMsg.Hello == nil {
		_ = enc.Encode(message{Type: "error", Error: "expected hello"})
		_ = conn.Close()
		return
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = enc.Encode(message{Type: "error", Error: "room closed"})
		_ = conn.Close()
		return
	}
	name := strings.TrimSpace(helloMsg.Hello.Name)
	if name == "" {
		s.mu.Unlock()
		_ = enc.Encode(message{Type: "error", Error: "name is required"})
		_ = conn.Close()
		return
	}

	var (
		id     string
		player *roomPlayer
	)
	if existingID, existingPlayer, ok := s.reconnectCandidateLocked(name); ok {
		if existingPlayer.Connected {
			s.mu.Unlock()
			_ = enc.Encode(message{Type: "error", Error: "player name already connected"})
			_ = conn.Close()
			return
		}
		id = existingID
		player = existingPlayer
		player.Connected = true
		if oldClient, ok := s.clients[id]; ok {
			_ = oldClient.conn.Close()
			delete(s.clients, id)
		}
		s.clients[id] = &clientConn{id: id, conn: conn, enc: enc, chatSeen: len(s.chatLog)}
		s.setClientNoticeLocked(id, s.reconnectNoticeLocked(id))
		s.setClientResumeLocked(id, s.reconnectResumeLocked(id))
		s.appendRoomLogLocked(fmt.Sprintf("%s reconnected.", player.Name))
	} else {
		if s.phase != phaseLobby {
			s.mu.Unlock()
			_ = enc.Encode(message{Type: "error", Error: "room already in progress"})
			_ = conn.Close()
			return
		}
		if len(s.order) >= maxPlayers {
			s.mu.Unlock()
			_ = enc.Encode(message{Type: "error", Error: "room is full"})
			_ = conn.Close()
			return
		}
		if _, ok := s.lib.Classes[helloMsg.Hello.ClassID]; !ok {
			s.mu.Unlock()
			_ = enc.Encode(message{Type: "error", Error: "unknown class"})
			_ = conn.Close()
			return
		}

		id = fmt.Sprintf("p%d", s.nextID)
		s.nextID++
		player = &roomPlayer{
			ID:        id,
			Name:      name,
			ClassID:   helloMsg.Hello.ClassID,
			Ready:     false,
			Connected: true,
		}
		if s.hostID == "" {
			s.hostID = id
		}
		s.order = append(s.order, id)
		s.players[id] = player
		s.clients[id] = &clientConn{id: id, conn: conn, enc: enc, chatSeen: len(s.chatLog)}
		s.setClientNoticeLocked(id, s.joinNoticeLocked(id))
		s.appendRoomLogLocked(fmt.Sprintf("%s joined as %s.", player.Name, player.ClassID))
	}
	if s.restoredFromSave && len(s.offlineSeatSummariesLocked()) == 0 {
		s.restoredFromSave = false
	}
	s.broadcastLocked()
	s.mu.Unlock()

	for {
		var msg message
		if err := dec.Decode(&msg); err != nil {
			s.removeClient(id, fmt.Sprintf("%s disconnected.", player.Name))
			return
		}
		if msg.Type != "command" || msg.Command == nil {
			continue
		}
		s.handleCommand(id, *msg.Command)
	}
}

func (s *server) removeClient(id, logLine string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		if client, ok := s.clients[id]; ok {
			_ = client.conn.Close()
			delete(s.clients, id)
		}
		return
	}

	idx := s.playerSeatIndexLocked(id)
	if client, ok := s.clients[id]; ok {
		_ = client.conn.Close()
		delete(s.clients, id)
	}
	if player, ok := s.players[id]; ok {
		player.Connected = false
	}
	s.appendRoomLogLocked(logLine)

	if id == s.hostID {
		s.shutdownLocked("Host disconnected. Room saved for restore.")
		return
	}

	if s.phase == phaseCombat && s.combat != nil && idx >= 0 && idx < len(s.combat.Coop.EndTurnVotes) {
		s.combat.Coop.EndTurnVotes[idx] = true
		if allVotesReady(s.combat.Coop.EndTurnVotes) {
			engine.EndPartyTurn(s.lib, s.connectedSeatRunsLocked(), s.combat)
			if !s.combat.Won && !s.combat.Lost {
				engine.StartPartyTurn(s.lib, s.connectedSeatRunsLocked(), s.combat)
			}
			s.resolveCombatEndLocked()
		}
	}
	s.broadcastLocked()
}

func (s *server) handleCommand(playerID string, cmd commandPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()

	player := s.players[playerID]
	if player == nil || !player.Connected {
		return
	}
	if cmd.Action == "say" {
		s.handleChatCommandLocked(playerID, player, cmd)
		if s.closed {
			return
		}
		s.broadcastLocked()
		return
	}
	if cmd.Action == "chat_seen" {
		s.handleChatSeenLocked(playerID)
		if s.closed {
			return
		}
		s.broadcastLocked()
		return
	}
	if cmd.Action == "host" {
		s.handleHostTransferCommandLocked(playerID, cmd)
		if s.closed {
			return
		}
		s.broadcastLocked()
		return
	}
	if cmd.Action == "accept_host" || cmd.Action == "deny_host" || cmd.Action == "cancel_host" {
		s.handleHostTransferDecisionLocked(playerID, cmd)
		if s.closed {
			return
		}
		s.broadcastLocked()
		return
	}

	switch s.phase {
	case phaseLobby:
		s.handleLobbyCommandLocked(playerID, player, cmd)
	case phaseMap:
		s.handleMapCommandLocked(playerID, cmd)
	case phaseCombat:
		s.handleCombatCommandLocked(playerID, player, cmd)
	case phaseReward:
		s.handleRewardCommandLocked(playerID, cmd)
	case phaseEvent:
		s.handleEventCommandLocked(playerID, cmd)
	case phaseShop:
		s.handleShopCommandLocked(playerID, cmd)
	case phaseRest:
		s.handleRestCommandLocked(playerID, cmd)
	case phaseEquipment:
		s.handleEquipmentCommandLocked(playerID, cmd)
	case phaseDeckAction:
		s.handleDeckActionCommandLocked(playerID, cmd)
	case phaseSummary:
		s.handleSummaryCommandLocked(playerID, cmd)
	}

	if s.closed {
		return
	}
	s.broadcastLocked()
}

func (s *server) handleHostTransferCommandLocked(playerID string, cmd commandPayload) {
	if !s.requireHostLocked(playerID, "Only the host can transfer host control.") {
		return
	}
	if cmd.Seat <= 0 || cmd.Seat > len(s.order) {
		s.appendRoomLogLocked("Host transfer failed: invalid seat.")
		return
	}
	targetID := s.order[cmd.Seat-1]
	if targetID == s.hostID {
		s.appendRoomLogLocked("Host transfer failed: target seat already owns the room.")
		return
	}
	target := s.players[targetID]
	if target == nil {
		s.appendRoomLogLocked("Host transfer failed: target seat is empty.")
		return
	}
	if !target.Connected {
		s.appendRoomLogLocked("Host transfer failed: target seat is offline.")
		return
	}
	if s.hostTransfer != nil {
		s.appendRoomLogLocked("Host transfer request already pending.")
		return
	}
	s.hostTransfer = &hostTransferRequest{FromID: playerID, ToID: targetID, Seat: cmd.Seat}
	s.appendRoomLogLocked(fmt.Sprintf("Host transfer requested: Seat %d %s must confirm.", cmd.Seat, target.Name))
	for id := range s.clients {
		switch id {
		case targetID:
			s.setClientNoticeLocked(id, fmt.Sprintf("Host transfer requested to Seat %d. Use accept-host or deny-host.", cmd.Seat))
			s.setClientResumeLocked(id, []string{
				fmt.Sprintf("Pending host transfer request to Seat %d.", cmd.Seat),
				"Use accept-host to take control or deny-host to reject the request.",
			})
		case playerID:
			s.setClientNoticeLocked(id, fmt.Sprintf("Host transfer request sent to Seat %d %s.", cmd.Seat, target.Name))
		default:
			s.setClientNoticeLocked(id, fmt.Sprintf("Host transfer pending for Seat %d %s.", cmd.Seat, target.Name))
		}
	}
}

func (s *server) handleHostTransferDecisionLocked(playerID string, cmd commandPayload) {
	request := s.hostTransfer
	if request == nil {
		s.appendRoomLogLocked("No host transfer request is pending.")
		return
	}
	switch cmd.Action {
	case "cancel_host":
		if playerID != request.FromID {
			s.appendRoomLogLocked("Only the current host can cancel the pending transfer.")
			return
		}
		target := s.players[request.ToID]
		targetName := "target"
		if target != nil {
			targetName = target.Name
		}
		s.hostTransfer = nil
		s.appendRoomLogLocked(fmt.Sprintf("Host transfer request to %s was canceled.", targetName))
		for id := range s.clients {
			switch id {
			case playerID:
				s.setClientNoticeLocked(id, "Pending host transfer canceled.")
			case request.ToID:
				s.setClientNoticeLocked(id, "Host transfer request canceled.")
			default:
				s.setClientNoticeLocked(id, "Host transfer request canceled.")
			}
		}
	case "accept_host":
		if playerID != request.ToID {
			s.appendRoomLogLocked("Only the requested seat can accept host transfer.")
			return
		}
		target := s.players[request.ToID]
		oldHost := s.players[s.hostID]
		oldHostName := "Host"
		targetName := "Host"
		if oldHost != nil {
			oldHostName = oldHost.Name
		}
		if target != nil {
			targetName = target.Name
		}
		s.hostID = request.ToID
		s.hostTransfer = nil
		s.appendRoomLogLocked(fmt.Sprintf("Host transferred from %s to Seat %d %s.", oldHostName, request.Seat, targetName))
		for id := range s.clients {
			switch id {
			case request.ToID:
				s.setClientNoticeLocked(id, fmt.Sprintf("You are now the host. Host control moved to Seat %d.", request.Seat))
				s.setClientResumeLocked(id, []string{
					fmt.Sprintf("Host transferred to you as Seat %d.", request.Seat),
					"Your host-only commands and decisions are now active.",
				})
			default:
				s.setClientNoticeLocked(id, fmt.Sprintf("Host transferred to Seat %d %s.", request.Seat, targetName))
			}
		}
	case "deny_host":
		if playerID != request.ToID {
			s.appendRoomLogLocked("Only the requested seat can deny host transfer.")
			return
		}
		target := s.players[request.ToID]
		targetName := "target"
		if target != nil {
			targetName = target.Name
		}
		s.hostTransfer = nil
		s.appendRoomLogLocked(fmt.Sprintf("Host transfer request was denied by %s.", targetName))
		for id := range s.clients {
			switch id {
			case request.FromID:
				s.setClientNoticeLocked(id, fmt.Sprintf("Host transfer request to %s was denied.", targetName))
			case request.ToID:
				s.setClientNoticeLocked(id, "Host transfer request denied.")
			default:
				s.setClientNoticeLocked(id, fmt.Sprintf("Host transfer request to %s was denied.", targetName))
			}
		}
	}
}

func (s *server) handleChatCommandLocked(playerID string, player *roomPlayer, cmd commandPayload) {
	text := strings.TrimSpace(cmd.Value)
	if text == "" {
		s.appendRoomLogLocked("Chat message cannot be empty.")
		return
	}
	if len(text) > 240 {
		text = text[:240]
	}
	entry := formatChannelEntry("Chat", player.Name, text)
	s.chatLog = tailStrings(append(s.chatLog, entry), maxChatLog)
	preview := text
	if len(preview) > 72 {
		preview = preview[:72] + "..."
	}
	for id, client := range s.clients {
		if client == nil {
			continue
		}
		if id == playerID {
			client.chatSeen = len(s.chatLog)
			s.setClientNoticeLocked(id, "Chat sent.")
			continue
		}
		s.setClientNoticeLocked(id, fmt.Sprintf("Chat from %s: %s", player.Name, preview))
	}
}

func (s *server) handleChatSeenLocked(playerID string) {
	if client := s.clients[playerID]; client != nil {
		client.chatSeen = len(s.chatLog)
	}
}

func (s *server) handleLobbyCommandLocked(playerID string, player *roomPlayer, cmd commandPayload) {
	switch cmd.Action {
	case "ready":
		player.Ready = !player.Ready
		state := "not ready"
		if player.Ready {
			state = "ready"
		}
		s.appendRoomLogLocked(fmt.Sprintf("%s is now %s.", player.Name, state))
	case "class":
		if _, ok := s.lib.Classes[cmd.Value]; !ok {
			s.appendRoomLogLocked(fmt.Sprintf("Unknown class %q.", cmd.Value))
			return
		}
		player.ClassID = cmd.Value
		player.Ready = false
		s.appendRoomLogLocked(fmt.Sprintf("%s switched to %s.", player.Name, cmd.Value))
	case "mode":
		if !s.requireHostLocked(playerID, "Only the host can change the mode.") {
			return
		}
		switch strings.ToLower(cmd.Value) {
		case string(engine.ModeStory):
			s.mode = engine.ModeStory
		case string(engine.ModeEndless):
			s.mode = engine.ModeEndless
		default:
			s.appendRoomLogLocked(fmt.Sprintf("Unknown mode %q.", cmd.Value))
			return
		}
		s.appendRoomLogLocked(fmt.Sprintf("Mode set to %s.", s.mode))
	case "seed":
		if !s.requireHostLocked(playerID, "Only the host can change the seed.") {
			return
		}
		seed, err := strconv.ParseInt(cmd.Value, 10, 64)
		if err != nil {
			s.appendRoomLogLocked("Seed must be an integer.")
			return
		}
		s.seed = seed
		s.appendRoomLogLocked(fmt.Sprintf("Seed set to %d.", seed))
	case "start":
		if !s.requireHostLocked(playerID, "Only the host can start the run.") {
			return
		}
		if !s.canStartRunLocked() {
			if len(s.offlineSeatSummariesLocked()) > 0 {
				s.appendRoomLogLocked("Offline reserved seats must reconnect or be dropped before starting.")
			} else {
				s.appendRoomLogLocked("Everyone must be ready before starting.")
			}
			return
		}
		if !s.allReadyLocked() {
			s.appendRoomLogLocked("Everyone must be ready before starting.")
			return
		}
		if err := s.startRunLocked(); err != nil {
			s.appendRoomLogLocked("Failed to start run: " + err.Error())
			return
		}
	case "drop":
		if !s.requireHostLocked(playerID, "Only the host can drop offline seats.") {
			return
		}
		if err := s.dropOfflineSeatLocked(cmd.Seat); err != nil {
			s.appendRoomLogLocked("Seat drop failed: " + err.Error())
			return
		}
	case "drop_all":
		if !s.requireHostLocked(playerID, "Only the host can drop offline seats.") {
			return
		}
		dropped := s.dropAllOfflineSeatsLocked()
		if dropped == 0 {
			s.appendRoomLogLocked("No offline seats to drop.")
			return
		}
		s.appendRoomLogLocked(fmt.Sprintf("Dropped %d offline seat(s).", dropped))
	case "abandon":
		if !s.requireHostLocked(playerID, "Only the host can abandon the room.") {
			return
		}
		s.abandonLocked("Host abandoned the room. Saved room cleared.")
	}
}

func (s *server) handleMapCommandLocked(playerID string, cmd commandPayload) {
	if cmd.Action != "node" {
		return
	}
	state := s.seatStateLocked(playerID)
	reachable := engine.ReachableNodes(s.run)
	if cmd.ItemIndex <= 0 || cmd.ItemIndex > len(reachable) {
		s.appendRoomLogLocked("Invalid node selection.")
		return
	}
	state.MapVote = cmd.ItemIndex
	if player := s.players[playerID]; player != nil {
		s.appendRoomLogLocked(fmt.Sprintf("%s voted for %s.", player.Name, describeMapVoteChoice(reachable, cmd.ItemIndex)))
	}
	if !s.allConnectedSeatsDoneLocked(func(seat *seatRunState) bool { return seat.MapVote > 0 }) {
		return
	}
	chosenIndex := s.weightedSeatVoteLocked()
	if summary := s.mapVoteSummaryLocked(reachable); len(summary) > 0 {
		s.appendRoomLogLocked("Route vote summary: " + strings.Join(summary, " | "))
	}
	if chosenIndex <= 0 || chosenIndex > len(reachable) {
		s.appendRoomLogLocked("Failed to resolve node vote.")
		return
	}
	node := reachable[chosenIndex-1]
	selected, err := engine.SelectNode(s.run, node.ID)
	if err != nil {
		s.appendRoomLogLocked("Failed to select node: " + err.Error())
		return
	}
	s.clearMapVotesLocked()
	s.currentNode = selected
	if odds := s.mapVoteOddsLocked(chosenIndex); odds != "" {
		s.appendRoomLogLocked(fmt.Sprintf("Weighted route roll picked %s (%s).", describeMapVoteChoice(reachable, chosenIndex), odds))
	} else {
		s.appendRoomLogLocked(fmt.Sprintf("Weighted route roll picked %s.", describeMapVoteChoice(reachable, chosenIndex)))
	}
	s.startNodeFlowLocked(selected)
}

func (s *server) startNodeFlowLocked(selected engine.Node) {
	switch selected.Kind {
	case engine.NodeMonster, engine.NodeElite, engine.NodeBoss:
		s.startCombatNodeFlowLocked(selected)
	case engine.NodeEvent:
		s.startEventNodeFlowLocked(selected)
	case engine.NodeShop:
		s.startShopNodeFlowLocked()
	case engine.NodeRest:
		s.startRestNodeFlowLocked()
	}
}

func (s *server) startCombatNodeFlowLocked(selected engine.Node) {
	party := s.combatPartyLocked()
	combat, err := engine.StartEncounterForParty(s.lib, s.run, party, selected)
	if err != nil {
		s.appendRoomLogLocked("Failed to start combat: " + err.Error())
		return
	}
	s.combat = combat
	engine.StartPartyTurn(s.lib, s.connectedSeatRunsLocked(), s.combat)
	s.phase = phaseCombat
	s.announcePhaseLocked(fmt.Sprintf("Encounter started: %s.", selected.Label()))
}

func (s *server) startEventNodeFlowLocked(selected engine.Node) {
	for _, id := range s.connectedSeatIDsLocked() {
		_, run := s.seatContextLocked(id)
		if run == nil {
			continue
		}
		eventState, err := engine.StartEvent(s.lib, run, selected)
		if err != nil {
			s.appendRoomLogLocked("Failed to start event: " + err.Error())
			return
		}
		state := s.seatStateLocked(id)
		state.Event = &eventState
		state.EventDone = false
	}
	s.phase = phaseEvent
	s.announcePhaseLocked("Each player can resolve their own event choice.")
}

func (s *server) startShopNodeFlowLocked() {
	for _, id := range s.connectedSeatIDsLocked() {
		state, run := s.seatContextLocked(id)
		if state == nil || run == nil {
			continue
		}
		shop := engine.StartShop(s.lib, run)
		state.Shop = &shop
		state.ShopDone = false
	}
	s.phase = phaseShop
	s.announcePhaseLocked("Each player has their own shop state. Leave after you finish shopping.")
}

func (s *server) startRestNodeFlowLocked() {
	s.restLog = nil
	s.phase = phaseRest
	s.announcePhaseLocked("The shared campfire is ready.")
}

func (s *server) handleCombatCommandLocked(playerID string, player *roomPlayer, cmd commandPayload) {
	if s.combat == nil {
		return
	}
	memberIndex := s.playerSeatIndexLocked(playerID)
	if memberIndex < 0 {
		s.appendRoomLogLocked("Unable to locate active seat.")
		return
	}

	target := engine.CombatTarget{Kind: engine.CombatTargetNone}
	if cmd.TargetKind != "" {
		target.Kind = engine.CombatTargetKind(cmd.TargetKind)
		target.Index = max(0, cmd.TargetIndex-1)
	}

	switch cmd.Action {
	case "play":
		if memberIndex >= len(s.combat.Seats) || cmd.CardIndex <= 0 || cmd.CardIndex > len(s.combat.Seats[memberIndex].Hand) {
			s.appendRoomLogLocked("Invalid hand index.")
			return
		}
		run := s.seatRunLocked(playerID)
		if run == nil {
			s.appendRoomLogLocked("Player run not available.")
			return
		}
		cardName := s.lib.Cards[s.combat.Seats[memberIndex].Hand[cmd.CardIndex-1].ID].Name
		if err := engine.PlaySeatCardWithTarget(s.lib, run.Player, s.combat, memberIndex, cmd.CardIndex-1, target); err != nil {
			s.appendRoomLogLocked("Play failed: " + err.Error())
			return
		}
		s.appendCombatActionLogLocked(playerID, fmt.Sprintf("打出 %s", cardName), target)
		s.handleCoopCombatActionLocked(memberIndex, player.Name)
	case "potion":
		if memberIndex >= len(s.combat.Seats) || cmd.ItemIndex <= 0 || cmd.ItemIndex > len(s.combat.Seats[memberIndex].Potions) {
			s.appendRoomLogLocked("Invalid potion index.")
			return
		}
		run := s.seatRunLocked(playerID)
		if run == nil {
			s.appendRoomLogLocked("Player run not available.")
			return
		}
		potionID := s.combat.Seats[memberIndex].Potions[cmd.ItemIndex-1]
		potionName := s.lib.Potions[potionID].Name
		if err := engine.UseSeatPotionWithTarget(s.lib, run.Player, s.combat, memberIndex, potionID, target); err != nil {
			s.appendRoomLogLocked("Potion failed: " + err.Error())
			return
		}
		s.appendCombatActionLogLocked(playerID, fmt.Sprintf("使用药水 %s", potionName), target)
		s.handleCoopCombatActionLocked(memberIndex, player.Name)
	case "end":
		if engine.RequestEndTurnVote(s.combat, memberIndex) {
			s.appendCombatActionLogLocked(playerID, "提交结束回合，所有在线座位已就绪", engine.CombatTarget{Kind: engine.CombatTargetNone})
			engine.EndPartyTurn(s.lib, s.connectedSeatRunsLocked(), s.combat)
			if !s.combat.Won && !s.combat.Lost {
				engine.StartPartyTurn(s.lib, s.connectedSeatRunsLocked(), s.combat)
			}
		} else {
			s.appendCombatActionLogLocked(playerID, "提交结束回合，等待其他队友", engine.CombatTarget{Kind: engine.CombatTargetNone})
			s.appendRoomLogLocked(fmt.Sprintf("%s voted to end the turn.", player.Name))
		}
	default:
		return
	}

	s.resolveCombatEndLocked()
}

func (s *server) appendCombatActionLogLocked(playerID, action string, target engine.CombatTarget) {
	if s.combat == nil {
		return
	}
	seat := s.playerSeatIndexLocked(playerID) + 1
	name := playerID
	if player := s.players[playerID]; player != nil {
		name = player.Name
	}
	line := fmt.Sprintf("Seat %d %s %s", seat, name, action)
	if target.Kind != engine.CombatTargetNone {
		line += " | " + engine.DescribeCombatTarget(s.combat, target)
	}
	s.combat.Log = append(s.combat.Log, engine.CombatLogEntry{Turn: s.combat.Turn, Text: line})
}

func (s *server) handleCoopCombatActionLocked(memberIndex int, playerName string) {
	if s.combat == nil {
		return
	}
	firstForSeat, uniqueActors := engine.RecordCoopAction(s.combat, memberIndex)
	if !firstForSeat || uniqueActors < 2 || s.combat.Coop.TeamComboDone {
		return
	}
	triggered := false
	players := s.connectedSeatRunsLocked()
	if len(players) == 0 && len(s.combat.SeatPlayers) > 0 {
		players = append(players, s.combat.SeatPlayers...)
	}
	if len(players) == 0 && s.run != nil {
		players = append(players, s.run.Player)
	}
	for seatIndex, run := range players {
		for _, relicID := range run.Relics {
			relic, ok := s.lib.Relics[relicID]
			if !ok {
				continue
			}
			for _, effect := range relic.Effects {
				if effect.Trigger != "team_combo" {
					continue
				}
				if err := engine.ApplySeatExternalCombatEffect(s.lib, run, s.combat, seatIndex, effect, engine.CombatTarget{}); err != nil {
					s.appendRoomLogLocked("Coop relic trigger failed: " + err.Error())
					continue
				}
				triggered = true
			}
		}
	}
	if triggered {
		s.combat.Coop.TeamComboDone = true
		s.combat.Log = append(s.combat.Log, engine.CombatLogEntry{
			Turn: s.combat.Turn,
			Text: fmt.Sprintf("协作连携触发：本回合已有 %d 名不同玩家行动", uniqueActors),
		})
		s.appendRoomLogLocked(fmt.Sprintf("Coop relic combo triggered after %s acted.", playerName))
	}
}

func (s *server) handleRewardCommandLocked(playerID string, cmd commandPayload) {
	state, _ := s.seatContextLocked(playerID)
	if s.combat == nil || state == nil || state.Reward == nil || state.RewardDone {
		return
	}

	switch cmd.Action {
	case "take":
		if cmd.ItemIndex <= 0 || cmd.ItemIndex > len(state.Reward.CardChoices) {
			s.appendRoomLogLocked("Invalid reward card.")
			return
		}
		cardID := state.Reward.CardChoices[cmd.ItemIndex-1].ID
		if state.Reward.EquipmentID != "" {
			if err := s.startEquipmentFlowLocked(playerID, "reward", cardID, "", ""); err != nil {
				s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
			}
			return
		}
		if err := s.applyRewardLocked(playerID, cardID, false); err != nil {
			s.appendRoomLogLocked("Reward resolution failed: " + err.Error())
		}
	case "skip":
		if state.Reward.EquipmentID != "" {
			if err := s.startEquipmentFlowLocked(playerID, "reward", "", "", ""); err != nil {
				s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
			}
			return
		}
		if err := s.applyRewardLocked(playerID, "", false); err != nil {
			s.appendRoomLogLocked("Reward resolution failed: " + err.Error())
		}
	}
}

func (s *server) handleEventCommandLocked(playerID string, cmd commandPayload) {
	state, run := s.seatContextLocked(playerID)
	if state == nil || state.Event == nil || state.EventDone {
		return
	}
	if cmd.Action != "choose" || cmd.ItemIndex <= 0 || cmd.ItemIndex > len(state.Event.Event.Choices) {
		s.appendRoomLogLocked("Invalid event choice.")
		return
	}
	choice := state.Event.Event.Choices[cmd.ItemIndex-1]
	if engine.EventChoiceEquipmentID(choice) != "" {
		if err := s.startEquipmentFlowLocked(playerID, "event", "", "", choice.ID); err != nil {
			s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
		}
		return
	}

	logStart := len(state.Event.Log)
	if err := engine.ResolveEventDecision(s.lib, run, state.Event, choice.ID, true); err != nil {
		s.appendRoomLogLocked("Event resolution failed: " + err.Error())
		return
	}
	if err := engine.AdvanceNonCombatNode(run, s.currentNode); err != nil {
		s.appendRoomLogLocked("Failed to advance after event: " + err.Error())
		return
	}
	state.EventDone = true
	s.appendRoomLogLocked(fmt.Sprintf("Event resolved: %s.", choice.Label))
	s.appendEventOutcomeLogsLocked(state.Event, choice, logStart)
	if s.allConnectedSeatsDoneLocked(func(seat *seatRunState) bool { return seat.EventDone }) {
		if err := s.advanceSharedRunLocked(); err != nil {
			s.appendRoomLogLocked("Failed to advance shared map after event: " + err.Error())
			return
		}
		s.afterNodeAdvanceLocked()
	}
}

func (s *server) handleShopCommandLocked(playerID string, cmd commandPayload) {
	state, run := s.seatContextLocked(playerID)
	if state == nil || state.Shop == nil || state.ShopDone {
		return
	}

	switch cmd.Action {
	case "leave":
		if err := engine.AdvanceNonCombatNode(run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to leave shop: " + err.Error())
			return
		}
		state.ShopDone = true
		if player := s.players[playerID]; player != nil {
			s.appendRoomLogLocked(fmt.Sprintf("%s left their shop.", player.Name))
		}
		if s.allConnectedSeatsDoneLocked(func(seat *seatRunState) bool { return seat.ShopDone }) {
			if err := s.advanceSharedRunLocked(); err != nil {
				s.appendRoomLogLocked("Failed to leave shop: " + err.Error())
				return
			}
			s.afterNodeAdvanceLocked()
		}
	case "buy":
		if cmd.ItemIndex <= 0 || cmd.ItemIndex > len(state.Shop.Offers) {
			s.appendRoomLogLocked("Invalid shop offer.")
			return
		}
		offer := state.Shop.Offers[cmd.ItemIndex-1]
		if offer.Kind == "equipment" {
			if err := s.startEquipmentFlowLocked(playerID, "shop", "", offer.ID, ""); err != nil {
				s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
			}
			return
		}
		if offer.Kind == "remove" {
			if err := s.startDeckActionLocked(playerID, "shop_remove", offer.Price); err != nil {
				s.appendRoomLogLocked(err.Error())
			}
			return
		}
		if offer.Kind == "service" {
			plan, err := engine.ShopOfferDeckActionPlan(s.lib, run, state.Shop, offer.ID)
			if err != nil {
				s.appendRoomLogLocked("Purchase failed: " + err.Error())
				return
			}
			if plan != nil {
				s.shopOfferID = offer.ID
				if err := s.startDeckActionPlanLocked(playerID, *plan); err != nil {
					s.appendRoomLogLocked("Purchase failed: " + err.Error())
				}
				return
			}
		}
		logStart := len(state.Shop.Log)
		if err := engine.ApplyShopPurchase(s.lib, run, state.Shop, offer.ID); err != nil {
			s.appendRoomLogLocked("Purchase failed: " + err.Error())
			return
		}
		if player := s.players[playerID]; player != nil {
			s.appendRoomLogLocked(fmt.Sprintf("%s purchased %s.", player.Name, offer.Name))
		}
		s.appendShopOutcomeLogsLocked(state.Shop, offer, logStart)
	}
}

func (s *server) handleRestCommandLocked(playerID string, cmd commandPayload) {
	if !s.requireHostLocked(playerID, "Only the host can resolve the campfire.") {
		return
	}
	if cmd.Action != "heal" && cmd.Action != "upgrade" {
		return
	}
	ids := s.connectedSeatIDsLocked()
	logs := make([]string, 0, len(ids))
	for _, id := range ids {
		state, run := s.seatContextLocked(id)
		if run == nil {
			continue
		}
		restState, err := engine.ResolveRest(s.lib, run, cmd.Action)
		if err != nil {
			s.appendRoomLogLocked("Campfire action failed: " + err.Error())
			return
		}
		if err := engine.AdvanceNonCombatNode(run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to leave campfire: " + err.Error())
			return
		}
		name := id
		if player := s.players[id]; player != nil && strings.TrimSpace(player.Name) != "" {
			name = player.Name
		}
		for _, line := range restState.Log {
			logs = append(logs, fmt.Sprintf("%s: %s", name, line))
		}
		if state != nil {
			state.RestLog = append([]string{}, restState.Log...)
		}
	}
	s.rebuildSharedRunPlayerLocked()
	s.restLog = logs
	if err := s.advanceSharedRunLocked(); err != nil {
		s.appendRoomLogLocked("Failed to leave campfire: " + err.Error())
		return
	}
	s.appendRoomLogLocked(fmt.Sprintf("Campfire action: %s.", cmd.Action))
	s.afterNodeAdvanceLocked()
}

func (s *server) handleEquipmentCommandLocked(playerID string, cmd commandPayload) {
	if s.flowOwner != playerID {
		return
	}
	if s.equipOffer == nil {
		return
	}
	state, run := s.seatContextLocked(playerID)
	if state == nil || run == nil {
		return
	}

	take := cmd.Action == "take"
	if cmd.Action != "take" && cmd.Action != "skip" {
		return
	}

	switch s.equipFrom {
	case "reward":
		if err := s.applyRewardLocked(playerID, s.rewardCard, take); err != nil {
			s.appendRoomLogLocked("Reward equipment failed: " + err.Error())
			return
		}
		s.appendRoomLogLocked(equipmentDecisionText(take, s.equipOffer.EquipmentID))
		s.clearEquipmentFlowLocked()
		if s.phase != phaseMap && s.phase != phaseSummary {
			s.phase = phaseReward
		}
	case "shop":
		logStart := len(state.Shop.Log)
		if err := engine.ApplyShopEquipmentPurchase(s.lib, run, state.Shop, s.shopOfferID, take); err != nil {
			s.appendRoomLogLocked("Shop equipment failed: " + err.Error())
			return
		}
		s.appendRoomLogLocked(equipmentDecisionText(take, s.equipOffer.EquipmentID))
		if offer, err := engineShopOfferByID(state.Shop, s.shopOfferID); err == nil {
			s.appendShopOutcomeLogsLocked(state.Shop, offer, logStart)
		}
		s.clearEquipmentFlowLocked()
		s.phase = phaseShop
		s.announcePhaseLocked("Returned to personal shop resolution.")
	case "event":
		logStart := len(state.Event.Log)
		choice := eventChoiceByID(state.Event, s.eventChoice)
		if choice != nil {
			if plan, err := engine.EventChoiceDeckActionPlan(s.lib, run, *choice, take); err != nil {
				s.appendRoomLogLocked("Event equipment failed: " + err.Error())
				return
			} else if plan != nil {
				if err := s.startDeckActionPlanLocked(playerID, *plan); err != nil {
					s.appendRoomLogLocked("Event equipment failed: " + err.Error())
				}
				return
			}
		}
		if err := engine.ResolveEventDecision(s.lib, run, state.Event, s.eventChoice, take); err != nil {
			s.appendRoomLogLocked("Event equipment failed: " + err.Error())
			return
		}
		if err := engine.AdvanceNonCombatNode(run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to advance after event: " + err.Error())
			return
		}
		state.EventDone = true
		s.appendRoomLogLocked(equipmentDecisionText(take, s.equipOffer.EquipmentID))
		if choice != nil {
			s.appendEventOutcomeLogsLocked(state.Event, *choice, logStart)
		}
		s.clearEquipmentFlowLocked()
		if s.allConnectedSeatsDoneLocked(func(seat *seatRunState) bool { return seat.EventDone }) {
			if err := s.advanceSharedRunLocked(); err != nil {
				s.appendRoomLogLocked("Failed to advance after event: " + err.Error())
				return
			}
			s.afterNodeAdvanceLocked()
		} else {
			s.phase = phaseEvent
		}
	}
}

func (s *server) handleDeckActionCommandLocked(playerID string, cmd commandPayload) {
	if s.flowOwner != playerID {
		return
	}
	state, run := s.seatContextLocked(playerID)
	if run == nil || state == nil {
		return
	}
	if cmd.Action == "back" {
		s.cancelDeckActionLocked()
		return
	}
	if cmd.Action != "choose" || cmd.ItemIndex <= 0 || cmd.ItemIndex > len(s.deckActionIndexes) {
		s.appendRoomLogLocked("Invalid deck choice.")
		return
	}

	deckIndex := s.deckActionIndexes[cmd.ItemIndex-1]
	switch s.deckActionMode {
	case "rest_upgrade":
		name, err := engine.UpgradeDeckCard(s.lib, &run.Player, deckIndex)
		if err != nil {
			s.appendRoomLogLocked("Upgrade failed: " + err.Error())
			return
		}
		state.RestLog = []string{"Upgraded " + name}
		if err := engine.AdvanceNonCombatNode(run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to leave campfire: " + err.Error())
			return
		}
		s.appendRoomLogLocked("Card upgraded at campfire.")
		s.clearDeckActionLocked()
		s.afterNodeAdvanceLocked()
	case "shop_remove":
		if err := engine.ApplyShopCardRemoval(s.lib, run, state.Shop, "remove-card", deckIndex); err != nil {
			s.appendRoomLogLocked("Card removal failed: " + err.Error())
			return
		}
		s.appendRoomLogLocked("Card removed from the deck.")
		s.clearDeckActionLocked()
		s.phase = phaseShop
		s.announcePhaseLocked("Returned to personal shop resolution.")
	case "shop_augment_card":
		if err := engine.ApplyShopServiceWithDeckChoice(s.lib, run, state.Shop, s.shopOfferID, deckIndex); err != nil {
			s.appendRoomLogLocked("Shop augment failed: " + err.Error())
			return
		}
		s.appendRoomLogLocked("Workshop service applied to the selected card.")
		s.clearDeckActionLocked()
		s.phase = phaseShop
		s.announcePhaseLocked("Returned to personal shop resolution.")
	case "event_augment_card":
		if state.Event == nil {
			s.appendRoomLogLocked("Event augment failed: event state unavailable.")
			return
		}
		logStart := len(state.Event.Log)
		choice := eventChoiceByID(state.Event, s.eventChoice)
		if err := engine.ResolveEventDecisionWithDeckChoice(s.lib, run, state.Event, s.eventChoice, s.deckActionTakeEquip, deckIndex); err != nil {
			s.appendRoomLogLocked("Event augment failed: " + err.Error())
			return
		}
		if err := engine.AdvanceNonCombatNode(run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to advance after event: " + err.Error())
			return
		}
		state.EventDone = true
		if choice != nil {
			s.appendEventOutcomeLogsLocked(state.Event, *choice, logStart)
		}
		s.clearDeckActionLocked()
		if s.allConnectedSeatsDoneLocked(func(seat *seatRunState) bool { return seat.EventDone }) {
			if err := s.advanceSharedRunLocked(); err != nil {
				s.appendRoomLogLocked("Failed to advance after event: " + err.Error())
				return
			}
			s.afterNodeAdvanceLocked()
		} else {
			s.phase = phaseEvent
		}
	}
}

func (s *server) handleSummaryCommandLocked(playerID string, cmd commandPayload) {
	switch cmd.Action {
	case "new":
		if !s.requireHostLocked(playerID, "Only the host can reset the room.") {
			return
		}
		s.resetLobbyLocked()
		s.appendRoomLogLocked("Returned to lobby. Players can ready up again.")
	case "abandon":
		if !s.requireHostLocked(playerID, "Only the host can abandon the room.") {
			return
		}
		s.abandonLocked("Host abandoned the room after summary. Saved room cleared.")
	}
}

func (s *server) startRunLocked() error {
	players, err := s.buildPartyStatesLocked()
	if err != nil {
		return err
	}
	if len(players) == 0 {
		return fmt.Errorf("no connected players")
	}
	if s.seed == 0 {
		s.seed = time.Now().UnixNano()
	}
	graph := engine.GenerateActMap(s.seed, 1, nil)
	if err := engine.ValidateMapConstraints(graph); err != nil {
		return err
	}

	s.run = &engine.RunState{
		Version:      1,
		Mode:         s.mode,
		Seed:         s.seed,
		Act:          1,
		CurrentFloor: 0,
		PartySize:    len(players),
		Status:       engine.RunStatusActive,
		Player:       players[0],
		Map:          graph,
		Reachable:    firstFloorNodeIDs(graph),
		History:      []string{},
	}
	if s.seatStates == nil {
		s.seatStates = map[string]*seatRunState{}
	}
	sharedGraph := s.run.Map
	reachable := append([]string{}, s.run.Reachable...)
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		state := s.seatStateLocked(id)
		base, err := buildBasePlayerState(s.lib, player.ClassID, player.Name)
		if err != nil {
			return err
		}
		state.Run = &engine.RunState{
			Version:      1,
			Mode:         s.mode,
			Seed:         s.seed,
			Act:          1,
			CurrentFloor: 0,
			PartySize:    len(players),
			Status:       engine.RunStatusActive,
			Player:       base,
			Map:          sharedGraph,
			Reachable:    append([]string{}, reachable...),
			History:      []string{},
		}
		state.Reward = nil
		state.RewardDone = false
		state.Event = nil
		state.EventDone = false
		state.Shop = nil
		state.ShopDone = false
		state.MapVote = 0
		state.RestLog = nil
	}
	s.flowOwner = ""
	s.rebuildSharedRunPlayerLocked()
	s.clearNodeFlowLocked()
	s.phase = phaseMap
	s.appendRoomLogLocked(fmt.Sprintf("Run started: %s seed=%d.", s.mode, s.seed))
	s.announcePhaseLocked("The shared map is ready.")
	return nil
}

func (s *server) resolveCombatEndLocked() {
	if s.combat == nil {
		return
	}
	if s.combat.Won {
		s.syncPartyFromCombatLocked()
		for _, id := range s.connectedSeatIDsLocked() {
			seat := s.seatStateLocked(id)
			if seat == nil || seat.Run == nil {
				continue
			}
			reward := engine.BuildReward(s.lib, seat.Run, s.currentNode, s.combat.RewardBasis)
			seat.Reward = &reward
			seat.RewardDone = false
		}
		s.phase = phaseReward
		s.appendRoomLogLocked("Combat won. Personal reward phase started.")
		s.announcePhaseLocked("Each player can resolve their own reward.")
		return
	}
	if s.combat.Lost {
		s.syncPartyFromCombatLocked()
		s.run.Status = engine.RunStatusLost
		for _, id := range s.order {
			seat := s.seatStateLocked(id)
			if seat == nil || seat.Run == nil {
				continue
			}
			seat.Run.Status = engine.RunStatusLost
			seat.Run.Player.HP = 0
		}
		s.phase = phaseSummary
		s.appendRoomLogLocked("The party was defeated.")
		s.announcePhaseLocked("Run summary opened after defeat.")
	}
}

func (s *server) applyRewardLocked(playerID, cardID string, takeEquipment bool) error {
	state, run := s.seatContextLocked(playerID)
	if state == nil || state.Run == nil || state.Reward == nil || s.combat == nil {
		return fmt.Errorf("reward not available")
	}
	combatCopy := *s.combat
	combatCopy.Reward = *state.Reward
	if err := engine.ApplyCombatResultDecision(s.lib, run, s.currentNode, &combatCopy, cardID, takeEquipment); err != nil {
		return err
	}
	state.RewardDone = true
	if cardID != "" {
		s.appendRoomLogLocked(fmt.Sprintf("Reward card added: %s.", s.lib.Cards[cardID].Name))
		if slices.Contains(s.lib.Cards[cardID].Flags, "coop_only") {
			s.appendRoomLogLocked(fmt.Sprintf("Picked CO-OP reward: %s.", s.lib.Cards[cardID].Name))
		}
	} else {
		s.appendRoomLogLocked("Reward card skipped.")
	}
	if state.Reward.RelicID != "" && slices.Contains(s.lib.Relics[state.Reward.RelicID].Flags, "coop_only") {
		s.appendRoomLogLocked(fmt.Sprintf("Reward included CO-OP relic: %s.", s.lib.Relics[state.Reward.RelicID].Name))
	}
	if s.allConnectedSeatsDoneLocked(func(seat *seatRunState) bool { return seat.RewardDone }) {
		if err := s.advanceSharedRunLocked(); err != nil {
			return err
		}
		s.afterNodeAdvanceLocked()
	}
	return nil
}

func (s *server) appendShopOutcomeLogsLocked(shop *engine.ShopState, offer engine.ShopOffer, logStart int) {
	if shop == nil {
		return
	}
	for _, line := range shop.Log[logStart:] {
		s.appendRoomLogLocked("Shop outcome: " + line)
	}
	if slices.Contains(shopOfferBadges(s.lib, offer), "CO-OP") {
		s.appendRoomLogLocked(fmt.Sprintf("Purchased CO-OP offer: %s.", offer.Name))
	}
}

func (s *server) appendEventOutcomeLogsLocked(eventState *engine.EventState, choice content.EventChoiceDef, logStart int) {
	if eventState == nil {
		return
	}
	for _, line := range eventState.Log[logStart:] {
		if line == choice.Label {
			continue
		}
		s.appendRoomLogLocked("Event outcome: " + line)
	}
	for _, line := range coopOutcomeLinesFromChoice(s.lib, choice) {
		s.appendRoomLogLocked(line)
	}
}

func coopOutcomeLinesFromChoice(lib *content.Library, choice content.EventChoiceDef) []string {
	out := []string{}
	for _, effect := range choice.Effects {
		switch effect.Op {
		case "add_card":
			if card, ok := lib.Cards[effect.CardID]; ok && slices.Contains(card.Flags, "coop_only") {
				out = append(out, fmt.Sprintf("Event granted CO-OP card: %s.", card.Name))
			}
		case "gain_relic":
			if relic, ok := lib.Relics[effect.ItemID]; ok && slices.Contains(relic.Flags, "coop_only") {
				out = append(out, fmt.Sprintf("Event granted CO-OP relic: %s.", relic.Name))
			}
		case "upgrade_relic":
			if relic, ok := lib.Relics[effect.ResultID]; ok && slices.Contains(relic.Flags, "coop_only") {
				out = append(out, fmt.Sprintf("Event upgraded into CO-OP relic: %s.", relic.Name))
			}
		case "augment_card":
			out = append(out, "Event applies a card augment choice.")
		}
	}
	return out
}

func engineShopOfferByID(shop *engine.ShopState, offerID string) (engine.ShopOffer, error) {
	if shop == nil {
		return engine.ShopOffer{}, fmt.Errorf("shop not available")
	}
	for _, offer := range shop.Offers {
		if offer.ID == offerID {
			return offer, nil
		}
	}
	return engine.ShopOffer{}, fmt.Errorf("offer %q not found", offerID)
}

func eventChoiceByID(state *engine.EventState, choiceID string) *content.EventChoiceDef {
	if state == nil {
		return nil
	}
	for i := range state.Event.Choices {
		if state.Event.Choices[i].ID == choiceID {
			return &state.Event.Choices[i]
		}
	}
	return nil
}

func (s *server) startEquipmentFlowLocked(playerID, source, rewardCard, shopOfferID, eventChoice string) error {
	state, run := s.seatContextLocked(playerID)
	if state == nil || run == nil {
		return fmt.Errorf("player state not available")
	}
	var (
		equipmentID string
		price       int
	)
	switch source {
	case "reward":
		if state.Reward == nil {
			return fmt.Errorf("reward not available")
		}
		equipmentID = state.Reward.EquipmentID
	case "shop":
		if state.Shop == nil {
			return fmt.Errorf("shop not available")
		}
		offer, err := findShopOfferByID(state.Shop, shopOfferID)
		if err != nil {
			return err
		}
		equipmentID = offer.ItemID
		price = offer.Price
	case "event":
		if state.Event == nil {
			return fmt.Errorf("event not available")
		}
		for _, choice := range state.Event.Event.Choices {
			if choice.ID == eventChoice {
				equipmentID = engine.EventChoiceEquipmentID(choice)
				break
			}
		}
	default:
		return fmt.Errorf("unknown equipment source %q", source)
	}
	if equipmentID == "" {
		return fmt.Errorf("no equipment available")
	}
	offer, err := engine.BuildEquipmentOffer(s.lib, run.Player, equipmentID, source, price)
	if err != nil {
		return err
	}
	s.equipOffer = &offer
	s.flowOwner = playerID
	s.equipFrom = source
	s.rewardCard = rewardCard
	s.shopOfferID = shopOfferID
	s.eventChoice = eventChoice
	s.phase = phaseEquipment
	s.announcePhaseLocked("An equipment replacement prompt is active.")
	return nil
}

func (s *server) startDeckActionLocked(playerID, mode string, price int) error {
	_, run := s.seatContextLocked(playerID)
	if run == nil {
		return fmt.Errorf("run not available")
	}
	indexes := []int{}
	title := ""
	subtitle := ""

	switch mode {
	case "rest_upgrade":
		indexes = engine.UpgradableCardIndexes(s.lib, run.Player.Deck)
		title = "Upgrade a card"
		subtitle = "Choose one card to upgrade at the campfire."
	case "shop_remove":
		if len(run.Player.Deck) == 0 {
			return fmt.Errorf("deck is empty")
		}
		indexes = make([]int, 0, len(run.Player.Deck))
		for i := range run.Player.Deck {
			indexes = append(indexes, i)
		}
		title = "Remove a card"
		subtitle = fmt.Sprintf("Choose one card to remove for %d gold.", price)
	default:
		return fmt.Errorf("unknown deck action mode %q", mode)
	}

	if len(indexes) == 0 {
		return fmt.Errorf("no valid cards for this action")
	}

	s.deckActionMode = mode
	s.deckActionTitle = title
	s.deckActionSubtitle = subtitle
	s.deckActionIndexes = indexes
	s.deckActionPrice = price
	s.deckActionEffect = nil
	s.deckActionTakeEquip = false
	s.flowOwner = playerID
	s.phase = phaseDeckAction
	s.announcePhaseLocked(title + ".")
	return nil
}

func (s *server) startDeckActionPlanLocked(playerID string, plan engine.DeckActionPlan) error {
	if len(plan.Indexes) == 0 {
		return fmt.Errorf("no valid cards for this action")
	}
	s.deckActionMode = plan.Mode
	s.deckActionTitle = plan.Title
	s.deckActionSubtitle = plan.Subtitle
	s.deckActionIndexes = append([]int{}, plan.Indexes...)
	s.deckActionPrice = plan.Price
	s.deckActionEffect = nil
	s.deckActionTakeEquip = plan.TakeEquipment
	if plan.Effect != nil {
		effect := *plan.Effect
		s.deckActionEffect = &effect
	}
	s.flowOwner = playerID
	s.phase = phaseDeckAction
	s.announcePhaseLocked(plan.Title + ".")
	return nil
}

func (s *server) cancelDeckActionLocked() {
	switch s.deckActionMode {
	case "rest_upgrade":
		s.phase = phaseRest
	case "shop_remove":
		s.phase = phaseShop
	default:
		s.phase = phaseMap
	}
	s.flowOwner = ""
	s.clearDeckActionLocked()
	s.announcePhaseLocked("Returned after cancelling the deck action.")
}

func (s *server) afterNodeAdvanceLocked() {
	if s.run == nil {
		s.phase = phaseLobby
		s.clearNodeFlowLocked()
		s.announcePhaseLocked("Returned to the LAN lobby.")
		return
	}
	if s.run.Status != engine.RunStatusActive {
		s.phase = phaseSummary
		s.clearNodeFlowLocked()
		s.announcePhaseLocked("Run summary opened.")
		return
	}
	s.clearNodeFlowLocked()
	s.phase = phaseMap
	s.announcePhaseLocked("Back on the shared map.")
}

func (s *server) clearNodeFlowLocked() {
	s.currentNode = engine.Node{}
	s.combat = nil
	s.restLog = nil
	s.flowOwner = ""
	s.clearMapVotesLocked()
	for _, state := range s.seatStates {
		if state == nil {
			continue
		}
		state.Reward = nil
		state.RewardDone = false
		state.Event = nil
		state.EventDone = false
		state.Shop = nil
		state.ShopDone = false
		state.RestLog = nil
	}
	s.clearEquipmentFlowLocked()
	s.clearDeckActionLocked()
}

func (s *server) clearEquipmentFlowLocked() {
	s.equipOffer = nil
	s.flowOwner = ""
	s.equipFrom = ""
	s.rewardCard = ""
	s.shopOfferID = ""
	s.eventChoice = ""
}

func (s *server) clearDeckActionLocked() {
	s.deckActionMode = ""
	s.deckActionTitle = ""
	s.deckActionSubtitle = ""
	s.deckActionIndexes = nil
	s.deckActionPrice = 0
	s.deckActionEffect = nil
	s.deckActionTakeEquip = false
	s.shopOfferID = ""
}

func (s *server) resetLobbyLocked() {
	s.run = nil
	for _, state := range s.seatStates {
		if state == nil {
			continue
		}
		state.Run = nil
		state.Reward = nil
		state.RewardDone = false
		state.Event = nil
		state.EventDone = false
		state.Shop = nil
		state.ShopDone = false
		state.MapVote = 0
		state.RestLog = nil
	}
	s.clearNodeFlowLocked()
	s.phase = phaseLobby
	s.seed = time.Now().UnixNano()
	for _, id := range s.order {
		if player := s.players[id]; player != nil {
			player.Ready = false
		}
	}
	s.announcePhaseLocked("Room reset to lobby.")
}

func (s *server) shutdownLocked(logLine string) {
	s.closeRoomLocked(logLine, "room closed: host session ended, room saved for restore", true)
}

func (s *server) abandonLocked(logLine string) {
	s.closeRoomLocked(logLine, "room closed: host abandoned the room and cleared the saved room", false)
}

func (s *server) closeRoomLocked(logLine, clientNotice string, persist bool) {
	if s.closed {
		return
	}
	if logLine != "" {
		s.appendRoomLogLocked(logLine)
	}
	for _, player := range s.players {
		if player != nil {
			player.Connected = false
		}
	}
	s.closed = true
	if persist {
		_ = s.persistLocked()
	} else {
		_ = clearSavedRoom(s.savePath)
	}
	for id, client := range s.clients {
		if clientNotice != "" {
			_ = client.enc.Encode(message{Type: "error", Error: clientNotice})
		}
		_ = client.conn.Close()
		delete(s.clients, id)
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

func (s *server) requireHostLocked(playerID, reason string) bool {
	if playerID == s.hostID {
		return true
	}
	s.appendRoomLogLocked(reason)
	return false
}

func runClient(addr, name, classID string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	return runClientOnConn(conn, name, classID)
}

func runClientWithRetry(addr, name, classID string, timeout time.Duration) error {
	conn, err := dialTCPWithRetry(addr, timeout)
	if err != nil {
		return err
	}
	return runClientOnConn(conn, name, classID)
}

func runClientOnConn(conn net.Conn, name, classID string) error {
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(message{
		Type:  "hello",
		Hello: &helloPayload{Name: name, ClassID: classID},
	}); err != nil {
		return err
	}

	snapshots := make(chan *roomSnapshot, 8)
	errs := make(chan error, 1)
	go func() {
		for {
			var msg message
			if err := dec.Decode(&msg); err != nil {
				errs <- err
				return
			}
			if msg.Type == "snapshot" && msg.Snapshot != nil {
				snapshots <- msg.Snapshot
			}
			if msg.Type == "error" {
				errs <- fmt.Errorf("%s", msg.Error)
				return
			}
		}
	}()

	inputs := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputs <- scanner.Text()
		}
		close(inputs)
	}()

	var current *roomSnapshot
	for {
		select {
		case snap := <-snapshots:
			current = snap
			renderSnapshot(snap)
		case line, ok := <-inputs:
			if !ok {
				return nil
			}
			if current == nil {
				continue
			}
			localCmd, handled := handleLocalClientCommand(current, line)
			if handled {
				if localCmd != nil {
					if err := enc.Encode(message{Type: "command", Command: localCmd}); err != nil {
						return err
					}
				}
				continue
			}
			cmd, quit, err := parseClientCommand(current, line)
			if err != nil {
				fmt.Println("Command error:", err)
				continue
			}
			if quit {
				return nil
			}
			if cmd != nil {
				if err := enc.Encode(message{Type: "command", Command: cmd}); err != nil {
					return err
				}
			}
		case err := <-errs:
			if isGracefulRoomClose(err) {
				if err != nil {
					fmt.Println(err.Error())
				}
				return nil
			}
			return err
		}
	}
}

func handleLocalClientCommand(snapshot *roomSnapshot, line string) (*commandPayload, bool) {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", " ":
		return nil, true
	case "help", "?":
		renderSnapshot(snapshot)
		fmt.Println("Local commands:")
		fmt.Println("- help / ?    redraw the current room view and show this local help")
		fmt.Println("- status      redraw the current room view")
		fmt.Println("- log         print the latest room log without waiting for a state change")
		fmt.Println("- chatlog     print the latest room chat and mark it as read on the room state")
		fmt.Println("- quit        disconnect this local client")
		return nil, true
	case "status":
		renderSnapshot(snapshot)
		return nil, true
	case "log":
		renderChannelSection("System", "Latest system log:", "System", snapshot.RoomLog)
		return nil, true
	case "chatlog":
		renderChannelSection("Chat", "Latest room chat:", "Chat", snapshot.ChatLog)
		return &commandPayload{Action: "chat_seen"}, true
	default:
		return nil, false
	}
}

func isGracefulRoomClose(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(err.Error()), "room closed:")
}

func parseClientCommand(snapshot *roomSnapshot, line string) (*commandPayload, bool, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return nil, false, nil
	}
	switch strings.ToLower(fields[0]) {
	case "quit", "exit":
		return nil, true, nil
	case "say", "chat":
		if len(strings.TrimSpace(line)) <= len(fields[0]) {
			return nil, false, fmt.Errorf("%s <text>", strings.ToLower(fields[0]))
		}
		text := strings.TrimSpace(line[len(fields[0]):])
		if text == "" {
			return nil, false, fmt.Errorf("%s <text>", strings.ToLower(fields[0]))
		}
		return &commandPayload{Action: "say", Value: text}, false, nil
	case "host":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("host <seat>")
		}
		seat, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, false, err
		}
		return &commandPayload{Action: "host", Seat: seat}, false, nil
	case "accept-host":
		return &commandPayload{Action: "accept_host"}, false, nil
	case "deny-host":
		return &commandPayload{Action: "deny_host"}, false, nil
	case "cancel-host":
		return &commandPayload{Action: "cancel_host"}, false, nil
	}

	switch snapshot.Phase {
	case phaseLobby:
		return parseLobbyCommand(fields)
	case phaseMap:
		return parseIndexedCommand(fields, "node", "node <index>")
	case phaseCombat:
		return parseCombatCommand(fields)
	case phaseReward:
		return parseRewardCommand(fields)
	case phaseEvent:
		return parseIndexedCommand(fields, "choose", "choose <index>")
	case phaseShop:
		if strings.EqualFold(fields[0], "leave") {
			return &commandPayload{Action: "leave"}, false, nil
		}
		return parseIndexedCommand(fields, "buy", "buy <index>")
	case phaseRest:
		switch strings.ToLower(fields[0]) {
		case "heal":
			return &commandPayload{Action: "heal"}, false, nil
		case "upgrade":
			return &commandPayload{Action: "upgrade"}, false, nil
		default:
			return nil, false, fmt.Errorf("expected `heal` or `upgrade`")
		}
	case phaseEquipment:
		switch strings.ToLower(fields[0]) {
		case "take":
			return &commandPayload{Action: "take"}, false, nil
		case "skip":
			return &commandPayload{Action: "skip"}, false, nil
		default:
			return nil, false, fmt.Errorf("expected `take` or `skip`")
		}
	case phaseDeckAction:
		if strings.EqualFold(fields[0], "back") {
			return &commandPayload{Action: "back"}, false, nil
		}
		return parseIndexedCommand(fields, "choose", "choose <index>")
	case phaseSummary:
		if strings.EqualFold(fields[0], "new") {
			return &commandPayload{Action: "new"}, false, nil
		}
		if strings.EqualFold(fields[0], "abandon") {
			return &commandPayload{Action: "abandon"}, false, nil
		}
		return nil, false, fmt.Errorf("expected `new`, `abandon`, or `quit`")
	default:
		return nil, false, fmt.Errorf("unknown phase %q", snapshot.Phase)
	}
}

func parseLobbyCommand(fields []string) (*commandPayload, bool, error) {
	switch strings.ToLower(fields[0]) {
	case "ready":
		return &commandPayload{Action: "ready"}, false, nil
	case "start":
		return &commandPayload{Action: "start"}, false, nil
	case "mode":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("mode <story|endless>")
		}
		return &commandPayload{Action: "mode", Value: strings.ToLower(fields[1])}, false, nil
	case "seed":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("seed <number>")
		}
		return &commandPayload{Action: "seed", Value: fields[1]}, false, nil
	case "class":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("class <id>")
		}
		return &commandPayload{Action: "class", Value: strings.ToLower(fields[1])}, false, nil
	case "drop":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("drop <seat>")
		}
		if strings.EqualFold(fields[1], "all") {
			return &commandPayload{Action: "drop_all"}, false, nil
		}
		seat, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, false, err
		}
		return &commandPayload{Action: "drop", Seat: seat}, false, nil
	case "abandon":
		return &commandPayload{Action: "abandon"}, false, nil
	default:
		return nil, false, fmt.Errorf("expected ready | class <id> | mode <story|endless> | seed <n> | start | drop <seat|all> | abandon")
	}
}

func parseCombatCommand(fields []string) (*commandPayload, bool, error) {
	switch strings.ToLower(fields[0]) {
	case "end":
		return &commandPayload{Action: "end"}, false, nil
	case "play":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("play <card#> [enemy|ally <target#>]")
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, false, err
		}
		cmd := &commandPayload{Action: "play", CardIndex: index}
		if len(fields) >= 4 {
			cmd.TargetKind = normalizeTargetKind(fields[2])
			targetIndex, err := strconv.Atoi(fields[3])
			if err != nil {
				return nil, false, err
			}
			cmd.TargetIndex = targetIndex
		}
		return cmd, false, nil
	case "potion":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("potion <slot#> [enemy|ally <target#>]")
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, false, err
		}
		cmd := &commandPayload{Action: "potion", ItemIndex: index}
		if len(fields) >= 4 {
			cmd.TargetKind = normalizeTargetKind(fields[2])
			targetIndex, err := strconv.Atoi(fields[3])
			if err != nil {
				return nil, false, err
			}
			cmd.TargetIndex = targetIndex
		}
		return cmd, false, nil
	default:
		return nil, false, fmt.Errorf("expected play | potion | end")
	}
}

func parseRewardCommand(fields []string) (*commandPayload, bool, error) {
	switch strings.ToLower(fields[0]) {
	case "skip":
		return &commandPayload{Action: "skip"}, false, nil
	case "take":
		if len(fields) < 2 {
			return nil, false, fmt.Errorf("take <card#>")
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, false, err
		}
		return &commandPayload{Action: "take", ItemIndex: index}, false, nil
	default:
		return nil, false, fmt.Errorf("expected take <card#> or skip")
	}
}

func parseIndexedCommand(fields []string, action, usage string) (*commandPayload, bool, error) {
	if !strings.EqualFold(fields[0], action) || len(fields) < 2 {
		return nil, false, fmt.Errorf("%s", usage)
	}
	index, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, false, err
	}
	return &commandPayload{Action: action, ItemIndex: index}, false, nil
}

func normalizeTargetKind(value string) string {
	switch strings.ToLower(value) {
	case "enemy", "e":
		return string(engine.CombatTargetEnemy)
	case "ally", "a", "friend":
		return string(engine.CombatTargetAlly)
	default:
		return ""
	}
}

func renderSnapshot(snapshot *roomSnapshot) {
	fmt.Print("\x1bc")
	switch snapshot.Phase {
	case phaseLobby:
		renderLobbySnapshot(snapshot)
	case phaseMap:
		renderMapSnapshot(snapshot)
	case phaseCombat:
		renderCombatSnapshot(snapshot)
	case phaseReward:
		renderRewardSnapshot(snapshot)
	case phaseEvent:
		renderEventSnapshot(snapshot)
	case phaseShop:
		renderShopSnapshot(snapshot)
	case phaseRest:
		renderRestSnapshot(snapshot)
	case phaseEquipment:
		renderEquipmentPrompt(snapshot)
	case phaseDeckAction:
		renderDeckActionSnapshot(snapshot)
	case phaseSummary:
		renderSummarySnapshot(snapshot)
	}
}

func renderRoomHeader(title string, snapshot *roomSnapshot) {
	fmt.Printf("== %s ==\n", title)
	host := snapshotPlayer(snapshot, snapshot.HostID)
	self := snapshotPlayer(snapshot, snapshot.SelfID)
	role := "member"
	if snapshot.SelfID == snapshot.HostID {
		role = "host"
	}
	hostName := snapshot.HostID
	if host != nil {
		hostName = host.Name
	}
	selfName := snapshot.SelfID
	selfClass := ""
	if self != nil {
		selfName = self.Name
		selfClass = self.ClassID
	}
	fmt.Printf("Room: %s | Host: %s | You: %s", snapshot.RoomAddr, hostName, selfName)
	if selfClass != "" {
		fmt.Printf(" [%s]", selfClass)
	}
	if snapshot.Seat > 0 {
		fmt.Printf(" | Seat: %d", snapshot.Seat)
	}
	fmt.Printf(" (%s)\n", role)
	if snapshot.PhaseTitle != "" {
		fmt.Printf("Phase: %s\n", snapshot.PhaseTitle)
	}
	renderDivider("Guidance")
	if snapshot.Banner != "" {
		fmt.Println("Notice:", snapshot.Banner)
	}
	if snapshot.PhaseHint != "" {
		fmt.Println("Status:", snapshot.PhaseHint)
	}
	if snapshot.ControlLabel != "" {
		fmt.Println("Control:", snapshot.ControlLabel)
	}
	if snapshot.RoleNote != "" {
		fmt.Println("Role:", snapshot.RoleNote)
	}
	if len(snapshot.Resume) > 0 {
		fmt.Println("Resume summary:")
		for _, line := range snapshot.Resume {
			fmt.Println("-", line)
		}
	}
	renderDivider("Room")
	connected := connectedSnapshotPlayers(snapshot.Players)
	fmt.Printf("Connected: %d/%d", connected, len(snapshot.Players))
	if len(snapshot.OfflineSeats) > 0 {
		fmt.Printf(" | Offline: %s\n", strings.Join(snapshot.OfflineSeats, ", "))
		fmt.Println("Reconnect: offline players can rejoin with the same name to reclaim their seat.")
	} else {
		fmt.Println()
	}
	if len(snapshot.WaitingOn) > 0 {
		fmt.Printf("Waiting on: %s\n", strings.Join(snapshot.WaitingOn, ", "))
	}
	if len(snapshot.SeatStatus) > 0 {
		renderDivider("Activity")
		fmt.Println("Seat status:")
		for _, line := range snapshot.SeatStatus {
			fmt.Println("-", line)
		}
	}
	if len(snapshot.Reconnect) > 0 {
		fmt.Println("Reconnect commands:")
		for _, line := range snapshot.Reconnect {
			fmt.Println("-", line)
		}
	}
	if len(snapshot.Commands) > 0 {
		renderDivider("Commands")
		fmt.Println("Quick commands:")
		for _, line := range snapshot.Commands {
			fmt.Println("-", line)
		}
	}
	if len(snapshot.Examples) > 0 {
		fmt.Println("Try next:")
		for _, line := range snapshot.Examples {
			fmt.Println("-", line)
		}
	}
	if len(snapshot.ChatLog) > 0 {
		renderDivider("Chat")
		if snapshot.ChatUnread > 0 {
			fmt.Printf("Recent chat (%d, unread %d):\n", len(snapshot.ChatLog), snapshot.ChatUnread)
		} else {
			fmt.Printf("Recent chat (%d):\n", len(snapshot.ChatLog))
		}
		recent := tailStrings(snapshot.ChatLog, 3)
		unreadShown := min(snapshot.ChatUnreadInView, len(recent))
		startUnread := len(recent) - unreadShown
		for i, line := range recent {
			prefix := "-"
			if i >= startUnread && unreadShown > 0 {
				prefix = "*"
			}
			fmt.Println(prefix, displayChannelEntry("Chat", line))
		}
		fmt.Println("Use `chatlog` to view the full recent room chat.")
	}
	if snapshot.ChatUnread > 0 {
		fmt.Printf("Unread chat: %d\n", snapshot.ChatUnread)
	}
	if snapshot.TransferNote != "" {
		fmt.Println("Transfer:", snapshot.TransferNote)
	}
	renderDivider("Client")
	fmt.Println("Local client commands: help | status | log | chatlog | quit")
	if snapshot.SelfID == snapshot.HostID {
		fmt.Println("Host note: `quit` keeps the room save. Use `abandon` to clear it and close the room.")
	} else {
		fmt.Println("Member note: if the room closes, wait for the host to restart it, then rejoin with the same name.")
	}
	fmt.Println()
}

func renderLobbySnapshot(snapshot *roomSnapshot) {
	lobby := snapshot.Lobby
	if lobby == nil {
		return
	}
	renderRoomHeader("LAN Lobby", snapshot)
	fmt.Printf("Mode: %s | Seed: %d\n", lobby.Mode, lobby.Seed)
	fmt.Println()
	fmt.Println("Players:")
	for _, player := range snapshot.Players {
		role := "member"
		if player.ID == snapshot.HostID {
			role = "host"
		}
		ready := "not ready"
		if player.Ready {
			ready = "ready"
		}
		if !player.Connected {
			ready = "offline"
		}
		fmt.Printf("- Seat %d %s [%s] %s (%s)\n", player.Seat, player.Name, player.ClassID, ready, role)
	}
	fmt.Println()
	fmt.Println("Available classes:", strings.Join(lobby.Classes, ", "))
	if len(snapshot.Recovery) > 0 {
		fmt.Println()
		fmt.Println("Recovery actions:")
		for _, line := range snapshot.Recovery {
			fmt.Println("-", line)
		}
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderMapSnapshot(snapshot *roomSnapshot) {
	view := snapshot.Map
	if view == nil {
		return
	}
	renderRoomHeader("Shared Map", snapshot)
	fmt.Printf("Mode: %s | Act %d | Next floor %d | Gold %d\n", view.Mode, view.Act, view.NextFloor, view.Gold)
	fmt.Println()
	fmt.Println("Party:")
	renderActorLines(snapshot, view.Party)
	fmt.Println()
	fmt.Println("Reachable nodes:")
	for _, node := range view.Reachable {
		fmt.Printf("- %d. %s\n", node.Index, node.Label)
	}
	if len(view.History) > 0 {
		fmt.Println()
		fmt.Println("History:")
		for _, line := range view.History {
			fmt.Println("-", line)
		}
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderCombatSnapshot(snapshot *roomSnapshot) {
	combat := snapshot.Combat
	if combat == nil {
		return
	}
	renderRoomHeader("Shared Combat", snapshot)
	fmt.Printf("Turn %d | Energy %d/%d\n", combat.Turn, combat.Energy, combat.MaxEnergy)
	renderHighlights(combat.Highlights)
	fmt.Println()
	fmt.Println("Party:")
	renderActorLines(snapshot, combat.Party)
	fmt.Println()
	fmt.Println("Enemies:")
	for _, enemy := range combat.Enemies {
		line := fmt.Sprintf("- %d. %s HP %d/%d Block %d | Intent: %s", enemy.Index, enemy.Name, enemy.HP, enemy.MaxHP, enemy.Block, enemy.Intent)
		if enemy.Status != "" {
			line += " | " + enemy.Status
		}
		fmt.Println(line)
	}
	fmt.Println()
	fmt.Println("Hand:")
	for _, card := range combat.Hand {
		fmt.Printf("- %d. [%d] %s%s | %s | %s\n", card.Index, card.Cost, card.Name, renderBadges(card.Badges), card.Summary, card.TargetHint)
	}
	if len(combat.Potions) > 0 {
		fmt.Println()
		fmt.Println("Potions:")
		for _, potion := range combat.Potions {
			fmt.Println("-", potion)
		}
	}
	if len(combat.EndTurnVotes) > 0 {
		fmt.Println()
		fmt.Println("End-turn votes:")
		for _, line := range combat.VoteStatus {
			fmt.Println("-", line)
		}
	}
	if len(combat.Logs) > 0 {
		fmt.Println()
		fmt.Println("Combat log:")
		for _, line := range combat.Logs {
			fmt.Println("-", line)
		}
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderRewardSnapshot(snapshot *roomSnapshot) {
	view := snapshot.Reward
	if view == nil {
		return
	}
	renderRoomHeader("Reward", snapshot)
	fmt.Printf("Gold +%d | Source: %s\n", view.Gold, view.Source)
	renderHighlights(view.Highlights)
	if view.Relic != "" {
		fmt.Printf("Relic: %s%s\n", view.Relic, renderBadges(view.RelicBadges))
	}
	if view.Potion != "" {
		fmt.Println("Potion:", view.Potion)
	}
	if view.Equipment != nil {
		fmt.Printf("Equipment pending: %s (%s)\n", view.Equipment.CandidateName, view.Equipment.Slot)
	}
	fmt.Println()
	fmt.Println("Cards:")
	for _, card := range view.Cards {
		fmt.Printf("- %d. %s%s | %s\n", card.Index, card.Name, renderBadges(card.Badges), card.Summary)
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderEventSnapshot(snapshot *roomSnapshot) {
	view := snapshot.Event
	if view == nil {
		return
	}
	renderRoomHeader("Event", snapshot)
	fmt.Println(view.Name + renderBadges(view.Badges))
	fmt.Println(view.Description)
	renderHighlights(view.Highlights)
	fmt.Println()
	for _, choice := range view.Choices {
		fmt.Printf("- %d. %s%s | %s\n", choice.Index, choice.Label, renderBadges(choice.Badges), choice.Description)
	}
	if len(view.Log) > 0 {
		fmt.Println()
		fmt.Println("Event log:")
		for _, line := range view.Log {
			fmt.Println("-", line)
		}
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderShopSnapshot(snapshot *roomSnapshot) {
	view := snapshot.Shop
	if view == nil {
		return
	}
	renderRoomHeader("Shop", snapshot)
	fmt.Printf("Gold: %d\n", view.Gold)
	renderHighlights(view.Highlights)
	fmt.Println()
	currentCategory := ""
	for _, offer := range view.Offers {
		if offer.Category != currentCategory {
			currentCategory = offer.Category
			fmt.Printf("== %s ==\n", currentCategory)
		}
		fmt.Printf("- %d. %s%s | %d gold | %s\n", offer.Index, offer.Name, renderBadges(offer.Badges), offer.Price, offer.Description)
	}
	if len(view.Log) > 0 {
		fmt.Println()
		fmt.Println("Shop log:")
		for _, line := range view.Log {
			fmt.Println("-", line)
		}
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderRestSnapshot(snapshot *roomSnapshot) {
	view := snapshot.Rest
	if view == nil {
		return
	}
	renderRoomHeader("Campfire", snapshot)
	fmt.Println("Party:")
	renderActorLines(snapshot, view.Party)
	if len(view.Log) > 0 {
		fmt.Println()
		fmt.Println("Campfire log:")
		for _, line := range view.Log {
			fmt.Println("-", line)
		}
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderEquipmentPrompt(snapshot *roomSnapshot) {
	view := snapshot.Equipment
	if view == nil {
		return
	}
	renderRoomHeader("Equipment Choice", snapshot)
	fmt.Printf("Source: %s | Slot: %s\n", view.Source, view.Slot)
	fmt.Printf("Candidate: %s (score %d)\n", view.CandidateName, view.CandidateScore)
	fmt.Println(view.CandidateDescription)
	if view.CurrentName != "" {
		fmt.Println()
		fmt.Printf("Current: %s (score %d)\n", view.CurrentName, view.CurrentScore)
		fmt.Println(view.CurrentDescription)
	}
	if view.Price > 0 {
		fmt.Printf("\nPrice: %d gold\n", view.Price)
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderDeckActionSnapshot(snapshot *roomSnapshot) {
	view := snapshot.Deck
	if view == nil {
		return
	}
	renderRoomHeader("Deck Action", snapshot)
	fmt.Println(view.Title)
	if view.Subtitle != "" {
		fmt.Println(view.Subtitle)
	}
	fmt.Println()
	for _, card := range view.Cards {
		fmt.Printf("- %d. %s | %s\n", card.Index, card.Name, card.Summary)
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderSummarySnapshot(snapshot *roomSnapshot) {
	view := snapshot.Summary
	if view == nil {
		return
	}
	renderRoomHeader("Run Summary", snapshot)
	fmt.Printf("Result: %s | Mode: %s | Act: %d | Floors: %d | Gold: %d | Deck: %d\n",
		view.Result, view.Mode, view.Act, view.Floors, view.Gold, view.DeckSize)
	fmt.Println()
	fmt.Println("Party:")
	renderActorLines(snapshot, view.Party)
	if len(view.History) > 0 {
		fmt.Println()
		fmt.Println("Route:")
		for _, line := range view.History {
			fmt.Println("-", line)
		}
	}
	renderRoomLog(snapshot.RoomLog)
}

func renderActorLines(snapshot *roomSnapshot, actors []actorSnapshot) {
	for _, actor := range actors {
		label := actor.Name
		for _, player := range snapshot.Players {
			if player.Seat != actor.Index {
				continue
			}
			if player.ID == snapshot.HostID {
				label += " [host]"
			}
			if player.Seat == snapshot.Seat {
				label += " [you]"
			}
			break
		}
		line := fmt.Sprintf("- %d. %s HP %d/%d Block %d", actor.Index, label, actor.HP, actor.MaxHP, actor.Block)
		if actor.MaxEnergy > 0 {
			line += fmt.Sprintf(" | Energy %d/%d", actor.Energy, actor.MaxEnergy)
		}
		if actor.Status != "" {
			line += " | " + actor.Status
		}
		fmt.Println(line)
	}
}

func renderBadges(badges []string) string {
	if len(badges) == 0 {
		return ""
	}
	return " [" + strings.Join(badges, "][") + "]"
}

func renderDivider(title string) {
	fmt.Printf("-- %s --\n", title)
}

func renderHighlights(lines []string) {
	if len(lines) == 0 {
		return
	}
	renderDivider("Room Focus")
	for _, line := range lines {
		fmt.Println("-", line)
	}
}

func renderRoomLog(lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Println()
	renderChannelSection("System", "System log:", "System", lines)
}

func flagBadges(flags []string) []string {
	badges := []string{}
	for _, flag := range flags {
		switch flag {
		case "coop_only":
			badges = append(badges, "CO-OP")
		}
	}
	return badges
}

func eventChoiceBadges(lib *content.Library, choice content.EventChoiceDef) []string {
	badges := []string{}
	for _, effect := range choice.Effects {
		switch effect.Op {
		case "add_card":
			if card, ok := lib.Cards[effect.CardID]; ok {
				badges = appendUniqueString(badges, flagBadges(card.Flags)...)
			}
		case "gain_relic", "upgrade_relic":
			if relic, ok := lib.Relics[effect.ItemID]; ok {
				badges = appendUniqueString(badges, flagBadges(relic.Flags)...)
			}
			if relic, ok := lib.Relics[effect.ResultID]; ok {
				badges = appendUniqueString(badges, flagBadges(relic.Flags)...)
			}
		}
	}
	return badges
}

func eventHighlights(flags []string, choices []choiceSnapshot) []string {
	highlights := []string{}
	if slices.Contains(flagBadges(flags), "CO-OP") {
		highlights = append(highlights, "This event is multiplayer-only.")
	}
	if coopChoices := countSnapshotsWithBadge(choices, "CO-OP"); coopChoices > 0 {
		highlights = append(highlights, fmt.Sprintf("%d choice(s) here lead into co-op-only rewards.", coopChoices))
	}
	return highlights
}

func shopOfferCategory(kind string) string {
	switch kind {
	case "card":
		return "Cards"
	case "relic":
		return "Relics"
	case "equipment":
		return "Equipment"
	case "potion":
		return "Potions"
	case "service":
		return "Services"
	case "heal", "remove":
		return "Support"
	default:
		return "Other"
	}
}

func shopOfferBadges(lib *content.Library, offer engine.ShopOffer) []string {
	badges := []string{}
	switch offer.Kind {
	case "service":
		badges = append(badges, "SERVICE")
		if strings.HasPrefix(offer.ItemID, "service_coop_") || strings.Contains(offer.ItemID, "combo") {
			badges = append(badges, "CO-OP")
		}
	case "card":
		if card, ok := lib.Cards[offer.CardID]; ok {
			badges = appendUniqueString(badges, flagBadges(card.Flags)...)
		}
	case "relic":
		if relic, ok := lib.Relics[offer.ItemID]; ok {
			badges = appendUniqueString(badges, flagBadges(relic.Flags)...)
		}
	}
	return badges
}

func shopHighlights(offers []shopOfferSnapshot) []string {
	highlights := []string{}
	coopOffers := countSnapshotsWithBadge(offers, "CO-OP")
	if coopOffers > 0 {
		highlights = append(highlights, fmt.Sprintf("%d co-op offer(s) are available in this shop.", coopOffers))
	}
	serviceOffers := countShopOffersByKind(offers, "service")
	if serviceOffers > 0 {
		highlights = append(highlights, fmt.Sprintf("%d service option(s) are available for shared team setup.", serviceOffers))
	}
	return highlights
}

func appendUniqueString(dst []string, items ...string) []string {
	for _, item := range items {
		if item == "" || slices.Contains(dst, item) {
			continue
		}
		dst = append(dst, item)
	}
	return dst
}

func countSnapshotsWithBadge[T interface{ getBadges() []string }](items []T, badge string) int {
	count := 0
	for _, item := range items {
		if slices.Contains(item.getBadges(), badge) {
			count++
		}
	}
	return count
}

func countShopOffersByKind(offers []shopOfferSnapshot, kind string) int {
	count := 0
	for _, offer := range offers {
		if offer.Kind == kind {
			count++
		}
	}
	return count
}

func (c cardSnapshot) getBadges() []string      { return c.Badges }
func (c choiceSnapshot) getBadges() []string    { return c.Badges }
func (s shopOfferSnapshot) getBadges() []string { return s.Badges }

func snapshotPlayer(snapshot *roomSnapshot, id string) *roomPlayer {
	for i := range snapshot.Players {
		if snapshot.Players[i].ID == id {
			return &snapshot.Players[i]
		}
	}
	return nil
}

func connectedSnapshotPlayers(players []roomPlayer) int {
	count := 0
	for _, player := range players {
		if player.Connected {
			count++
		}
	}
	return count
}

func offlineSnapshotPlayers(players []roomPlayer) []string {
	offline := []string{}
	for _, player := range players {
		if !player.Connected {
			offline = append(offline, player.Name)
		}
	}
	return offline
}

func buildBasePlayerState(lib *content.Library, classID, name string) (engine.PlayerState, error) {
	class, ok := lib.Classes[classID]
	if !ok {
		return engine.PlayerState{}, fmt.Errorf("unknown class %q", classID)
	}
	player := engine.PlayerState{
		ClassID:        class.ID,
		Name:           name,
		MaxHP:          class.BaseHP,
		HP:             class.BaseHP,
		MaxEnergy:      class.MaxEnergy,
		Gold:           class.BaseGold,
		Deck:           make([]engine.DeckCard, 0, len(class.StartingDeck)),
		Relics:         append([]string{}, class.StartingRelics...),
		Potions:        append([]string{}, class.StartingPotions...),
		PotionCapacity: 2,
		PermanentStats: map[string]int{},
	}
	for _, id := range class.StartingDeck {
		player.Deck = append(player.Deck, engine.DeckCard{CardID: id})
	}
	for _, eqID := range class.StartingEquipment {
		item := lib.Equipments[eqID]
		switch item.Slot {
		case "weapon":
			player.Equipment.Weapon = eqID
		case "armor":
			player.Equipment.Armor = eqID
		case "accessory":
			player.Equipment.Accessory = eqID
		}
	}
	return player, nil
}

func buildSharedLeader(players []engine.PlayerState) engine.PlayerState {
	leader := players[0]
	relicSet := map[string]struct{}{}
	for _, relicID := range leader.Relics {
		relicSet[relicID] = struct{}{}
	}
	for _, player := range players[1:] {
		leader.Deck = append(leader.Deck, player.Deck...)
		for _, relicID := range player.Relics {
			if _, ok := relicSet[relicID]; ok {
				continue
			}
			relicSet[relicID] = struct{}{}
			leader.Relics = append(leader.Relics, relicID)
		}
		for _, potionID := range player.Potions {
			if len(leader.Potions) < leader.PotionCapacity+len(players)-1 {
				leader.Potions = append(leader.Potions, potionID)
			}
		}
	}
	leader.PotionCapacity = 2 + max(0, len(players)-1)
	leader.MaxEnergy = players[0].MaxEnergy + max(0, len(players)-1)
	return leader
}

func firstFloorNodeIDs(graph engine.MapGraph) []string {
	ids := make([]string, 0, len(graph.Floors[0]))
	for _, node := range graph.Floors[0] {
		ids = append(ids, node.ID)
	}
	return ids
}

func clonePartyStates(players []engine.PlayerState) []engine.PlayerState {
	out := make([]engine.PlayerState, 0, len(players))
	for _, player := range players {
		copied := player
		copied.Deck = append([]engine.DeckCard{}, player.Deck...)
		copied.Relics = append([]string{}, player.Relics...)
		copied.Potions = append([]string{}, player.Potions...)
		if player.PermanentStats != nil {
			copied.PermanentStats = map[string]int{}
			for key, value := range player.PermanentStats {
				copied.PermanentStats[key] = value
			}
		}
		out = append(out, copied)
	}
	return out
}

func partySnapshotsFromMembers(players []engine.PlayerState) []actorSnapshot {
	actors := make([]actorSnapshot, 0, len(players))
	for i, player := range players {
		actors = append(actors, actorSnapshot{
			Index: i + 1,
			Name:  player.Name,
			HP:    player.HP,
			MaxHP: player.MaxHP,
		})
	}
	return actors
}

func classIDs(lib *content.Library) []string {
	ids := make([]string, 0, len(lib.ClassList()))
	for _, class := range lib.ClassList() {
		ids = append(ids, class.ID)
	}
	return ids
}

func describeTargetKind(kind engine.CombatTargetKind) string {
	switch kind {
	case engine.CombatTargetEnemy:
		return "Single enemy"
	case engine.CombatTargetEnemies:
		return "All enemies"
	case engine.CombatTargetAlly:
		return "Single ally"
	case engine.CombatTargetAllies:
		return "All allies"
	default:
		return "No target"
	}
}

func equipmentDecisionText(take bool, equipmentID string) string {
	action := "Skipped equipment"
	if take {
		action = "Equipped"
	}
	return fmt.Sprintf("%s: %s.", action, equipmentID)
}

func findShopOfferByID(shop *engine.ShopState, offerID string) (engine.ShopOffer, error) {
	for _, offer := range shop.Offers {
		if offer.ID == offerID {
			return offer, nil
		}
	}
	return engine.ShopOffer{}, fmt.Errorf("offer %q not found", offerID)
}

func allVotesReady(votes []bool) bool {
	if len(votes) == 0 {
		return true
	}
	for _, voted := range votes {
		if !voted {
			return false
		}
	}
	return true
}

func tailStrings(values []string, count int) []string {
	if len(values) <= count {
		return append([]string{}, values...)
	}
	return append([]string{}, values[len(values)-count:]...)
}

func tailCombatLogs(values []engine.CombatLogEntry, count int) []engine.CombatLogEntry {
	if len(values) <= count {
		return append([]engine.CombatLogEntry{}, values...)
	}
	return append([]engine.CombatLogEntry{}, values[len(values)-count:]...)
}

func clamp(value, lo, hi int) int {
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
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
