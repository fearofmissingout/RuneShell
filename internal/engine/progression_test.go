package engine

import (
	"slices"
	"testing"

	"cmdcards/internal/content"
)

func TestNewRunUsesProfileLoadoutAndPerks(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	profile := DefaultProfile(lib)
	profile.UnlockedEquipments = append(profile.UnlockedEquipments, "ward_shell", "mirror_band")
	profile.ClassLoadouts["vanguard"] = EquipmentSlots{
		Weapon:    "ironblade",
		Armor:     "ward_shell",
		Accessory: "mirror_band",
	}
	profile.Perks["bonus_start_block"] = 4

	run, err := NewRun(lib, profile, ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	if run.Player.Equipment.Armor != "ward_shell" {
		t.Fatalf("expected custom armor loadout, got %q", run.Player.Equipment.Armor)
	}
	if run.Player.Equipment.Accessory != "mirror_band" {
		t.Fatalf("expected custom accessory loadout, got %q", run.Player.Equipment.Accessory)
	}
	if run.Player.PermanentStats["combat_start_block"] != 4 {
		t.Fatalf("expected combat start block perk to propagate, got %#v", run.Player.PermanentStats)
	}
}

func TestFilterEventChoicesForRelicUpgrade(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{Relics: []string{"war_banner"}}
	event := filterEventChoices(player, lib.Events["relic_annealer"])
	if len(event.Choices) != 1 {
		t.Fatalf("expected exactly one relic upgrade choice, got %#v", event.Choices)
	}
	if event.Choices[0].ID != "awaken_banner" {
		t.Fatalf("expected awaken_banner choice, got %#v", event.Choices[0])
	}
}

func TestApplyOutOfCombatUpgradeRelic(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{Relics: []string{"war_banner"}}
	_, err = applyOutOfCombatEffectsDecision(lib, &player, []content.Effect{
		{Op: "upgrade_relic", ItemID: "war_banner", ResultID: "war_banner_awakened"},
	}, true, -1)
	if err != nil {
		t.Fatalf("applyOutOfCombatEffectsDecision() error = %v", err)
	}
	if len(player.Relics) != 1 || player.Relics[0] != "war_banner_awakened" {
		t.Fatalf("expected relic to be upgraded, got %#v", player.Relics)
	}
}

func TestEventChoiceDeckActionPlanBuildsAugmentSelection(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	choice := lib.Events["inked_vellum"].Choices[0]
	plan, err := EventChoiceDeckActionPlan(lib, run, choice, true)
	if err != nil {
		t.Fatalf("EventChoiceDeckActionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("expected a deck action plan")
	}
	if plan.Mode != "event_augment_card" {
		t.Fatalf("expected event_augment_card mode, got %q", plan.Mode)
	}
	if len(plan.Indexes) == 0 {
		t.Fatal("expected at least one candidate card")
	}
	if plan.Effect == nil || plan.Effect.Op != "augment_card" {
		t.Fatalf("expected augment_card effect payload, got %#v", plan.Effect)
	}
	for _, idx := range plan.Indexes {
		card := run.Player.Deck[idx]
		def := lib.Cards[card.CardID]
		if !slices.Contains(def.Tags, "attack") {
			t.Fatalf("expected only attack-tagged cards in plan, got %s", card.CardID)
		}
	}
}

func TestCombatScopedAugmentAppliesOnlyToNextCombat(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 77)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	state := &EventState{Event: lib.Events["inked_vellum"]}
	if err := ResolveEventDecisionWithDeckChoice(lib, run, state, "flash_charge", true, 0); err != nil {
		t.Fatalf("ResolveEventDecisionWithDeckChoice() error = %v", err)
	}
	if len(run.Player.Deck[0].Augments) != 1 {
		t.Fatalf("expected queued combat augment on deck, got %#v", run.Player.Deck[0].Augments)
	}

	combat, err := StartEncounter(lib, run, Node{ID: "n1", Kind: NodeMonster, Act: 1, Floor: 1, Index: 0})
	if err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}
	found := false
	for _, card := range combat.DrawPile {
		if card.ID == run.Player.Deck[0].CardID && len(card.Augments) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected queued combat augment to appear in the next combat")
	}
	if len(run.Player.Deck[0].Augments) != 0 {
		t.Fatalf("expected queued combat augment to be cleared from persistent deck after combat start, got %#v", run.Player.Deck[0].Augments)
	}
	second, err := StartEncounter(lib, run, Node{ID: "n2", Kind: NodeMonster, Act: 1, Floor: 1, Index: 1})
	if err != nil {
		t.Fatalf("StartEncounter(second) error = %v", err)
	}
	for _, card := range second.DrawPile {
		if len(card.Augments) > 0 {
			t.Fatalf("expected temporary augment to be consumed before second combat, got %#v", card.Augments)
		}
	}
}

func TestEventChoiceDeckActionPlanSupportsTurnScopedBuildPackage(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	choice := lib.Events["bulwark_blueprint"].Choices[1]
	plan, err := EventChoiceDeckActionPlan(lib, run, choice, true)
	if err != nil {
		t.Fatalf("EventChoiceDeckActionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("expected a deck action plan")
	}
	if plan.Effect == nil || plan.Effect.Scope != "turn" {
		t.Fatalf("expected turn-scoped augment effect, got %#v", plan.Effect)
	}
	if len(plan.Indexes) == 0 {
		t.Fatal("expected at least one upgradable candidate card")
	}
}

func TestResolveEventDecisionWithDeckChoiceAppliesBuildPackageAugment(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 23)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	state := EventState{Event: lib.Events["ember_tablet"]}
	choice := lib.Events["ember_tablet"].Choices[0]
	plan, err := EventChoiceDeckActionPlan(lib, run, choice, true)
	if err != nil {
		t.Fatalf("EventChoiceDeckActionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("expected event deck action plan")
	}
	if err := ResolveEventDecisionWithDeckChoice(lib, run, &state, choice.ID, true, plan.Indexes[0]); err != nil {
		t.Fatalf("ResolveEventDecisionWithDeckChoice() error = %v", err)
	}
	augments := run.Player.Deck[plan.Indexes[0]].Augments
	if len(augments) != 1 {
		t.Fatalf("expected selected card to gain one augment, got %#v", augments)
	}
	if len(augments[0].Effects) != 1 || augments[0].Effects[0].Op != "apply_status" || augments[0].Effects[0].Status != "burn" {
		t.Fatalf("expected burn augment payload, got %#v", augments[0])
	}
}

func TestNewCombatAppliesCombatStartBlockPerk(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		Name:      "Test",
		HP:        50,
		MaxHP:     50,
		MaxEnergy: 3,
		Deck:      []DeckCard{{CardID: "slash"}},
		PermanentStats: map[string]int{
			"combat_start_block": 4,
		},
	}
	encounter := lib.Encounters["act1_raider"]
	combat := NewCombat(lib, player, encounter, 1)
	if combat.Player.Block != 4 {
		t.Fatalf("expected combat start block 4, got %d", combat.Player.Block)
	}
}
