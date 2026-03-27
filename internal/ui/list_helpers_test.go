package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/netplay"

	"github.com/charmbracelet/lipgloss"
)

func TestIndexedListLineTruncatesToWidth(t *testing.T) {
	line := indexedListLine(0, "这是一个非常长非常长非常长的图鉴条目名称 [职业 | 稀有]", 24)
	if got := lipgloss.Width(line); got > 24 {
		t.Fatalf("expected line width <= 24, got %d: %q", got, line)
	}
	if !strings.Contains(line, "...") {
		t.Fatalf("expected truncated line to contain ellipsis, got %q", line)
	}
}

func TestIndexedListLineKeepsShortText(t *testing.T) {
	line := indexedListLine(1, "短条目", 24)
	if strings.Contains(line, "...") {
		t.Fatalf("did not expect ellipsis for short line, got %q", line)
	}
}

func TestListPageWindow(t *testing.T) {
	window := listPageWindow(27, 14, pagedListSize)
	if window.Start != 10 || window.End != 20 {
		t.Fatalf("expected second page window 10..20, got %#v", window)
	}
	if window.Page != 2 || window.TotalPages != 3 {
		t.Fatalf("expected page 2/3, got %#v", window)
	}
}

func TestRenderCombatFitsNarrowWidth(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	var class content.ClassDef
	for _, item := range lib.ClassList() {
		class = item
		break
	}
	if len(class.StartingDeck) < 2 {
		t.Fatalf("expected starting deck for class %q", class.ID)
	}

	var encounter content.EncounterDef
	for _, item := range lib.Encounters {
		encounter = item
		break
	}

	run := &engine.RunState{
		Player: engine.PlayerState{
			Name:           "先锋测试位",
			HP:             42,
			MaxHP:          50,
			Gold:           88,
			PotionCapacity: 2,
			Deck: []engine.DeckCard{
				{CardID: class.StartingDeck[0]},
				{CardID: class.StartingDeck[1]},
			},
		},
	}
	combat := &engine.CombatState{
		Player: engine.CombatActor{
			Name:      "先锋测试位",
			HP:        42,
			MaxHP:     50,
			Block:     9,
			Energy:    3,
			MaxEnergy: 3,
			Statuses: map[string]engine.Status{
				"regen": {Name: "regen", Stacks: 2},
			},
		},
		Allies: []engine.CombatActor{{
			Name:      "协同后卫",
			HP:        31,
			MaxHP:     40,
			Block:     4,
			Energy:    3,
			MaxEnergy: 3,
			Statuses: map[string]engine.Status{
				"taunt": {Name: "taunt", Stacks: 1},
			},
		}},
		Enemy: engine.CombatEnemy{
			CombatActor:   engine.CombatActor{Name: encounter.Name, HP: encounter.HP, MaxHP: encounter.HP, Statuses: map[string]engine.Status{}},
			CurrentIntent: encounter.IntentCycle[0],
		},
		Enemies: []engine.CombatEnemy{{
			CombatActor:   engine.CombatActor{Name: encounter.Name + "A", HP: encounter.HP, MaxHP: encounter.HP, Statuses: map[string]engine.Status{}},
			CurrentIntent: encounter.IntentCycle[0],
		}, {
			CombatActor:   engine.CombatActor{Name: encounter.Name + "B", HP: encounter.HP, MaxHP: encounter.HP, Statuses: map[string]engine.Status{}},
			CurrentIntent: encounter.IntentCycle[0],
		}},
		Hand:     []engine.RuntimeCard{{ID: class.StartingDeck[0]}, {ID: class.StartingDeck[1]}},
		DrawPile: []engine.RuntimeCard{{ID: class.StartingDeck[0]}},
		Discard:  []engine.RuntimeCard{{ID: class.StartingDeck[1]}},
		Turn:     3,
		Log:      []engine.CombatLogEntry{{Turn: 2, Text: "协同后卫为全队施加了很长很长的护盾说明，用来验证窄窗口换行是否稳定。"}},
	}

	rendered := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 0, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 1}, 38, 24)
	assertRenderWidth(t, rendered, 38)
	if !strings.Contains(rendered, "友方战线") || !strings.Contains(rendered, "敌方战线") {
		t.Fatalf("expected combat strips in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "能量 3/3") {
		t.Fatalf("expected friendly strip to show energy, got %q", rendered)
	}
	if !strings.Contains(rendered, "自身状态") || !strings.Contains(rendered, "当前目标") {
		t.Fatalf("expected self status and target summary in combat render, got %q", rendered)
	}
	if strings.Index(rendered, "自身状态") > strings.Index(rendered, "敌方信息") {
		t.Fatalf("expected self status panel to appear before enemy info, got %q", rendered)
	}
	if !strings.Contains(rendered, "当前按键") || !strings.Contains(rendered, "1-6: 切检视页") || !strings.Contains(rendered, "切换手牌/药水") {
		t.Fatalf("expected combat key bar in render, got %q", rendered)
	}
	if strings.Count(rendered, "当前目标") < 2 {
		t.Fatalf("expected both enemy and self panels to show target chips, got %q", rendered)
	}
	if !strings.Contains(rendered, "战斗日志 [紧凑]") {
		t.Fatalf("expected compact combat log label in narrow render, got %q", rendered)
	}
	if !strings.Contains(rendered, "[目标]") {
		t.Fatalf("expected explicit target label in combat strips, got %q", rendered)
	}
	if !strings.Contains(rendered, "检视页: 概览") {
		t.Fatalf("expected inspect pane label in combat detail panel, got %q", rendered)
	}
}

