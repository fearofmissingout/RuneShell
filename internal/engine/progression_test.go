package engine

import (
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
	}, true)
	if err != nil {
		t.Fatalf("applyOutOfCombatEffectsDecision() error = %v", err)
	}
	if len(player.Relics) != 1 || player.Relics[0] != "war_banner_awakened" {
		t.Fatalf("expected relic to be upgraded, got %#v", player.Relics)
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
