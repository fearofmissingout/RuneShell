package ui

import (
	"encoding/json"
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

	rendered := RenderCombat(DefaultTheme(), lib, run, combat, 0, 0, engine.CombatTarget{Kind: engine.CombatTargetEnemy, Index: 1}, 38, 24)
	assertRenderWidth(t, rendered, 38)
	if !strings.Contains(rendered, "友方战线") || !strings.Contains(rendered, "敌方战线") {
		t.Fatalf("expected combat strips in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "能量 3/3") {
		t.Fatalf("expected friendly strip to show energy, got %q", rendered)
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

	rendered := RenderMultiplayerRoom(DefaultTheme(), &snapshot, []string{"仅房主: 购买 Quick Slash (65 金币)"}, 0, true, "", MultiplayerCombatState{}, 56, 24)
	assertRenderWidth(t, rendered, 56)
	if !strings.Contains(rendered, "当前身份") {
		t.Fatalf("expected identity section, got %q", rendered)
	}
	if !strings.Contains(rendered, "Seat 2 | Guest | [arcanist] | 队员") {
		t.Fatalf("expected seat identity line, got %q", rendered)
	}
	if !strings.Contains(rendered, "当前控制:") || !strings.Contains(rendered, "当前只能建议购买内容") || !strings.Contains(rendered, "购物与离店由房主执行。") {
		t.Fatalf("expected control summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "仅房主: 购买 Quick Slash (65 金币)") {
		t.Fatalf("expected action label in render, got %q", rendered)
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