func TestRenderCodexFitsNarrowWidth(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	rendered := RenderCodex(DefaultTheme(), lib, 0, 0, 42)
	assertRenderWidth(t, rendered, 42)
	if !strings.Contains(rendered, "图鉴") {
		t.Fatalf("expected codex title, got %q", rendered)
	}
}

func TestRenderProgressionFitsNarrowWidth(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	profile := engine.Profile{
		MetaCurrency: 120,
		Perks: map[string]int{
			"bonus_max_hp":      1,
			"bonus_start_gold":  1,
			"extra_potion_slot": 0,
		},
	}

	rendered := RenderProgression(DefaultTheme(), lib, profile, 0, 0, 44)
	assertRenderWidth(t, rendered, 44)
	if !strings.Contains(rendered, "局外养成") {
		t.Fatalf("expected progression title, got %q", rendered)
	}
}

func TestRenderMapTreeOverlayShowsCurrentAndReachableNodes(t *testing.T) {
	run := &engine.RunState{
		Act:          1,
		CurrentFloor: 1,
		Reachable:    []string{"a1-f2-n0", "a1-f2-n1"},
		Map: engine.MapGraph{Act: 1, Floors: [][]engine.Node{
			{{ID: "a1-f1-n0", Act: 1, Floor: 1, Index: 0, Kind: engine.NodeMonster, Edges: []string{"a1-f2-n0", "a1-f2-n1"}}},
			{{ID: "a1-f2-n0", Act: 1, Floor: 2, Index: 0, Kind: engine.NodeEvent}, {ID: "a1-f2-n1", Act: 1, Floor: 2, Index: 1, Kind: engine.NodeShop}},
		}},
	}
	rendered := RenderMapTreeOverlay(DefaultTheme(), run, engine.Node{ID: "a1-f2-n0", Act: 1, Floor: 2, Index: 0, Kind: engine.NodeEvent}, 72, 24)
	assertRenderWidth(t, rendered, 72)
	if !strings.Contains(rendered, "地图总览 Act 1") || !strings.Contains(rendered, "树状地图") {
		t.Fatalf("expected map overlay title and tree block, got %q", rendered)
	}
	if !strings.Contains(rendered, "[当前]") || !strings.Contains(rendered, "[可达]") {
		t.Fatalf("expected current and reachable markers in map overlay, got %q", rendered)
	}
	if !strings.Contains(rendered, "=>") || !strings.Contains(rendered, "[可达:F2-1]") {
		t.Fatalf("expected highlighted path edges in map overlay, got %q", rendered)
	}
	if !strings.Contains(rendered, "第01层") || !strings.Contains(rendered, "第02层") {
		t.Fatalf("expected floor sections in map overlay, got %q", rendered)
	}
}

func TestRenderMultiplayerMapTreeOverlayShowsSharedReachablePath(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"shared_map":{
			"act":1,
			"next_floor":2,
			"current_floor":1,
			"current_node_id":"a1-f1-n0",
			"reachable":[
				{"id":"a1-f2-n0","index":1,"floor":2,"kind":"monster","label":"A1 F2 Monster"},
				{"id":"a1-f2-n1","index":2,"floor":2,"kind":"event","label":"A1 F2 Event"}
			],
			"graph":[
				[{"id":"a1-f1-n0","index":0,"floor":1,"kind":"monster","edges":["a1-f2-n0","a1-f2-n1"]}],
				[{"id":"a1-f2-n0","index":0,"floor":2,"kind":"monster","edges":[]},{"id":"a1-f2-n1","index":1,"floor":2,"kind":"event","edges":[]}]
			]
		}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	rendered := RenderMultiplayerMapTreeOverlay(DefaultTheme(), &snapshot, 72, 24)
	assertRenderWidth(t, rendered, 72)
	if !strings.Contains(rendered, "Shared Map Overlay Act 1") || !strings.Contains(rendered, "Shared position") {
		t.Fatalf("expected multiplayer map overlay heading, got %q", rendered)
	}
	if !strings.Contains(rendered, "[可达]") || !strings.Contains(rendered, "=>") {
		t.Fatalf("expected multiplayer overlay to highlight reachable path, got %q", rendered)
	}
}

