package engine

import (
	"slices"
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

func TestReplyAugmentRepeatsAttachedCard(t *testing.T) {
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

	combat := NewCombat(lib, player, encounter, 15)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = nil
	refreshSeat0FromLegacy(combat)
	if err := AddCombatCardAugment(combat, 0, CombatCardPileHand, 0, ReplyCardAugment(CardEffectScopeCombat, 1)); err != nil {
		t.Fatalf("AddCombatCardAugment() error = %v", err)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard() error = %v", err)
	}
	if want := 18; combat.Enemy.HP != want {
		t.Fatalf("expected reply augment to repeat slash and leave enemy HP %d, got %d", want, combat.Enemy.HP)
	}
}

func TestRunScopeCardAugmentCarriesIntoCombat(t *testing.T) {
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
	if err := AddRunCardAugment(&player, 0, ReplyCardAugment(CardEffectScopeRun, 1)); err != nil {
		t.Fatalf("AddRunCardAugment() error = %v", err)
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

	combat := NewCombat(lib, player, encounter, 17)
	if got := len(combat.DrawPile); got != 1 {
		t.Fatalf("expected one runtime card copied from deck, got %d", got)
	}
	if got := len(combat.DrawPile[0].Augments); got != 1 {
		t.Fatalf("expected run augment to copy into runtime card, got %d", got)
	}
	combat.Hand = []RuntimeCard{combat.DrawPile[0]}
	combat.DrawPile = nil

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard() error = %v", err)
	}
	if want := 18; combat.Enemy.HP != want {
		t.Fatalf("expected run augment to stay active in combat and leave enemy HP %d, got %d", want, combat.Enemy.HP)
	}
}

func TestTurnScopedCardAugmentExpiresAtTurnEnd(t *testing.T) {
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

	combat := NewCombat(lib, player, encounter, 19)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = nil
	refreshSeat0FromLegacy(combat)
	if err := AddCombatCardAugment(combat, 0, CombatCardPileHand, 0, ReplyCardAugment(CardEffectScopeTurn, 1)); err != nil {
		t.Fatalf("AddCombatCardAugment() error = %v", err)
	}

	EndPlayerTurn(lib, player, combat)
	if got := len(combat.Discard); got != 1 {
		t.Fatalf("expected discarded hand after turn end, got %d", got)
	}
	if got := len(combat.Discard[0].Augments); got != 0 {
		t.Fatalf("expected turn-scoped augment to expire, got %#v", combat.Discard[0].Augments)
	}
}

func TestGenericCardAugmentCanGrantDrawAndEnergy(t *testing.T) {
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
		Deck:      []DeckCard{{CardID: "slash"}, {CardID: "guard"}},
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

	combat := NewCombat(lib, player, encounter, 41)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = []RuntimeCard{{ID: "guard"}}
	refreshSeat0FromLegacy(combat)
	augment := NewCardAugment("charged_cantrip", CardEffectScopeCombat,
		content.Effect{Op: "draw", Value: 1},
		content.Effect{Op: "gain_energy", Value: 1},
	)
	if err := AddCombatCardAugment(combat, 0, CombatCardPileHand, 0, augment); err != nil {
		t.Fatalf("AddCombatCardAugment() error = %v", err)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard() error = %v", err)
	}
	if want := 3; combat.Player.Energy != want {
		t.Fatalf("expected slash to refund one energy and end at %d, got %d", want, combat.Player.Energy)
	}
	if got := len(combat.Hand); got != 1 || combat.Hand[0].ID != "guard" {
		t.Fatalf("expected augment to draw guard into hand, got %#v", combat.Hand)
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

func TestAddCombatCardEffectAddsStatusCardsToHandAndDraw(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{ClassID: "vanguard", Name: "tester", MaxHP: 80, HP: 80, MaxEnergy: 3}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "hexsmith",
		Kind:       "monster",
		Act:        2,
		HP:         30,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{{
			Name: "hex",
			Effects: []content.Effect{
				{Op: "add_combat_card", Target: "all_allies", ItemType: "hand", CardID: "slow_form"},
				{Op: "add_combat_card", Target: "all_allies", ItemType: "draw", CardID: "bleed_wound"},
			},
		}},
	}

	combat := NewCombat(lib, player, encounter, 5)
	StartPlayerTurn(lib, player, combat)
	EndPlayerTurn(lib, player, combat)

	if len(combat.Hand) != 1 || combat.Hand[0].ID != "slow_form" {
		t.Fatalf("expected slow_form in hand, got %#v", combat.Hand)
	}
	if len(combat.DrawPile) == 0 || combat.DrawPile[len(combat.DrawPile)-1].ID != "bleed_wound" {
		t.Fatalf("expected bleed_wound in draw pile, got %#v", combat.DrawPile)
	}
}

func TestUnplayableStatusCardCannotBePlayed(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{ClassID: "vanguard", Name: "tester", MaxHP: 80, HP: 80, MaxEnergy: 3}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}}},
	}
	combat := NewCombat(lib, player, encounter, 7)
	combat.Hand = []RuntimeCard{{ID: "bleed_wound"}}
	combat.DrawPile = nil
	if err := PlayCard(lib, player, combat, 0); err == nil {
		t.Fatal("expected bleed_wound to be unplayable")
	}
}

