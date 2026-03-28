package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/netplay"
	"cmdcards/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
)

func startCommandCaptureSession(t *testing.T) (*netplay.Session, <-chan map[string]any) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	commands := make(chan map[string]any, 8)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			close(commands)
			return
		}
		defer conn.Close()

		dec := json.NewDecoder(conn)
		var hello map[string]any
		if err := dec.Decode(&hello); err != nil {
			close(commands)
			return
		}
		for {
			var msg map[string]any
			if err := dec.Decode(&msg); err != nil {
				close(commands)
				return
			}
			if msg["type"] != "command" {
				continue
			}
			command, _ := msg["command"].(map[string]any)
			commands <- command
		}
	}()

	session, err := netplay.StartJoinedSession(listener.Addr().String(), "Tester", "vanguard")
	if err != nil {
		_ = listener.Close()
		t.Fatalf("StartJoinedSession() error = %v", err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		_ = listener.Close()
		<-serverDone
	})
	return session, commands
}

func mustReceiveCommand(t *testing.T, commands <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case command := <-commands:
		return command
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected captured multiplayer command")
		return nil
	}
}

func ensureNoSecondCommand(t *testing.T, commands <-chan map[string]any) {
	t.Helper()
	select {
	case command := <-commands:
		t.Fatalf("expected no second multiplayer command, got %#v", command)
	case <-time.After(120 * time.Millisecond):
	}
}

