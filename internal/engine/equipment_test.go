package engine

import (
	"math/rand"
	"testing"

	"cmdcards/internal/content"
)

func TestBuildEquipmentOfferUsesCurrentSlot(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		ClassID: "vanguard",
		Deck: []DeckCard{
			{CardID: "slash"},
			{CardID: "slash"},
			{CardID: "guard"},
		},
		Equipment: EquipmentSlots{Weapon: "ironblade"},
	}

	offer, err := BuildEquipmentOffer(lib, player, "sunsteel_blade", "reward", 0)
	if err != nil {
		t.Fatalf("BuildEquipmentOffer() error = %v", err)
	}
	if offer.Slot != "weapon" {
		t.Fatalf("expected weapon slot, got %s", offer.Slot)
	}
	if offer.CurrentEquipmentID != "ironblade" {
		t.Fatalf("expected current equipment ironblade, got %s", offer.CurrentEquipmentID)
	}
	if offer.CandidateScore <= offer.CurrentScore {
		t.Fatalf("expected candidate score %d to beat current score %d", offer.CandidateScore, offer.CurrentScore)
	}
}

func TestApplyCombatResultDecisionCanSkipEquipment(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 7)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	node := Node{
		ID:    "A1F1E1",
		Act:   1,
		Floor: 1,
		Kind:  NodeElite,
		Edges: []string{"A1F2M1"},
	}
	combat := &CombatState{
		Player: CombatActor{HP: run.Player.HP - 5},
		Reward: RewardState{
			Gold:        20,
			EquipmentID: "sunsteel_blade",
		},
	}

	if err := ApplyCombatResultDecision(lib, run, node, combat, "", false); err != nil {
		t.Fatalf("ApplyCombatResultDecision() error = %v", err)
	}
	if run.Player.Equipment.Weapon != "ironblade" {
		t.Fatalf("expected to keep ironblade, got %s", run.Player.Equipment.Weapon)
	}
	if run.Player.Gold <= 0 {
		t.Fatalf("expected gold reward to apply, got %d", run.Player.Gold)
	}
	if run.Stats.ElitesWon != 1 {
		t.Fatalf("expected elite win count 1, got %d", run.Stats.ElitesWon)
	}
}

func TestApplyShopEquipmentPurchaseCancelKeepsGold(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 11)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	startGold := run.Player.Gold
	shop := ShopState{
		Offers: []ShopOffer{
			{
				ID:     "equipment-sunsteel",
				Kind:   "equipment",
				Name:   "炽钢刃",
				Price:  76,
				ItemID: "sunsteel_blade",
			},
		},
	}

	if err := ApplyShopEquipmentPurchase(lib, run, &shop, "equipment-sunsteel", false); err != nil {
		t.Fatalf("ApplyShopEquipmentPurchase() error = %v", err)
	}
	if run.Player.Gold != startGold {
		t.Fatalf("expected gold %d after cancel, got %d", startGold, run.Player.Gold)
	}
	if run.Player.Equipment.Weapon != "ironblade" {
		t.Fatalf("expected to keep ironblade, got %s", run.Player.Equipment.Weapon)
	}
	if len(shop.Log) == 0 {
		t.Fatal("expected cancel log entry")
	}
}

func TestRewardEquipmentIDAvoidsStarterPool(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	rng := rand.New(rand.NewSource(19))
	for i := 0; i < 20; i++ {
		id := rewardEquipmentID(lib, rng, "elite")
		if id == "" {
			t.Fatal("expected non-empty equipment reward")
		}
		if lib.Equipments[id].Rarity == "starter" {
			t.Fatalf("expected non-starter equipment, got %s", id)
		}
	}
}

func TestResolveEventDecisionCanSkipEquipment(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 23)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Player.Equipment.Accessory = "mirror_band"

	state := EventState{Event: lib.Events["ancient_forge"]}
	if err := ResolveEventDecision(lib, run, &state, "take_blade", false); err != nil {
		t.Fatalf("ResolveEventDecision() error = %v", err)
	}
	if run.Player.Equipment.Accessory != "mirror_band" {
		t.Fatalf("expected to keep mirror_band, got %s", run.Player.Equipment.Accessory)
	}
	if !state.Done {
		t.Fatal("expected event state to be marked done")
	}
	if len(state.Log) == 0 {
		t.Fatal("expected event log output")
	}
}

func TestResolveEventDecisionWithDeckChoiceAppliesRunAugment(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 23)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	state := EventState{Event: lib.Events["inked_vellum"]}
	choice := lib.Events["inked_vellum"].Choices[0]
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
	if len(run.Player.Deck[plan.Indexes[0]].Augments) != 1 {
		t.Fatalf("expected selected card to gain augment, got %#v", run.Player.Deck[plan.Indexes[0]].Augments)
	}
	if !state.Done {
		t.Fatal("expected event state to be marked done")
	}
}
