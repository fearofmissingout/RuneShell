package netplay

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
)

type testClient struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

func TestDialTCPWithRetryHandlesDelayedListener(t *testing.T) {
	temp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := temp.Addr().String()
	_ = temp.Close()

	ready := make(chan struct{})
	go func() {
		time.Sleep(180 * time.Millisecond)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			close(ready)
			return
		}
		defer listener.Close()
		close(ready)
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	conn, err := dialTCPWithRetry(addr, time.Second)
	if err != nil {
		t.Fatalf("dialTCPWithRetry() error = %v", err)
	}
	_ = conn.Close()
	<-ready
}

func TestServerLobbyMapCombatAndSharedEndTurn(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()
	go srv.serve()

	host := mustConnectClient(t, srv.roomAddr, "Host", "vanguard")
	defer host.close()
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 1
	})

	guest := mustConnectClient(t, srv.roomAddr, "Guest", "arcanist")
	defer guest.close()
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 2
	})
	_ = guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 2
	})

	host.sendCommand(t, commandPayload{Action: "ready"})
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && playerReady(s.Players, "Host")
	})
	guest.sendCommand(t, commandPayload{Action: "ready"})
	hostReady := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && playerReady(s.Players, "Host") && playerReady(s.Players, "Guest") && s.CanStart
	})
	if !hostReady.CanStart {
		t.Fatalf("expected host snapshot to allow start once everyone is ready")
	}

	host.sendCommand(t, commandPayload{Action: "start"})
	hostMap := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseMap && s.Map != nil && len(s.Map.Reachable) >= 1
	})
	guestMap := guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseMap && s.Map != nil && len(s.Map.Reachable) >= 1
	})
	if len(hostMap.Map.Party) != 2 || len(guestMap.Map.Party) != 2 {
		t.Fatalf("expected both players to appear in map snapshot")
	}

	host.sendCommand(t, commandPayload{Action: "node", ItemIndex: 1})
	hostCombat := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseCombat && s.Combat != nil && len(s.Combat.Party) == 2 && len(s.Combat.Enemies) >= 1
	})
	guestCombat := guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseCombat && s.Combat != nil && len(s.Combat.Party) == 2 && len(s.Combat.Enemies) >= 1
	})
	if len(hostCombat.Combat.EndTurnVotes) != 2 || len(guestCombat.Combat.EndTurnVotes) != 2 {
		t.Fatalf("expected shared end-turn votes for two players")
	}

	cardIndex := 1
	targetKind := ""
	targetIndex := 0
	for _, card := range hostCombat.Combat.Hand {
		if strings.Contains(strings.ToLower(card.TargetHint), "enemy") {
			cardIndex = card.Index
			targetKind = "enemy"
			targetIndex = 1
			break
		}
	}
	host.sendCommand(t, commandPayload{
		Action:      "play",
		CardIndex:   cardIndex,
		TargetKind:  targetKind,
		TargetIndex: targetIndex,
	})
	postPlay := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseCombat && s.Combat != nil && len(s.Combat.Hand) < len(hostCombat.Combat.Hand)
	})
	if postPlay.Combat.Energy >= hostCombat.Combat.Energy {
		t.Fatalf("expected energy to decrease after playing a card")
	}

	host.sendCommand(t, commandPayload{Action: "end"})
	hostVote := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseCombat && s.Combat != nil && len(s.Combat.EndTurnVotes) == 2 && s.Combat.EndTurnVotes[0] && !s.Combat.EndTurnVotes[1]
	})
	if !hostVote.Combat.EndTurnVotes[0] || hostVote.Combat.EndTurnVotes[1] {
		t.Fatalf("expected only host vote to be set after first end-turn vote")
	}

	turnBefore := hostVote.Combat.Turn
	guest.sendCommand(t, commandPayload{Action: "end"})
	hostNextTurn := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseCombat && s.Combat != nil && s.Combat.Turn > turnBefore
	})
	if hostNextTurn.Combat.Turn <= turnBefore {
		t.Fatalf("expected turn to advance after all players vote end turn")
	}
}

func TestServerNonCombatPhaseHandlers(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Ready: true, Connected: true}
	if err := srv.startRunLocked(); err != nil {
		t.Fatalf("startRunLocked() error = %v", err)
	}

	monsterNode := engine.Node{ID: "n1", Act: 1, Floor: 1, Kind: engine.NodeMonster, Edges: []string{"n2"}}
	srv.currentNode = monsterNode
	srv.combat = &engine.CombatState{
		Player: engine.CombatActor{Name: "Host", HP: 60, MaxHP: 80},
		Reward: engine.RewardState{
			Gold:           10,
			CardChoices:    []content.CardDef{lib.Cards["slash"]},
			SourceNodeKind: engine.NodeMonster,
		},
	}
	srv.phase = phaseReward
	srv.reward = &srv.combat.Reward
	if err := srv.applyRewardLocked("slash", false); err != nil {
		t.Fatalf("applyRewardLocked() error = %v", err)
	}
	if srv.phase != phaseMap {
		t.Fatalf("expected reward resolution to return to map, got %s", srv.phase)
	}

	shopNode := engine.Node{ID: "shop1", Act: 1, Floor: 2, Kind: engine.NodeShop, Edges: []string{"n3"}}
	shop := engine.StartShop(lib, srv.run)
	srv.currentNode = shopNode
	srv.shopState = &shop
	srv.phase = phaseShop
	beforeGold := srv.run.Player.Gold
	srv.handleShopCommandLocked("p0", commandPayload{Action: "buy", ItemIndex: 1})
	if srv.run.Player.Gold >= beforeGold {
		t.Fatalf("expected shop purchase to reduce gold")
	}
	srv.handleShopCommandLocked("p0", commandPayload{Action: "leave"})
	if srv.phase != phaseMap {
		t.Fatalf("expected leaving shop to return to map, got %s", srv.phase)
	}

	restNode := engine.Node{ID: "rest1", Act: 1, Floor: 3, Kind: engine.NodeRest, Edges: []string{"n4"}}
	srv.currentNode = restNode
	srv.phase = phaseRest
	srv.handleRestCommandLocked("p0", commandPayload{Action: "heal"})
	if srv.phase != phaseMap {
		t.Fatalf("expected campfire heal to return to map, got %s", srv.phase)
	}
}