func TestModelCanStartStoryRun(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	m := newModel(lib, store, engine.DefaultProfile(lib), nil)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if m1.screen != screenClass {
		t.Fatalf("expected screenClass after selecting story, got %s", m1.screen)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(model)
	if m2.screen != screenMap {
		t.Fatalf("expected screenMap after selecting class, got %s", m2.screen)
	}
	if m2.run == nil {
		t.Fatal("expected run to be created")
	}
	if len(engine.ReachableNodes(m2.run)) == 0 {
		t.Fatal("expected run to have reachable nodes")
	}

	loadedRun, err := store.LoadRun()
	if err != nil {
		t.Fatalf("LoadRun() error = %v", err)
	}
	if loadedRun == nil || loadedRun.Checkpoint == nil {
		t.Fatalf("expected active run checkpoint to be persisted, got %#v", loadedRun)
	}
	if loadedRun.Checkpoint.Screen != string(screenMap) {
		t.Fatalf("expected checkpoint screen %q, got %q", screenMap, loadedRun.Checkpoint.Screen)
	}
}

func TestModelCanContinueFromCheckpoint(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Checkpoint = &engine.RunCheckpoint{
		Screen: string(screenShop),
		CurrentNode: &engine.Node{
			ID:    "shop-node",
			Act:   1,
			Floor: 2,
			Index: 0,
			Kind:  engine.NodeShop,
		},
		ShopState: &engine.ShopState{
			Offers: []engine.ShopOffer{{ID: "remove-card", Kind: "remove", Price: 75}},
		},
	}

	m := newModel(lib, store, engine.DefaultProfile(lib), run)
	if len(m.menuItems) < 2 || m.menuItems[0] != menuContinue || m.menuItems[1] != menuAbandon {
		t.Fatalf("expected continue/abandon menu items, got %#v", m.menuItems)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if m1.screen != screenShop {
		t.Fatalf("expected checkpoint screen %q, got %q", screenShop, m1.screen)
	}
	if m1.shopState == nil || len(m1.shopState.Offers) != 1 {
		t.Fatalf("expected shop state to be restored, got %#v", m1.shopState)
	}
	if m1.currentNode.Kind != engine.NodeShop {
		t.Fatalf("expected current node to be restored, got %#v", m1.currentNode)
	}
}

func TestModelCanContinueDeckActionCheckpoint(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Checkpoint = &engine.RunCheckpoint{
		Screen:             string(screenDeckAct),
		CurrentNode:        &engine.Node{ID: "event-node", Act: 1, Floor: 2, Index: 0, Kind: engine.NodeEvent},
		EventState:         &engine.EventState{Event: lib.Events["bulwark_blueprint"]},
		EventChoice:        "opening_spark",
		DeckActionMode:     "event_augment_card",
		DeckActionTitle:    "Bulwark Blueprint",
		DeckActionSubtitle: "Choose a card to gain Opening Spark for 1 turn.",
		DeckActionIndexes:  []int{0, 1},
		DeckActionEffect: &content.Effect{
			Op:       "augment_card",
			Name:     "opening_spark",
			Scope:    "turn",
			Selector: "choose_upgradable",
			Effects: []content.Effect{
				{Op: "draw", Value: 1},
			},
		},
		DeckActionTakeEquip: true,
	}

	m := newModel(lib, store, engine.DefaultProfile(lib), run)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if m1.screen != screenDeckAct {
		t.Fatalf("expected checkpoint screen %q, got %q", screenDeckAct, m1.screen)
	}
	if m1.eventState == nil || m1.eventState.Event.ID != "bulwark_blueprint" {
		t.Fatalf("expected event state to be restored, got %#v", m1.eventState)
	}
	if m1.deckActionMode != "event_augment_card" || len(m1.deckActionIndexes) != 2 {
		t.Fatalf("expected deck action state to be restored, got mode=%q indexes=%v", m1.deckActionMode, m1.deckActionIndexes)
	}
	if m1.deckActionEffect == nil || m1.deckActionEffect.Scope != "turn" || len(m1.deckActionEffect.Effects) != 1 {
		t.Fatalf("expected deck action effect to be restored, got %#v", m1.deckActionEffect)
	}
	if !m1.deckActionTakeEquip {
		t.Fatalf("expected deck action take-equipment flag to be restored")
	}
}

func TestMenuCanOpenCodexAndProgression(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	m := newModel(lib, store, engine.DefaultProfile(lib), nil)

	if !hasMenuItem(m.menuItems, menuCodexLabel) || !hasMenuItem(m.menuItems, menuProgressionLabel) {
		t.Fatalf("expected codex and progression menu items, got %#v", m.menuItems)
	}
	if !hasMenuItem(m.menuItems, menuMultiplayer) {
		t.Fatalf("expected multiplayer menu item, got %#v", m.menuItems)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(model).screen != screenCodex {
		t.Fatalf("expected screenCodex, got %q", next.(model).screen)
	}

	m = newModel(lib, store, engine.DefaultProfile(lib), nil)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(model).screen != screenProfile {
		t.Fatalf("expected screenProfile, got %q", next.(model).screen)
	}
}

func TestMenuCanOpenMultiplayerAndLaunchHost(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	m := newModel(lib, store, engine.DefaultProfile(lib), nil)

	for i, item := range m.menuItems {
		if item == menuMultiplayer {
			m.index = i
			break
		}
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if m1.screen != screenMultiplayerMenu {
		t.Fatalf("expected multiplayer menu screen, got %q", m1.screen)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(model)
	if m2.screen != screenMultiplayerCreate {
		t.Fatalf("expected multiplayer create screen, got %q", m2.screen)
	}

	m2.index = multiplayerCreateFieldLaunch
	next, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := next.(model)
	if !m3.multiplayerConnecting {
		t.Fatalf("expected host launch to enter connecting state")
	}
	if cmd == nil {
		t.Fatal("expected async command when launching host flow")
	}
}

func TestMultiplayerCreateVisibleLaunchAndBackMatchActions(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerCreate
	m.index = 0
	m.setMultiplayerCreateFocus()

	for i := 0; i < 4; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(model)
	}
	if got := multiplayerCreateLines(m)[m.index]; got != m.theme.Text("multiplayer.create.launch") {
		t.Fatalf("expected visible launch item at index %d, got %q", m.index, got)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if !m1.multiplayerConnecting {
		t.Fatalf("expected launch item to start connecting")
	}
	if cmd == nil {
		t.Fatal("expected async command when selecting visible launch item")
	}

	m = newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerCreate
	m.index = 0
	m.setMultiplayerCreateFocus()
	for i := 0; i < 5; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(model)
	}
	if got := multiplayerCreateLines(m)[m.index]; got != m.theme.Text("multiplayer.back") {
		t.Fatalf("expected visible back item at index %d, got %q", m.index, got)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(model)
	if m2.screen != screenMultiplayerMenu {
		t.Fatalf("expected visible back item to return to multiplayer menu, got %q", m2.screen)
	}
}

func TestProfileSettingsCanSwitchLanguage(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	m := newModel(lib, store, engine.DefaultProfile(lib), nil)
	m.screen = screenProfile
	m.profileTab = profileTabSettings
	m.index = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if m1.profile.Language != "en-US" {
		t.Fatalf("expected profile language to switch to en-US, got %q", m1.profile.Language)
	}
	if m1.theme.Lang != "en-US" {
		t.Fatalf("expected theme language to switch to en-US, got %q", m1.theme.Lang)
	}
	if got := multiplayerCreateLines(m1)[0]; got == "" || got == "你的名字: <本地显示名>" {
		t.Fatalf("expected localized multiplayer create line after language switch, got %q", got)
	}
}

func TestMenuCanOpenMultiplayerAndLaunchJoin(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	m := newModel(lib, store, engine.DefaultProfile(lib), nil)
	for i, item := range m.menuItems {
		if item == menuMultiplayer {
			m.index = i
			break
		}
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := next.(model)
	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := next.(model)
	if m3.screen != screenMultiplayerJoin {
		t.Fatalf("expected multiplayer join screen, got %q", m3.screen)
	}

	m3.index = multiplayerJoinFieldLaunch
	next, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m4 := next.(model)
	if !m4.multiplayerConnecting {
		t.Fatalf("expected join launch to enter connecting state")
	}
	if cmd == nil {
		t.Fatal("expected async command when launching join flow")
	}
}

func TestCombatDigitKeysSwitchInspectPane(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), run)
	m.screen = screenCombat
	m.combat = &engine.CombatState{}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m1 := next.(model)
	if m1.combatPane != 2 {
		t.Fatalf("expected combat pane 2 after pressing 3, got %d", m1.combatPane)
	}
	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	m2 := next.(model)
	if m2.combatPane != 5 {
		t.Fatalf("expected combat pane 5 after pressing 6, got %d", m2.combatPane)
	}
}

func TestCombatLogPageKeysAdvanceAndRewind(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), run)
	m.screen = screenCombat
	m.combat = &engine.CombatState{}
	m.combatTopPage = 2

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	m1 := next.(model)
	if m1.combatLogPage != 1 {
		t.Fatalf("expected comma to advance combat log page, got %d", m1.combatLogPage)
	}
	if m1.combatTopPage != 2 {
		t.Fatalf("expected comma to leave combat top page unchanged, got %d", m1.combatTopPage)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	m2 := next.(model)
	if m2.combatLogPage != 0 {
		t.Fatalf("expected period to rewind combat log page, got %d", m2.combatLogPage)
	}

	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	m3 := next.(model)
	if m3.combatLogPage != 0 {
		t.Fatalf("expected period at first page to stay pinned at zero, got %d", m3.combatLogPage)
	}
}

func TestMultiplayerInspectLogPageKeysAdvanceAndRewind(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerRoom
	var snapshot netplay.Snapshot
	logs := make([]string, 0, 12)
	for i := 1; i <= 12; i++ {
		logs = append(logs, fmt.Sprintf("T%d entry-%02d", i, i))
	}
	jsonBlob, err := json.Marshal(map[string]any{
		"phase": "combat",
		"combat": map[string]any{
			"logs": logs,
			"party": []map[string]any{{
				"index":  1,
				"name":   "Host",
				"hp":     50,
				"max_hp": 50,
			}},
			"enemies": []map[string]any{{
				"index":  1,
				"name":   "Slime",
				"hp":     18,
				"max_hp": 24,
				"intent": "Attack",
			}},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	m.multiplayerSnapshot = &snapshot
	m.multiplayerRoomFocus = multiplayerRoomFocusInspect
	m.multiplayerInspectPane = 3
	m.multiplayerCombatTopPage = 2

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	m1 := next.(model)
	if m1.multiplayerInspectLogPage != 1 {
		t.Fatalf("expected comma to advance multiplayer inspect log page, got %d", m1.multiplayerInspectLogPage)
	}
	if m1.multiplayerCombatTopPage != 2 {
		t.Fatalf("expected comma to leave multiplayer top page unchanged, got %d", m1.multiplayerCombatTopPage)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	m2 := next.(model)
	if m2.multiplayerInspectLogPage != 0 {
		t.Fatalf("expected period to rewind multiplayer inspect log page, got %d", m2.multiplayerInspectLogPage)
	}

	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	m3 := next.(model)
	if m3.multiplayerInspectLogPage != 0 {
		t.Fatalf("expected period at first page to stay pinned at zero, got %d", m3.multiplayerInspectLogPage)
	}
}

func TestCombatSubmitThrottlePreventsRapidRepeat(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	combat, err := engine.StartEncounter(lib, run, engine.Node{ID: "n1", Kind: engine.NodeMonster, Act: 1, Floor: 1, Index: 0})
	if err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}
	engine.StartPlayerTurn(lib, run.Player, combat)
	if len(combat.Hand) == 0 {
		t.Fatal("expected opening hand for throttle test")
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), run)
	m.screen = screenCombat
	m.combat = combat
	fixed := time.Unix(1700000000, 0)
	m.now = func() time.Time { return fixed }
	m.syncCombatTarget()

	before := len(m.combat.Hand)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if got := len(m1.combat.Hand); got != before-1 {
		t.Fatalf("expected first enter to play one card, hand %d -> %d", before, got)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(model)
	if got := len(m2.combat.Hand); got != len(m1.combat.Hand) {
		t.Fatalf("expected rapid second enter to be throttled, hand stayed %d but got %d", len(m1.combat.Hand), got)
	}
	if !strings.Contains(m2.message, "slow down") {
		t.Fatalf("expected throttle message, got %q", m2.message)
	}
}

func TestMapOverlayCanToggleDuringRun(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), run)
	m.screen = screenCombat
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m1 := next.(model)
	if !m1.showMapOverlay {
		t.Fatalf("expected map overlay to open on m")
	}
	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := next.(model)
	if m2.showMapOverlay {
		t.Fatalf("expected map overlay to close on esc")
	}
}

func TestMapOverlayCanToggleInMultiplayerMapPhase(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerRoom
	var snapshot netplay.Snapshot
	if err := json.Unmarshal([]byte(`{"phase":"map","map":{"act":1,"next_floor":2}}`), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal(map) error = %v", err)
	}
	m.multiplayerSnapshot = &snapshot
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m1 := next.(model)
	if !m1.showMapOverlay {
		t.Fatalf("expected multiplayer map overlay to open on m during map phase")
	}
}

func TestMapOverlayCanToggleInMultiplayerCombatWithSharedMap(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerRoom
	var snapshot netplay.Snapshot
	if err := json.Unmarshal([]byte(`{"phase":"combat","shared_map":{"act":1,"next_floor":3,"current_floor":2,"current_node_id":"n2"}}`), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal(shared_map) error = %v", err)
	}
	m.multiplayerSnapshot = &snapshot
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m1 := next.(model)
	if !m1.showMapOverlay {
		t.Fatalf("expected multiplayer map overlay to open on m when only shared_map is present")
	}
}

func TestStatsOverlayCanToggleDuringRun(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), run)
	m.screen = screenCombat
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m1 := next.(model)
	if !m1.showStatsOverlay {
		t.Fatalf("expected stats overlay to open on K")
	}
	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := next.(model)
	if m2.showStatsOverlay {
		t.Fatalf("expected stats overlay to close on esc")
	}
}

func TestStatsOverlayCanToggleInMultiplayerRoom(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerRoom
	var snapshot netplay.Snapshot
	if err := json.Unmarshal([]byte(`{"phase":"combat","stats":{"seat_name":"Guest","combat_turns":3,"combat":{"damage_dealt":12},"run_turns":8,"run":{"damage_taken":5}}}`), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal(stats) error = %v", err)
	}
	m.multiplayerSnapshot = &snapshot
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m1 := next.(model)
	if !m1.showStatsOverlay {
		t.Fatalf("expected multiplayer stats overlay to open on K")
	}
}

func TestMultiplayerRoomCanToggleActionFocus(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerRoom
	m.multiplayerSnapshot = &netplay.Snapshot{Examples: []string{"ready"}}
	m.syncMultiplayerRoomSelection()

	if m.multiplayerRoomFocus != multiplayerRoomFocusActions {
		t.Fatalf("expected room to default to quick actions focus, got %d", m.multiplayerRoomFocus)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := next.(model)
	if m1.multiplayerRoomFocus != multiplayerRoomFocusInput {
		t.Fatalf("expected tab to move focus to input, got %d", m1.multiplayerRoomFocus)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := next.(model)
	if m2.multiplayerRoomFocus != multiplayerRoomFocusActions {
		t.Fatalf("expected second tab to move focus back to quick actions, got %d", m2.multiplayerRoomFocus)
	}
}

func TestMultiplayerRoomTemplateActionPrefillsInput(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerRoom
	m.multiplayerSnapshot = &netplay.Snapshot{Commands: []string{"node <index>"}}
	m.syncMultiplayerRoomSelection()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if got := m1.multiplayerCommandInput.Value(); got != "node <index>" {
		t.Fatalf("expected template action to prefill input, got %q", got)
	}
	if m1.multiplayerRoomFocus != multiplayerRoomFocusInput {
		t.Fatalf("expected template action to move focus to input, got %d", m1.multiplayerRoomFocus)
	}
}

func TestMultiplayerRoomQuitActionReturnsMenu(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenMultiplayerRoom
	m.multiplayerSnapshot = &netplay.Snapshot{Commands: []string{"quit"}}
	m.syncMultiplayerRoomSelection()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if m1.screen != screenMultiplayerMenu {
		t.Fatalf("expected quit action to return to multiplayer menu, got %q", m1.screen)
	}
	if m1.multiplayerSnapshot != nil {
		t.Fatalf("expected quit action to clear snapshot, got %#v", m1.multiplayerSnapshot)
	}
}

func TestMultiplayerQuickActionsUseReadableLabels(t *testing.T) {
	snapshot := &netplay.Snapshot{
		Phase:    "lobby",
		SelfID:   "seat-1",
		HostID:   "seat-1",
		Examples: []string{"ready", "start", "chat hello", "quit"},
	}

	actions := multiplayerQuickActions(snapshot)
	labels := multiplayerQuickActionLabels(actions)
	joined := strings.Join(labels, " | ")
	if !strings.Contains(joined, "Mark yourself ready") {
		t.Fatalf("expected ready label in %q", joined)
	}
	if !strings.Contains(joined, "Host only: Start this run") {
		t.Fatalf("expected start label in %q", joined)
	}
	if !strings.Contains(joined, "Send chat: hello") {
		t.Fatalf("expected chat template label in %q", joined)
	}
	if !strings.Contains(joined, "Leave the current room") {
		t.Fatalf("expected quit label in %q", joined)
	}
}

func TestMultiplayerQuickActionsDescribeMapNode(t *testing.T) {
	snapshot := &netplay.Snapshot{
		Phase:    "map",
		SelfID:   "seat-1",
		HostID:   "seat-1",
		Examples: []string{"node 1"},
	}

	actions := multiplayerQuickActions(snapshot)
	if len(actions) != 1 {
		t.Fatalf("expected one quick action, got %d", len(actions))
	}
	if got := actions[0].Label; got != "Host only: Go to node 1" {
		t.Fatalf("expected map node label, got %q", got)
	}
	if actions[0].Command != "node 1" {
		t.Fatalf("expected underlying command to stay raw, got %q", actions[0].Command)
	}
}

func TestMultiplayerQuickActionsMarkHostOnlyCommands(t *testing.T) {
	nonHost := &netplay.Snapshot{Phase: "map", Commands: []string{"node <index>"}, SelfID: "seat-2", HostID: "seat-1"}
	actions := multiplayerQuickActions(nonHost)
	if len(actions) != 1 {
		t.Fatalf("expected one action, got %d", len(actions))
	}
	if got := actions[0].Label; got != "Host only (locked): Choose the next map node" {
		t.Fatalf("expected non-host label, got %q", got)
	}

	host := &netplay.Snapshot{Phase: "map", Commands: []string{"node <index>"}, SelfID: "seat-1", HostID: "seat-1"}
	actions = multiplayerQuickActions(host)
	if got := actions[0].Label; got != "Host only: Choose the next map node" {
		t.Fatalf("expected host label, got %q", got)
	}
}

func TestMultiplayerQuickActionsDescribeCombatNames(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"phase":"combat",
		"examples":["play 1 enemy 1", "potion 1 enemy 1", "end"],
		"combat":{
			"turn":2,
			"energy":3,
			"max_energy":3,
			"party":[{"index":1,"name":"Host","hp":50,"max_hp":60,"block":0}],
			"enemies":[{"index":1,"name":"Slime","hp":18,"max_hp":24,"block":0,"intent":"Attack"}],
			"hand":[{"index":1,"name":"Strike","cost":1,"summary":"Deal 6 damage","target_hint":"Single enemy"}],
			"potions":["Smoke Bomb"]
		}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	actions := multiplayerQuickActions(&snapshot)
	labels := multiplayerQuickActionLabels(actions)
	joined := strings.Join(labels, " | ")
	if !strings.Contains(joined, "Play Strike -> enemy 1 Slime") {
		t.Fatalf("expected card+target label in %q", joined)
	}
	if !strings.Contains(joined, "Use Potion Smoke Bomb -> enemy 1 Slime") {
		t.Fatalf("expected potion+target label in %q", joined)
	}
	if !strings.Contains(joined, "End this turn and submit your vote") {
		t.Fatalf("expected end-turn label in %q", joined)
	}
}

func TestMultiplayerQuickActionsDescribeRewardShopAndEventNames(t *testing.T) {
	reward := &netplay.Snapshot{Phase: "reward", SelfID: "host", HostID: "host"}
	if err := json.Unmarshal([]byte(`{
		"phase":"reward",
		"self_id":"host",
		"host_id":"host",
		"examples":["take 1","skip"],
		"reward":{"gold":30,"source":"combat","cards":[{"index":1,"name":"Shield Wall","summary":"Gain 8 block"}]}
	}`), reward); err != nil {
		t.Fatalf("json.Unmarshal(reward) error = %v", err)
	}
	rewardLabels := strings.Join(multiplayerQuickActionLabels(multiplayerQuickActions(reward)), " | ")
	if !strings.Contains(rewardLabels, "Host only: Take reward card Shield Wall") {
		t.Fatalf("expected named reward label in %q", rewardLabels)
	}
	if !strings.Contains(rewardLabels, "Host only: Skip reward") {
		t.Fatalf("expected reward skip label in %q", rewardLabels)
	}

	shop := &netplay.Snapshot{Phase: "shop", SelfID: "host", HostID: "host"}
	if err := json.Unmarshal([]byte(`{
		"phase":"shop",
		"self_id":"host",
		"host_id":"host",
		"examples":["buy 2","leave"],
		"shop":{"gold":90,"offers":[{"index":2,"name":"Guard","description":"Gain 5 block","price":40}]}
	}`), shop); err != nil {
		t.Fatalf("json.Unmarshal(shop) error = %v", err)
	}
	shopLabels := strings.Join(multiplayerQuickActionLabels(multiplayerQuickActions(shop)), " | ")
	if !strings.Contains(shopLabels, "Host only: Buy Guard (40 gold)") {
		t.Fatalf("expected named shop label in %q", shopLabels)
	}
	if !strings.Contains(shopLabels, "Host only: Leave the shop") {
		t.Fatalf("expected leave shop label in %q", shopLabels)
	}

	event := &netplay.Snapshot{Phase: "event", SelfID: "host", HostID: "host"}
	if err := json.Unmarshal([]byte(`{
		"phase":"event",
		"self_id":"host",
		"host_id":"host",
		"examples":["choose 1"],
		"event":{"name":"Shrine","description":"A strange shrine.","choices":[{"index":1,"label":"Pray","description":"Gain a blessing."}]}
	}`), event); err != nil {
		t.Fatalf("json.Unmarshal(event) error = %v", err)
	}
	eventLabels := strings.Join(multiplayerQuickActionLabels(multiplayerQuickActions(event)), " | ")
	if !strings.Contains(eventLabels, "Host only: Choose event 1: Pray") {
		t.Fatalf("expected named event label in %q", eventLabels)
	}
}

func TestMultiplayerCombatBuildsCardAndPotionCommands(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"phase":"combat",
		"self_id":"seat-1",
		"host_id":"seat-1",
		"combat":{
			"turn":1,
			"energy":3,
			"max_energy":3,
			"party":[
				{"index":1,"name":"Host","hp":50,"max_hp":60,"block":0},
				{"index":2,"name":"Guest","hp":38,"max_hp":40,"block":4}
			],
			"enemies":[{"index":1,"name":"Slime","hp":18,"max_hp":24,"block":0,"intent":"Attack"}],
			"hand":[
				{"index":1,"name":"Strike","cost":1,"summary":"Deal 6 damage","target_hint":"Single enemy"},
				{"index":2,"name":"Guard Ally","cost":1,"summary":"Gain 6 block","target_hint":"Single ally"}
			],
            "potions":["Smoke Bomb"]
		}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.multiplayerSnapshot = &snapshot
	m.screen = screenMultiplayerRoom
	m.syncMultiplayerRoomSelection()

	cmd, err := m.buildSelectedMultiplayerCombatCommand()
	if err != nil {
		t.Fatalf("buildSelectedMultiplayerCombatCommand() error = %v", err)
	}
	if cmd.Action != "play" || cmd.CardIndex != 1 || cmd.TargetKind != "enemy" || cmd.TargetIndex != 1 {
		t.Fatalf("expected first card command with enemy target, got %#v", cmd)
	}

	m.multiplayerCombatIndex = 1
	m.syncMultiplayerCombatTarget()
	cmd, err = m.buildSelectedMultiplayerCombatCommand()
	if err != nil {
		t.Fatalf("buildSelectedMultiplayerCombatCommand() ally error = %v", err)
	}
	if cmd.Action != "play" || cmd.CardIndex != 2 || cmd.TargetKind != "ally" || cmd.TargetIndex != 1 {
		t.Fatalf("expected ally-targeted card command, got %#v", cmd)
	}

	m.multiplayerCombatMode = multiplayerCombatModePotion
	m.syncMultiplayerCombatTarget()
	m.multiplayerCombatTarget = multiplayerTargetState{Kind: engine.CombatTargetAlly, Index: 2}
	cmd, err = m.buildSelectedMultiplayerCombatCommand()
	if err != nil {
		t.Fatalf("buildSelectedMultiplayerCombatCommand() potion error = %v", err)
	}
	if cmd.Action != "potion" || cmd.ItemIndex != 1 || cmd.TargetKind != "ally" || cmd.TargetIndex != 2 {
		t.Fatalf("expected potion command targeting ally 2, got %#v", cmd)
	}
}

func TestMultiplayerCombatSubmitThrottlePreventsRapidRepeat(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	session, commands := startCommandCaptureSession(t)

	var snapshot netplay.Snapshot
	if err := json.Unmarshal([]byte(`{
		"phase":"combat",
		"self_id":"seat-1",
		"host_id":"seat-1",
		"combat":{
			"turn":1,
			"energy":3,
			"max_energy":3,
			"party":[{"index":1,"name":"Host","hp":50,"max_hp":60,"block":0}],
			"enemies":[{"index":1,"name":"Slime","hp":18,"max_hp":24,"block":0,"intent":"Attack"}],
			"hand":[{"index":1,"name":"Strike","cost":1,"summary":"Deal 6 damage","target_hint":"Single enemy"}]
		}
	}`), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.multiplayerSession = session
	m.multiplayerSnapshot = &snapshot
	m.screen = screenMultiplayerRoom
	m.now = func() time.Time { return time.Unix(1700000000, 0) }
	m.syncMultiplayerRoomSelection()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	command := mustReceiveCommand(t, commands)
	if command["action"] != "play" {
		t.Fatalf("expected play command, got %#v", command)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(model)
	ensureNoSecondCommand(t, commands)
	if !strings.Contains(m2.message, "操作过快") {
		t.Fatalf("expected throttle message, got %q", m2.message)
	}
}

func TestMultiplayerCombatTabSwitchesBetweenOperationsAndChat(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	var snapshot netplay.Snapshot
	if err := json.Unmarshal([]byte(`{
		"phase":"combat",
		"combat":{
			"turn":1,
			"energy":3,
			"max_energy":3,
			"party":[{"index":1,"name":"Host","hp":50,"max_hp":60,"block":0}],
			"enemies":[{"index":1,"name":"Slime","hp":18,"max_hp":24,"block":0,"intent":"Attack"}],
			"hand":[{"index":1,"name":"Strike","cost":1,"summary":"Deal 6 damage","target_hint":"Single enemy"}]
		}
	}`), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.multiplayerSnapshot = &snapshot
	m.screen = screenMultiplayerRoom
	m.syncMultiplayerRoomSelection()
	if m.multiplayerRoomFocus != multiplayerRoomFocusActions {
		t.Fatalf("expected combat room to start on operations focus, got %d", m.multiplayerRoomFocus)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := next.(model)
	if m1.multiplayerRoomFocus != multiplayerRoomFocusInspect {
		t.Fatalf("expected first tab to switch combat room to inspect focus, got %d", m1.multiplayerRoomFocus)
	}
	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := next.(model)
	if m2.multiplayerRoomFocus != multiplayerRoomFocusInput {
		t.Fatalf("expected second tab to switch combat room to chat input, got %d", m2.multiplayerRoomFocus)
	}
	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := next.(model)
	if m3.multiplayerRoomFocus != multiplayerRoomFocusActions {
		t.Fatalf("expected third tab to switch back to operations, got %d", m3.multiplayerRoomFocus)
	}
	if m3.multiplayerInspectPane != 0 {
		t.Fatalf("expected inspect pane to remain unchanged during tab cycling, got %d", m3.multiplayerInspectPane)
	}
}

func TestMultiplayerStructuredCommandsCoverRewardShopAndSummary(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)

	reward := &netplay.Snapshot{}
	if err := json.Unmarshal([]byte(`{"phase":"reward","reward":{"gold":30,"source":"combat","cards":[{"index":1,"name":"Shield Wall","summary":"Gain 8 block"}]}}`), reward); err != nil {
		t.Fatalf("json.Unmarshal(reward) error = %v", err)
	}
	m.multiplayerSnapshot = reward
	m.syncMultiplayerRoomSelection()
	cmd, err := m.buildSelectedStructuredMultiplayerCommand()
	if err != nil {
		t.Fatalf("buildSelectedStructuredMultiplayerCommand(reward) error = %v", err)
	}
	if cmd.Action != "take" || cmd.ItemIndex != 1 {
		t.Fatalf("expected reward take command, got %#v", cmd)
	}

	shop := &netplay.Snapshot{}
	if err := json.Unmarshal([]byte(`{"phase":"shop","shop":{"gold":100,"offers":[{"index":1,"kind":"card","name":"Quick Slash","description":"Deal 7 damage","price":65},{"index":2,"kind":"card","name":"Guard","description":"Gain 5 block","price":50}]}}`), shop); err != nil {
		t.Fatalf("json.Unmarshal(shop) error = %v", err)
	}
	m.multiplayerSnapshot = shop
	m.syncMultiplayerRoomSelection()
	m.multiplayerStructuredIndex = 1
	cmd, err = m.buildSelectedStructuredMultiplayerCommand()
	if err != nil {
		t.Fatalf("buildSelectedStructuredMultiplayerCommand(shop) error = %v", err)
	}
	if cmd.Action != "buy" || cmd.ItemIndex != 2 {
		t.Fatalf("expected shop buy command for second offer, got %#v", cmd)
	}

	summary := &netplay.Snapshot{}
	if err := json.Unmarshal([]byte(`{"phase":"summary","summary":{"result":"victory","mode":"story","act":1,"floors":10}}`), summary); err != nil {
		t.Fatalf("json.Unmarshal(summary) error = %v", err)
	}
	m.multiplayerSnapshot = summary
	m.syncMultiplayerRoomSelection()
	m.multiplayerStructuredIndex = 1
	cmd, err = m.buildSelectedStructuredMultiplayerCommand()
	if err != nil {
		t.Fatalf("buildSelectedStructuredMultiplayerCommand(summary) error = %v", err)
	}
	if cmd.Action != "abandon" {
		t.Fatalf("expected summary abandon command, got %#v", cmd)
	}
}

func TestMultiplayerMapSubmitThrottlePreventsRapidRepeat(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	session, commands := startCommandCaptureSession(t)

	var snapshot netplay.Snapshot
	if err := json.Unmarshal([]byte(`{
		"phase":"map",
		"self_id":"seat-1",
		"host_id":"seat-1",
		"map":{
			"act":1,
			"next_floor":2,
			"reachable":[
				{"index":1,"floor":2,"kind":"monster","label":"A1 F2 Monster"},
				{"index":2,"floor":2,"kind":"shop","label":"A1 F2 Shop"}
			]
		}
	}`), &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.multiplayerSession = session
	m.multiplayerSnapshot = &snapshot
	m.screen = screenMultiplayerRoom
	m.now = func() time.Time { return time.Unix(1700000000, 0) }
	m.syncMultiplayerRoomSelection()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	command := mustReceiveCommand(t, commands)
	if command["action"] != "node" {
		t.Fatalf("expected node command, got %#v", command)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(model)
	ensureNoSecondCommand(t, commands)
	if !strings.Contains(m2.message, "操作过快") {
		t.Fatalf("expected throttle message, got %q", m2.message)
	}
}

func TestCodexUsesLeftRightPageFlipWithoutWrapping(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenCodex
	m.codexTab = codexTabCards
	m.index = 0

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 := next.(model)
	if m1.index != 10 {
		t.Fatalf("expected right page flip to go to index 10, got %d", m1.index)
	}

	for i := 0; i < 9; i++ {
		next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyDown})
		m1 = next.(model)
	}
	if m1.index != 19 {
		t.Fatalf("expected to reach last item on page 2 at index 19, got %d", m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 = next.(model)
	if m1.index != 20 {
		t.Fatalf("expected right page flip from page 2 to go to index 20, got %d", m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 = next.(model)
	if m1.index != 21 {
		t.Fatalf("expected down to continue within page 3 at index 21, got %d", m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 = next.(model)
	if m1.index != 30 {
		t.Fatalf("expected right page flip to jump to index 30, got %d", m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m1 = next.(model)
	if m1.index != 20 {
		t.Fatalf("expected left page flip to return to previous page start, got %d", m1.index)
	}
}

func TestCodexCanFlipBeyondThirdPage(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenCodex
	m.codexTab = codexTabCards

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 := next.(model)
	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 = next.(model)
	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 = next.(model)

	if m1.index != 30 {
		t.Fatalf("expected to reach fourth page start at index 30, got %d", m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 = next.(model)
	if m1.index != 31 {
		t.Fatalf("expected to keep navigating after the third page, got %d", m1.index)
	}
}

func TestCodexCanSwitchTabsWithNumberKeys(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenCodex
	m.codexTab = codexTabCards
	m.index = 12

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m1 := next.(model)
	if m1.codexTab != codexTabEquipments || m1.index != 0 {
		t.Fatalf("expected number key 3 to switch to equipment tab and reset selection, got tab=%d index=%d", m1.codexTab, m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	m1 = next.(model)
	if m1.codexTab != codexTabCards || m1.index != 0 {
		t.Fatalf("expected number key 1 to switch to card tab and reset selection, got tab=%d index=%d", m1.codexTab, m1.index)
	}
}

func TestCodexCanFlipPagesWithBracketKeys(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	m := newModel(lib, storage.NewStore(t.TempDir()), engine.DefaultProfile(lib), nil)
	m.screen = screenCodex
	m.codexTab = codexTabCards

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m1 := next.(model)
	if m1.index != 10 {
		t.Fatalf("expected ] to flip to next page start at index 10, got %d", m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m1 = next.(model)
	if m1.index != 20 {
		t.Fatalf("expected second ] to flip to index 20, got %d", m1.index)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	m1 = next.(model)
	if m1.index != 10 {
		t.Fatalf("expected [ to flip back to index 10, got %d", m1.index)
	}
}

func DisabledTestProgramCanOpenCodexAndFlipToThirdPage(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	start := newModel(lib, store, engine.DefaultProfile(lib), nil)

	var output bytes.Buffer
	program := tea.NewProgram(
		start,
		tea.WithInput(nil),
		tea.WithOutput(&output),
		tea.WithoutRenderer(),
		tea.WithoutSignals(),
	)

	type runResult struct {
		model tea.Model
		err   error
	}
	done := make(chan runResult, 1)
	go func() {
		finalModel, err := program.Run()
		done <- runResult{model: finalModel, err: err}
	}()

	program.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	program.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	program.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(20 * time.Millisecond)
	program.Send(tea.KeyMsg{Type: tea.KeyRight})
	time.Sleep(20 * time.Millisecond)
	program.Send(tea.KeyMsg{Type: tea.KeyRight})
	time.Sleep(20 * time.Millisecond)
	program.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(20 * time.Millisecond)
	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	var result runResult
	select {
	case result = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("program did not exit after simulated codex navigation")
	}
	if result.err != nil {
		t.Fatalf("program.Run() error = %v", result.err)
	}

	got := result.model.(model)
	if got.screen != screenCodex {
		t.Fatalf("expected to remain on codex screen, got %q", got.screen)
	}
	if got.codexTab != codexTabCards {
		t.Fatalf("expected codex card tab, got %d", got.codexTab)
	}
	if got.codexItemCount() < 46 {
		t.Fatalf("expected at least 46 card codex items, got %d", got.codexItemCount())
	}
	if got.index != 21 {
		t.Fatalf("expected navigation to land on card index 21, got %d", got.index)
	}

	view := got.View()
	if !strings.Contains(view, "46") || !strings.Contains(view, "5") {
		t.Fatalf("expected codex view to show 46 cards and 5 pages, got view: %s", view)
	}
}

func TestModelCanAbandonActiveRun(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Checkpoint = &engine.RunCheckpoint{Screen: string(screenMap)}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	m := newModel(lib, store, engine.DefaultProfile(lib), run)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)

	if m1.run != nil {
		t.Fatalf("expected run to be cleared after abandon, got %#v", m1.run)
	}
	if m1.screen != screenMenu {
		t.Fatalf("expected screenMenu after abandon, got %q", m1.screen)
	}
	if hasMenuItem(m1.menuItems, menuContinue) || hasMenuItem(m1.menuItems, menuAbandon) {
		t.Fatalf("expected continue/abandon items to be removed, got %#v", m1.menuItems)
	}

	loadedRun, err := store.LoadRun()
	if err != nil {
		t.Fatalf("LoadRun() error = %v", err)
	}
	if loadedRun != nil {
		t.Fatalf("expected saved run to be cleared, got %#v", loadedRun)
	}
}

func TestAfterNodeAdvancePersistsMapCheckpoint(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	m := newModel(lib, store, engine.DefaultProfile(lib), run)
	m.screen = screenShop
	m.currentNode = engine.Node{
		ID:    "shop-node",
		Act:   1,
		Floor: 2,
		Index: 0,
		Kind:  engine.NodeShop,
	}
	m.shopState = &engine.ShopState{
		Offers: []engine.ShopOffer{{ID: "remove-card", Kind: "remove", Price: 75}},
	}

	next, _ := m.afterNodeAdvance()
	m1 := next.(model)
	if m1.screen != screenMap {
		t.Fatalf("expected screenMap after node advance, got %q", m1.screen)
	}
	if m1.shopState != nil || m1.currentNode.ID != "" {
		t.Fatalf("expected transient node state to be cleared, got shop=%#v node=%#v", m1.shopState, m1.currentNode)
	}

	loadedRun, err := store.LoadRun()
	if err != nil {
		t.Fatalf("LoadRun() error = %v", err)
	}
	if loadedRun == nil || loadedRun.Checkpoint == nil {
		t.Fatalf("expected checkpoint to be persisted, got %#v", loadedRun)
	}
	if loadedRun.Checkpoint.Screen != string(screenMap) {
		t.Fatalf("expected checkpoint screen %q, got %q", screenMap, loadedRun.Checkpoint.Screen)
	}
	if loadedRun.Checkpoint.CurrentNode != nil || loadedRun.Checkpoint.ShopState != nil {
		t.Fatalf("expected transient node state to be absent from checkpoint, got %#v", loadedRun.Checkpoint)
	}
}

func TestQuitSavesCheckpointFromNodeScreen(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	m := newModel(lib, store, engine.DefaultProfile(lib), run)
	m.screen = screenShop
	m.currentNode = engine.Node{
		ID:    "shop-node",
		Act:   1,
		Floor: 2,
		Index: 0,
		Kind:  engine.NodeShop,
	}
	m.shopState = &engine.ShopState{
		Offers: []engine.ShopOffer{{ID: "remove-card", Kind: "remove", Price: 75}},
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m1 := next.(model)
	if m1.err != nil {
		t.Fatalf("expected quit to save without error, got %v", m1.err)
	}
	if cmd == nil {
		t.Fatal("expected quit command to be returned")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}

	loadedRun, err := store.LoadRun()
	if err != nil {
		t.Fatalf("LoadRun() error = %v", err)
	}
	if loadedRun == nil || loadedRun.Checkpoint == nil {
		t.Fatalf("expected checkpoint to be saved on quit, got %#v", loadedRun)
	}
	if loadedRun.Checkpoint.Screen != string(screenShop) {
		t.Fatalf("expected checkpoint screen %q, got %q", screenShop, loadedRun.Checkpoint.Screen)
	}
	if loadedRun.Checkpoint.CurrentNode == nil || loadedRun.Checkpoint.CurrentNode.ID != "shop-node" {
		t.Fatalf("expected current node to be saved on quit, got %#v", loadedRun.Checkpoint.CurrentNode)
	}
}

func TestShopAugmentServiceUsesDeckActionFlow(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	run, err := engine.NewRun(lib, engine.DefaultProfile(lib), engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Player.Gold = 200
	m := newModel(lib, store, engine.DefaultProfile(lib), run)
	m.screen = screenShop
	m.shopState = &engine.ShopState{Offers: []engine.ShopOffer{{
		ID:          "service-echo-workshop",
		Kind:        "service",
		Name:        "Echo Workshop",
		Description: "Choose a card and grant it an extra echo this combat.",
		Price:       66,
		ItemID:      "service_echo_workshop",
	}}}
	m.currentNode = engine.Node{ID: "shop-node", Act: 1, Floor: 2, Index: 0, Kind: engine.NodeShop}

	next, _ := m.updateShop(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := next.(model)
	if m1.screen != screenDeckAct {
		t.Fatalf("expected deck action screen, got %q", m1.screen)
	}
	if m1.deckActionMode != "shop_augment_card" {
		t.Fatalf("expected shop_augment_card mode, got %q", m1.deckActionMode)
	}
	if len(m1.deckActionIndexes) == 0 {
		t.Fatal("expected candidate cards for shop augment flow")
	}

	next, _ = m1.updateDeckAction(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := next.(model)
	if m2.screen != screenShop {
		t.Fatalf("expected to return to shop after resolving augment service, got %q", m2.screen)
	}
	if len(m2.run.Player.Deck[m1.deckActionIndexes[0]].Augments) != 1 {
		t.Fatalf("expected selected card to gain augment, got %#v", m2.run.Player.Deck[m1.deckActionIndexes[0]].Augments)
	}
}

func hasMenuItem(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