func TestRenderCombatUsesExpandedLogWhenSpaceAllows(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	var class content.ClassDef
	for _, item := range lib.ClassList() {
		class = item
		break
	}

	entries := make([]engine.CombatLogEntry, 0, 16)
	for i := 1; i <= 16; i++ {
		entries = append(entries, engine.CombatLogEntry{Turn: i, Text: fmt.Sprintf("entry-%02d", i)})
	}

	combat := &engine.CombatState{
		Player:  engine.CombatActor{Name: "先锋测试位", HP: 42, MaxHP: 50, Block: 9, Energy: 3, MaxEnergy: 3},
		Enemy:   engine.CombatEnemy{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40}},
		Enemies: []engine.CombatEnemy{{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40}}},
		Hand:    []engine.RuntimeCard{{ID: class.StartingDeck[0]}},
		Turn:    3,
		Log:     entries,
	}
	run := &engine.RunState{Player: engine.PlayerState{Name: "先锋测试位", Deck: []engine.DeckCard{{CardID: class.StartingDeck[0]}}, PotionCapacity: 2}}

	narrow := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 0, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 120, 24)
	tall := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 0, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 120, 42)
	assertRenderWidth(t, narrow, 120)
	assertRenderWidth(t, tall, 120)
	if strings.Count(tall, "entry-") <= strings.Count(narrow, "entry-") {
		t.Fatalf("expected tall combat render to show more log entries than narrow render, narrow=%q tall=%q", narrow, tall)
	}
	if !strings.Contains(narrow, "entry-16") || !strings.Contains(tall, "entry-16") {
		t.Fatalf("expected the latest combat log entry to remain visible, narrow=%q tall=%q", narrow, tall)
	}
	if strings.Contains(narrow, "entry-01") {
		t.Fatalf("expected narrow combat render to clip the oldest entry, got %q", narrow)
	}
	if !strings.Contains(tall, "战斗日志 [展开]") {
		t.Fatalf("expected expanded combat log label in tall render, got %q", tall)
	}
}

func TestRenderCombatShowsPotionSelectionPanel(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run := &engine.RunState{
		Player: engine.PlayerState{
			Name:           "先锋测试位",
			HP:             42,
			MaxHP:          50,
			Gold:           88,
			PotionCapacity: 3,
			Potions:        []string{"tonic_minor", "potion_focus"},
			Deck:           []engine.DeckCard{{CardID: "slash"}},
		},
	}
	combat := &engine.CombatState{
		Player:  engine.CombatActor{Name: "先锋测试位", HP: 42, MaxHP: 50, Energy: 3, MaxEnergy: 3, Statuses: map[string]engine.Status{}},
		Enemy:   engine.CombatEnemy{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}},
		Enemies: []engine.CombatEnemy{{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}}},
		Hand:    []engine.RuntimeCard{{ID: "slash"}},
		Turn:    2,
		Seats: []engine.CombatSeatState{{
			Hand:    []engine.RuntimeCard{{ID: "slash"}},
			Potions: []string{"tonic_minor", "potion_focus"},
		}},
	}

	rendered := RenderCombat(DefaultTheme(), lib, run, combat, 0, 1, true, 0, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 110, 32)
	assertRenderWidth(t, rendered, 110)
	if !strings.Contains(rendered, "药水槽") || !strings.Contains(rendered, "聚能药剂") {
		t.Fatalf("expected visible potion section in combat panel, got %q", rendered)
	}
	if !strings.Contains(rendered, "[2] 聚能药剂") {
		t.Fatalf("expected selected potion slot marker in combat panel, got %q", rendered)
	}
	if !strings.Contains(rendered, "手牌槽") {
		t.Fatalf("expected hand and potion sections to share the action panel, got %q", rendered)
	}
}

func TestRenderCombatHighlightsPendingRepeatState(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run := &engine.RunState{
		Player: engine.PlayerState{
			Name:           "先锋测试位",
			HP:             42,
			MaxHP:          50,
			Gold:           88,
			PotionCapacity: 3,
			Potions:        []string{"potion_echo"},
			Deck:           []engine.DeckCard{{CardID: "slash"}},
		},
	}
	combat := &engine.CombatState{
		Player:  engine.CombatActor{Name: "先锋测试位", HP: 42, MaxHP: 50, Energy: 3, MaxEnergy: 3, Statuses: map[string]engine.Status{}},
		Enemy:   engine.CombatEnemy{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}},
		Enemies: []engine.CombatEnemy{{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}}},
		Hand:    []engine.RuntimeCard{{ID: "slash"}},
		Turn:    2,
		Seats: []engine.CombatSeatState{{
			Hand:            []engine.RuntimeCard{{ID: "slash"}},
			Potions:         []string{"potion_echo"},
			NextCardRepeats: 2,
		}},
	}

	rendered := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 5, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 110, 32)
	assertRenderWidth(t, rendered, 110)
	if !strings.Contains(rendered, "连发待机") || !strings.Contains(rendered, "下一张牌额外重复 2 次") {
		t.Fatalf("expected repeat status banner in combat render, got %q", rendered)
	}
	if !strings.Contains(rendered, "回响药剂") {
		t.Fatalf("expected new potion name to render in combat UI, got %q", rendered)
	}
}