func TestSnapshotIncludesSeatsAndWaitingHints(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Ready: true, Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "Guest", ClassID: "arcanist", Ready: false, Connected: true}
	srv.phase = phaseLobby

	lobbySnap := srv.snapshotLocked("p0")
	if lobbySnap.Seat != 1 {
		t.Fatalf("expected host seat 1, got %d", lobbySnap.Seat)
	}
	if len(lobbySnap.Players) != 2 || lobbySnap.Players[1].Seat != 2 {
		t.Fatalf("expected second player to occupy seat 2")
	}
	if len(lobbySnap.WaitingOn) != 1 || !strings.Contains(lobbySnap.WaitingOn[0], "Seat 2 Guest") {
		t.Fatalf("expected lobby waiting list to include seat 2 guest, got %v", lobbySnap.WaitingOn)
	}
	if len(lobbySnap.SeatStatus) != 2 || !strings.Contains(lobbySnap.SeatStatus[0], "ready-host") || !strings.Contains(lobbySnap.SeatStatus[1], "waiting: ready up") {
		t.Fatalf("expected lobby seat status summary, got %v", lobbySnap.SeatStatus)
	}
	if lobbySnap.CanStart {
		t.Fatalf("expected lobby with unready guest to be unable to start")
	}
	if len(lobbySnap.Recovery) != 0 {
		t.Fatalf("expected no recovery actions while all seats are connected, got %v", lobbySnap.Recovery)
	}
	if len(lobbySnap.Commands) < 5 || !containsString(lobbySnap.Commands, "start") || !containsString(lobbySnap.Commands, "drop <seat|all>") {
		t.Fatalf("expected host lobby quick commands, got %v", lobbySnap.Commands)
	}
	if lobbySnap.ControlLabel != "Host room setup" {
		t.Fatalf("expected host lobby control label, got %q", lobbySnap.ControlLabel)
	}
	if !strings.Contains(lobbySnap.RoleNote, "control room settings") {
		t.Fatalf("expected host role note in lobby, got %q", lobbySnap.RoleNote)
	}
	if !containsString(lobbySnap.Examples, "class vanguard") || !containsString(lobbySnap.Examples, "ready") {
		t.Fatalf("expected lobby examples for host, got %v", lobbySnap.Examples)
	}
	if len(lobbySnap.Resume) != 0 {
		t.Fatalf("expected no resume summary before a restored room, got %v", lobbySnap.Resume)
	}

	srv.players["p1"].Ready = true
	if err := srv.startRunLocked(); err != nil {
		t.Fatalf("startRunLocked() error = %v", err)
	}
	selected, err := engine.SelectNode(srv.run, srv.run.Reachable[0])
	if err != nil {
		t.Fatalf("SelectNode() error = %v", err)
	}
	srv.currentNode = selected
	combat, err := engine.StartEncounterForParty(srv.lib, srv.run, srv.combatPartyLocked(), selected)
	if err != nil {
		t.Fatalf("StartEncounterForParty() error = %v", err)
	}
	srv.combat = combat
	engine.StartPlayerTurn(srv.lib, srv.run.Player, srv.combat)
	srv.phase = phaseCombat
	srv.combat.Coop.EndTurnVotes = []bool{true, false}

	combatSnap := srv.snapshotLocked("p0")
	if len(combatSnap.WaitingOn) != 1 || !strings.Contains(combatSnap.WaitingOn[0], "Seat 2 Guest") {
		t.Fatalf("expected combat waiting list to include seat 2 guest, got %v", combatSnap.WaitingOn)
	}
	if len(combatSnap.Combat.VoteStatus) != 2 || !strings.Contains(combatSnap.Combat.VoteStatus[1], "Seat 2 Guest [arcanist]: waiting") {
		t.Fatalf("expected combat vote status for guest, got %v", combatSnap.Combat.VoteStatus)
	}
	if len(combatSnap.Combat.Party) != 2 || combatSnap.Combat.Party[0].Energy == 0 || combatSnap.Combat.Party[0].MaxEnergy == 0 {
		t.Fatalf("expected combat party energy snapshot, got %+v", combatSnap.Combat.Party)
	}
	if len(combatSnap.SeatStatus) != 2 || !strings.Contains(combatSnap.SeatStatus[0], "ready") || !strings.Contains(combatSnap.SeatStatus[1], "acting") {
		t.Fatalf("expected combat seat status summary, got %v", combatSnap.SeatStatus)
	}

	srv.players["p1"].Connected = false
	offlineSnap := srv.snapshotLocked("p0")
	if len(offlineSnap.OfflineSeats) != 1 || !strings.Contains(offlineSnap.OfflineSeats[0], "Seat 2 Guest") {
		t.Fatalf("expected offline seat summary for guest, got %v", offlineSnap.OfflineSeats)
	}
	if len(offlineSnap.Reconnect) != 1 || !strings.Contains(offlineSnap.Reconnect[0], "--name Guest --class arcanist") {
		t.Fatalf("expected reconnect command for offline guest, got %v", offlineSnap.Reconnect)
	}
	if !containsString(offlineSnap.Commands, "play <card#> [enemy|ally <target#>]") || !containsString(offlineSnap.Commands, "end") {
		t.Fatalf("expected combat quick commands, got %v", offlineSnap.Commands)
	}
	if len(offlineSnap.Recovery) != 0 {
		t.Fatalf("expected no lobby recovery actions during combat, got %v", offlineSnap.Recovery)
	}
	if len(offlineSnap.WaitingOn) != 0 {
		t.Fatalf("expected offline guest to stop blocking waiting list, got %v", offlineSnap.WaitingOn)
	}
	if offlineSnap.ControlLabel != "Shared combat control" {
		t.Fatalf("expected combat control label, got %q", offlineSnap.ControlLabel)
	}
	if !strings.Contains(offlineSnap.RoleNote, "Every connected seat can play cards") {
		t.Fatalf("expected combat role note, got %q", offlineSnap.RoleNote)
	}
	if len(offlineSnap.Examples) == 0 || !containsString(offlineSnap.Examples, "end") {
		t.Fatalf("expected combat examples, got %v", offlineSnap.Examples)
	}
	if len(offlineSnap.Combat.VoteStatus) != 2 || !strings.Contains(offlineSnap.Combat.VoteStatus[1], "offline-auto-ready") {
		t.Fatalf("expected offline guest vote status, got %v", offlineSnap.Combat.VoteStatus)
	}
	if len(offlineSnap.SeatStatus) != 2 || !strings.Contains(offlineSnap.SeatStatus[1], "offline-auto-ready") {
		t.Fatalf("expected offline guest seat status, got %v", offlineSnap.SeatStatus)
	}
}

func TestLobbyOfflineSeatMustReconnectOrBeDropped(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	host := &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Ready: true, Connected: true}
	guest := &roomPlayer{ID: "p1", Name: "Guest", ClassID: "arcanist", Ready: false, Connected: false}
	srv.players["p0"] = host
	srv.players["p1"] = guest
	srv.phase = phaseLobby

	if srv.canStartRunLocked() {
		t.Fatalf("expected offline reserved seat to block starting")
	}

	srv.handleLobbyCommandLocked("p0", host, commandPayload{Action: "start"})
	if srv.phase != phaseLobby {
		t.Fatalf("expected room to remain in lobby when start is blocked, got %s", srv.phase)
	}
	lastLog := srv.roomLog[len(srv.roomLog)-1]
	if !strings.Contains(lastLog, "Offline reserved seats") {
		t.Fatalf("expected blocked-start log, got %q", lastLog)
	}

	srv.handleLobbyCommandLocked("p0", host, commandPayload{Action: "drop", Seat: 2})
	if len(srv.order) != 1 {
		t.Fatalf("expected dropped guest seat, got order %v", srv.order)
	}
	if _, ok := srv.players["p1"]; ok {
		t.Fatalf("expected dropped guest to be removed from player map")
	}
	if !srv.canStartRunLocked() {
		t.Fatalf("expected host to be able to start after dropping offline seat")
	}

	srv.order = []string{"p0", "p1", "p2"}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "GuestA", ClassID: "arcanist", Ready: false, Connected: false}
	srv.players["p2"] = &roomPlayer{ID: "p2", Name: "GuestB", ClassID: "vanguard", Ready: false, Connected: false}
	srv.restoredFromSave = true
	snap := srv.snapshotLocked("p0")
	if len(snap.Recovery) != 3 || !strings.Contains(snap.Recovery[2], "drop all") {
		t.Fatalf("expected seat-specific and drop-all recovery actions, got %v", snap.Recovery)
	}
	if len(snap.Reconnect) != 2 || !strings.Contains(snap.Reconnect[0], "--name GuestA --class arcanist") || !strings.Contains(snap.Reconnect[1], "--name GuestB --class vanguard") {
		t.Fatalf("expected reconnect commands for offline seats, got %v", snap.Reconnect)
	}
	if len(snap.Resume) == 0 || !strings.Contains(snap.Resume[0], "Recovered phase: LAN Lobby") {
		t.Fatalf("expected restored lobby resume summary, got %v", snap.Resume)
	}

	srv.handleLobbyCommandLocked("p0", host, commandPayload{Action: "drop_all"})
	if len(srv.order) != 1 {
		t.Fatalf("expected all offline seats dropped, got order %v", srv.order)
	}
	if !srv.canStartRunLocked() {
		t.Fatalf("expected host to be able to start after drop_all")
	}
}

