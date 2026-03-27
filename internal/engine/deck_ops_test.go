package engine

import (
	"testing"

	"cmdcards/internal/content"
)

func TestUpgradeDeckCard(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	player := PlayerState{
		Deck: []DeckCard{{CardID: "windstep"}},
	}
	name, err := UpgradeDeckCard(lib, &player, 0)
	if err != nil {
		t.Fatalf("UpgradeDeckCard() error = %v", err)
	}
	if name != "风步" {
		t.Fatalf("expected upgraded card 风步, got %s", name)
	}
	if !player.Deck[0].Upgraded {
		t.Fatal("expected card to be marked upgraded")
	}
}

func TestApplyShopCardRemovalConsumesOffer(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 31)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	shop := ShopState{
		Offers: []ShopOffer{
			{
				ID:          "remove-card",
				Kind:        "remove",
				Name:        "精简牌组",
				Description: "移除一张牌。",
				Price:       68,
			},
		},
	}
	startGold := run.Player.Gold
	startDeck := len(run.Player.Deck)

	if err := ApplyShopCardRemoval(lib, run, &shop, "remove-card", 0); err != nil {
		t.Fatalf("ApplyShopCardRemoval() error = %v", err)
	}
	if len(run.Player.Deck) != startDeck-1 {
		t.Fatalf("expected deck size %d, got %d", startDeck-1, len(run.Player.Deck))
	}
	if run.Player.Gold != startGold-68 {
		t.Fatalf("expected gold %d, got %d", startGold-68, run.Player.Gold)
	}
	if len(shop.Offers) != 0 {
		t.Fatalf("expected removal service to be consumed, got %d offers", len(shop.Offers))
	}
}

func TestShopOfferDeckActionPlanBuildsAugmentSelection(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 31)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Player.Gold = 200
	shop := ShopState{Offers: []ShopOffer{{
		ID:          "service-echo-workshop",
		Kind:        "service",
		Name:        "回响工坊",
		Description: "选择一张攻击牌，本局使其使用时额外抽 1 张牌。",
		Price:       66,
		ItemID:      "service_echo_workshop",
	}}}

	plan, err := ShopOfferDeckActionPlan(lib, run, &shop, "service-echo-workshop")
	if err != nil {
		t.Fatalf("ShopOfferDeckActionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("expected shop augment service to produce a deck action plan")
	}
	if plan.Mode != "shop_augment_card" {
		t.Fatalf("expected shop_augment_card mode, got %q", plan.Mode)
	}
	if plan.Effect == nil || plan.Effect.Op != "augment_card" {
		t.Fatalf("expected augment effect payload, got %#v", plan.Effect)
	}
	if len(plan.Indexes) == 0 {
		t.Fatal("expected candidate cards for shop augment service")
	}
}

func TestApplyShopServiceWithDeckChoiceConsumesOffer(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run, err := NewRun(lib, DefaultProfile(lib), ModeStory, "vanguard", 31)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Player.Gold = 200
	shop := ShopState{Offers: []ShopOffer{{
		ID:          "service-echo-workshop",
		Kind:        "service",
		Name:        "回响工坊",
		Description: "选择一张攻击牌，本局使其使用时额外抽 1 张牌。",
		Price:       66,
		ItemID:      "service_echo_workshop",
	}}}
	plan, err := ShopOfferDeckActionPlan(lib, run, &shop, "service-echo-workshop")
	if err != nil {
		t.Fatalf("ShopOfferDeckActionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("expected deck action plan")
	}
	startGold := run.Player.Gold
	if err := ApplyShopServiceWithDeckChoice(lib, run, &shop, "service-echo-workshop", plan.Indexes[0]); err != nil {
		t.Fatalf("ApplyShopServiceWithDeckChoice() error = %v", err)
	}
	if run.Player.Gold != startGold-66 {
		t.Fatalf("expected gold %d, got %d", startGold-66, run.Player.Gold)
	}
	if len(run.Player.Deck[plan.Indexes[0]].Augments) != 1 {
		t.Fatalf("expected chosen card to gain augment, got %#v", run.Player.Deck[plan.Indexes[0]].Augments)
	}
	if len(shop.Offers) != 0 {
		t.Fatalf("expected service offer to be consumed, got %d offers", len(shop.Offers))
	}
}

func TestShopServiceDefinitionsExposeFirstBuildPackage(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	tests := []struct {
		name       string
		classID    string
		wantItemID string
		wantScope  string
		wantTag    string
	}{
		{name: "draw attack", classID: "vanguard", wantItemID: "service_echo_workshop", wantScope: "run", wantTag: "attack"},
		{name: "combat energy", classID: "vanguard", wantItemID: "service_flash_workshop", wantScope: "combat", wantTag: ""},
		{name: "burn attack", classID: "vanguard", wantItemID: "service_ember_workshop", wantScope: "run", wantTag: "attack"},
		{name: "skill bastion", classID: "vanguard", wantItemID: "service_bastion_workshop", wantScope: "run", wantTag: "skill"},
		{name: "opening turn", classID: "vanguard", wantItemID: "service_opening_workshop", wantScope: "turn", wantTag: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			run, err := NewRun(lib, DefaultProfile(lib), ModeStory, tc.classID, 31)
			if err != nil {
				t.Fatalf("NewRun() error = %v", err)
			}
			run.Player.Gold = 200
			def, ok := shopServiceDefinitionByItemID(lib, run, tc.wantItemID)
			if !ok {
				t.Fatalf("expected shop service %q to be available", tc.wantItemID)
			}
			if def.Effect == nil || def.Effect.Op != "augment_card" {
				t.Fatalf("expected augment_card effect payload, got %#v", def.Effect)
			}
			if def.Effect.Scope != tc.wantScope {
				t.Fatalf("expected scope %q, got %#v", tc.wantScope, def.Effect)
			}
			if def.Effect.Tag != tc.wantTag {
				t.Fatalf("expected tag %q, got %#v", tc.wantTag, def.Effect)
			}
		})
	}
}