func TestRenderCombatHighlightsFilteredRepeatState(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run := &engine.RunState{
		Player: engine.PlayerState{
			Name:           "先锋测试位",
			HP:             42,
			MaxHP:          50,
			Gold:           88,
			PotionCapacity: 3,
			Potions:        []string{"potion_battle_echo"},
			Deck:           []engine.DeckCard{{CardID: "slash"}},
		},
	}
	combat := &engine.CombatState{
		Player:  engine.CombatActor{Name: "先锋测试位", HP: 42, MaxHP: 50, Energy: 3, MaxEnergy: 3, Statuses: map[string]engine.Status{}},
		Enemy:   engine.CombatEnemy{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}},
		Enemies: []engine.CombatEnemy{{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}}},
		Hand:    []engine.RuntimeCard{{ID: "slash"}},
		Turn:    2,
		Seats: []engine.CombatSeatState{{
			Hand:               []engine.RuntimeCard{{ID: "slash"}},
			Potions:            []string{"potion_battle_echo"},
			PendingCardRepeats: []engine.PendingCardRepeat{{Count: 1, Tag: "attack"}},
		}},
	}

	rendered := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 5, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 110, 32)
	assertRenderWidth(t, rendered, 110)
	if !strings.Contains(rendered, "下一张攻击牌额外重复 1 次") {
		t.Fatalf("expected filtered repeat banner in combat render, got %q", rendered)
	}
}

func TestRenderCombatShowsCardAugmentSummary(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run := &engine.RunState{
		Player: engine.PlayerState{
			Name:           "先锋测试位",
			HP:             42,
			MaxHP:          50,
			Gold:           88,
			PotionCapacity: 3,
			Deck:           []engine.DeckCard{{CardID: "slash"}},
		},
	}
	combat := &engine.CombatState{
		Player:  engine.CombatActor{Name: "先锋测试位", HP: 42, MaxHP: 50, Energy: 3, MaxEnergy: 3, Statuses: map[string]engine.Status{}},
		Enemy:   engine.CombatEnemy{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}},
		Enemies: []engine.CombatEnemy{{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}}},
		Hand:    []engine.RuntimeCard{{ID: "slash", Augments: []engine.CardAugment{engine.ReplyCardAugment(engine.CardEffectScopeCombat, 1)}}},
		Turn:    2,
		Seats: []engine.CombatSeatState{{
			Hand: []engine.RuntimeCard{{ID: "slash", Augments: []engine.CardAugment{engine.ReplyCardAugment(engine.CardEffectScopeCombat, 1)}}},
		}},
	}

	rendered := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 0, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 110, 32)
	assertRenderWidth(t, rendered, 110)
	if !strings.Contains(rendered, "本牌额外重复 1 次") {
		t.Fatalf("expected augmented card summary in combat render, got %q", rendered)
	}
}

func TestRenderCombatShowsGenericAugmentSummary(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	augment := engine.NewCardAugment("charged_cantrip", engine.CardEffectScopeCombat,
		content.Effect{Op: "draw", Value: 1},
		content.Effect{Op: "gain_energy", Value: 1},
	)
	run := &engine.RunState{
		Player: engine.PlayerState{
			Name:           "先锋测试位",
			HP:             42,
			MaxHP:          50,
			Gold:           88,
			PotionCapacity: 3,
			Deck:           []engine.DeckCard{{CardID: "slash"}},
		},
	}
	combat := &engine.CombatState{
		Player:  engine.CombatActor{Name: "先锋测试位", HP: 42, MaxHP: 50, Energy: 3, MaxEnergy: 3, Statuses: map[string]engine.Status{}},
		Enemy:   engine.CombatEnemy{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}},
		Enemies: []engine.CombatEnemy{{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40, Statuses: map[string]engine.Status{}}}},
		Hand:    []engine.RuntimeCard{{ID: "slash", Augments: []engine.CardAugment{augment}}},
		Turn:    2,
		Seats: []engine.CombatSeatState{{
			Hand: []engine.RuntimeCard{{ID: "slash", Augments: []engine.CardAugment{augment}}},
		}},
	}

	rendered := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 0, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 110, 32)
	assertRenderWidth(t, rendered, 110)
	if !strings.Contains(rendered, "抽 1 张牌") || !strings.Contains(rendered, "获得 1 能量") {
		t.Fatalf("expected generic augment summary in combat render, got %q", rendered)
	}
}

func TestRenderCombatLogPanelUsesFixedRecentWindow(t *testing.T) {
	entries := []engine.CombatLogEntry{}
	for i := 1; i <= 9; i++ {
		entries = append(entries, engine.CombatLogEntry{Turn: i, Text: fmt.Sprintf("第 %d 条日志", i)})
	}
	rendered := renderCombatLogPanel(DefaultTheme(), entries, 0, 40, panelContentWidth(40), 24, "紧凑")
	if !strings.Contains(rendered, "第 ") || !strings.Contains(rendered, "前面 ") || !strings.Contains(rendered, "后面 ") {
		t.Fatalf("expected combat log panel to use fixed recent window hints, got %q", rendered)
	}
	if !strings.Contains(rendered, "T9 第 9 条日志") {
		t.Fatalf("expected fixed combat log window to keep latest logs visible, got %q", rendered)
	}
	if strings.Contains(rendered, "T1 第 1 条日志") {
		t.Fatalf("expected fixed combat log window to clip oldest logs, got %q", rendered)
	}
}