func TestServerPersistenceRoundTrip(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	savePath := filepath.Join(t.TempDir(), "room.json")
	srv, err := newServerWithSavePath(lib, "127.0.0.1:0", savePath)
	if err != nil {
		t.Fatalf("newServerWithSavePath() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Ready: true, Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "Guest", ClassID: "arcanist", Ready: true, Connected: true}
	srv.nextID = 2

	if err := srv.startRunLocked(); err != nil {
		t.Fatalf("startRunLocked() error = %v", err)
	}
	if len(srv.run.Reachable) == 0 {
		t.Fatalf("expected reachable nodes after start")
	}

	nodeID := srv.run.Reachable[0]
	selected, err := engine.SelectNode(srv.run, nodeID)
	if err != nil {
		t.Fatalf("SelectNode() error = %v", err)
	}
	srv.currentNode = selected
	party := srv.combatPartyLocked()
	combat, err := engine.StartEncounterForParty(srv.lib, srv.run, party, selected)
	if err != nil {
		t.Fatalf("StartEncounterForParty() error = %v", err)
	}
	srv.combat = combat
	engine.StartPlayerTurn(srv.lib, srv.run.Player, srv.combat)
	srv.phase = phaseCombat
	srv.chatLog = []string{"12:00 Host: hello team"}

	if err := srv.persistLocked(); err != nil {
		t.Fatalf("persistLocked() error = %v", err)
	}
	_ = srv.listener.Close()

	restored, ok, err := loadServerFromSavePath(lib, "127.0.0.1:0", savePath)
	if err != nil {
		t.Fatalf("loadServerFromSavePath() error = %v", err)
	}
	defer func() {
		_ = restored.listener.Close()
	}()
	if !ok {
		t.Fatalf("expected saved room to restore")
	}
	if restored.phase != phaseCombat {
		t.Fatalf("expected restored phase %q, got %q", phaseCombat, restored.phase)
	}
	if restored.hostID != "p0" || len(restored.order) != 2 {
		t.Fatalf("expected restored host/order to match saved room")
	}
	if restored.run == nil || restored.combat == nil {
		t.Fatalf("expected restored run and combat state")
	}
	if restored.currentNode.ID != selected.ID {
		t.Fatalf("expected restored current node %q, got %q", selected.ID, restored.currentNode.ID)
	}
	if len(restored.chatLog) != 1 || !strings.Contains(restored.chatLog[0], "hello team") {
		t.Fatalf("expected restored chat log, got %v", restored.chatLog)
	}
	if len(restored.partyMembers) != 2 {
		t.Fatalf("expected restored party size 2, got %d", len(restored.partyMembers))
	}
	for _, player := range restored.players {
		if player.Connected {
			t.Fatalf("expected restored players to be offline until they reconnect")
		}
	}
}

func TestServerReconnectIntoSavedRun(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	savePath := filepath.Join(t.TempDir(), "room.json")
	srv, err := newServerWithSavePath(lib, "127.0.0.1:0", savePath)
	if err != nil {
		t.Fatalf("newServerWithSavePath() error = %v", err)
	}
	go srv.serve()

	host := mustConnectClient(t, srv.roomAddr, "Host", "vanguard")
	defer host.close()
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 1
	})

	guest := mustConnectClient(t, srv.roomAddr, "Guest", "arcanist")
	defer guest.close()
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 2
	})
	_ = guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 2
	})

	host.sendCommand(t, commandPayload{Action: "ready"})
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && playerReady(s.Players, "Host")
	})
	guest.sendCommand(t, commandPayload{Action: "ready"})
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && s.CanStart
	})

	host.sendCommand(t, commandPayload{Action: "start"})
	mapSnap := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseMap && s.Map != nil && len(s.Map.Party) == 2
	})
	if len(mapSnap.Map.Reachable) == 0 {
		t.Fatalf("expected reachable nodes after start")
	}
	_ = guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseMap && s.Map != nil && len(s.Map.Party) == 2
	})

	srv.mu.Lock()
	srv.shutdownLocked("Test restart.")
	srv.mu.Unlock()
	host.close()
	guest.close()

	restored, ok, err := loadServerFromSavePath(lib, "127.0.0.1:0", savePath)
	if err != nil {
		t.Fatalf("loadServerFromSavePath() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected saved room to restore")
	}
	restored.restoredFromSave = true
	defer func() {
		restored.mu.Lock()
		restored.shutdownLocked("Test done.")
		restored.mu.Unlock()
	}()
	go restored.serve()

	host2 := mustConnectClient(t, restored.roomAddr, "Host", "vanguard")
	defer host2.close()
	hostRecovered := host2.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseMap && s.Map != nil && len(s.Players) == 2 && playerConnected(s.Players, "Host") && !playerConnected(s.Players, "Guest")
	})
	if hostRecovered.SelfID != restored.hostID {
		t.Fatalf("expected host to reconnect into saved host seat")
	}
	if hostRecovered.Seat != 1 {
		t.Fatalf("expected restored host to reclaim seat 1, got %d", hostRecovered.Seat)
	}
	if !strings.Contains(hostRecovered.Banner, "Saved room restored") {
		t.Fatalf("expected host recovery banner, got %q", hostRecovered.Banner)
	}
	if !strings.Contains(hostRecovered.PhaseHint, "Recovered room loaded from disk") {
		t.Fatalf("expected host recovery phase hint, got %q", hostRecovered.PhaseHint)
	}
	if len(hostRecovered.Resume) == 0 || !strings.Contains(hostRecovered.Resume[0], "Recovered phase: Shared Map") {
		t.Fatalf("expected host resume summary, got %v", hostRecovered.Resume)
	}
	if len(hostRecovered.Reconnect) != 1 || !strings.Contains(hostRecovered.Reconnect[0], "--name Guest --class arcanist") {
		t.Fatalf("expected reconnect command for missing guest, got %v", hostRecovered.Reconnect)
	}
	if !containsString(hostRecovered.Commands, "node <index>") {
		t.Fatalf("expected shared map quick command for host, got %v", hostRecovered.Commands)
	}
	if hostRecovered.ControlLabel != "Host route selection" {
		t.Fatalf("expected host control label after restore, got %q", hostRecovered.ControlLabel)
	}
	if !strings.Contains(hostRecovered.RoleNote, "choose the next shared node") {
		t.Fatalf("expected host role note after restore, got %q", hostRecovered.RoleNote)
	}
	if !containsString(hostRecovered.Examples, "node 1") {
		t.Fatalf("expected host example command after restore, got %v", hostRecovered.Examples)
	}

	guest2 := mustConnectClient(t, restored.roomAddr, "Guest", "arcanist")
	defer guest2.close()
	guestRecovered := guest2.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseMap && s.Map != nil && len(s.Players) == 2 && playerConnected(s.Players, "Host") && playerConnected(s.Players, "Guest")
	})
	if guestRecovered.Phase != phaseMap || guestRecovered.Map == nil {
		t.Fatalf("expected guest to reconnect into saved map phase")
	}
	if guestRecovered.Seat != 2 {
		t.Fatalf("expected guest to reclaim seat 2, got %d", guestRecovered.Seat)
	}
	if !strings.Contains(guestRecovered.Banner, "Rejoined seat 2") {
		t.Fatalf("expected guest reconnect banner, got %q", guestRecovered.Banner)
	}
	if len(guestRecovered.Resume) == 0 || !strings.Contains(guestRecovered.Resume[0], "Recovered phase: Shared Map") {
		t.Fatalf("expected guest resume summary, got %v", guestRecovered.Resume)
	}
	if guestRecovered.ControlLabel != "Waiting for host route selection" {
		t.Fatalf("expected guest control label after restore, got %q", guestRecovered.ControlLabel)
	}
	if !strings.Contains(guestRecovered.RoleNote, "host chooses the route") {
		t.Fatalf("expected guest role note after restore, got %q", guestRecovered.RoleNote)
	}
	if !containsString(guestRecovered.Examples, "chat waiting on route") {
		t.Fatalf("expected waiting member chat example after restore, got %v", guestRecovered.Examples)
	}
	if !containsString(guestRecovered.Commands, "quit") {
		t.Fatalf("expected member quick commands, got %v", guestRecovered.Commands)
	}
}

