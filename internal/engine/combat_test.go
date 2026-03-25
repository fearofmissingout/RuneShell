package engine

import (
	"testing"

	"cmdcards/internal/content"
)

func TestCombatDamageModifiersOrder(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "vanguard",
		Name:      "tester",
		MaxHP:     80,
		HP:        80,
		MaxEnergy: 3,
		Deck:      []DeckCard{{CardID: "slash"}},
		Relics:    []string{"war_banner"},
		Equipment: EquipmentSlots{Weapon: "ironblade"},
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         30,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombat(lib, player, encounter, 7)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = nil
	combat.Enemy.Statuses["vulnerable"] = Status{Name: "vulnerable", Stacks: 1, Duration: 2}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard() error = %v", err)
	}

	if want := 19; combat.Enemy.HP != want {
		t.Fatalf("expected enemy HP %d after relic+equipment+status modifiers, got %d", want, combat.Enemy.HP)
	}
}

func TestPotionRelicEquipmentStack(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "vanguard",
		Name:      "tester",
		MaxHP:     80,
		HP:        80,
		MaxEnergy: 3,
		Deck:      []DeckCard{{CardID: "slash"}},
		Relics:    []string{"war_banner"},
		Equipment: EquipmentSlots{Weapon: "ironblade", Armor: "guardmail"},
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         40,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "jab", Effects: []content.Effect{{Op: "damage", Value: 8, Tag: "attack"}}},
		},
	}

	combat := NewCombat(lib, player, encounter, 9)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = nil
	combat.Enemy.Statuses["vulnerable"] = Status{Name: "vulnerable", Stacks: 1, Duration: 2}

	if err := UsePotion(lib, player, combat, "potion_fury"); err != nil {
		t.Fatalf("UsePotion() error = %v", err)
	}
	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard() error = %v", err)
	}

	if want := 26; combat.Enemy.HP != want {
		t.Fatalf("expected enemy HP %d after potion+relic+equipment stack, got %d", want, combat.Enemy.HP)
	}

	EndPlayerTurn(lib, player, combat)
	if want := 73; combat.Player.HP != want {
		t.Fatalf("expected armor to reduce incoming damage to HP %d, got %d", want, combat.Player.HP)
	}
}

func TestEnemyIntentTargetsOpponent(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "vanguard",
		Name:      "tester",
		MaxHP:     80,
		HP:        80,
		MaxEnergy: 3,
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "caster",
		Kind:       "monster",
		Act:        1,
		HP:         30,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{
				Name: "curse",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "opponent", Status: "vulnerable", Value: 1, Duration: 2},
				},
			},
		},
	}

	combat := NewCombat(lib, player, encounter, 3)
	StartPlayerTurn(lib, player, combat)
	EndPlayerTurn(lib, player, combat)

	if got := statusStacks(combat.Player.Statuses, "vulnerable"); got != 1 {
		t.Fatalf("expected player to receive vulnerable, got %d", got)
	}
	if got := statusStacks(combat.Enemy.Statuses, "vulnerable"); got != 0 {
		t.Fatalf("expected enemy vulnerable to stay 0, got %d", got)
	}
}

func TestStatusWindowsAndModifiers(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "arcanist",
		Name:      "tester",
		MaxHP:     80,
		HP:        80,
		MaxEnergy: 3,
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "training dummy",
		Kind:       "monster",
		Act:        1,
		HP:         40,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombat(lib, player, encounter, 11)
	applyStatus(&combat.Player, "focus", 2, 2)
	applyStatus(&combat.Player, "burn", 3, 2)
	applyStatus(&combat.Player, "regen", 2, 2)
	applyStatus(&combat.Player, "frail", 1, 2)

	StartPlayerTurn(lib, player, combat)
	if want := 77; combat.Player.HP != want {
		t.Fatalf("expected burn to reduce HP to %d at turn start, got %d", want, combat.Player.HP)
	}

	combat.Hand = []RuntimeCard{{ID: "guard"}, {ID: "spark"}}
	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard(guard) error = %v", err)
	}
	if want := 3; combat.Player.Block != want {
		t.Fatalf("expected frail to reduce block gain to %d, got %d", want, combat.Player.Block)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard(spark) error = %v", err)
	}
	if want := 33; combat.Enemy.HP != want {
		t.Fatalf("expected focus to increase spell damage and enemy HP to %d, got %d", want, combat.Enemy.HP)
	}

	EndPlayerTurn(lib, player, combat)
	if want := 79; combat.Player.HP != want {
		t.Fatalf("expected regen to heal HP to %d at turn end, got %d", want, combat.Player.HP)
	}
}