func TestRenderCombatLogPanelSupportsPaging(t *testing.T) {
	entries := []engine.CombatLogEntry{}
	for i := 1; i <= 9; i++ {
		entries = append(entries, engine.CombatLogEntry{Turn: i, Text: fmt.Sprintf("第 %d 条日志", i)})
	}
	latest := renderCombatLogPanel(DefaultTheme(), entries, 0, 42, panelContentWidth(42), 24, "紧凑")
	older := renderCombatLogPanel(DefaultTheme(), entries, 1, 42, panelContentWidth(42), 24, "紧凑")
	if !strings.Contains(latest, "战斗日志 [紧凑]") || !strings.Contains(latest, "T9 第 9 条日志") {
		t.Fatalf("expected latest log page to keep the newest log visible, got %q", latest)
	}
	if strings.Contains(latest, "T1 第 1 条日志") {
		t.Fatalf("expected latest log page to hide oldest entries, got %q", latest)
	}
	if !strings.Contains(older, "战斗日志 [紧凑]") || !strings.Contains(older, "T4 第 4 条日志") {
		t.Fatalf("expected older combat log page to show earlier entries, got %q", older)
	}
	if strings.Contains(older, "T9 第 9 条日志") {
		t.Fatalf("expected older combat log page to differ from latest page, got %q", older)
	}
}

func TestRenderCombatTopInfoSupportsPaging(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	var class content.ClassDef
	for _, item := range lib.ClassList() {
		class = item
		break
	}

	deck := make([]engine.DeckCard, 0, 12)
	for i := 0; i < 12; i++ {
		deck = append(deck, engine.DeckCard{CardID: class.StartingDeck[i%len(class.StartingDeck)]})
	}

	combat := &engine.CombatState{
		Player:  engine.CombatActor{Name: "先锋测试位", HP: 42, MaxHP: 50, Block: 9, Energy: 3, MaxEnergy: 3},
		Enemy:   engine.CombatEnemy{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40}},
		Enemies: []engine.CombatEnemy{{CombatActor: engine.CombatActor{Name: "Jaw Worm", HP: 32, MaxHP: 40}}},
		Hand:    []engine.RuntimeCard{{ID: class.StartingDeck[0]}},
		Turn:    3,
	}
	run := &engine.RunState{Player: engine.PlayerState{Name: "先锋测试位", Deck: deck, PotionCapacity: 2}}

	firstPage := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 4, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 110, 30)
	secondPage := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, false, 4, 1, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 0}, 110, 30)
	assertRenderWidth(t, firstPage, 110)
	assertRenderWidth(t, secondPage, 110)
	if !strings.Contains(firstPage, "第 1/") {
		t.Fatalf("expected first page hint in top combat info, got %q", firstPage)
	}
	if !strings.Contains(secondPage, "第 ") || firstPage == secondPage {
		t.Fatalf("expected top combat info paging to produce a distinct later page, got %q", secondPage)
	}
}

func TestFixedPanelBodyLinesShowPagedHint(t *testing.T) {
	theme := DefaultTheme()
	lines := []string{
		theme.Normal.Render("第1行"),
		theme.Normal.Render("第2行"),
		theme.Normal.Render("第3行"),
		theme.Normal.Render("第4行"),
		theme.Normal.Render("第5行"),
		theme.Normal.Render("第6行"),
	}
	visible := fixedPanelBodyLines(lines, 5, theme, 20)
	if len(visible) != 5 {
		t.Fatalf("expected fixed panel body line count 5, got %d", len(visible))
	}
	joined := strings.Join(visible, "\n")
	if !strings.Contains(joined, "第 1/2 页") || !strings.Contains(joined, "还有 3 行") {
		t.Fatalf("expected paged overflow hint in fixed panel body, got %q", joined)
	}
}

func TestRenderEventFitsNarrowWidth(t *testing.T) {
	state := engine.EventState{
		Event: content.EventDef{
			Name:        "狭窄回廊",
			Description: "一段明显过长的事件描述会在小终端中换行展示，避免直接顶出边框造成阅读困难。",
			Choices: []content.EventChoiceDef{{
				ID:          "observe",
				Label:       "观察墙面",
				Description: "仔细检查刻痕并记录所有异常细节，确认文本在小宽度下不会越界。",
			}},
		},
	}

	rendered := RenderEvent(DefaultTheme(), state, 0, 34)
	assertRenderWidth(t, rendered, 34)
	if !strings.Contains(rendered, "狭窄回廊") {
		t.Fatalf("expected event title, got %q", rendered)
	}
}