func TestServerAbandonClearsSavedRoom(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	savePath := filepath.Join(t.TempDir(), "room.json")
	srv, err := newServerWithSavePath(lib, "127.0.0.1:0", savePath)
	if err != nil {
		t.Fatalf("newServerWithSavePath() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0"}
	host := &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Ready: true, Connected: true}
	srv.players["p0"] = host
	if err := srv.persistLocked(); err != nil {
		t.Fatalf("persistLocked() error = %v", err)
	}
	if _, err := os.Stat(savePath); err != nil {
		t.Fatalf("expected save file before abandon, got %v", err)
	}

	srv.handleLobbyCommandLocked("p0", host, commandPayload{Action: "abandon"})
	if !srv.closed {
		t.Fatalf("expected room to close after abandon")
	}
	if _, err := os.Stat(savePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected save file to be removed after abandon, got %v", err)
	}
}

func TestIsGracefulRoomClose(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: true},
		{name: "eof", err: io.EOF, want: true},
		{name: "closed-prefix", err: errors.New("room closed: host session ended"), want: true},
		{name: "other", err: errors.New("boom"), want: false},
	}
	for _, tc := range cases {
		if got := isGracefulRoomClose(tc.err); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestRenderRewardSnapshotForMemberUsesSnapshotCommands(t *testing.T) {
	snapshot := &roomSnapshot{
		SelfID:       "p1",
		HostID:       "p0",
		RoomAddr:     "127.0.0.1:7777",
		Phase:        phaseReward,
		PhaseTitle:   "Reward",
		PhaseHint:    "Waiting for the host.",
		ControlLabel: "Waiting for host decision",
		RoleNote:     "You are waiting for the host to resolve the shared reward.",
		Players: []roomPlayer{
			{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true},
			{ID: "p1", Seat: 2, Name: "Guest", ClassID: "arcanist", Connected: true},
		},
		Commands: []string{"quit"},
		Reward: &rewardSnapshot{
			Gold:   20,
			Source: "monster",
			Cards: []cardSnapshot{
				{Index: 1, Name: "Slash", Summary: "Deal 6 damage"},
			},
		},
	}

	out := captureStdout(t, func() {
		renderRewardSnapshot(snapshot)
	})
	if !strings.Contains(out, "Control: Waiting for host decision") {
		t.Fatalf("expected control label in reward render, got %q", out)
	}
	if !strings.Contains(out, "Quick commands:") || !strings.Contains(out, "- quit") {
		t.Fatalf("expected header quick commands for member reward render, got %q", out)
	}
	if strings.Contains(out, "take <card#>") || strings.Contains(out, "skip") {
		t.Fatalf("expected member reward render to avoid host-only commands, got %q", out)
	}
}

func TestRenderMapSnapshotForHostUsesSnapshotCommands(t *testing.T) {
	snapshot := &roomSnapshot{
		SelfID:       "p0",
		HostID:       "p0",
		RoomAddr:     "127.0.0.1:7777",
		Phase:        phaseMap,
		PhaseTitle:   "Shared Map",
		PhaseHint:    "Choose the next node.",
		ControlLabel: "Host route selection",
		RoleNote:     "You choose the next shared node for the whole room.",
		Players: []roomPlayer{
			{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true},
			{ID: "p1", Seat: 2, Name: "Guest", ClassID: "arcanist", Connected: true},
		},
		Commands: []string{"node <index>", "quit"},
		Examples: []string{"node 1"},
		Map: &mapSnapshot{
			Mode:      "story",
			Act:       1,
			NextFloor: 2,
			Gold:      99,
			Reachable: []nodeSnapshot{{Index: 1, Floor: 2, Kind: "monster", Label: "A1 F2 Monster"}},
		},
	}

	out := captureStdout(t, func() {
		renderMapSnapshot(snapshot)
	})
	if !strings.Contains(out, "Control: Host route selection") {
		t.Fatalf("expected host control label in map render, got %q", out)
	}
	if !strings.Contains(out, "Quick commands:") || !strings.Contains(out, "node <index>") {
		t.Fatalf("expected host map quick commands in header, got %q", out)
	}
	if !strings.Contains(out, "Try next:") || !strings.Contains(out, "node 1") {
		t.Fatalf("expected host map example command, got %q", out)
	}
}

func TestAnnouncePhaseLockedSetsBannerAndResume(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Ready: true, Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "Guest", ClassID: "arcanist", Ready: true, Connected: true}
	srv.clients["p0"] = &clientConn{id: "p0", enc: json.NewEncoder(io.Discard)}
	srv.clients["p1"] = &clientConn{id: "p1", enc: json.NewEncoder(io.Discard)}
	srv.phase = phaseLobby
	srv.mode = engine.ModeStory
	srv.seed = 42

	srv.announcePhaseLocked("Room reset to lobby.")

	hostSnap := srv.snapshotLocked("p0")
	if !strings.Contains(hostSnap.Banner, "Phase changed: LAN Lobby. Room reset to lobby.") {
		t.Fatalf("expected host phase-change banner, got %q", hostSnap.Banner)
	}
	if len(hostSnap.Resume) == 0 || !strings.Contains(strings.Join(hostSnap.Resume, "\n"), "Suggested next:") {
		t.Fatalf("expected host phase-change resume lines, got %v", hostSnap.Resume)
	}

	guestSnap := srv.snapshotLocked("p1")
	if !strings.Contains(guestSnap.Banner, "Phase changed: LAN Lobby. Room reset to lobby.") {
		t.Fatalf("expected guest phase-change banner, got %q", guestSnap.Banner)
	}
	if len(guestSnap.Resume) == 0 || !strings.Contains(strings.Join(guestSnap.Resume, "\n"), "Control: Seat setup") {
		t.Fatalf("expected guest phase-change resume lines, got %v", guestSnap.Resume)
	}
}