func TestHandEndTriggerStatusCardDealsDamageAndPurges(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{ClassID: "vanguard", Name: "tester", MaxHP: 80, HP: 80, MaxEnergy: 3}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}}},
	}
	combat := NewCombat(lib, player, encounter, 9)
	combat.Hand = []RuntimeCard{{ID: "bleed_wound"}}
	combat.DrawPile = nil
	refreshSeat0FromLegacy(combat)

	EndPlayerTurn(lib, player, combat)

	if combat.Player.HP != 78 {
		t.Fatalf("expected bleed_wound to deal 2 direct HP damage, got %d", combat.Player.HP)
	}
	if len(combat.Exhaust) != 1 || combat.Exhaust[0].ID != "bleed_wound" {
		t.Fatalf("expected bleed_wound to purge into exhaust, got %#v", combat.Exhaust)
	}
	if len(combat.Discard) != 0 {
		t.Fatalf("expected no discarded bleed_wound, got %#v", combat.Discard)
	}
}

func TestStatusCardsAreExcludedFromStandardPools(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 21)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	for _, card := range standardCardsForRun(run, lib.NeutralCards()) {
		if slices.Contains(card.Flags, "status_card") {
			t.Fatalf("expected standard card pool to exclude status cards, got %q", card.ID)
		}
	}

	shop := StartShop(lib, run)
	for _, offer := range shop.Offers {
		if offer.CardID == "" {
			continue
		}
		if slices.Contains(lib.Cards[offer.CardID].Flags, "status_card") {
			t.Fatalf("expected shop to exclude status cards, got %q", offer.CardID)
		}
	}

	reward := BuildReward(lib, run, Node{ID: "m", Kind: NodeMonster, Act: 1, Floor: 1, Index: 0}, content.EncounterDef{ID: "dummy", Name: "dummy", Kind: "monster", Act: 1, HP: 10, CardReward: 3, IntentCycle: []content.EnemyIntentDef{{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}}}})
	for _, card := range reward.CardChoices {
		if slices.Contains(card.Flags, "status_card") {
			t.Fatalf("expected reward to exclude status cards, got %q", card.ID)
		}
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

func TestRepeatNextCardQueuesAndConsumesOnPlay(t *testing.T) {
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

	combat := NewCombat(lib, player, encounter, 17)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = nil

	if err := ApplyExternalCombatEffect(lib, player, combat, content.Effect{Op: "repeat_next_card", Value: 2}, CombatTarget{}); err != nil {
		t.Fatalf("ApplyExternalCombatEffect() error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 2 {
		t.Fatalf("expected pending repeats 2, got %d", got)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard() error = %v", err)
	}

	if want := 12; combat.Enemy.HP != want {
		t.Fatalf("expected enemy HP %d after repeated slash, got %d", want, combat.Enemy.HP)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 0 {
		t.Fatalf("expected repeats to be consumed, got %d", got)
	}
	if want := 2; combat.Player.Energy != want {
		t.Fatalf("expected card cost paid once and energy %d, got %d", want, combat.Player.Energy)
	}
	if got := len(combat.Discard); got != 1 {
		t.Fatalf("expected card to be discarded once, got %d", got)
	}
}

func TestPotionEchoAppliesRepeatNextCard(t *testing.T) {
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
		Potions:   []string{"potion_echo"},
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

	combat := NewCombat(lib, player, encounter, 23)
	combat.Hand = []RuntimeCard{{ID: "slash"}}
	combat.DrawPile = nil

	if err := UsePotion(lib, player, combat, "potion_echo"); err != nil {
		t.Fatalf("UsePotion() error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 2 {
		t.Fatalf("expected echo potion to queue 2 repeats, got %d", got)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard() error = %v", err)
	}
	if want := 12; combat.Enemy.HP != want {
		t.Fatalf("expected repeated slash to reduce enemy HP to %d, got %d", want, combat.Enemy.HP)
	}
}

func TestFilteredRepeatWaitsForMatchingCardTag(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "arcanist",
		Name:      "tester",
		MaxHP:     70,
		HP:        70,
		MaxEnergy: 3,
		Deck:      []DeckCard{{CardID: "spark"}, {CardID: "slash"}},
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
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombat(lib, player, encounter, 29)
	combat.Hand = []RuntimeCard{{ID: "spark"}, {ID: "slash"}}
	combat.DrawPile = nil

	if err := ApplyExternalCombatEffect(lib, player, combat, content.Effect{Op: "repeat_next_card", Value: 1, Tag: "attack"}, CombatTarget{}); err != nil {
		t.Fatalf("ApplyExternalCombatEffect() error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 1 {
		t.Fatalf("expected pending filtered repeat 1, got %d", got)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard(spark) error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 1 {
		t.Fatalf("expected attack repeat to survive non-attack card, got %d", got)
	}
	if want := 35; combat.Enemy.HP != want {
		t.Fatalf("expected spark to resolve once and enemy HP %d, got %d", want, combat.Enemy.HP)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard(slash) error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 0 {
		t.Fatalf("expected attack repeat to be consumed by slash, got %d", got)
	}
	if want := 23; combat.Enemy.HP != want {
		t.Fatalf("expected slash to resolve twice and enemy HP %d, got %d", want, combat.Enemy.HP)
	}
}

func TestPotionArcaneEchoOnlyRepeatsSpell(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID:   "arcanist",
		Name:      "tester",
		MaxHP:     70,
		HP:        70,
		MaxEnergy: 3,
		Deck:      []DeckCard{{CardID: "slash"}, {CardID: "spark"}},
		Potions:   []string{"potion_arcane_echo"},
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
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombat(lib, player, encounter, 31)
	combat.Hand = []RuntimeCard{{ID: "slash"}, {ID: "spark"}}
	combat.DrawPile = nil

	if err := UsePotion(lib, player, combat, "potion_arcane_echo"); err != nil {
		t.Fatalf("UsePotion() error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 1 {
		t.Fatalf("expected arcane echo to queue 1 repeat, got %d", got)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard(slash) error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 1 {
		t.Fatalf("expected spell-only repeat to survive attack card, got %d", got)
	}

	if err := PlayCard(lib, player, combat, 0); err != nil {
		t.Fatalf("PlayCard(spark) error = %v", err)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 0 {
		t.Fatalf("expected spell-only repeat to be consumed, got %d", got)
	}
	if want := 24; combat.Enemy.HP != want {
		t.Fatalf("expected slash once and spark twice to leave enemy HP %d, got %d", want, combat.Enemy.HP)
	}
}

func TestRepeatNextCardStaysSeatLocal(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	players := []PlayerState{
		{ClassID: "vanguard", Name: "lead", MaxHP: 80, HP: 80, MaxEnergy: 3, Deck: []DeckCard{{CardID: "slash"}}},
		{ClassID: "vanguard", Name: "ally", MaxHP: 80, HP: 80, MaxEnergy: 3, Deck: []DeckCard{{CardID: "slash"}}},
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
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 19)
	combat.Seats[0].Hand = []RuntimeCard{{ID: "slash"}}
	combat.Seats[1].Hand = []RuntimeCard{{ID: "slash"}}
	syncLegacySeat0(combat)

	if err := ApplySeatExternalCombatEffect(lib, players[1], combat, 1, content.Effect{Op: "repeat_next_card", Value: 2}, CombatTarget{}); err != nil {
		t.Fatalf("ApplySeatExternalCombatEffect() error = %v", err)
	}

	if err := PlaySeatCardWithTarget(lib, players[0], combat, 0, 0, CombatTarget{Kind: CombatTargetEnemy, Index: 0}); err != nil {
		t.Fatalf("PlaySeatCardWithTarget(seat0) error = %v", err)
	}
	if want := 74; combat.Enemies[0].HP != want {
		t.Fatalf("expected seat 0 normal damage to enemy HP %d, got %d", want, combat.Enemies[0].HP)
	}

	if err := PlaySeatCardWithTarget(lib, players[1], combat, 1, 0, CombatTarget{Kind: CombatTargetEnemy, Index: 0}); err != nil {
		t.Fatalf("PlaySeatCardWithTarget(seat1) error = %v", err)
	}
	if want := 56; combat.Enemies[0].HP != want {
		t.Fatalf("expected seat 1 repeated damage to enemy HP %d, got %d", want, combat.Enemies[0].HP)
	}
	if got := PendingNextCardRepeats(combat, 0); got != 0 {
		t.Fatalf("expected seat 0 repeats to stay 0, got %d", got)
	}
	if got := PendingNextCardRepeats(combat, 1); got != 0 {
		t.Fatalf("expected seat 1 repeats to be consumed, got %d", got)
	}
}

func TestPartyCombatSeatActionsStayIsolated(t *testing.T) {
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
			Deck:      []DeckCard{{CardID: "slash"}},
		},
		{
			ClassID:   "arcanist",
			Name:      "ally",
			MaxHP:     70,
			HP:        70,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "guard"}},
			Potions:   []string{"potion_fury"},
		},
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
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 13)
	combat.Seats[0].Hand = []RuntimeCard{{ID: "slash"}}
	combat.Seats[1].Hand = []RuntimeCard{{ID: "guard"}}
	combat.Seats[0].Potions = nil
	combat.Seats[1].Potions = []string{"potion_fury"}
	syncLegacySeat0(combat)

	if err := PlaySeatCardWithTarget(lib, players[0], combat, 0, 0, CombatTarget{Kind: CombatTargetEnemy, Index: 0}); err != nil {
		t.Fatalf("PlaySeatCardWithTarget() error = %v", err)
	}
	if got := len(combat.Seats[0].Hand); got != 0 {
		t.Fatalf("expected seat 0 hand to be consumed, got %d", got)
	}
	if got := len(combat.Seats[1].Hand); got != 1 {
		t.Fatalf("expected seat 1 hand to stay intact, got %d", got)
	}

	if err := UseSeatPotionWithTarget(lib, players[1], combat, 1, "potion_fury", CombatTarget{}); err != nil {
		t.Fatalf("UseSeatPotionWithTarget() error = %v", err)
	}
	if got := len(combat.Seats[1].Potions); got != 0 {
		t.Fatalf("expected seat 1 potion to be consumed, got %d", got)
	}
	if got := len(combat.Seats[0].Potions); got != 0 {
		t.Fatalf("expected seat 0 potions to remain unchanged, got %d", got)
	}
	if got := len(combat.Seats[1].PotionsUsed); got != 1 || combat.Seats[1].PotionsUsed[0] != "potion_fury" {
		t.Fatalf("expected seat 1 potion usage log, got %v", combat.Seats[1].PotionsUsed)
	}
}
