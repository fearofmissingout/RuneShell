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