func TestHandleLocalClientCommandHelpAndLog(t *testing.T) {
	snapshot := &roomSnapshot{
		SelfID:       "p1",
		HostID:       "p0",
		RoomAddr:     "127.0.0.1:7777",
		Phase:        phaseLobby,
		PhaseTitle:   "LAN Lobby",
		PhaseHint:    "Choose a class and toggle ready.",
		ControlLabel: "Seat setup",
		RoleNote:     "Pick a class, toggle ready, and wait for the host to launch the room.",
		Players: []roomPlayer{
			{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true},
			{ID: "p1", Seat: 2, Name: "Guest", ClassID: "arcanist", Connected: true},
		},
		Commands: []string{"class <id>", "ready", "quit"},
		Examples: []string{"class arcanist", "ready"},
		ChatLog:  []string{"12:00 Host: hello", "12:01 Guest: hi"},
		RoomLog:  []string{"Host joined.", "Guest joined."},
		Lobby:    &lobbySnapshot{Mode: "story", Seed: 7, Classes: []string{"vanguard", "arcanist"}},
	}

	helpOut := captureStdout(t, func() {
		if _, handled := handleLocalClientCommand(snapshot, "help"); !handled {
			t.Fatalf("expected help to be handled locally")
		}
	})
	if !strings.Contains(helpOut, "Local commands:") || !strings.Contains(helpOut, "help / ?") {
		t.Fatalf("expected local help output, got %q", helpOut)
	}
	if !strings.Contains(helpOut, "chatlog") {
		t.Fatalf("expected chatlog in local help output, got %q", helpOut)
	}

	logOut := captureStdout(t, func() {
		if _, handled := handleLocalClientCommand(snapshot, "log"); !handled {
			t.Fatalf("expected log to be handled locally")
		}
	})
	if !strings.Contains(logOut, "-- System --") || !strings.Contains(logOut, "System | Guest joined.") {
		t.Fatalf("expected local log output, got %q", logOut)
	}

	var localChatCmd *commandPayload
	chatOut := captureStdout(t, func() {
		var handled bool
		localChatCmd, handled = handleLocalClientCommand(snapshot, "chatlog")
		if !handled {
			t.Fatalf("expected chatlog to be handled locally")
		}
	})
	if !strings.Contains(chatOut, "-- Chat --") || !strings.Contains(chatOut, "12:01 Chat | Guest: hi") {
		t.Fatalf("expected local chatlog output, got %q", chatOut)
	}
	if localChatCmd == nil || localChatCmd.Action != "chat_seen" {
		t.Fatalf("expected chatlog to emit chat_seen command, got %#v", localChatCmd)
	}
}

func TestParseClientCommandSupportsChatAcrossPhases(t *testing.T) {
	snapshot := &roomSnapshot{Phase: phaseCombat}

	cmd, quit, err := parseClientCommand(snapshot, "chat focus left enemy")
	if err != nil {
		t.Fatalf("parseClientCommand(chat) error = %v", err)
	}
	if quit {
		t.Fatalf("expected chat command not to quit")
	}
	if cmd == nil || cmd.Action != "say" || cmd.Value != "focus left enemy" {
		t.Fatalf("expected parsed say command, got %#v", cmd)
	}

	cmd, quit, err = parseClientCommand(snapshot, "say hello team")
	if err != nil {
		t.Fatalf("parseClientCommand(say) error = %v", err)
	}
	if quit || cmd == nil || cmd.Action != "say" || cmd.Value != "hello team" {
		t.Fatalf("expected parsed say command from say alias, got %#v quit=%v", cmd, quit)
	}

	cmd, quit, err = parseClientCommand(snapshot, "host 2")
	if err != nil {
		t.Fatalf("parseClientCommand(host) error = %v", err)
	}
	if quit || cmd == nil || cmd.Action != "host" || cmd.Seat != 2 {
		t.Fatalf("expected parsed host transfer command, got %#v quit=%v", cmd, quit)
	}

	cmd, quit, err = parseClientCommand(snapshot, "accept-host")
	if err != nil {
		t.Fatalf("parseClientCommand(accept-host) error = %v", err)
	}
	if quit || cmd == nil || cmd.Action != "accept_host" {
		t.Fatalf("expected parsed accept_host command, got %#v quit=%v", cmd, quit)
	}
}

func TestServerChatBroadcastAndSnapshot(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()
	go srv.serve()

	host := mustConnectClient(t, srv.roomAddr, "Host", "vanguard")
	defer host.close()
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 1
	})

	guest := mustConnectClient(t, srv.roomAddr, "Guest", "arcanist")
	defer guest.close()
	_ = host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 2
	})
	_ = guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.Phase == phaseLobby && len(s.Players) == 2
	})

	host.sendCommand(t, commandPayload{Action: "say", Value: "focus left enemy"})

	hostChat := host.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return len(s.ChatLog) > 0 && strings.Contains(s.ChatLog[len(s.ChatLog)-1], "Host: focus left enemy")
	})
	if hostChat.ChatUnread != 0 {
		t.Fatalf("expected sender unread count 0, got %d", hostChat.ChatUnread)
	}
	if !containsString(hostChat.Commands, "chat <text>") {
		t.Fatalf("expected chat command in snapshot commands, got %v", hostChat.Commands)
	}
	if strings.Contains(strings.Join(hostChat.RoomLog, "\n"), "focus left enemy") {
		t.Fatalf("expected chat not to pollute system room log, got %v", hostChat.RoomLog)
	}

	guestChat := guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return len(s.ChatLog) > 0 && strings.Contains(s.ChatLog[len(s.ChatLog)-1], "Host: focus left enemy")
	})
	if guestChat.ChatUnread != 1 {
		t.Fatalf("expected guest unread count 1, got %d", guestChat.ChatUnread)
	}
	if !strings.Contains(guestChat.Banner, "Chat from Host: focus left enemy") {
		t.Fatalf("expected guest chat banner, got %q", guestChat.Banner)
	}

	guest.sendCommand(t, commandPayload{Action: "chat_seen"})
	guestSeen := guest.waitForSnapshot(t, func(s *roomSnapshot) bool {
		return s.ChatUnread == 0
	})
	if guestSeen.ChatUnread != 0 {
		t.Fatalf("expected guest unread count cleared after chat_seen, got %d", guestSeen.ChatUnread)
	}
}

