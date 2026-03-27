package storage

import (
	"os"
	"path/filepath"
	"testing"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
)

func TestStoreSaveAndLoad(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	dir := t.TempDir()
	store := NewStore(dir)

	profile := engine.DefaultProfile(lib)
	profile.MetaCurrency = 7
	if err := store.SaveProfile(profile); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	run, err := engine.NewRun(lib, profile, engine.ModeStory, "vanguard", 42)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	run.Checkpoint = &engine.RunCheckpoint{
		Screen: string("shop"),
		CurrentNode: &engine.Node{
			ID:    "act1-floor2-node0",
			Act:   1,
			Floor: 2,
			Index: 0,
			Kind:  engine.NodeShop,
		},
		ShopState: &engine.ShopState{
			Offers: []engine.ShopOffer{{ID: "remove-card", Kind: "remove", Price: 75}},
		},
		ShopOfferID:        "remove-card",
		DeckActionMode:     "shop_remove",
		DeckActionTitle:    "remove",
		DeckActionSubtitle: "price",
		DeckActionIndexes:  []int{0, 1},
		DeckActionPrice:    75,
		DeckActionEffect: &content.Effect{
			Op:       "augment_card",
			Name:     "opening_spark",
			Scope:    "turn",
			Selector: "choose_upgradable",
			Effects: []content.Effect{
				{Op: "draw", Value: 1},
			},
		},
		DeckActionTakeEquip: true,
	}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	loadedProfile, err := store.LoadProfile(lib)
	if err != nil {
		t.Fatalf("LoadProfile() error = %v", err)
	}
	if loadedProfile.MetaCurrency != 7 {
		t.Fatalf("expected meta currency 7, got %d", loadedProfile.MetaCurrency)
	}

	loadedRun, err := store.LoadRun()
	if err != nil {
		t.Fatalf("LoadRun() error = %v", err)
	}
	if loadedRun == nil || loadedRun.Player.ClassID != "vanguard" {
		t.Fatalf("expected saved run to load correctly, got %#v", loadedRun)
	}
	if loadedRun.Checkpoint == nil || loadedRun.Checkpoint.Screen != "shop" {
		t.Fatalf("expected checkpoint to round-trip, got %#v", loadedRun.Checkpoint)
	}
	if loadedRun.Checkpoint.ShopState == nil || len(loadedRun.Checkpoint.ShopState.Offers) != 1 {
		t.Fatalf("expected shop state to round-trip, got %#v", loadedRun.Checkpoint.ShopState)
	}
	if loadedRun.Checkpoint.DeckActionMode != "shop_remove" || loadedRun.Checkpoint.DeckActionPrice != 75 {
		t.Fatalf("expected deck action state to round-trip, got %#v", loadedRun.Checkpoint)
	}
	if loadedRun.Checkpoint.DeckActionEffect == nil || loadedRun.Checkpoint.DeckActionEffect.Scope != "turn" {
		t.Fatalf("expected deck action effect to round-trip, got %#v", loadedRun.Checkpoint.DeckActionEffect)
	}
	if !loadedRun.Checkpoint.DeckActionTakeEquip {
		t.Fatalf("expected deck action take-equipment flag to round-trip, got %#v", loadedRun.Checkpoint)
	}
}

func TestLoadProfileAppliesDefaultsForLegacyFile(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	dir := t.TempDir()
	store := NewStore(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	legacy := `{"unlocked_classes":["vanguard"],"meta_currency":3}`
	if err := os.WriteFile(filepath.Join(dir, "profile.json"), []byte(legacy), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	profile, err := store.LoadProfile(lib)
	if err != nil {
		t.Fatalf("LoadProfile() error = %v", err)
	}
	if profile.Version != engine.CurrentProfileVersion {
		t.Fatalf("expected default version %d, got %d", engine.CurrentProfileVersion, profile.Version)
	}
	if profile.Perks == nil || profile.ContentUnlocks == nil {
		t.Fatal("expected default maps to be initialized")
	}
	if len(profile.UnlockedEquipments) == 0 {
		t.Fatal("expected unlocked equipments to be initialized")
	}
	if len(profile.ClassLoadouts) == 0 {
		t.Fatal("expected class loadouts to be initialized")
	}
}
