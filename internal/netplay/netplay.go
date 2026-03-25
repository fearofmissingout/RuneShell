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
	"sync"
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

type message struct {
	Type     string          `json:"type"`
	Hello    *helloPayload   `json:"hello,omitempty"`
	Command  *commandPayload `json:"command,omitempty"`
	Snapshot *roomSnapshot   `json:"snapshot,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type helloPayload struct {
	Name    string `json:"name"`
	ClassID string `json:"class_id"`
}

type commandPayload struct {
	Action      string `json:"action"`
	Value       string `json:"value,omitempty"`
	Seat        int    `json:"seat,omitempty"`
	CardIndex   int    `json:"card_index,omitempty"`
	ItemIndex   int    `json:"item_index,omitempty"`
	TargetKind  string `json:"target_kind,omitempty"`
	TargetIndex int    `json:"target_index,omitempty"`
}

type roomPlayer struct {
	ID        string `json:"id"`
	Seat      int    `json:"seat,omitempty"`
	Name      string `json:"name"`
	ClassID   string `json:"class_id"`
	Ready     bool   `json:"ready"`
	Connected bool   `json:"connected"`
}

type actorSnapshot struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	HP        int    `json:"hp"`
	MaxHP     int    `json:"max_hp"`
	Energy    int    `json:"energy,omitempty"`
	MaxEnergy int    `json:"max_energy,omitempty"`
	Block     int    `json:"block"`
	Status    string `json:"status"`
}

type enemySnapshot struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	HP     int    `json:"hp"`
	MaxHP  int    `json:"max_hp"`
	Block  int    `json:"block"`
	Status string `json:"status"`
	Intent string `json:"intent"`
}

type cardSnapshot struct {
	Index      int      `json:"index"`
	Name       string   `json:"name"`
	Cost       int      `json:"cost,omitempty"`
	Summary    string   `json:"summary"`
	TargetHint string   `json:"target_hint,omitempty"`
	Badges     []string `json:"badges,omitempty"`
}

type nodeSnapshot struct {
	Index int    `json:"index"`
	Floor int    `json:"floor"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type choiceSnapshot struct {
	Index       int      `json:"index"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Badges      []string `json:"badges,omitempty"`
}

type shopOfferSnapshot struct {
	Index       int      `json:"index"`
	Kind        string   `json:"kind"`
	Category    string   `json:"category,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int      `json:"price"`
	Badges      []string `json:"badges,omitempty"`
}

type equipmentSnapshot struct {
	Source               string `json:"source"`
	Slot                 string `json:"slot"`
	CandidateName        string `json:"candidate_name"`
	CandidateDescription string `json:"candidate_description"`
	CurrentName          string `json:"current_name,omitempty"`
	CurrentDescription   string `json:"current_description,omitempty"`
	Price                int    `json:"price,omitempty"`
	CandidateScore       int    `json:"candidate_score"`
	CurrentScore         int    `json:"current_score,omitempty"`
}

type lobbySnapshot struct {
	Mode    string   `json:"mode"`
	Seed    int64    `json:"seed"`
	Classes []string `json:"classes"`
}

type mapSnapshot struct {
	Mode      string          `json:"mode"`
	Act       int             `json:"act"`
	NextFloor int             `json:"next_floor"`
	Gold      int             `json:"gold"`
	Party     []actorSnapshot `json:"party"`
	Reachable []nodeSnapshot  `json:"reachable"`
	History   []string        `json:"history"`
}

type combatSnapshot struct {
	Turn         int             `json:"turn"`
	Energy       int             `json:"energy"`
	MaxEnergy    int             `json:"max_energy"`
	Party        []actorSnapshot `json:"party"`
	Enemies      []enemySnapshot `json:"enemies"`
	Hand         []cardSnapshot  `json:"hand"`
	Potions      []string        `json:"potions"`
	EndTurnVotes []bool          `json:"end_turn_votes"`
	VoteStatus   []string        `json:"vote_status,omitempty"`
	Logs         []string        `json:"logs"`
	Highlights   []string        `json:"highlights,omitempty"`
}

type rewardSnapshot struct {
	Gold        int                `json:"gold"`
	Source      string             `json:"source"`
	Cards       []cardSnapshot     `json:"cards"`
	Potion      string             `json:"potion,omitempty"`
	Relic       string             `json:"relic,omitempty"`
	RelicBadges []string           `json:"relic_badges,omitempty"`
	Equipment   *equipmentSnapshot `json:"equipment,omitempty"`
	Highlights  []string           `json:"highlights,omitempty"`
}

type eventSnapshot struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Badges      []string         `json:"badges,omitempty"`
	Choices     []choiceSnapshot `json:"choices"`
	Log         []string         `json:"log"`
	Highlights  []string         `json:"highlights,omitempty"`
}

type shopSnapshot struct {
	Gold       int                 `json:"gold"`
	Offers     []shopOfferSnapshot `json:"offers"`
	Log        []string            `json:"log"`
	Highlights []string            `json:"highlights,omitempty"`
}

type restSnapshot struct {
	Party []actorSnapshot `json:"party"`
	Log   []string        `json:"log"`
}

type deckActionSnapshot struct {
	Mode     string         `json:"mode"`
	Title    string         `json:"title"`
	Subtitle string         `json:"subtitle"`
	Cards    []cardSnapshot `json:"cards"`
}

type summarySnapshot struct {
	Result   string          `json:"result"`
	Mode     string          `json:"mode"`
	Act      int             `json:"act"`
	Floors   int             `json:"floors"`
	Gold     int             `json:"gold"`
	DeckSize int             `json:"deck_size"`
	Party    []actorSnapshot `json:"party"`
	History  []string        `json:"history"`
}

type roomSnapshot struct {
	SelfID           string              `json:"self_id"`
	Seat             int                 `json:"seat,omitempty"`
	HostID           string              `json:"host_id"`
	RoomAddr         string              `json:"room_addr"`
	Phase            string              `json:"phase"`
	PhaseTitle       string              `json:"phase_title,omitempty"`
	PhaseHint        string              `json:"phase_hint,omitempty"`
	ControlLabel     string              `json:"control_label,omitempty"`
	RoleNote         string              `json:"role_note,omitempty"`
	Banner           string              `json:"banner,omitempty"`
	Players          []roomPlayer        `json:"players"`
	OfflineSeats     []string            `json:"offline_seats,omitempty"`
	WaitingOn        []string            `json:"waiting_on,omitempty"`
	SeatStatus       []string            `json:"seat_status,omitempty"`
	Recovery         []string            `json:"recovery,omitempty"`
	Reconnect        []string            `json:"reconnect,omitempty"`
	Resume           []string            `json:"resume,omitempty"`
	Commands         []string            `json:"commands,omitempty"`
	Examples         []string            `json:"examples,omitempty"`
	CanStart         bool                `json:"can_start"`
	Lobby            *lobbySnapshot      `json:"lobby,omitempty"`
	Map              *mapSnapshot        `json:"map,omitempty"`
	Combat           *combatSnapshot     `json:"combat,omitempty"`
	Reward           *rewardSnapshot     `json:"reward,omitempty"`
	Event            *eventSnapshot      `json:"event,omitempty"`
	Shop             *shopSnapshot       `json:"shop,omitempty"`
	Rest             *restSnapshot       `json:"rest,omitempty"`
	Equipment        *equipmentSnapshot  `json:"equipment,omitempty"`
	Deck             *deckActionSnapshot `json:"deck,omitempty"`
	Summary          *summarySnapshot    `json:"summary,omitempty"`
	ChatLog          []string            `json:"chat_log,omitempty"`
	ChatUnread       int                 `json:"chat_unread,omitempty"`
	ChatUnreadInView int                 `json:"chat_unread_in_view,omitempty"`
	TransferNote     string              `json:"transfer_note,omitempty"`
	RoomLog          []string            `json:"room_log,omitempty"`
}