func TestHostTransferRequiresAcceptanceAndPersists(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	savePath := filepath.Join(t.TempDir(), "room.json")
	srv, err := newServerWithSavePath(lib, "127.0.0.1:0", savePath)
	if err != nil {
		t.Fatalf("newServerWithSavePath() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Ready: true, Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "Guest", ClassID: "arcanist", Ready: true, Connected: true}
	srv.clients["p0"] = &clientConn{id: "p0", enc: json.NewEncoder(io.Discard)}
	srv.clients["p1"] = &clientConn{id: "p1", enc: json.NewEncoder(io.Discard)}
	srv.phase = phaseLobby
	srv.mode = engine.ModeStory
	srv.seed = 42

	srv.mu.Lock()
	srv.handleHostTransferCommandLocked("p0", commandPayload{Action: "host", Seat: 2})

	if srv.hostID != "p0" {
		srv.mu.Unlock()
		t.Fatalf("expected host to stay p0 until acceptance, got %q", srv.hostID)
	}
	if srv.hostTransfer == nil || srv.hostTransfer.ToID != "p1" {
		srv.mu.Unlock()
		t.Fatalf("expected pending host transfer to p1, got %#v", srv.hostTransfer)
	}

	targetSnap := srv.snapshotLocked("p1")
	if targetSnap.HostID != "p0" {
		srv.mu.Unlock()
		t.Fatalf("expected snapshot host id to stay old host before acceptance, got %q", targetSnap.HostID)
	}
	if targetSnap.ControlLabel != "Seat setup" {
		srv.mu.Unlock()
		t.Fatalf("expected pending target to remain member until acceptance, got %q", targetSnap.ControlLabel)
	}
	if !containsString(targetSnap.Commands, "accept-host") || !containsString(targetSnap.Commands, "deny-host") {
		srv.mu.Unlock()
		t.Fatalf("expected acceptance commands for target, got %v", targetSnap.Commands)
	}
	if !strings.Contains(targetSnap.Banner, "Host transfer requested to Seat 2") {
		srv.mu.Unlock()
		t.Fatalf("expected target transfer request banner, got %q", targetSnap.Banner)
	}

	oldHostSnap := srv.snapshotLocked("p0")
	if oldHostSnap.ControlLabel != "Host room setup" {
		srv.mu.Unlock()
		t.Fatalf("expected old host to remain host until acceptance, got %q", oldHostSnap.ControlLabel)
	}
	if !containsString(oldHostSnap.Commands, "cancel-host") {
		srv.mu.Unlock()
		t.Fatalf("expected old host to be able to cancel pending transfer, got %v", oldHostSnap.Commands)
	}
	if !strings.Contains(oldHostSnap.Banner, "Host transfer request sent to Seat 2 Guest") {
		srv.mu.Unlock()
		t.Fatalf("expected old host request banner, got %q", oldHostSnap.Banner)
	}

	srv.handleHostTransferDecisionLocked("p1", commandPayload{Action: "accept_host"})
	if srv.hostID != "p1" {
		srv.mu.Unlock()
		t.Fatalf("expected host transfer to p1 after acceptance, got %q", srv.hostID)
	}
	if srv.hostTransfer != nil {
		srv.mu.Unlock()
		t.Fatalf("expected pending transfer cleared after acceptance, got %#v", srv.hostTransfer)
	}

	newHostSnap := srv.snapshotLocked("p1")
	if newHostSnap.HostID != "p1" {
		srv.mu.Unlock()
		t.Fatalf("expected snapshot host id to switch after acceptance, got %q", newHostSnap.HostID)
	}
	if newHostSnap.ControlLabel != "Host room setup" {
		srv.mu.Unlock()
		t.Fatalf("expected accepted host control label, got %q", newHostSnap.ControlLabel)
	}
	if !containsString(newHostSnap.Commands, "start") {
		srv.mu.Unlock()
		t.Fatalf("expected host-only commands for accepted host, got %v", newHostSnap.Commands)
	}

	if err := srv.persistLocked(); err != nil {
		srv.mu.Unlock()
		t.Fatalf("persistLocked() error = %v", err)
	}
	srv.mu.Unlock()
	_ = srv.listener.Close()

	restored, ok, err := loadServerFromSavePath(lib, "127.0.0.1:0", savePath)
	if err != nil {
		t.Fatalf("loadServerFromSavePath() error = %v", err)
	}
	defer func() {
		_ = restored.listener.Close()
	}()
	if !ok {
		t.Fatalf("expected restored room after host transfer")
	}
	if restored.hostID != "p1" {
		t.Fatalf("expected transferred host id to persist, got %q", restored.hostID)
	}
}

func TestCoopRelicComboTriggersAfterDifferentSeatsAct(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "Guest", ClassID: "arcanist", Connected: true}
	srv.run = &engine.RunState{
		Mode:      engine.ModeStory,
		Seed:      7,
		PartySize: 2,
		Player: engine.PlayerState{
			Name:      "Host",
			MaxEnergy: 3,
			Relics:    []string{"linked_bracers"},
			Deck:      []engine.DeckCard{{CardID: "slash"}},
		},
	}
	srv.combat = &engine.CombatState{
		Player: engine.CombatActor{Name: "Host", HP: 60, MaxHP: 60, Energy: 3, MaxEnergy: 3, Statuses: map[string]engine.Status{}},
		Allies: []engine.CombatActor{
			{Name: "Guest", HP: 55, MaxHP: 55, Statuses: map[string]engine.Status{}},
		},
		Enemies: []engine.CombatEnemy{
			{
				CombatActor: engine.CombatActor{Name: "Dummy", HP: 40, MaxHP: 40, Statuses: map[string]engine.Status{}},
				IntentCycle: []content.EnemyIntentDef{{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}}},
			},
		},
		Turn: 1,
		Coop: engine.CombatCoopState{Enabled: true, EndTurnVotes: make([]bool, 2)},
	}

	srv.handleCoopCombatActionLocked(0, "Host")
	if srv.combat.Player.Block != 0 || srv.combat.Allies[0].Block != 0 {
		t.Fatalf("expected no combo trigger after first unique actor, got blocks %d/%d", srv.combat.Player.Block, srv.combat.Allies[0].Block)
	}

	srv.handleCoopCombatActionLocked(1, "Guest")
	if srv.combat.Player.Block != 3 || srv.combat.Allies[0].Block != 3 {
		t.Fatalf("expected linked bracers to grant both allies 3 block, got %d/%d", srv.combat.Player.Block, srv.combat.Allies[0].Block)
	}
	if !srv.combat.Coop.TeamComboDone {
		t.Fatal("expected team combo to be marked done")
	}
	if len(srv.combat.Log) == 0 || !strings.Contains(srv.combat.Log[len(srv.combat.Log)-1].Text, "协作连携触发") {
		t.Fatalf("expected combat log combo line, got %#v", srv.combat.Log)
	}
}

func TestCombatActionLogIncludesSeatAndPlayerName(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "Guest", Connected: true}
	srv.combat = &engine.CombatState{
		Player: engine.CombatActor{Name: "Host", HP: 50, MaxHP: 50, Statuses: map[string]engine.Status{}},
		Allies: []engine.CombatActor{{Name: "Guest", HP: 40, MaxHP: 40, Statuses: map[string]engine.Status{}}},
		Enemies: []engine.CombatEnemy{
			{CombatActor: engine.CombatActor{Name: "Dummy", HP: 20, MaxHP: 20, Statuses: map[string]engine.Status{}}},
		},
		Turn: 2,
	}

	srv.appendCombatActionLogLocked("p1", "打出 掩护阵型", engine.CombatTarget{Kind: engine.CombatTargetAlly, Index: 1})
	if len(srv.combat.Log) != 1 {
		t.Fatalf("expected one combat log entry, got %d", len(srv.combat.Log))
	}
	if !strings.Contains(srv.combat.Log[0].Text, "Seat 2 Guest 打出 掩护阵型") {
		t.Fatalf("expected seat/name action log, got %q", srv.combat.Log[0].Text)
	}
	if !strings.Contains(srv.combat.Log[0].Text, "目标") {
		t.Fatalf("expected target description in action log, got %q", srv.combat.Log[0].Text)
	}
}

func TestHandleCoopCombatActionCanTriggerDrawAndHealRelics(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Name: "Host", ClassID: "vanguard", Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Name: "Guest", ClassID: "arcanist", Connected: true}
	srv.run = &engine.RunState{
		Mode:      engine.ModeStory,
		Seed:      7,
		PartySize: 2,
		Player: engine.PlayerState{
			Name:      "Host",
			HP:        74,
			MaxHP:     80,
			MaxEnergy: 3,
			Relics:    []string{"battlefield_manual", "relay_rations"},
			Deck:      []engine.DeckCard{{CardID: "slash"}},
		},
	}
	srv.combat = &engine.CombatState{
		Player: engine.CombatActor{Name: "Host", HP: 74, MaxHP: 80, Energy: 3, MaxEnergy: 3, Statuses: map[string]engine.Status{}},
		Allies: []engine.CombatActor{
			{Name: "Guest", HP: 61, MaxHP: 70, Statuses: map[string]engine.Status{}},
		},
		Enemies: []engine.CombatEnemy{
			{
				CombatActor: engine.CombatActor{Name: "Dummy", HP: 40, MaxHP: 40, Statuses: map[string]engine.Status{}},
				IntentCycle: []content.EnemyIntentDef{{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}}},
			},
		},
		DrawPile: []engine.RuntimeCard{{ID: "slash"}},
		Turn:     1,
		Coop:     engine.CombatCoopState{Enabled: true, EndTurnVotes: make([]bool, 2)},
	}

	srv.handleCoopCombatActionLocked(0, "Host")
	if len(srv.combat.Hand) != 0 || srv.combat.Player.HP != 74 || srv.combat.Allies[0].HP != 61 {
		t.Fatalf("expected no trigger after first actor, got hand=%d hp=%d ally=%d", len(srv.combat.Hand), srv.combat.Player.HP, srv.combat.Allies[0].HP)
	}

	srv.handleCoopCombatActionLocked(1, "Guest")
	if len(srv.combat.Hand) != 1 {
		t.Fatalf("expected battlefield manual to draw 1, got hand=%d", len(srv.combat.Hand))
	}
	if srv.combat.Player.HP != 76 || srv.combat.Allies[0].HP != 63 {
		t.Fatalf("expected relay rations to heal team, got hp=%d ally=%d", srv.combat.Player.HP, srv.combat.Allies[0].HP)
	}
	if !srv.combat.Coop.TeamComboDone {
		t.Fatal("expected team combo to be marked done")
	}
}

