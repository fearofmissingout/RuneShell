package content

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"

	"cmdcards/internal/i18n"
)

var validOps = map[string]struct{}{
	"damage":              {},
	"block":               {},
	"draw":                {},
	"apply_status":        {},
	"cleanse_status":      {},
	"modify_damage":       {},
	"modify_taken_damage": {},
	"gain_gold":           {},
	"lose_hp":             {},
	"heal":                {},
	"gain_max_hp":         {},
	"gain_relic":          {},
	"gain_equipment":      {},
	"gain_potion":         {},
	"upgrade_relic":       {},
	"upgrade_card":        {},
	"add_card":            {},
	"gain_energy":         {},
	"potion_capacity":     {},
	"repeat_next_card":    {},
	"reply":               {},
	"augment_card":        {},
	"add_combat_card":     {},
}

func LoadEmbedded() (*Library, error) {
	return LoadFS(embeddedFiles)
}

func LoadFS(files fs.FS) (*Library, error) {
	lib := &Library{
		Language:   i18n.DefaultLanguage,
		Classes:    map[string]ClassDef{},
		Cards:      map[string]CardDef{},
		Relics:     map[string]RelicDef{},
		Potions:    map[string]PotionDef{},
		Equipments: map[string]EquipmentDef{},
		Encounters: map[string]EncounterDef{},
		Events:     map[string]EventDef{},
	}

	var classes []ClassDef
	if err := decodeFile(files, "assets/classes.json", &classes); err != nil {
		return nil, err
	}
	for _, item := range classes {
		if item.ID == "" {
			return nil, fmt.Errorf("classes: empty id")
		}
		if _, exists := lib.Classes[item.ID]; exists {
			return nil, fmt.Errorf("classes: duplicate id %q", item.ID)
		}
		lib.Classes[item.ID] = item
		lib.classOrder = append(lib.classOrder, item.ID)
	}

	var cards []CardDef
	if err := decodeFile(files, "assets/cards.json", &cards); err != nil {
		return nil, err
	}
	for _, item := range cards {
		if item.ID == "" {
			return nil, fmt.Errorf("cards: empty id")
		}
		if _, exists := lib.Cards[item.ID]; exists {
			return nil, fmt.Errorf("cards: duplicate id %q", item.ID)
		}
		if item.ClassID != "neutral" {
			if _, exists := lib.Classes[item.ClassID]; !exists {
				return nil, fmt.Errorf("cards: unknown class %q for %q", item.ClassID, item.ID)
			}
		}
		if err := validateEffects("card "+item.ID, item.Effects); err != nil {
			return nil, err
		}
		if err := validateEffects("card "+item.ID+" upgrade", item.UpgradeEffects); err != nil {
			return nil, err
		}
		lib.Cards[item.ID] = item
		lib.cardOrder = append(lib.cardOrder, item.ID)
	}

	var relics []RelicDef
	if err := decodeFile(files, "assets/relics.json", &relics); err != nil {
		return nil, err
	}
	for _, item := range relics {
		if _, exists := lib.Relics[item.ID]; exists {
			return nil, fmt.Errorf("relics: duplicate id %q", item.ID)
		}
		if err := validateEffects("relic "+item.ID, item.Effects); err != nil {
			return nil, err
		}
		lib.Relics[item.ID] = item
		lib.relicOrder = append(lib.relicOrder, item.ID)
	}

	var potions []PotionDef
	if err := decodeFile(files, "assets/potions.json", &potions); err != nil {
		return nil, err
	}
	for _, item := range potions {
		if _, exists := lib.Potions[item.ID]; exists {
			return nil, fmt.Errorf("potions: duplicate id %q", item.ID)
		}
		if err := validateEffects("potion "+item.ID, item.Effects); err != nil {
			return nil, err
		}
		lib.Potions[item.ID] = item
		lib.potionOrder = append(lib.potionOrder, item.ID)
	}

	var equipments []EquipmentDef
	if err := decodeFile(files, "assets/equipments.json", &equipments); err != nil {
		return nil, err
	}
	for _, item := range equipments {
		if _, exists := lib.Equipments[item.ID]; exists {
			return nil, fmt.Errorf("equipments: duplicate id %q", item.ID)
		}
		if item.Slot == "" {
			return nil, fmt.Errorf("equipments: %q missing slot", item.ID)
		}
		if err := validateEffects("equipment "+item.ID, item.Effects); err != nil {
			return nil, err
		}
		lib.Equipments[item.ID] = item
		lib.equipmentOrder = append(lib.equipmentOrder, item.ID)
	}

	var encounters []EncounterDef
	if err := decodeFile(files, "assets/encounters.json", &encounters); err != nil {
		return nil, err
	}
	for _, item := range encounters {
		if _, exists := lib.Encounters[item.ID]; exists {
			return nil, fmt.Errorf("encounters: duplicate id %q", item.ID)
		}
		if item.Kind == "" || item.Act < 1 {
			return nil, fmt.Errorf("encounters: invalid encounter %q", item.ID)
		}
		if len(item.IntentCycle) == 0 {
			return nil, fmt.Errorf("encounters: %q has no intent cycle", item.ID)
		}
		if err := validateEffects("encounter "+item.ID+" passives", item.Passives); err != nil {
			return nil, err
		}
		for _, intent := range item.IntentCycle {
			if err := validateEffects("encounter "+item.ID+" intent "+intent.Name, intent.Effects); err != nil {
				return nil, err
			}
		}
		lib.Encounters[item.ID] = item
		lib.encounterOrder = append(lib.encounterOrder, item.ID)
	}

	var events []EventDef
	if err := decodeFile(files, "assets/events.json", &events); err != nil {
		return nil, err
	}
	for _, item := range events {
		if _, exists := lib.Events[item.ID]; exists {
			return nil, fmt.Errorf("events: duplicate id %q", item.ID)
		}
		if len(item.Choices) == 0 {
			return nil, fmt.Errorf("events: %q has no choices", item.ID)
		}
		for _, choice := range item.Choices {
			if err := validateEffects("event "+item.ID+" choice "+choice.ID, choice.Effects); err != nil {
				return nil, err
			}
		}
		lib.Events[item.ID] = item
		lib.eventOrder = append(lib.eventOrder, item.ID)
	}

	for _, class := range lib.Classes {
		if class.BaseHP <= 0 || class.MaxEnergy <= 0 {
			return nil, fmt.Errorf("class %q invalid stats", class.ID)
		}
		if len(class.StartingDeck) == 0 {
			return nil, fmt.Errorf("class %q missing starting deck", class.ID)
		}
		for _, id := range class.StartingDeck {
			card, ok := lib.Cards[id]
			if !ok {
				return nil, fmt.Errorf("class %q starting deck unknown card %q", class.ID, id)
			}
			if card.ClassID != "neutral" && card.ClassID != class.ID {
				return nil, fmt.Errorf("class %q starting deck invalid card %q", class.ID, id)
			}
		}
		for _, id := range class.CardPool {
			card, ok := lib.Cards[id]
			if !ok {
				return nil, fmt.Errorf("class %q card pool unknown card %q", class.ID, id)
			}
			if card.ClassID != class.ID {
				return nil, fmt.Errorf("class %q card pool invalid card %q", class.ID, id)
			}
		}
		for _, id := range class.StartingRelics {
			if _, ok := lib.Relics[id]; !ok {
				return nil, fmt.Errorf("class %q unknown starting relic %q", class.ID, id)
			}
		}
		for _, id := range class.StartingPotions {
			if _, ok := lib.Potions[id]; !ok {
				return nil, fmt.Errorf("class %q unknown starting potion %q", class.ID, id)
			}
		}
		for _, id := range class.StartingEquipment {
			if _, ok := lib.Equipments[id]; !ok {
				return nil, fmt.Errorf("class %q unknown starting equipment %q", class.ID, id)
			}
		}
		if len(class.CardPool) < 10 {
			return nil, fmt.Errorf("class %q must provide at least 10 class cards", class.ID)
		}
	}

	return lib, nil
}

