package engine

import (
	"testing"

	"cmdcards/internal/content"
)

func TestNewCombatForPartyScalesEncounterAndVotes(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	players := []PlayerState{
		{Name: "p1", HP: 70, MaxHP: 70, MaxEnergy: 3, Deck: []DeckCard{{CardID: "slash"}}},
		{Name: "p2", HP: 60, MaxHP: 60, MaxEnergy: 3, Deck: []DeckCard{{CardID: "guard"}}},
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
			{Name: "slam", Effects: []content.Effect{{Op: "damage", Value: 6}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 9)
	if !combat.Coop.Enabled {
		t.Fatal("expected coop state to be enabled")
	}
	if combat.Enemy.HP != 60 {
		t.Fatalf("expected scaled enemy HP 60, got %d", combat.Enemy.HP)
	}
	if got := combat.Enemy.CurrentIntent.Effects[0].Value; got != 12 {
		t.Fatalf("expected scaled enemy damage 12, got %d", got)
	}
	if RequestEndTurnVote(combat, 0) {
		t.Fatal("expected first vote to wait for teammate")
	}
	if !RequestEndTurnVote(combat, 1) {
		t.Fatal("expected second vote to complete the shared end turn")
	}
}

func TestDealPlayerSideDamageUsesBlockThenSplitsOverflow(t *testing.T) {
	combat := &CombatState{
		Player: CombatActor{Name: "p1", HP: 40, MaxHP: 40, Block: 8, Statuses: map[string]Status{}},
		Allies: []CombatActor{
			{Name: "p2", HP: 40, MaxHP: 40, Block: 4, Statuses: map[string]Status{}},
		},
	}

	dealt := DealPlayerSideDamage(combat, 18)
	if dealt != 6 {
		t.Fatalf("expected overflow hp damage 6, got %d", dealt)
	}
	if combat.Player.Block != 0 || combat.Allies[0].Block != 0 {
		t.Fatalf("expected both blocks exhausted, got %d and %d", combat.Player.Block, combat.Allies[0].Block)
	}
	if combat.Player.HP != 37 || combat.Allies[0].HP != 37 {
		t.Fatalf("expected overflow to split evenly, got hp %d and %d", combat.Player.HP, combat.Allies[0].HP)
	}
}