func TestRenderRoomHeaderMarksUnreadChatAndYouSeat(t *testing.T) {
	snapshot := &roomSnapshot{
		SelfID:           "p1",
		Seat:             2,
		HostID:           "p0",
		RoomAddr:         "127.0.0.1:7777",
		Phase:            phaseCombat,
		PhaseTitle:       "Shared Combat",
		ControlLabel:     "Shared combat control",
		Players:          []roomPlayer{{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true}, {ID: "p1", Seat: 2, Name: "Guest", ClassID: "arcanist", Connected: true}},
		ChatLog:          []string{"10:00 Host: ready", "10:01 Guest: ok", "10:02 Host: focus left"},
		ChatUnread:       2,
		ChatUnreadInView: 2,
		Combat: &combatSnapshot{
			Party: []actorSnapshot{
				{Index: 1, Name: "Host", HP: 50, MaxHP: 50, Block: 0},
				{Index: 2, Name: "Guest", HP: 40, MaxHP: 40, Block: 6},
			},
		},
		SeatStatus: []string{"Seat 1 Host [vanguard] [host]: ready", "Seat 2 Guest [arcanist] [you]: acting"},
	}

	out := captureStdout(t, func() {
		renderRoomHeader("Shared Combat", snapshot)
		renderActorLines(snapshot, snapshot.Combat.Party)
	})
	if !strings.Contains(out, "-- Guidance --") || !strings.Contains(out, "-- Activity --") || !strings.Contains(out, "-- Chat --") || !strings.Contains(out, "-- Client --") {
		t.Fatalf("expected header section dividers, got %q", out)
	}
	if !strings.Contains(out, "Unread chat: 2") {
		t.Fatalf("expected unread chat line, got %q", out)
	}
	if !strings.Contains(out, "* 10:01 Chat | Guest: ok") || !strings.Contains(out, "* 10:02 Chat | Host: focus left") {
		t.Fatalf("expected unread chat markers, got %q", out)
	}
	if !strings.Contains(out, "Seat 2 Guest [arcanist] [you]: acting") {
		t.Fatalf("expected seat status lines in header, got %q", out)
	}
	if !strings.Contains(out, "Host [host]") || !strings.Contains(out, "Guest [you]") {
		t.Fatalf("expected actor markers for host/you, got %q", out)
	}
}

func TestBuildSnapshotsMarkCoopContent(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0", "p1"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true}
	srv.players["p1"] = &roomPlayer{ID: "p1", Seat: 2, Name: "Guest", ClassID: "arcanist", Connected: true}
	srv.run = &engine.RunState{
		Mode:      engine.ModeStory,
		Seed:      7,
		PartySize: 2,
		Player: engine.PlayerState{
			Name:      "Host",
			MaxHP:     60,
			HP:        60,
			MaxEnergy: 3,
			Gold:      120,
			Deck:      []engine.DeckCard{{CardID: "slash"}},
		},
	}

	shop := engine.StartShop(lib, srv.run)
	srv.shopState = &shop
	shopSnap := srv.buildShopSnapshotLocked()
	foundService := false
	for _, offer := range shopSnap.Offers {
		if offer.Kind == "service" {
			foundService = true
			if !containsString(offer.Badges, "SERVICE") || !containsString(offer.Badges, "CO-OP") {
				t.Fatalf("expected coop shop service badges, got %#v", offer.Badges)
			}
		}
	}
	if !foundService {
		t.Fatal("expected at least one coop service offer in multiplayer shop snapshot")
	}

	srv.eventState = &engine.EventState{Event: lib.Events["war_council"]}
	eventSnap := srv.buildEventSnapshotLocked()
	if !containsString(eventSnap.Badges, "CO-OP") {
		t.Fatalf("expected coop event badge, got %#v", eventSnap.Badges)
	}
	if len(eventSnap.Choices) == 0 || !containsString(eventSnap.Choices[0].Badges, "CO-OP") {
		t.Fatalf("expected coop event choice badge, got %#v", eventSnap.Choices)
	}

	srv.reward = &engine.RewardState{
		Gold:        20,
		CardChoices: []content.CardDef{lib.Cards["pack_tactics"]},
		RelicID:     "linked_bracers",
	}
	rewardSnap := srv.buildRewardSnapshotLocked()
	if !containsString(rewardSnap.Cards[0].Badges, "CO-OP") {
		t.Fatalf("expected coop reward card badge, got %#v", rewardSnap.Cards[0].Badges)
	}
	if !containsString(rewardSnap.RelicBadges, "CO-OP") {
		t.Fatalf("expected coop reward relic badge, got %#v", rewardSnap.RelicBadges)
	}
}

func TestRenderShopAndEventSnapshotsShowCoopLabels(t *testing.T) {
	shopSnapshot := &roomSnapshot{
		SelfID:     "p0",
		HostID:     "p0",
		RoomAddr:   "127.0.0.1:7777",
		Phase:      phaseShop,
		PhaseTitle: "Shop",
		Players:    []roomPlayer{{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true}},
		Shop: &shopSnapshot{
			Gold: 120,
			Highlights: []string{
				"2 co-op offer(s) are available in this shop.",
				"1 service option(s) are available for shared team setup.",
			},
			Offers: []shopOfferSnapshot{
				{Index: 1, Category: "Support", Name: "Heal", Price: 24},
				{Index: 2, Category: "Services", Name: "Combo Drill", Price: 72, Badges: []string{"SERVICE", "CO-OP"}, Description: "Upgrade a card and heal 10."},
				{Index: 3, Category: "Cards", Name: "Pack Tactics", Price: 52, Badges: []string{"CO-OP"}, Description: "Gain team momentum."},
			},
		},
	}
	shopOut := captureStdout(t, func() {
		renderShopSnapshot(shopSnapshot)
	})
	if !strings.Contains(shopOut, "-- Room Focus --") || !strings.Contains(shopOut, "co-op offer(s) are available") {
		t.Fatalf("expected shop room-focus summary, got %q", shopOut)
	}
	if !strings.Contains(shopOut, "== Services ==") || !strings.Contains(shopOut, "Combo Drill [SERVICE][CO-OP]") {
		t.Fatalf("expected service grouping and coop badge, got %q", shopOut)
	}
	if !strings.Contains(shopOut, "== Cards ==") || !strings.Contains(shopOut, "Pack Tactics [CO-OP]") {
		t.Fatalf("expected card grouping and coop badge, got %q", shopOut)
	}

	eventSnapshot := &roomSnapshot{
		SelfID:     "p0",
		HostID:     "p0",
		RoomAddr:   "127.0.0.1:7777",
		Phase:      phaseEvent,
		PhaseTitle: "Event",
		Players:    []roomPlayer{{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true}},
		Event: &eventSnapshot{
			Name:        "War Council",
			Description: "A shared strategy table is waiting.",
			Badges:      []string{"CO-OP"},
			Highlights: []string{
				"This event is multiplayer-only.",
				"1 choice(s) here lead into co-op-only rewards.",
			},
			Choices: []choiceSnapshot{
				{Index: 1, Label: "Share Plan", Description: "Gain Pack Tactics.", Badges: []string{"CO-OP"}},
			},
		},
	}
	eventOut := captureStdout(t, func() {
		renderEventSnapshot(eventSnapshot)
	})
	if !strings.Contains(eventOut, "-- Room Focus --") || !strings.Contains(eventOut, "This event is multiplayer-only.") {
		t.Fatalf("expected event room-focus summary, got %q", eventOut)
	}
	if !strings.Contains(eventOut, "War Council [CO-OP]") || !strings.Contains(eventOut, "Share Plan [CO-OP]") {
		t.Fatalf("expected coop labels in event render, got %q", eventOut)
	}
}

