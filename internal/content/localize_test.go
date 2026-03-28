package content

import (
	"strings"
	"testing"

	"cmdcards/internal/i18n"
)

func TestLocalizeLibraryEnglishContent(t *testing.T) {
	lib, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	en := LocalizeLibrary(lib, i18n.LangEnUS)
	if en.Language != i18n.LangEnUS {
		t.Fatalf("expected English library language, got %q", en.Language)
	}
	if got := en.Classes["vanguard"].Name; got != "Vanguard" {
		t.Fatalf("expected Vanguard class name, got %q", got)
	}
	if got := en.Cards["slash"].Name; got != "Slash" {
		t.Fatalf("expected Slash card name, got %q", got)
	}
	if got := en.Cards["slash"].Description; got != "Deal 6 damage" {
		t.Fatalf("expected localized slash description, got %q", got)
	}
	if got := en.Relics["war_banner"].Description; got != "Gain Strength 1" {
		t.Fatalf("expected localized relic description, got %q", got)
	}
	if got := en.Events["ancient_forge"].Name; got != "Ancient Forge" {
		t.Fatalf("expected localized event name, got %q", got)
	}
	if !strings.Contains(en.Events["ancient_forge"].Description, "furnace") {
		t.Fatalf("expected localized event description, got %q", en.Events["ancient_forge"].Description)
	}
	if got := en.Encounters["act1_raider"].Name; got != "Rift Raider" {
		t.Fatalf("expected localized encounter name, got %q", got)
	}
	if !strings.Contains(en.Encounters["act1_raider"].IntentCycle[0].Description, "Deal") {
		t.Fatalf("expected localized intent description, got %q", en.Encounters["act1_raider"].IntentCycle[0].Description)
	}
}