func TestEncounterPassivesTrigger(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "vanguard",
		Name:      "tester",
		MaxHP:     80,
		HP:        80,
		MaxEnergy: 3,
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "boss",
		Kind:       "boss",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		Passives: []content.Effect{
			{Op: "block", Trigger: "combat_start", Value: 5},
			{Op: "heal", Trigger: "enemy_turn_end", Value: 4},
		},
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombat(lib, player, encounter, 21)
	if want := 5; combat.Enemy.Block != want {
		t.Fatalf("expected combat start passive block %d, got %d", want, combat.Enemy.Block)
	}

	combat.Enemy.HP = 12
	StartPlayerTurn(lib, player, combat)
	EndPlayerTurn(lib, player, combat)

	if want := 16; combat.Enemy.HP != want {
		t.Fatalf("expected enemy turn end passive heal to HP %d, got %d", want, combat.Enemy.HP)
	}
}

func TestDrawCardsRespectsHandLimit(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "vanguard",
		Name:      "tester",
		MaxHP:     80,
		HP:        80,
		MaxEnergy: 3,
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         30,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombat(lib, player, encounter, 7)
	combat.Hand = []RuntimeCard{
		{ID: "slash"}, {ID: "slash"}, {ID: "slash"}, {ID: "guard"}, {ID: "guard"},
		{ID: "guard"}, {ID: "ember_strike"}, {ID: "windstep"}, {ID: "shield_bash"},
	}
	combat.DrawPile = []RuntimeCard{
		{ID: "fortify"},
		{ID: "iron_wave"},
		{ID: "battle_focus"},
	}

	drawCards(lib, combat, 3)

	if got := len(combat.Hand); got != combatHandLimit {
		t.Fatalf("expected hand limit %d, got %d", combatHandLimit, got)
	}
	if got := len(combat.Discard); got != 2 {
		t.Fatalf("expected overflow cards to go to discard, got %d", got)
	}
	if got := len(combat.DrawPile); got != 0 {
		t.Fatalf("expected draw pile to be consumed, got %d", got)
	}
}

func TestPlayCardWithTargetHitsSelectedEnemy(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "vanguard",
		Name:      "tester",
		MaxHP:     80,
		HP:        80,
		MaxEnergy: 3,
		Deck:      []DeckCard{{CardID: "slash"}},
	}
	encounters := []content.EncounterDef{
		{
			ID:         "dummy-a",
			Name:       "dummy-a",
			Kind:       "monster",
			Act:        1,
			HP:         20,
			GoldReward: 0,
			CardReward: 0,
			IntentCycle: []content.EnemyIntentDef{
				{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
			},
		},
		{
			ID:         "dummy-b",
			Name:       "dummy-b",
			Kind:       "monster",
			Act:        1,
			HP:         20,
			GoldReward: 0,
			CardReward: 0,
			IntentCycle: []content.EnemyIntentDef{
				{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
			},
		},
	}

	combat := NewCombatWithEnemies(lib, player, encounters, encounters[0], 7)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = nil

	if err := PlayCardWithTarget(lib, player, combat, 0, CombatTarget{Kind: CombatTargetEnemy, Index: 1}); err != nil {
		t.Fatalf("PlayCardWithTarget() error = %v", err)
	}

	if want := 20; combat.Enemies[0].HP != want {
		t.Fatalf("expected first enemy HP %d, got %d", want, combat.Enemies[0].HP)
	}
	if want := 14; combat.Enemies[1].HP != want {
		t.Fatalf("expected selected enemy HP %d, got %d", want, combat.Enemies[1].HP)
	}
}

func TestPlayCardWithTargetCanGrantBlockToAlly(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	players := []PlayerState{
		{
			ClassID:   "vanguard",
			Name:      "lead",
			MaxHP:     80,
			HP:        80,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "guard"}},
		},
		{
			ClassID:   "arcanist",
			Name:      "ally",
			MaxHP:     70,
			HP:        70,
			MaxEnergy: 3,
		},
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 9)
	combat.Hand = []RuntimeCard{{ID: "guard"}}
	combat.DrawPile = nil

	if err := PlayCardWithTarget(lib, players[0], combat, 0, CombatTarget{Kind: CombatTargetAlly, Index: 1}); err != nil {
		t.Fatalf("PlayCardWithTarget() error = %v", err)
	}

	if want := 0; combat.Player.Block != want {
		t.Fatalf("expected leader block %d, got %d", want, combat.Player.Block)
	}
	if want := 5; combat.Allies[0].Block != want {
		t.Fatalf("expected ally block %d, got %d", want, combat.Allies[0].Block)
	}
}