func TestRenderMultiplayerRoomShowsIdentitySummary(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"self_id":"seat-2",
		"host_id":"seat-1",
		"seat":2,
		"room_addr":"127.0.0.1:7777",
		"phase":"shop",
		"phase_title":"Shop",
		"players":[
			{"id":"seat-1","seat":1,"name":"Host","class_id":"vanguard","ready":true,"connected":true},
			{"id":"seat-2","seat":2,"name":"Guest","class_id":"arcanist","ready":true,"connected":true}
		],
		"shop":{"gold":100,"offers":[{"index":1,"kind":"card","name":"Quick Slash","description":"Deal 7 damage","price":65}]}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	rendered := RenderMultiplayerRoom(DefaultTheme(), &snapshot, []string{"Host only: Buy Quick Slash (65 gold)"}, 0, true, "", MultiplayerCombatState{}, 56, 24)
	assertRenderWidth(t, rendered, 56)
	if !strings.Contains(rendered, "Identity") {
		t.Fatalf("expected identity section, got %q", rendered)
	}
	if !strings.Contains(rendered, "Seat 2 | Guest | [arcanist] | Player") {
		t.Fatalf("expected seat identity line, got %q", rendered)
	}
	if !strings.Contains(rendered, "Control:") || !strings.Contains(rendered, "shop purchases") {
		t.Fatalf("expected control summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "Phase Controls") || !strings.Contains(rendered, "1. Quick Slash | 65 gold") {
		t.Fatalf("expected structured sidebar option in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "Focus") || !strings.Contains(rendered, "Phase Ops") {
		t.Fatalf("expected global focus bar in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "Keys") || !strings.Contains(rendered, "up/down: phase selection") {
		t.Fatalf("expected structured key bar in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "IP 127.0.0.1") || !strings.Contains(rendered, "Port 7777") {
		t.Fatalf("expected room IP and port in render, got %q", rendered)
	}
}

func TestRenderMultiplayerRoomShowsStructuredSidebarForShopPhase(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"self_id":"seat-2",
		"host_id":"seat-1",
		"seat":2,
		"room_addr":"127.0.0.1:7777",
		"phase":"shop",
		"phase_title":"Shop",
		"commands":["buy <index>","leave"],
		"players":[
			{"id":"seat-1","seat":1,"name":"Host","class_id":"vanguard","ready":true,"connected":true},
			{"id":"seat-2","seat":2,"name":"Guest","class_id":"arcanist","ready":true,"connected":true}
		],
		"shop":{"gold":100,"offers":[{"index":1,"kind":"card","name":"Quick Slash","description":"Deal 7 damage","price":65},{"index":2,"kind":"card","name":"Guard","description":"Gain 5 block","price":40}]}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	rendered := RenderMultiplayerRoom(DefaultTheme(), &snapshot, []string{"Host only: Buy Quick Slash (65 gold)"}, 0, true, "", MultiplayerCombatState{
		SelectedIndex:  1,
		SelectionLabel: "商店商品 Guard",
	}, 90, 24)
	assertRenderWidth(t, rendered, 90)
	if !strings.Contains(rendered, "Phase Controls") || !strings.Contains(rendered, "Focus: phase controls") {
		t.Fatalf("expected structured sidebar title and focus, got %q", rendered)
	}
	if !strings.Contains(rendered, "Focus") || !strings.Contains(rendered, "Chat Input") {
		t.Fatalf("expected global focus bar in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "2. Guard | 40 gold") {
		t.Fatalf("expected structured sidebar options, got %q", rendered)
	}
	if !strings.Contains(rendered, "Selection") || !strings.Contains(rendered, "商店商品 Guard") {
		t.Fatalf("expected structured selection summary, got %q", rendered)
	}
}

func TestRenderMultiplayerRoomStructuredSidebarReflectsChatFocus(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"self_id":"seat-2",
		"host_id":"seat-1",
		"seat":2,
		"room_addr":"127.0.0.1:7777",
		"phase":"reward",
		"phase_title":"Reward",
		"commands":["take <card#>","skip"],
		"reward":{"gold":30,"source":"combat","cards":[{"index":1,"name":"Shield Wall","summary":"Gain 8 block"}]}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	rendered := RenderMultiplayerRoom(DefaultTheme(), &snapshot, nil, 0, false, "take 1", MultiplayerCombatState{
		ChatFocused:    true,
		SelectedIndex:  0,
		SelectionLabel: "奖励卡 Shield Wall",
	}, 88, 24)
	assertRenderWidth(t, rendered, 88)
	if !strings.Contains(rendered, "Focus: chat input") {
		t.Fatalf("expected chat focus hint in structured sidebar, got %q", rendered)
	}
	if !strings.Contains(rendered, "Focus") || !strings.Contains(rendered, "Phase Ops") {
		t.Fatalf("expected global focus bar chips, got %q", rendered)
	}
	if !strings.Contains(rendered, "Keys") || !strings.Contains(rendered, "Enter: send command") {
		t.Fatalf("expected chat key bar in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "Tab switches between suggested actions") && !strings.Contains(rendered, "Outside combat:") {
		t.Fatalf("expected updated structured input help text, got %q", rendered)
	}
}

func TestRenderMultiplayerRoomCombatShowsTopSummary(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"self_id":"seat-2",
		"host_id":"seat-1",
		"seat":2,
		"room_addr":"127.0.0.1:7777",
		"phase":"combat",
		"phase_title":"Combat",
		"players":[
			{"id":"seat-1","seat":1,"name":"Host","class_id":"vanguard","ready":true,"connected":true},
			{"id":"seat-2","seat":2,"name":"Guest","class_id":"arcanist","ready":true,"connected":true}
		],
		"combat":{
			"turn":3,
			"energy":2,
			"max_energy":3,
			"deck_size":11,
			"draw_count":7,
			"discard_count":2,
			"exhaust_count":1,
			"party":[
				{"index":1,"name":"Host","hp":38,"max_hp":50,"energy":3,"max_energy":3,"block":4,"status":"regen 1"},
				{"index":2,"name":"Guest","hp":29,"max_hp":40,"energy":2,"max_energy":3,"block":7,"status":"focus 1"}
			],
			"enemies":[
				{"index":1,"name":"Jaw Worm","hp":32,"max_hp":40,"block":6,"status":"weak 1","intent":"Deal 11 damage"}
			],
			"hand":[
				{"index":1,"name":"Zap","cost":1,"summary":"Deal 8 damage","target_hint":"enemy"}
			],
			"potions":["1. Smoke Bomb | Leave combat"],
			"vote_status":["Seat 1 Host [vanguard]: ready","Seat 2 Guest [arcanist]: acting"]
		}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	rendered := RenderMultiplayerRoom(DefaultTheme(), &snapshot, nil, 0, true, "", MultiplayerCombatState{
		Enabled:        true,
		ModeLabel:      "手牌",
		SelectionLabel: "Zap",
		TargetKind:     "enemy",
		TargetIndex:    1,
		TargetLabel:    "敌方 1",
	}, 100, 28)
	assertRenderWidth(t, rendered, 100)
	if !strings.Contains(rendered, "Jaw Worm") {
		t.Fatalf("expected focused enemy summary in render, got %q", rendered)
	}
	if strings.Count(rendered, "Target") < 2 {
		t.Fatalf("expected both enemy and self summary panels to show target chips, got %q", rendered)
	}
	if !strings.Contains(rendered, "Seat Status") || !strings.Contains(rendered, "Guest") {
		t.Fatalf("expected self status panel in multiplayer combat summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "Potions 1 | Deck 11") {
		t.Fatalf("expected self inventory summary in multiplayer combat status panel, got %q", rendered)
	}
	if !strings.Contains(rendered, "Combat Info") {
		t.Fatalf("expected combat info summary in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "第 1/2 页") || !strings.Contains(rendered, "还有 3 行") {
		t.Fatalf("expected fixed-height combat info panel to show page hint, got %q", rendered)
	}
	if !strings.Contains(rendered, "Inspect Pane:") {
		t.Fatalf("expected inspect page label in multiplayer combat summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "Selection") || !strings.Contains(rendered, "Zap") || !strings.Contains(rendered, "Target") || !strings.Contains(rendered, "敌方 1") {
		t.Fatalf("expected current combat selection summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "Zap [current]") || !strings.Contains(rendered, "Jaw Worm [target]") {
		t.Fatalf("expected explicit current card and target labels in combat lists, got %q", rendered)
	}
}

func TestRenderMultiplayerRoomCombatShowsOperationsSidebarWhenInspectNotFocused(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"self_id":"seat-2",
		"host_id":"seat-1",
		"seat":2,
		"room_addr":"127.0.0.1:7777",
		"phase":"combat",
		"phase_title":"Combat",
		"combat":{
			"turn":3,
			"energy":2,
			"max_energy":3,
			"deck_size":11,
			"draw_count":7,
			"discard_count":2,
			"exhaust_count":1,
			"party":[{"index":1,"name":"Host","hp":38,"max_hp":50,"energy":3,"max_energy":3,"block":4,"status":"regen 1"}],
			"enemies":[{"index":1,"name":"Jaw Worm","hp":32,"max_hp":40,"block":6,"status":"weak 1","intent":"Deal 11 damage"}],
			"hand":[{"index":1,"name":"Zap","cost":1,"summary":"Deal 8 damage","target_hint":"enemy"}],
			"potions":["1. Smoke Bomb | Leave combat"],
			"vote_status":["Seat 1 Host [vanguard]: ready","Seat 2 Guest [arcanist]: acting"]
		},
		"commands":["play <card#> [enemy|ally <target#>]","end"]
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	rendered := RenderMultiplayerRoom(DefaultTheme(), &snapshot, nil, 0, true, "", MultiplayerCombatState{
		Enabled:           true,
		OperationsFocused: true,
		ModeLabel:         "手牌",
		SelectionLabel:    "Zap",
		TargetKind:        "enemy",
		TargetIndex:       1,
		TargetLabel:       "敌方 1",
	}, 100, 28)
	assertRenderWidth(t, rendered, 100)
	if !strings.Contains(rendered, "Combat Controls") || !strings.Contains(rendered, "Focus: combat controls") {
		t.Fatalf("expected combat operations sidebar, got %q", rendered)
	}
	if !strings.Contains(rendered, "Focus") || !strings.Contains(rendered, "Combat Inspect") {
		t.Fatalf("expected combat focus bar, got %q", rendered)
	}
	if !strings.Contains(rendered, "Keys") || !strings.Contains(rendered, "z: hand/potion") {
		t.Fatalf("expected combat key bar, got %q", rendered)
	}
	if !strings.Contains(rendered, "Zap [current]") || !strings.Contains(rendered, "Jaw Worm [target]") {
		t.Fatalf("expected explicit current card and target labels, got %q", rendered)
	}
	if !strings.Contains(rendered, "Tab: combat controls / combat inspect / chat input") {
		t.Fatalf("expected combat focus help in sidebar, got %q", rendered)
	}
}

func TestRenderCombatInspectSidebarShowsSeatAwarePanes(t *testing.T) {
	var snapshot netplay.Snapshot
	jsonBlob := []byte(`{
		"self_id":"seat-2",
		"host_id":"seat-1",
		"seat":2,
		"room_addr":"127.0.0.1:7777",
		"phase":"combat",
		"phase_title":"Combat",
		"combat":{
			"turn":4,
			"energy":1,
			"max_energy":3,
			"deck_size":12,
			"draw_count":5,
			"discard_count":4,
			"exhaust_count":2,
			"draw_pile":["Zap","Spark"],
			"discard_pile":["Guard"],
			"exhaust_pile":["Nova"],
			"effects":["自身状态 | focus 2","遗物 | 每回合首张技能牌减费 1"],
			"logs":["T1 叠加易伤","T2 抽到 Spark","T3 使用 Guard","T4 造成 8 点伤害","T5 触发遗物","T6 结束回合投票"],
			"potions":["1. Smoke Bomb | Leave combat"],
			"vote_status":["Seat 1 Host [vanguard]: ready","Seat 2 Guest [arcanist]: acting"]
		}
	}`)
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	pilePane := renderCombatInspectSidebar(DefaultTheme(), &snapshot, MultiplayerCombatState{InspectPane: 1, InspectFocused: true}, 36)
	if !strings.Contains(pilePane, "Draw Pile") || !strings.Contains(pilePane, "Zap") || !strings.Contains(pilePane, "Nova") {
		t.Fatalf("expected pile pane contents, got %q", pilePane)
	}

	effectsPane := renderCombatInspectSidebar(DefaultTheme(), &snapshot, MultiplayerCombatState{InspectPane: 2, InspectFocused: true}, 36)
	if !strings.Contains(effectsPane, "focus 2") || !strings.Contains(effectsPane, "每回合首张技能牌减费 1") {
		t.Fatalf("expected effects pane contents, got %q", effectsPane)
	}

	votePane := renderCombatInspectSidebar(DefaultTheme(), &snapshot, MultiplayerCombatState{InspectPane: 4, InspectFocused: true}, 36)
	if !strings.Contains(votePane, "Seat 1 Host [vanguard]: ready") || !strings.Contains(votePane, "Potions") || !strings.Contains(votePane, "Smoke Bomb") {
		t.Fatalf("expected vote pane contents, got %q", votePane)
	}

	logPaneCompact := renderCombatInspectSidebar(DefaultTheme(), &snapshot, MultiplayerCombatState{InspectPane: 3, InspectFocused: true}, 36)
	if !strings.Contains(logPaneCompact, "Logs [Compact]") {
		t.Fatalf("expected compact inspect log label, got %q", logPaneCompact)
	}
	if !strings.Contains(logPaneCompact, "T6 结束回合投票") {
		t.Fatalf("expected compact inspect log pane to show recent entries, got %q", logPaneCompact)
	}

	logPaneExpanded := renderCombatInspectSidebar(DefaultTheme(), &snapshot, MultiplayerCombatState{InspectPane: 3, InspectFocused: true}, 52)
	if !strings.Contains(logPaneExpanded, "Logs [Expanded]") {
		t.Fatalf("expected expanded inspect log label, got %q", logPaneExpanded)
	}
}

func TestRenderCombatInspectSidebarPagesExtendedLogs(t *testing.T) {
	var snapshot netplay.Snapshot
	logs := make([]string, 0, 18)
	for i := 1; i <= 18; i++ {
		logs = append(logs, fmt.Sprintf("T%d entry-%02d", i, i))
	}
	jsonBlob, err := json.Marshal(map[string]any{
		"phase": "combat",
		"combat": map[string]any{
			"logs": logs,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := json.Unmarshal(jsonBlob, &snapshot); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	latest := renderCombatInspectSidebar(DefaultTheme(), &snapshot, MultiplayerCombatState{InspectPane: 3, InspectFocused: true}, 36)
	older := renderCombatInspectSidebar(DefaultTheme(), &snapshot, MultiplayerCombatState{InspectPane: 3, InspectFocused: true, InspectLogPage: 1}, 36)
	if !strings.Contains(latest, "Logs [Compact]") || !strings.Contains(older, "Logs [Compact]") {
		t.Fatalf("expected compact inspect log panes, got latest=%q older=%q", latest, older)
	}
	if !strings.Contains(latest, "entry-18") {
		t.Fatalf("expected latest multiplayer log page to keep newest entry, got %q", latest)
	}
	if strings.Contains(latest, "entry-15") {
		t.Fatalf("expected latest multiplayer log page to hide earlier entry-15, got %q", latest)
	}
	if !strings.Contains(older, "entry-15") {
		t.Fatalf("expected older multiplayer log page to show earlier entry-15, got %q", older)
	}
	if strings.Contains(older, "entry-18") {
		t.Fatalf("expected older multiplayer log page to differ from latest page, got %q", older)
	}
}

func TestRoomHostPortSplitsHostAndPort(t *testing.T) {
	host, port := roomHostPort("127.0.0.1:7777")
	if host != "127.0.0.1" || port != "7777" {
		t.Fatalf("expected split host/port, got %q/%q", host, port)
	}

	host, port = roomHostPort("bad-address")
	if host != "bad-address" || port != "" {
		t.Fatalf("expected fallback host-only parse, got %q/%q", host, port)
	}
}

func assertRenderWidth(t *testing.T, rendered string, width int) {
	t.Helper()
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("expected line width <= %d, got %d: %q", width, got, line)
		}
	}
}