func decodeFile(files fs.FS, name string, out any) error {
	data, err := fs.ReadFile(files, name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}

func validateEffects(scope string, effects []Effect) error {
	for _, effect := range effects {
		if effect.Op == "" {
			return fmt.Errorf("%s: effect missing op", scope)
		}
		if _, ok := validOps[effect.Op]; !ok {
			return fmt.Errorf("%s: unsupported op %q", scope, effect.Op)
		}
		if effect.Op == "gain_relic" || effect.Op == "gain_equipment" || effect.Op == "gain_potion" || effect.Op == "add_card" {
			if effect.ItemID == "" && effect.CardID == "" {
				return fmt.Errorf("%s: %s requires item/card id", scope, effect.Op)
			}
		}
		if effect.Op == "upgrade_relic" && (effect.ItemID == "" || effect.ResultID == "") {
			return fmt.Errorf("%s: upgrade_relic requires item_id and result_id", scope)
		}
		if effect.Op == "add_combat_card" {
			if effect.CardID == "" {
				return fmt.Errorf("%s: add_combat_card requires card_id", scope)
			}
			switch effect.ItemType {
			case "", "hand", "draw", "discard":
			default:
				return fmt.Errorf("%s: add_combat_card has invalid item_type %q", scope, effect.ItemType)
			}
		}
		if effect.Op == "augment_card" {
			if len(effect.Effects) == 0 {
				return fmt.Errorf("%s: augment_card requires nested effects", scope)
			}
			switch effect.Scope {
			case "", "run", "combat", "turn":
			default:
				return fmt.Errorf("%s: augment_card has invalid scope %q", scope, effect.Scope)
			}
			switch effect.Selector {
			case "", "choose", "all", "choose_upgradable", "all_upgradable":
			default:
				return fmt.Errorf("%s: augment_card has invalid selector %q", scope, effect.Selector)
			}
			if err := validateEffects(scope+": augment_card", effect.Effects); err != nil {
				return err
			}
		}
		if (effect.Op == "apply_status" || effect.Op == "cleanse_status") && effect.Status == "" {
			return fmt.Errorf("%s: %s requires status", scope, effect.Op)
		}
	}
	return nil
}

func (l *Library) CardsForClass(classID string) []CardDef {
	ids := slices.Clone(l.Classes[classID].CardPool)
	out := make([]CardDef, 0, len(ids))
	for _, id := range ids {
		out = append(out, l.Cards[id])
	}
	return out
}

func (l *Library) NeutralCards() []CardDef {
	out := []CardDef{}
	for _, id := range l.cardOrder {
		card := l.Cards[id]
		if card.ClassID == "neutral" {
			out = append(out, card)
		}
	}
	return out
}