func TestRenderRewardSnapshotShowsCoopSummary(t *testing.T) {
	snapshot := &roomSnapshot{
		SelfID:     "p0",
		HostID:     "p0",
		RoomAddr:   "127.0.0.1:7777",
		Phase:      phaseReward,
		PhaseTitle: "Reward",
		Players:    []roomPlayer{{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true}},
		Reward: &rewardSnapshot{
			Gold:        40,
			Source:      "elite",
			Relic:       "Linked Bracers",
			RelicBadges: []string{"CO-OP"},
			Highlights:  []string{"Reward pool contains 1 co-op card choice(s).", "Reward relic is multiplayer-only."},
			Cards: []cardSnapshot{
				{Index: 1, Name: "Pack Tactics", Summary: "Gain team momentum.", Badges: []string{"CO-OP"}},
			},
		},
	}
	out := captureStdout(t, func() {
		renderRewardSnapshot(snapshot)
	})
	if !strings.Contains(out, "-- Room Focus --") || !strings.Contains(out, "Reward relic is multiplayer-only.") {
		t.Fatalf("expected reward room-focus summary, got %q", out)
	}
	if !strings.Contains(out, "Linked Bracers [CO-OP]") || !strings.Contains(out, "Pack Tactics [CO-OP]") {
		t.Fatalf("expected reward coop badges, got %q", out)
	}
}

func TestRoomLogFeedbackMarksCoopOutcomes(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	srv, err := newServer(lib, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	defer func() {
		_ = srv.listener.Close()
	}()

	srv.hostID = "p0"
	srv.order = []string{"p0"}
	srv.players["p0"] = &roomPlayer{ID: "p0", Seat: 1, Name: "Host", ClassID: "vanguard", Connected: true}
	srv.run = &engine.RunState{
		Mode:      engine.ModeStory,
		Seed:      7,
		Act:       1,
		PartySize: 2,
		Status:    engine.RunStatusActive,
		Player: engine.PlayerState{
			ClassID:        "vanguard",
			Name:           "Host",
			HP:             60,
			MaxHP:          60,
			MaxEnergy:      3,
			Gold:           200,
			Deck:           []engine.DeckCard{{CardID: "slash"}},
			PotionCapacity: 2,
		},
	}
	srv.partyMembers = []engine.PlayerState{{Name: "Host", HP: 60, MaxHP: 60, MaxEnergy: 3}}

	srv.reward = &engine.RewardState{CardChoices: []content.CardDef{lib.Cards["pack_tactics"]}, RelicID: "linked_bracers"}
	srv.combat = &engine.CombatState{Player: engine.CombatActor{HP: 60, MaxHP: 60}, Reward: *srv.reward}
	srv.currentNode = engine.Node{ID: "A1F1M1", Act: 1, Floor: 1, Kind: engine.NodeMonster, Edges: []string{"A1F2M1"}}
	if err := srv.applyRewardLocked("pack_tactics", false); err != nil {
		t.Fatalf("applyRewardLocked() error = %v", err)
	}
	joinedRewardLog := strings.Join(srv.roomLog, "\n")
	if !strings.Contains(joinedRewardLog, "Picked CO-OP reward: "+lib.Cards["pack_tactics"].Name) {
		t.Fatalf("expected picked coop reward log, got %q", joinedRewardLog)
	}
	if !strings.Contains(joinedRewardLog, "Reward included CO-OP relic: "+lib.Relics["linked_bracers"].Name) {
		t.Fatalf("expected coop relic reward log, got %q", joinedRewardLog)
	}

	srv.phase = phaseShop
	srv.shopState = &engine.ShopState{Offers: []engine.ShopOffer{{
		ID:          "service-coop-card",
		Kind:        "service",
		Name:        "协同简报",
		Description: "获得一张协作牌。",
		Price:       20,
		ItemID:      "service_coop_card",
	}}}
	srv.handleShopCommandLocked("p0", commandPayload{Action: "buy", ItemIndex: 1})
	joinedShopLog := strings.Join(srv.roomLog, "\n")
	if !strings.Contains(joinedShopLog, "Shop outcome: [CO-OP] 参加协同简报") {
		t.Fatalf("expected coop service outcome log, got %q", joinedShopLog)
	}
	if !strings.Contains(joinedShopLog, "Purchased CO-OP offer: 协同简报.") {
		t.Fatalf("expected coop service summary log, got %q", joinedShopLog)
	}

	srv.phase = phaseEvent
	srv.currentNode = engine.Node{ID: "A1F2E1", Act: 1, Floor: 2, Kind: engine.NodeEvent, Edges: []string{"A1F3M1"}}
	srv.eventState = &engine.EventState{Event: lib.Events["war_council"]}
	srv.handleEventCommandLocked("p0", commandPayload{Action: "choose", ItemIndex: 1})
	joinedEventLog := strings.Join(srv.roomLog, "\n")
	if !strings.Contains(joinedEventLog, "Event outcome: [CO-OP] 获得卡牌 "+lib.Cards["pack_tactics"].Name) {
		t.Fatalf("expected coop event outcome log, got %q", joinedEventLog)
	}
	if !strings.Contains(joinedEventLog, "Event granted CO-OP card: "+lib.Cards["pack_tactics"].Name) {
		t.Fatalf("expected coop event summary log, got %q", joinedEventLog)
	}
	if !hasFormattedChannelPrefix(srv.roomLog[len(srv.roomLog)-1]) {
		t.Fatalf("expected formatted room log prefix, got %q", srv.roomLog[len(srv.roomLog)-1])
	}
}

func mustConnectClient(t *testing.T, addr, name, classID string) *testClient {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial(%q) error = %v", addr, err)
	}
	client := &testClient{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}
	if err := client.enc.Encode(message{
		Type:  "hello",
		Hello: &helloPayload{Name: name, ClassID: classID},
	}); err != nil {
		_ = conn.Close()
		t.Fatalf("hello encode error = %v", err)
	}
	return client
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()

	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(data)
}

func (c *testClient) sendCommand(t *testing.T, cmd commandPayload) {
	t.Helper()

	if err := c.enc.Encode(message{Type: "command", Command: &cmd}); err != nil {
		t.Fatalf("command encode error = %v", err)
	}
}

func (c *testClient) waitForSnapshot(t *testing.T, match func(*roomSnapshot) bool) *roomSnapshot {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.conn.SetReadDeadline(time.Now().Add(750 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline() error = %v", err)
		}
		var msg message
		if err := c.dec.Decode(&msg); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			t.Fatalf("Decode() error = %v", err)
		}
		if msg.Type == "error" {
			t.Fatalf("server error: %s", msg.Error)
		}
		if msg.Type == "snapshot" && msg.Snapshot != nil && match(msg.Snapshot) {
			return msg.Snapshot
		}
	}
	t.Fatalf("timed out waiting for matching snapshot")
	return nil
}

func (c *testClient) close() {
	if c == nil || c.conn == nil {
		return
	}
	_ = c.conn.Close()
}

func playerReady(players []roomPlayer, name string) bool {
	for _, player := range players {
		if player.Name == name {
			return player.Ready
		}
	}
	return false
}

func playerConnected(players []roomPlayer, name string) bool {
	for _, player := range players {
		if player.Name == name {
			return player.Connected
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