type clientConn struct {
	id       string
	conn     net.Conn
	enc      *json.Encoder
	notice   string
	resume   []string
	chatSeen int
}

type hostTransferRequest struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Seat   int    `json:"seat"`
}

type server struct {
	lib      *content.Library
	roomAddr string
	listener net.Listener
	savePath string

	mu      sync.Mutex
	nextID  int
	hostID  string
	order   []string
	players map[string]*roomPlayer
	clients map[string]*clientConn
	closed  bool

	phase            string
	mode             engine.GameMode
	seed             int64
	chatLog          []string
	roomLog          []string
	hostTransfer     *hostTransferRequest
	restoredFromSave bool

	run          *engine.RunState
	partyMembers []engine.PlayerState
	currentNode  engine.Node

	combat     *engine.CombatState
	reward     *engine.RewardState
	eventState *engine.EventState
	shopState  *engine.ShopState
	restLog    []string

	equipOffer  *engine.EquipmentOfferState
	equipFrom   string
	rewardCard  string
	shopOfferID string
	eventChoice string

	deckActionMode     string
	deckActionTitle    string
	deckActionSubtitle string
	deckActionIndexes  []int
	deckActionPrice    int
}

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
			engine.EndPlayerTurn(s.lib, s.run.Player, s.combat)
			if !s.combat.Won && !s.combat.Lost {
				engine.StartPlayerTurn(s.lib, s.run.Player, s.combat)
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
	if !s.requireHostLocked(playerID, "Only the host can choose the next node.") {
		return
	}
	if cmd.Action != "node" {
		return
	}
	reachable := engine.ReachableNodes(s.run)
	if cmd.ItemIndex <= 0 || cmd.ItemIndex > len(reachable) {
		s.appendRoomLogLocked("Invalid node selection.")
		return
	}
	node := reachable[cmd.ItemIndex-1]
	selected, err := engine.SelectNode(s.run, node.ID)
	if err != nil {
		s.appendRoomLogLocked("Failed to select node: " + err.Error())
		return
	}
	s.currentNode = selected
	s.appendRoomLogLocked(fmt.Sprintf("Path chosen: A%d F%d %s.", selected.Act, selected.Floor, engine.NodeKindName(selected.Kind)))

	switch selected.Kind {
	case engine.NodeMonster, engine.NodeElite, engine.NodeBoss:
		party := s.combatPartyLocked()
		combat, err := engine.StartEncounterForParty(s.lib, s.run, party, selected)
		if err != nil {
			s.appendRoomLogLocked("Failed to start combat: " + err.Error())
			return
		}
		s.combat = combat
		engine.StartPlayerTurn(s.lib, s.run.Player, s.combat)
		s.phase = phaseCombat
		s.announcePhaseLocked(fmt.Sprintf("Encounter started: %s.", selected.Label()))
	case engine.NodeEvent:
		state, err := engine.StartEvent(s.lib, s.run, selected)
		if err != nil {
			s.appendRoomLogLocked("Failed to start event: " + err.Error())
			return
		}
		s.eventState = &state
		s.phase = phaseEvent
		s.announcePhaseLocked("A shared event choice is waiting.")
	case engine.NodeShop:
		shop := engine.StartShop(s.lib, s.run)
		s.shopState = &shop
		s.phase = phaseShop
		s.announcePhaseLocked("The shared shop is open.")
	case engine.NodeRest:
		s.restLog = nil
		s.phase = phaseRest
		s.announcePhaseLocked("The shared campfire is ready.")
	}
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
		if cmd.CardIndex <= 0 || cmd.CardIndex > len(s.combat.Hand) {
			s.appendRoomLogLocked("Invalid hand index.")
			return
		}
		cardName := s.lib.Cards[s.combat.Hand[cmd.CardIndex-1].ID].Name
		if err := engine.PlayCardWithTarget(s.lib, s.run.Player, s.combat, cmd.CardIndex-1, target); err != nil {
			s.appendRoomLogLocked("Play failed: " + err.Error())
			return
		}
		s.appendCombatActionLogLocked(playerID, fmt.Sprintf("打出 %s", cardName), target)
		s.handleCoopCombatActionLocked(memberIndex, player.Name)
	case "potion":
		if cmd.ItemIndex <= 0 || cmd.ItemIndex > len(s.run.Player.Potions) {
			s.appendRoomLogLocked("Invalid potion index.")
			return
		}
		potionID := s.run.Player.Potions[cmd.ItemIndex-1]
		potionName := s.lib.Potions[potionID].Name
		if err := engine.UsePotionWithTarget(s.lib, s.run.Player, s.combat, potionID, target); err != nil {
			s.appendRoomLogLocked("Potion failed: " + err.Error())
			return
		}
		s.run.Player.Potions = append(s.run.Player.Potions[:cmd.ItemIndex-1], s.run.Player.Potions[cmd.ItemIndex:]...)
		s.appendCombatActionLogLocked(playerID, fmt.Sprintf("使用药水 %s", potionName), target)
		s.handleCoopCombatActionLocked(memberIndex, player.Name)
	case "end":
		if engine.RequestEndTurnVote(s.combat, memberIndex) {
			s.appendCombatActionLogLocked(playerID, "提交结束回合，所有在线座位已就绪", engine.CombatTarget{Kind: engine.CombatTargetNone})
			engine.EndPlayerTurn(s.lib, s.run.Player, s.combat)
			if !s.combat.Won && !s.combat.Lost {
				engine.StartPlayerTurn(s.lib, s.run.Player, s.combat)
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
	if s.combat == nil || s.run == nil {
		return
	}
	firstForSeat, uniqueActors := engine.RecordCoopAction(s.combat, memberIndex)
	if !firstForSeat || uniqueActors < 2 || s.combat.Coop.TeamComboDone {
		return
	}
	triggered := false
	for _, relicID := range s.run.Player.Relics {
		relic, ok := s.lib.Relics[relicID]
		if !ok {
			continue
		}
		for _, effect := range relic.Effects {
			if effect.Trigger != "team_combo" {
				continue
			}
			if err := engine.ApplyExternalCombatEffect(s.lib, s.run.Player, s.combat, effect, engine.CombatTarget{}); err != nil {
				s.appendRoomLogLocked("Coop relic trigger failed: " + err.Error())
				continue
			}
			triggered = true
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
	if !s.requireHostLocked(playerID, "Only the host can resolve rewards.") {
		return
	}
	if s.reward == nil || s.combat == nil {
		return
	}

	switch cmd.Action {
	case "take":
		if cmd.ItemIndex <= 0 || cmd.ItemIndex > len(s.reward.CardChoices) {
			s.appendRoomLogLocked("Invalid reward card.")
			return
		}
		cardID := s.reward.CardChoices[cmd.ItemIndex-1].ID
		if s.reward.EquipmentID != "" {
			if err := s.startEquipmentFlowLocked("reward", cardID, "", ""); err != nil {
				s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
			}
			return
		}
		if err := s.applyRewardLocked(cardID, false); err != nil {
			s.appendRoomLogLocked("Reward resolution failed: " + err.Error())
		}
	case "skip":
		if s.reward.EquipmentID != "" {
			if err := s.startEquipmentFlowLocked("reward", "", "", ""); err != nil {
				s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
			}
			return
		}
		if err := s.applyRewardLocked("", false); err != nil {
			s.appendRoomLogLocked("Reward resolution failed: " + err.Error())
		}
	}
}

func (s *server) handleEventCommandLocked(playerID string, cmd commandPayload) {
	if !s.requireHostLocked(playerID, "Only the host can resolve events.") {
		return
	}
	if s.eventState == nil {
		return
	}
	if cmd.Action != "choose" || cmd.ItemIndex <= 0 || cmd.ItemIndex > len(s.eventState.Event.Choices) {
		s.appendRoomLogLocked("Invalid event choice.")
		return
	}
	choice := s.eventState.Event.Choices[cmd.ItemIndex-1]
	if engine.EventChoiceEquipmentID(choice) != "" {
		if err := s.startEquipmentFlowLocked("event", "", "", choice.ID); err != nil {
			s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
		}
		return
	}

	beforeHP, beforeMax := s.leaderVitalsLocked()
	logStart := len(s.eventState.Log)
	if err := engine.ResolveEventDecision(s.lib, s.run, s.eventState, choice.ID, true); err != nil {
		s.appendRoomLogLocked("Event resolution failed: " + err.Error())
		return
	}
	s.applyLeaderDeltaLocked(beforeHP, beforeMax)
	if err := engine.AdvanceNonCombatNode(s.run, s.currentNode); err != nil {
		s.appendRoomLogLocked("Failed to advance after event: " + err.Error())
		return
	}
	s.appendRoomLogLocked(fmt.Sprintf("Event resolved: %s.", choice.Label))
	s.appendEventOutcomeLogsLocked(choice, logStart)
	s.afterNodeAdvanceLocked()
}

func (s *server) handleShopCommandLocked(playerID string, cmd commandPayload) {
	if !s.requireHostLocked(playerID, "Only the host can resolve the shop.") {
		return
	}
	if s.shopState == nil {
		return
	}

	switch cmd.Action {
	case "leave":
		if err := engine.AdvanceNonCombatNode(s.run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to leave shop: " + err.Error())
			return
		}
		s.appendRoomLogLocked("Left the shop.")
		s.afterNodeAdvanceLocked()
	case "buy":
		if cmd.ItemIndex <= 0 || cmd.ItemIndex > len(s.shopState.Offers) {
			s.appendRoomLogLocked("Invalid shop offer.")
			return
		}
		offer := s.shopState.Offers[cmd.ItemIndex-1]
		if offer.Kind == "equipment" {
			if err := s.startEquipmentFlowLocked("shop", "", offer.ID, ""); err != nil {
				s.appendRoomLogLocked("Equipment prompt failed: " + err.Error())
			}
			return
		}
		if offer.Kind == "remove" {
			if err := s.startDeckActionLocked("shop_remove", offer.Price); err != nil {
				s.appendRoomLogLocked(err.Error())
			}
			return
		}
		beforeHP, beforeMax := s.leaderVitalsLocked()
		logStart := len(s.shopState.Log)
		if err := engine.ApplyShopPurchase(s.lib, s.run, s.shopState, offer.ID); err != nil {
			s.appendRoomLogLocked("Purchase failed: " + err.Error())
			return
		}
		s.applyLeaderDeltaLocked(beforeHP, beforeMax)
		s.appendRoomLogLocked(fmt.Sprintf("Purchased %s.", offer.Name))
		s.appendShopOutcomeLogsLocked(offer, logStart)
	}
}

func (s *server) handleRestCommandLocked(playerID string, cmd commandPayload) {
	if !s.requireHostLocked(playerID, "Only the host can resolve the campfire.") {
		return
	}
	if cmd.Action != "heal" && cmd.Action != "upgrade" {
		return
	}
	if cmd.Action == "upgrade" && len(engine.UpgradableCardIndexes(s.lib, s.run.Player.Deck)) > 0 {
		if err := s.startDeckActionLocked("rest_upgrade", 0); err != nil {
			s.appendRoomLogLocked(err.Error())
		}
		return
	}

	beforeHP, beforeMax := s.leaderVitalsLocked()
	state, err := engine.ResolveRest(s.lib, s.run, cmd.Action)
	if err != nil {
		s.appendRoomLogLocked("Campfire action failed: " + err.Error())
		return
	}
	s.restLog = append([]string{}, state.Log...)
	s.applyLeaderDeltaLocked(beforeHP, beforeMax)
	if err := engine.AdvanceNonCombatNode(s.run, s.currentNode); err != nil {
		s.appendRoomLogLocked("Failed to leave campfire: " + err.Error())
		return
	}
	s.appendRoomLogLocked(fmt.Sprintf("Campfire action: %s.", cmd.Action))
	s.afterNodeAdvanceLocked()
}

func (s *server) handleEquipmentCommandLocked(playerID string, cmd commandPayload) {
	if !s.requireHostLocked(playerID, "Only the host can resolve equipment choices.") {
		return
	}
	if s.equipOffer == nil {
		return
	}

	take := cmd.Action == "take"
	if cmd.Action != "take" && cmd.Action != "skip" {
		return
	}

	switch s.equipFrom {
	case "reward":
		if err := s.applyRewardLocked(s.rewardCard, take); err != nil {
			s.appendRoomLogLocked("Reward equipment failed: " + err.Error())
			return
		}
		s.appendRoomLogLocked(equipmentDecisionText(take, s.equipOffer.EquipmentID))
	case "shop":
		logStart := len(s.shopState.Log)
		if err := engine.ApplyShopEquipmentPurchase(s.lib, s.run, s.shopState, s.shopOfferID, take); err != nil {
			s.appendRoomLogLocked("Shop equipment failed: " + err.Error())
			return
		}
		s.appendRoomLogLocked(equipmentDecisionText(take, s.equipOffer.EquipmentID))
		if offer, err := engineShopOfferByID(s.shopState, s.shopOfferID); err == nil {
			s.appendShopOutcomeLogsLocked(offer, logStart)
		}
		s.clearEquipmentFlowLocked()
		s.phase = phaseShop
		s.announcePhaseLocked("Returned to the shared shop.")
	case "event":
		beforeHP, beforeMax := s.leaderVitalsLocked()
		logStart := len(s.eventState.Log)
		choice := eventChoiceByID(s.eventState, s.eventChoice)
		if err := engine.ResolveEventDecision(s.lib, s.run, s.eventState, s.eventChoice, take); err != nil {
			s.appendRoomLogLocked("Event equipment failed: " + err.Error())
			return
		}
		s.applyLeaderDeltaLocked(beforeHP, beforeMax)
		if err := engine.AdvanceNonCombatNode(s.run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to advance after event: " + err.Error())
			return
		}
		s.appendRoomLogLocked(equipmentDecisionText(take, s.equipOffer.EquipmentID))
		if choice != nil {
			s.appendEventOutcomeLogsLocked(*choice, logStart)
		}
		s.afterNodeAdvanceLocked()
	}
}

func (s *server) handleDeckActionCommandLocked(playerID string, cmd commandPayload) {
	if !s.requireHostLocked(playerID, "Only the host can resolve deck actions.") {
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
		name, err := engine.UpgradeDeckCard(s.lib, &s.run.Player, deckIndex)
		if err != nil {
			s.appendRoomLogLocked("Upgrade failed: " + err.Error())
			return
		}
		s.restLog = []string{"Upgraded " + name}
		if err := engine.AdvanceNonCombatNode(s.run, s.currentNode); err != nil {
			s.appendRoomLogLocked("Failed to leave campfire: " + err.Error())
			return
		}
		s.appendRoomLogLocked("Card upgraded at campfire.")
		s.afterNodeAdvanceLocked()
	case "shop_remove":
		if err := engine.ApplyShopCardRemoval(s.lib, s.run, s.shopState, "remove-card", deckIndex); err != nil {
			s.appendRoomLogLocked("Card removal failed: " + err.Error())
			return
		}
		s.appendRoomLogLocked("Card removed from the deck.")
		s.clearDeckActionLocked()
		s.phase = phaseShop
		s.announcePhaseLocked("Returned to the shared shop.")
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
	leader := buildSharedLeader(players)
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
		Player:       leader,
		Map:          graph,
		Reachable:    firstFloorNodeIDs(graph),
		History:      []string{},
	}
	s.partyMembers = clonePartyStates(players)
	s.partyMembers[0].HP = leader.HP
	s.partyMembers[0].MaxHP = leader.MaxHP
	s.partyMembers[0].MaxEnergy = leader.MaxEnergy
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
		engine.FinishCombat(s.lib, s.run, s.currentNode, s.combat)
		reward := s.combat.Reward
		s.reward = &reward
		s.phase = phaseReward
		s.appendRoomLogLocked("Combat won. Reward phase started.")
		s.announcePhaseLocked("Choose the shared reward.")
		return
	}
	if s.combat.Lost {
		s.syncPartyFromCombatLocked()
		s.run.Status = engine.RunStatusLost
		s.run.Player.HP = 0
		for i := range s.partyMembers {
			s.partyMembers[i].HP = 0
		}
		s.phase = phaseSummary
		s.appendRoomLogLocked("The party was defeated.")
		s.announcePhaseLocked("Run summary opened after defeat.")
	}
}

func (s *server) applyRewardLocked(cardID string, takeEquipment bool) error {
	s.syncPartyFromCombatLocked()
	beforeHP, beforeMax := s.leaderVitalsLocked()
	if err := engine.ApplyCombatResultDecision(s.lib, s.run, s.currentNode, s.combat, cardID, takeEquipment); err != nil {
		return err
	}
	s.applyLeaderDeltaLocked(beforeHP, beforeMax)
	if cardID != "" {
		s.appendRoomLogLocked(fmt.Sprintf("Reward card added: %s.", s.lib.Cards[cardID].Name))
		if slices.Contains(s.lib.Cards[cardID].Flags, "coop_only") {
			s.appendRoomLogLocked(fmt.Sprintf("Picked CO-OP reward: %s.", s.lib.Cards[cardID].Name))
		}
	} else {
		s.appendRoomLogLocked("Reward card skipped.")
	}
	if s.reward != nil && s.reward.RelicID != "" && slices.Contains(s.lib.Relics[s.reward.RelicID].Flags, "coop_only") {
		s.appendRoomLogLocked(fmt.Sprintf("Reward included CO-OP relic: %s.", s.lib.Relics[s.reward.RelicID].Name))
	}
	s.afterNodeAdvanceLocked()
	return nil
}

func (s *server) appendShopOutcomeLogsLocked(offer engine.ShopOffer, logStart int) {
	if s.shopState == nil {
		return
	}
	for _, line := range s.shopState.Log[logStart:] {
		s.appendRoomLogLocked("Shop outcome: " + line)
	}
	if slices.Contains(shopOfferBadges(s.lib, offer), "CO-OP") {
		s.appendRoomLogLocked(fmt.Sprintf("Purchased CO-OP offer: %s.", offer.Name))
	}
}

func (s *server) appendEventOutcomeLogsLocked(choice content.EventChoiceDef, logStart int) {
	if s.eventState == nil {
		return
	}
	for _, line := range s.eventState.Log[logStart:] {
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

func (s *server) startEquipmentFlowLocked(source, rewardCard, shopOfferID, eventChoice string) error {
	var (
		equipmentID string
		price       int
	)
	switch source {
	case "reward":
		if s.reward == nil {
			return fmt.Errorf("reward not available")
		}
		equipmentID = s.reward.EquipmentID
	case "shop":
		if s.shopState == nil {
			return fmt.Errorf("shop not available")
		}
		offer, err := findShopOfferByID(s.shopState, shopOfferID)
		if err != nil {
			return err
		}
		equipmentID = offer.ItemID
		price = offer.Price
	case "event":
		if s.eventState == nil {
			return fmt.Errorf("event not available")
		}
		for _, choice := range s.eventState.Event.Choices {
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
	offer, err := engine.BuildEquipmentOffer(s.lib, s.run.Player, equipmentID, source, price)
	if err != nil {
		return err
	}
	s.equipOffer = &offer
	s.equipFrom = source
	s.rewardCard = rewardCard
	s.shopOfferID = shopOfferID
	s.eventChoice = eventChoice
	s.phase = phaseEquipment
	s.announcePhaseLocked("An equipment replacement prompt is active.")
	return nil
}

func (s *server) startDeckActionLocked(mode string, price int) error {
	indexes := []int{}
	title := ""
	subtitle := ""

	switch mode {
	case "rest_upgrade":
		indexes = engine.UpgradableCardIndexes(s.lib, s.run.Player.Deck)
		title = "Upgrade a card"
		subtitle = "Choose one card to upgrade at the campfire."
	case "shop_remove":
		if len(s.run.Player.Deck) == 0 {
			return fmt.Errorf("deck is empty")
		}
		indexes = make([]int, 0, len(s.run.Player.Deck))
		for i := range s.run.Player.Deck {
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
	s.phase = phaseDeckAction
	s.announcePhaseLocked(title + ".")
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
	s.reward = nil
	s.eventState = nil
	s.shopState = nil
	s.restLog = nil
	s.clearEquipmentFlowLocked()
	s.clearDeckActionLocked()
}

func (s *server) clearEquipmentFlowLocked() {
	s.equipOffer = nil
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
}

func (s *server) resetLobbyLocked() {
	s.run = nil
	s.partyMembers = nil
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

func (s *server) buildPartyStatesLocked() ([]engine.PlayerState, error) {
	players := make([]engine.PlayerState, 0, len(s.order))
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		state, err := buildBasePlayerState(s.lib, player.ClassID, player.Name)
		if err != nil {
			return nil, err
		}
		players = append(players, state)
	}
	return players, nil
}

func (s *server) combatPartyLocked() []engine.PlayerState {
	if s.run == nil {
		return nil
	}
	party := make([]engine.PlayerState, 0, len(s.partyMembers))
	leader := s.run.Player
	if len(s.partyMembers) > 0 {
		leader.Name = s.partyMembers[0].Name
		leader.ClassID = s.partyMembers[0].ClassID
		leader.HP = s.partyMembers[0].HP
		leader.MaxHP = s.partyMembers[0].MaxHP
	}
	party = append(party, leader)
	for i := 1; i < len(s.partyMembers); i++ {
		party = append(party, s.partyMembers[i])
	}
	return party
}

func (s *server) syncPartyFromCombatLocked() {
	if s.combat == nil || len(s.partyMembers) == 0 {
		return
	}
	s.partyMembers[0].HP = s.combat.Player.HP
	s.partyMembers[0].MaxHP = s.combat.Player.MaxHP
	s.run.Player.HP = s.combat.Player.HP
	s.run.Player.MaxHP = s.combat.Player.MaxHP
	for i := range s.combat.Allies {
		if i+1 >= len(s.partyMembers) {
			break
		}
		s.partyMembers[i+1].HP = s.combat.Allies[i].HP
		s.partyMembers[i+1].MaxHP = s.combat.Allies[i].MaxHP
	}
}

func (s *server) leaderVitalsLocked() (int, int) {
	if len(s.partyMembers) == 0 {
		return s.run.Player.HP, s.run.Player.MaxHP
	}
	return s.partyMembers[0].HP, s.partyMembers[0].MaxHP
}

func (s *server) applyLeaderDeltaLocked(beforeHP, beforeMax int) {
	if len(s.partyMembers) == 0 || s.run == nil {
		return
	}
	deltaMax := s.run.Player.MaxHP - beforeMax
	deltaHP := s.run.Player.HP - beforeHP

	s.partyMembers[0].HP = s.run.Player.HP
	s.partyMembers[0].MaxHP = s.run.Player.MaxHP
	for i := 1; i < len(s.partyMembers); i++ {
		s.partyMembers[i].MaxHP = max(1, s.partyMembers[i].MaxHP+deltaMax)
		s.partyMembers[i].HP = clamp(s.partyMembers[i].HP+deltaHP, 0, s.partyMembers[i].MaxHP)
	}
}

func (s *server) connectedPlayerIDsLocked() []string {
	ids := []string{}
	for _, id := range s.order {
		player := s.players[id]
		if player != nil && player.Connected {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *server) playerSeatIndexLocked(playerID string) int {
	for index, id := range s.order {
		if id == playerID {
			if len(s.partyMembers) > 0 && index >= len(s.partyMembers) {
				return -1
			}
			return index
		}
	}
	return -1
}

func (s *server) allReadyLocked() bool {
	connected := 0
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		connected++
		if !player.Ready {
			return false
		}
	}
	return connected > 0
}

func (s *server) canStartRunLocked() bool {
	if !s.allReadyLocked() {
		return false
	}
	return len(s.offlineSeatSummariesLocked()) == 0
}

func (s *server) dropOfflineSeatLocked(seat int) error {
	if seat <= 0 || seat > len(s.order) {
		return fmt.Errorf("invalid seat %d", seat)
	}
	id := s.order[seat-1]
	if id == s.hostID {
		return fmt.Errorf("cannot drop the host seat")
	}
	player := s.players[id]
	if player == nil {
		return fmt.Errorf("seat %d is empty", seat)
	}
	if player.Connected {
		return fmt.Errorf("seat %d is still connected", seat)
	}
	delete(s.players, id)
	delete(s.clients, id)
	s.order = append(s.order[:seat-1], s.order[seat:]...)
	s.appendRoomLogLocked(fmt.Sprintf("Dropped offline seat %d (%s).", seat, player.Name))
	if len(s.offlineSeatSummariesLocked()) == 0 {
		s.restoredFromSave = false
	}
	return nil
}

func (s *server) dropAllOfflineSeatsLocked() int {
	dropped := 0
	for seat := len(s.order); seat >= 1; seat-- {
		id := s.order[seat-1]
		if id == s.hostID {
			continue
		}
		player := s.players[id]
		if player == nil || player.Connected {
			continue
		}
		if err := s.dropOfflineSeatLocked(seat); err == nil {
			dropped++
		}
	}
	return dropped
}

func (s *server) setClientNoticeLocked(id, notice string) {
	if notice == "" {
		return
	}
	if client := s.clients[id]; client != nil {
		client.notice = notice
	}
}

func (s *server) setClientResumeLocked(id string, lines []string) {
	if len(lines) == 0 {
		return
	}
	if client := s.clients[id]; client != nil {
		client.resume = append([]string{}, lines...)
	}
}

func (s *server) announcePhaseLocked(reason string) {
	title := phaseDisplayName(s.phase)
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		notice := fmt.Sprintf("Phase changed: %s.", title)
		if reason != "" {
			notice = fmt.Sprintf("%s %s", notice, reason)
		}
		s.setClientNoticeLocked(id, notice)
		s.setClientResumeLocked(id, s.phaseAnnouncementLinesLocked(id))
	}
}

func (s *server) phaseAnnouncementLinesLocked(selfID string) []string {
	lines := []string{}
	lines = append(lines, s.phaseResumeLinesLocked()...)
	if control := s.controlLabelLocked(selfID); control != "" {
		lines = append(lines, "Control: "+control)
	}
	if note := s.roleNoteLocked(selfID); note != "" {
		lines = append(lines, note)
	}
	examples := s.exampleCommandsLocked(selfID)
	if len(examples) > 0 {
		lines = append(lines, "Suggested next: "+strings.Join(examples, " | "))
	} else {
		lines = append(lines, "Suggested next: wait for the next room update, or use `help`, `status`, or `log` locally.")
	}
	return compactStrings(lines)
}

func (s *server) consumeClientNoticeLocked(id string) string {
	client := s.clients[id]
	if client == nil || client.notice == "" {
		return ""
	}
	notice := client.notice
	client.notice = ""
	return notice
}

func (s *server) consumeClientResumeLocked(id string) []string {
	client := s.clients[id]
	if client == nil || len(client.resume) == 0 {
		return nil
	}
	lines := append([]string{}, client.resume...)
	client.resume = nil
	return lines
}

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
	lines = append(lines, s.phaseResumeLinesLocked()...)
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
		if player.ID == s.hostID {
			return "acting: choose route"
		}
		return "waiting: host route"
	case phaseCombat:
		if s.combat == nil || index >= len(s.combat.Coop.EndTurnVotes) {
			return "acting"
		}
		if s.combat.Coop.EndTurnVotes[index] {
			return "ready"
		}
		return "acting"
	case phaseReward:
		if player.ID == s.hostID {
			return "acting: resolve reward"
		}
		return "waiting: host reward"
	case phaseEvent:
		if player.ID == s.hostID {
			return "acting: resolve event"
		}
		return "waiting: host event"
	case phaseShop:
		if player.ID == s.hostID {
			return "acting: shop decision"
		}
		return "waiting: host shop"
	case phaseRest:
		if player.ID == s.hostID {
			return "acting: campfire choice"
		}
		return "waiting: host campfire"
	case phaseEquipment:
		if player.ID == s.hostID {
			return "acting: equipment choice"
		}
		return "waiting: host equipment"
	case phaseDeckAction:
		if player.ID == s.hostID {
			return "acting: deck action"
		}
		return "waiting: host deck action"
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
	summary = append(summary, s.phaseResumeLinesLocked()...)
	if len(s.offlineSeatSummariesLocked()) > 0 {
		summary = append(summary, fmt.Sprintf("Offline reserved seats: %d.", len(s.offlineSeatSummariesLocked())))
	}
	if len(s.waitingOnLocked()) > 0 {
		summary = append(summary, fmt.Sprintf("Currently waiting on %d connected seat(s).", len(s.waitingOnLocked())))
	}
	return summary
}

func (s *server) phaseResumeLinesLocked() []string {
	switch s.phase {
	case phaseLobby:
		return []string{fmt.Sprintf("Mode %s, seed %d.", s.mode, s.seed)}
	case phaseMap:
		if s.run != nil {
			return []string{fmt.Sprintf("Act %d, next floor %d, gold %d.", s.run.Act, s.run.CurrentFloor+1, s.run.Player.Gold)}
		}
	case phaseCombat:
		if s.combat != nil {
			return []string{fmt.Sprintf("Combat turn %d, energy %d/%d.", s.combat.Turn, s.combat.Player.Energy, s.combat.Player.MaxEnergy)}
		}
	case phaseReward:
		return []string{"Reward choice is still pending."}
	case phaseEvent:
		return []string{"Event choice is still pending."}
	case phaseShop:
		if s.run != nil {
			return []string{fmt.Sprintf("Shop is open with %d gold available.", s.run.Player.Gold)}
		}
	case phaseRest:
		return []string{"Campfire action is still pending."}
	case phaseEquipment:
		return []string{"Equipment replacement prompt is still pending."}
	case phaseDeckAction:
		return []string{"Deck action prompt is still pending."}
	case phaseSummary:
		return []string{"Run summary is waiting for the host's next decision."}
	}
	return nil
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

func (s *server) commandHintsLocked(selfID string) []string {
	isHost := selfID == s.hostID
	base := []string{"chat <text>", "quit"}
	switch s.phase {
	case phaseLobby:
		if isHost {
			base = []string{
				"class <id>",
				"ready",
				"mode <story|endless>",
				"seed <n>",
				"start",
				"drop <seat|all>",
				"chat <text>",
				"abandon",
				"quit",
			}
		} else {
			base = []string{"class <id>", "ready", "chat <text>", "quit"}
		}
	case phaseMap:
		if isHost {
			base = []string{"node <index>", "chat <text>", "quit"}
		}
	case phaseCombat:
		base = []string{"play <card#> [enemy|ally <target#>]", "potion <slot#> [enemy|ally <target#>]", "end", "chat <text>", "quit"}
	case phaseReward:
		if isHost {
			base = []string{"take <card#>", "skip", "chat <text>", "quit"}
		}
	case phaseEvent:
		if isHost {
			base = []string{"choose <index>", "chat <text>", "quit"}
		}
	case phaseShop:
		if isHost {
			base = []string{"buy <index>", "leave", "chat <text>", "quit"}
		}
	case phaseRest:
		if isHost {
			base = []string{"heal", "upgrade", "chat <text>", "quit"}
		}
	case phaseEquipment:
		if isHost {
			base = []string{"take", "skip", "chat <text>", "quit"}
		}
	case phaseDeckAction:
		if isHost {
			base = []string{"choose <index>", "back", "chat <text>", "quit"}
		}
	case phaseSummary:
		if isHost {
			base = []string{"new", "abandon", "chat <text>", "quit"}
		}
	}
	return s.appendTransferCommandsLocked(selfID, base)
}

func (s *server) roleNoteLocked(selfID string) string {
	if s.hostTransfer != nil {
		switch selfID {
		case s.hostTransfer.FromID:
			return "Host transfer is pending. Wait for the requested seat to accept, or use cancel-host."
		case s.hostTransfer.ToID:
			return "You have a pending host transfer request. Accept to take room control, or deny to keep the current host."
		}
	}
	isHost := selfID == s.hostID
	switch s.phase {
	case phaseLobby:
		if isHost {
			return "You control room settings, can start the next run, and can clear offline reserved seats."
		}
		return "Pick a class, toggle ready, and wait for the host to launch the room."
	case phaseMap:
		if isHost {
			return "You choose the next shared node for the whole room."
		}
		return "The host chooses the route. You stay synced to the shared map."
	case phaseCombat:
		return "Every connected seat can play cards and vote end turn. The enemy turn starts only after all connected seats vote."
	case phaseReward:
		if isHost {
			return "You resolve the shared reward for the room."
		}
		return "You are waiting for the host to resolve the shared reward."
	case phaseEvent:
		if isHost {
			return "You choose the event outcome for the room."
		}
		return "You are waiting for the host to choose the event outcome."
	case phaseShop:
		if isHost {
			return "You control shop purchases and when the room leaves the shop."
		}
		return "You are waiting for the host to finish shopping."
	case phaseRest:
		if isHost {
			return "You choose the campfire action for the room."
		}
		return "You are waiting for the host to resolve the campfire."
	case phaseEquipment:
		if isHost {
			return "You decide whether the offered equipment replaces the current slot item."
		}
		return "You are waiting for the host to accept or skip the equipment offer."
	case phaseDeckAction:
		if isHost {
			return "You resolve the shared deck action."
		}
		return "You are waiting for the host to resolve the shared deck action."
	case phaseSummary:
		if isHost {
			return "You can start the next run or abandon the room save from here."
		}
		return "You are waiting for the host to continue or close the room."
	default:
		return ""
	}
}

func (s *server) controlLabelLocked(selfID string) string {
	isHost := selfID == s.hostID
	switch s.phase {
	case phaseLobby:
		if isHost {
			return "Host room setup"
		}
		return "Seat setup"
	case phaseMap:
		if isHost {
			return "Host route selection"
		}
		return "Waiting for host route selection"
	case phaseCombat:
		return "Shared combat control"
	case phaseReward, phaseEvent, phaseShop, phaseRest, phaseEquipment, phaseDeckAction, phaseSummary:
		if isHost {
			return "Host-only room decision"
		}
		return "Waiting for host decision"
	default:
		return ""
	}
}

func (s *server) exampleCommandsLocked(selfID string) []string {
	isHost := selfID == s.hostID
	player := s.players[selfID]
	if s.hostTransfer != nil {
		switch selfID {
		case s.hostTransfer.FromID:
			return []string{"chat host transfer pending", "cancel-host"}
		case s.hostTransfer.ToID:
			return []string{"accept-host", "deny-host", "chat taking host"}
		default:
			return []string{"chat waiting on host transfer"}
		}
	}
	switch s.phase {
	case phaseLobby:
		examples := []string{}
		classID := ""
		if player != nil {
			classID = player.ClassID
		}
		if classID == "" {
			classes := classIDs(s.lib)
			if len(classes) > 0 {
				classID = classes[0]
			}
		}
		if classID != "" {
			examples = append(examples, fmt.Sprintf("class %s", classID))
		}
		examples = append(examples, "ready")
		if isHost {
			examples = append(examples, fmt.Sprintf("seed %d", s.seed))
			examples = append(examples, fmt.Sprintf("mode %s", s.mode))
			for seat, id := range s.order {
				offlinePlayer := s.players[id]
				if offlinePlayer == nil || offlinePlayer.Connected {
					continue
				}
				examples = append(examples, fmt.Sprintf("drop %d", seat+1))
				break
			}
			if len(s.offlineSeatSummariesLocked()) > 1 {
				examples = append(examples, "drop all")
			}
			if s.canStartRunLocked() {
				examples = append(examples, "start")
			}
		}
		examples = append(examples, "chat ready when you are")
		return compactStrings(s.appendTransferExamplesLocked(selfID, examples))
	case phaseMap:
		if !isHost || s.run == nil {
			return []string{"chat waiting on route"}
		}
		reachable := engine.ReachableNodes(s.run)
		if len(reachable) == 0 {
			return s.appendTransferExamplesLocked(selfID, []string{"chat route?"})
		}
		examples := []string{"node 1", "chat taking node 1"}
		return s.appendTransferExamplesLocked(selfID, examples)
	case phaseCombat:
		if s.run == nil || s.combat == nil {
			return []string{"chat ready to end", "end"}
		}
		examples := []string{}
		view := buildCombatSnapshot(s.lib, s.run, s.combat, s.order, s.players)
		if view != nil && len(view.Hand) > 0 {
			examples = append(examples, combatCommandExample(view.Hand[0]))
		}
		if len(s.run.Player.Potions) > 0 {
			examples = append(examples, "potion 1")
		}
		examples = append(examples, "chat focus left enemy")
		examples = append(examples, "end")
		return compactStrings(s.appendTransferExamplesLocked(selfID, examples))
	case phaseReward:
		if !isHost {
			return []string{"chat take the first card"}
		}
		if s.reward != nil && len(s.reward.CardChoices) > 0 {
			examples := []string{"take 1", "skip", "chat taking card 1"}
			return s.appendTransferExamplesLocked(selfID, examples)
		}
		examples := []string{"skip", "chat skipping reward"}
		return s.appendTransferExamplesLocked(selfID, examples)
	case phaseEvent:
		if !isHost || s.eventState == nil || len(s.eventState.Event.Choices) == 0 {
			return []string{"chat choose 1 if you agree"}
		}
		examples := []string{"choose 1", "chat choosing option 1"}
		return s.appendTransferExamplesLocked(selfID, examples)
	case phaseShop:
		if !isHost || s.shopState == nil {
			return []string{"chat want anything from shop?"}
		}
		if len(s.shopState.Offers) > 0 {
			examples := []string{"buy 1", "leave", "chat buying offer 1"}
			return s.appendTransferExamplesLocked(selfID, examples)
		}
		examples := []string{"leave", "chat leaving shop"}
		return s.appendTransferExamplesLocked(selfID, examples)
	case phaseRest:
		if !isHost {
			return []string{"chat heal or upgrade?"}
		}
		examples := []string{"heal", "upgrade", "chat upgrading at campfire"}
		return s.appendTransferExamplesLocked(selfID, examples)
	case phaseEquipment:
		if !isHost {
			return []string{"chat take or skip?"}
		}
		examples := []string{"take", "skip", "chat equipping this"}
		return s.appendTransferExamplesLocked(selfID, examples)
	case phaseDeckAction:
		if !isHost || s.run == nil {
			return []string{"chat choose 1?"}
		}
		if len(s.deckActionIndexes) > 0 {
			examples := []string{"choose 1", "back", "chat picking card 1"}
			return s.appendTransferExamplesLocked(selfID, examples)
		}
		examples := []string{"back", "chat backing out"}
		return s.appendTransferExamplesLocked(selfID, examples)
	case phaseSummary:
		if !isHost {
			return []string{"chat gg"}
		}
		examples := []string{"new", "abandon", "chat reset?"}
		return s.appendTransferExamplesLocked(selfID, examples)
	default:
		return nil
	}
}

func (s *server) phaseHintLocked(selfID string, waitingOn []string) string {
	isHost := selfID == s.hostID
	offline := s.offlineSeatSummariesLocked()
	parts := []string{}
	if transfer := s.transferNoteLocked(selfID); transfer != "" {
		parts = append(parts, transfer)
	}
	if s.restoredFromSave && len(offline) > 0 {
		if isHost {
			parts = append(parts, "Recovered room loaded from disk. Wait for offline seats to reconnect with the same names, or keep playing with reserved seats offline.")
		} else {
			parts = append(parts, "This room was restored from disk. Offline seats can reclaim their spots by reconnecting with the same names.")
		}
	}
	switch s.phase {
	case phaseLobby:
		if len(waitingOn) > 0 {
			parts = append(parts, "Waiting for connected players to ready.")
		}
		if len(offline) > 0 {
			if isHost {
				parts = append(parts, "Offline seats block a new run. Ask them to reconnect with the same names or use `drop <seat>` in the lobby.")
			} else {
				parts = append(parts, "Offline seats must reconnect before the host can start a new run.")
			}
		} else if isHost {
			parts = append(parts, "Everyone online is ready. Start when you want.")
		} else {
			parts = append(parts, "Choose a class and toggle ready.")
		}
	case phaseMap:
		if isHost {
			parts = append(parts, "Choose the next node.")
		} else {
			parts = append(parts, "Waiting for the host to choose the next node.")
		}
	case phaseCombat:
		if len(waitingOn) > 0 {
			parts = append(parts, "Play cards, then end turn. Enemy turn waits for every connected seat to vote.")
		} else {
			parts = append(parts, "All connected seats are ready to end the turn.")
		}
		if len(offline) > 0 {
			parts = append(parts, "Offline seats stay reserved and count as auto-ready for end-turn voting.")
		}
	case phaseReward:
		parts = append(parts, phaseHostWaitHint(isHost, "Resolve the reward choice."))
	case phaseEvent:
		parts = append(parts, phaseHostWaitHint(isHost, "Resolve the event choice."))
	case phaseShop:
		parts = append(parts, phaseHostWaitHint(isHost, "Host can buy or leave the shop."))
	case phaseRest:
		parts = append(parts, phaseHostWaitHint(isHost, "Host chooses heal or upgrade."))
	case phaseEquipment:
		parts = append(parts, phaseHostWaitHint(isHost, "Host confirms whether to equip the new item."))
	case phaseDeckAction:
		parts = append(parts, phaseHostWaitHint(isHost, "Host resolves the deck action selection."))
	case phaseSummary:
		parts = append(parts, phaseHostWaitHint(isHost, "Host can start a new run or abandon the room."))
	}
	return strings.Join(compactStrings(parts), " ")
}

func phaseHostWaitHint(isHost bool, hostText string) string {
	if isHost {
		return hostText
	}
	return "Waiting for the host."
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

func (s *server) broadcastLocked() {
	if s.closed {
		return
	}
	_ = s.persistLocked()
	for _, id := range s.order {
		client := s.clients[id]
		if client == nil {
			continue
		}
		_ = client.enc.Encode(message{Type: "snapshot", Snapshot: s.snapshotLocked(id)})
	}
}

func (s *server) snapshotLocked(selfID string) *roomSnapshot {
	players := make([]roomPlayer, 0, len(s.order))
	for seat, id := range s.order {
		if player := s.players[id]; player != nil {
			players = append(players, roomPlayer{
				ID:        player.ID,
				Seat:      seat + 1,
				Name:      player.Name,
				ClassID:   player.ClassID,
				Ready:     player.Ready,
				Connected: player.Connected,
			})
		}
	}
	waitingOn := s.waitingOnLocked()
	snap := &roomSnapshot{
		SelfID:           selfID,
		Seat:             s.playerSeatIndexLocked(selfID) + 1,
		HostID:           s.hostID,
		RoomAddr:         s.roomAddr,
		Phase:            s.phase,
		PhaseTitle:       phaseDisplayName(s.phase),
		PhaseHint:        s.phaseHintLocked(selfID, waitingOn),
		ControlLabel:     s.controlLabelLocked(selfID),
		RoleNote:         s.roleNoteLocked(selfID),
		Banner:           s.consumeClientNoticeLocked(selfID),
		Players:          players,
		OfflineSeats:     s.offlineSeatSummariesLocked(),
		WaitingOn:        waitingOn,
		SeatStatus:       s.seatStatusLocked(selfID),
		Recovery:         s.recoveryActionsLocked(selfID),
		Reconnect:        s.reconnectCommandsLocked(selfID),
		Resume:           append(s.consumeClientResumeLocked(selfID), s.resumeSummaryLocked(selfID)...),
		Commands:         s.commandHintsLocked(selfID),
		Examples:         s.exampleCommandsLocked(selfID),
		CanStart:         selfID == s.hostID && s.phase == phaseLobby && s.canStartRunLocked(),
		ChatLog:          tailStrings(s.chatLog, 10),
		ChatUnread:       s.chatUnreadLocked(selfID),
		ChatUnreadInView: min(s.chatUnreadLocked(selfID), len(tailStrings(s.chatLog, 10))),
		TransferNote:     s.transferNoteLocked(selfID),
		RoomLog:          tailStrings(s.roomLog, 12),
	}

	switch s.phase {
	case phaseLobby:
		snap.Lobby = &lobbySnapshot{
			Mode:    string(s.mode),
			Seed:    s.seed,
			Classes: classIDs(s.lib),
		}
	case phaseMap:
		snap.Map = s.buildMapSnapshotLocked()
	case phaseCombat:
		snap.Combat = s.buildCombatSnapshotLocked()
	case phaseReward:
		snap.Reward = s.buildRewardSnapshotLocked()
	case phaseEvent:
		snap.Event = s.buildEventSnapshotLocked()
	case phaseShop:
		snap.Shop = s.buildShopSnapshotLocked()
	case phaseRest:
		snap.Rest = s.buildRestSnapshotLocked()
	case phaseEquipment:
		snap.Equipment = buildEquipmentSnapshot(s.lib, s.equipOffer)
	case phaseDeckAction:
		snap.Deck = s.buildDeckActionSnapshotLocked()
	case phaseSummary:
		snap.Summary = s.buildSummarySnapshotLocked()
	}
	return snap
}

func (s *server) buildMapSnapshotLocked() *mapSnapshot {
	reachable := engine.ReachableNodes(s.run)
	nodes := make([]nodeSnapshot, 0, len(reachable))
	for i, node := range reachable {
		nodes = append(nodes, nodeSnapshot{
			Index: i + 1,
			Floor: node.Floor,
			Kind:  string(node.Kind),
			Label: fmt.Sprintf("A%d F%d %s", node.Act, node.Floor, engine.NodeKindName(node.Kind)),
		})
	}
	return &mapSnapshot{
		Mode:      string(s.run.Mode),
		Act:       s.run.Act,
		NextFloor: s.run.CurrentFloor + 1,
		Gold:      s.run.Player.Gold,
		Party:     partySnapshotsFromMembers(s.partyMembers),
		Reachable: nodes,
		History:   tailStrings(s.run.History, 8),
	}
}

func (s *server) buildCombatSnapshotLocked() *combatSnapshot {
	return buildCombatSnapshot(s.lib, s.run, s.combat, s.order, s.players)
}

func buildCombatSnapshot(lib *content.Library, run *engine.RunState, combat *engine.CombatState, order []string, players map[string]*roomPlayer) *combatSnapshot {
	if run == nil || combat == nil {
		return nil
	}
	party := make([]actorSnapshot, 0, 1+len(combat.Allies))
	for i, actor := range engine.PartyMembersView(combat) {
		party = append(party, actorSnapshot{
			Index:     i + 1,
			Name:      actor.Name,
			HP:        actor.HP,
			MaxHP:     actor.MaxHP,
			Energy:    actor.Energy,
			MaxEnergy: actor.MaxEnergy,
			Block:     actor.Block,
			Status:    engine.DescribeStatuses(actor.Statuses),
		})
	}
	enemies := make([]enemySnapshot, 0, len(combat.Enemies))
	for i, enemy := range combat.Enemies {
		enemies = append(enemies, enemySnapshot{
			Index:  i + 1,
			Name:   enemy.Name,
			HP:     enemy.HP,
			MaxHP:  enemy.MaxHP,
			Block:  enemy.Block,
			Status: engine.DescribeStatuses(enemy.Statuses),
			Intent: engine.DescribeIntent(enemy.CurrentIntent),
		})
	}
	hand := make([]cardSnapshot, 0, len(combat.Hand))
	for i, card := range combat.Hand {
		def := lib.Cards[card.ID]
		hand = append(hand, cardSnapshot{
			Index:      i + 1,
			Name:       engine.CardStateName(lib, card.ID, card.Upgraded),
			Cost:       def.Cost,
			Summary:    engine.CardStateSummary(lib, card.ID, card.Upgraded),
			TargetHint: describeTargetKind(engine.CardTargetKindForCard(lib, card)),
			Badges:     flagBadges(def.Flags),
		})
	}
	potions := make([]string, 0, len(run.Player.Potions))
	for i, potionID := range run.Player.Potions {
		if potion, ok := lib.Potions[potionID]; ok {
			potions = append(potions, fmt.Sprintf("%d. %s | %s", i+1, potion.Name, potion.Description))
		}
	}
	logs := []string{}
	for _, entry := range tailCombatLogs(combat.Log, 10) {
		logs = append(logs, fmt.Sprintf("T%d %s", entry.Turn, entry.Text))
	}
	voteStatus := buildVoteStatus(order, players, combat.Coop.EndTurnVotes)
	highlights := []string{}
	if coopCards := countSnapshotsWithBadge(hand, "CO-OP"); coopCards > 0 {
		highlights = append(highlights, fmt.Sprintf("%d co-op card(s) are currently in hand.", coopCards))
	}
	return &combatSnapshot{
		Turn:         combat.Turn,
		Energy:       combat.Player.Energy,
		MaxEnergy:    combat.Player.MaxEnergy,
		Party:        party,
		Enemies:      enemies,
		Hand:         hand,
		Potions:      potions,
		EndTurnVotes: append([]bool{}, combat.Coop.EndTurnVotes...),
		VoteStatus:   voteStatus,
		Logs:         logs,
		Highlights:   highlights,
	}
}

func (s *server) buildRewardSnapshotLocked() *rewardSnapshot {
	if s.reward == nil {
		return nil
	}
	cards := make([]cardSnapshot, 0, len(s.reward.CardChoices))
	for i, card := range s.reward.CardChoices {
		cards = append(cards, cardSnapshot{
			Index:   i + 1,
			Name:    card.Name,
			Cost:    card.Cost,
			Summary: engine.DescribeEffects(s.lib, card.Effects),
			Badges:  flagBadges(card.Flags),
		})
	}
	snap := &rewardSnapshot{
		Gold:   s.reward.Gold,
		Source: string(s.reward.SourceNodeKind),
		Cards:  cards,
	}
	if s.reward.PotionID != "" {
		snap.Potion = s.lib.Potions[s.reward.PotionID].Name
	}
	if s.reward.RelicID != "" {
		relic := s.lib.Relics[s.reward.RelicID]
		snap.Relic = relic.Name
		snap.RelicBadges = flagBadges(relic.Flags)
	}
	if s.reward.EquipmentID != "" {
		if offer, err := engine.BuildEquipmentOffer(s.lib, s.run.Player, s.reward.EquipmentID, "reward", 0); err == nil {
			snap.Equipment = buildEquipmentSnapshot(s.lib, &offer)
		}
	}
	if coopCards := countSnapshotsWithBadge(cards, "CO-OP"); coopCards > 0 {
		snap.Highlights = append(snap.Highlights, fmt.Sprintf("Reward pool contains %d co-op card choice(s).", coopCards))
	}
	if slices.Contains(snap.RelicBadges, "CO-OP") {
		snap.Highlights = append(snap.Highlights, "Reward relic is multiplayer-only.")
	}
	return snap
}

func (s *server) buildEventSnapshotLocked() *eventSnapshot {
	if s.eventState == nil {
		return nil
	}
	choices := make([]choiceSnapshot, 0, len(s.eventState.Event.Choices))
	for i, choice := range s.eventState.Event.Choices {
		choices = append(choices, choiceSnapshot{
			Index:       i + 1,
			Label:       choice.Label,
			Description: choice.Description,
			Badges:      eventChoiceBadges(s.lib, choice),
		})
	}
	return &eventSnapshot{
		Name:        s.eventState.Event.Name,
		Description: s.eventState.Event.Description,
		Badges:      flagBadges(s.eventState.Event.Flags),
		Choices:     choices,
		Log:         append([]string{}, s.eventState.Log...),
		Highlights:  eventHighlights(s.eventState.Event.Flags, choices),
	}
}

func (s *server) buildShopSnapshotLocked() *shopSnapshot {
	if s.shopState == nil {
		return nil
	}
	offers := make([]shopOfferSnapshot, 0, len(s.shopState.Offers))
	for i, offer := range s.shopState.Offers {
		offers = append(offers, shopOfferSnapshot{
			Index:       i + 1,
			Kind:        offer.Kind,
			Category:    shopOfferCategory(offer.Kind),
			Name:        offer.Name,
			Description: offer.Description,
			Price:       offer.Price,
			Badges:      shopOfferBadges(s.lib, offer),
		})
	}
	return &shopSnapshot{
		Gold:       s.run.Player.Gold,
		Offers:     offers,
		Log:        append([]string{}, s.shopState.Log...),
		Highlights: shopHighlights(offers),
	}
}

func (s *server) buildRestSnapshotLocked() *restSnapshot {
	return &restSnapshot{
		Party: partySnapshotsFromMembers(s.partyMembers),
		Log:   append([]string{}, s.restLog...),
	}
}

func buildEquipmentSnapshot(lib *content.Library, offer *engine.EquipmentOfferState) *equipmentSnapshot {
	if offer == nil {
		return nil
	}
	candidate := lib.Equipments[offer.EquipmentID]
	snap := &equipmentSnapshot{
		Source:               offer.Source,
		Slot:                 engine.EquipmentSlotName(offer.Slot),
		CandidateName:        candidate.Name,
		CandidateDescription: candidate.Description,
		Price:                offer.Price,
		CandidateScore:       offer.CandidateScore,
		CurrentScore:         offer.CurrentScore,
	}
	if offer.CurrentEquipmentID != "" {
		current := lib.Equipments[offer.CurrentEquipmentID]
		snap.CurrentName = current.Name
		snap.CurrentDescription = current.Description
	}
	return snap
}

func (s *server) buildDeckActionSnapshotLocked() *deckActionSnapshot {
	cards := make([]cardSnapshot, 0, len(s.deckActionIndexes))
	for i, deckIndex := range s.deckActionIndexes {
		card := s.run.Player.Deck[deckIndex]
		cards = append(cards, cardSnapshot{
			Index:   i + 1,
			Name:    engine.CardStateName(s.lib, card.CardID, card.Upgraded),
			Summary: engine.CardStateSummary(s.lib, card.CardID, card.Upgraded),
		})
	}
	return &deckActionSnapshot{
		Mode:     s.deckActionMode,
		Title:    s.deckActionTitle,
		Subtitle: s.deckActionSubtitle,
		Cards:    cards,
	}
}

func (s *server) buildSummarySnapshotLocked() *summarySnapshot {
	if s.run == nil {
		return nil
	}
	return &summarySnapshot{
		Result:   string(s.run.Status),
		Mode:     string(s.run.Mode),
		Act:      s.run.Act,
		Floors:   s.run.Stats.ClearedFloors,
		Gold:     s.run.Player.Gold,
		DeckSize: len(s.run.Player.Deck),
		Party:    partySnapshotsFromMembers(s.partyMembers),
		History:  tailStrings(s.run.History, 12),
	}
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
