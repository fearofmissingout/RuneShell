package engine

import (
	"cmdcards/internal/content"
	"cmdcards/internal/i18n"
)

func EquipmentSlotName(slot string) string {
	return EquipmentSlotNameFor(i18n.DefaultLanguage, slot)
}

func CurrentEquipmentID(player PlayerState, slot string) string {
	switch slot {
	case "weapon":
		return player.Equipment.Weapon
	case "armor":
		return player.Equipment.Armor
	case "accessory":
		return player.Equipment.Accessory
	default:
		return ""
	}
}

func BuildEquipmentOffer(lib *content.Library, player PlayerState, equipmentID, source string, price int) (EquipmentOfferState, error) {
	item, ok := lib.Equipments[equipmentID]
	if !ok {
		return EquipmentOfferState{}, errUnknownEquipment(equipmentID)
	}
	currentID := CurrentEquipmentID(player, item.Slot)
	offer := EquipmentOfferState{
		Source:             source,
		EquipmentID:        equipmentID,
		Slot:               item.Slot,
		CurrentEquipmentID: currentID,
		Price:              price,
		CandidateScore:     EquipmentPowerScore(lib, player, equipmentID),
	}
	if currentID != "" {
		offer.CurrentScore = EquipmentPowerScore(lib, player, currentID)
	}
	return offer, nil
}

func EquipmentPowerScore(lib *content.Library, player PlayerState, equipmentID string) int {
	item, ok := lib.Equipments[equipmentID]
	if !ok {
		return 0
	}

	attackTags := deckTagCount(lib, player.Deck, "attack")
	spellTags := deckTagCount(lib, player.Deck, "spell")
	score := rarityScore(item.Rarity)

	for _, effect := range item.Effects {
		switch effect.Op {
		case "modify_damage":
			weight := 4
			switch effect.Trigger {
			case "attack":
				weight += 1 + attackTags/4
			case "spell":
				weight += 1 + spellTags/4
			default:
				weight += 1
			}
			score += effect.Value * weight
		case "modify_taken_damage":
			if effect.Value < 0 {
				score += -effect.Value * 6
			} else {
				score -= effect.Value * 4
			}
		case "block":
			weight := 2
			if isTurnStartWindow(effect.Trigger) {
				weight = 3
			}
			score += effect.Value * weight
		case "draw":
			weight := 5
			if isTurnStartWindow(effect.Trigger) {
				weight = 7
			}
			score += effect.Value * weight
		case "apply_status":
			score += equipmentStatusScore(effect)
		case "cleanse_status":
			score += 5 * max(1, effect.Value)
		case "heal":
			weight := 2
			if effect.Trigger == "player_turn_end" || effect.Trigger == "turn_end" {
				weight = 3
			}
			score += effect.Value * weight
		case "gain_energy":
			score += effect.Value * 9
		case "potion_capacity":
			score += effect.Value * 7
		case "repeat_next_card":
			score += max(1, effect.Value) * 8
		}
	}

	return score
}

func ShouldTakeEquipmentOffer(offer EquipmentOfferState) bool {
	if offer.CurrentEquipmentID == "" {
		return true
	}
	return offer.CandidateScore >= offer.CurrentScore+2
}

func deckTagCount(lib *content.Library, deck []DeckCard, tag string) int {
	count := 0
	for _, card := range deck {
		def, ok := lib.Cards[card.CardID]
		if !ok {
			continue
		}
		for _, item := range def.Tags {
			if item == tag {
				count++
				break
			}
		}
	}
	return count
}

func rarityScore(rarity string) int {
	switch rarity {
	case "legendary":
		return 12
	case "rare":
		return 8
	case "uncommon":
		return 4
	case "common":
		return 2
	default:
		return 0
	}
}

func equipmentStatusScore(effect content.Effect) int {
	stacks := max(1, effect.Value)
	base := 4
	switch effect.Status {
	case "strength":
		base = 6
	case "focus":
		base = 7
	case "guard":
		base = 6
	case "regen":
		base = 5
	case "thorns":
		base = 5
	case "vulnerable":
		base = 6
	case "weak":
		base = 5
	case "frail":
		base = 5
	case "burn":
		base = 6
	case "poison":
		base = 6
	}
	if effect.Trigger == "combat_start" {
		base++
	}
	return stacks * base
}

func isTurnStartWindow(trigger string) bool {
	return trigger == "turn_start" || trigger == "player_turn_start" || trigger == "enemy_turn_start"
}

func errUnknownEquipment(equipmentID string) error {
	return &unknownEquipmentError{id: equipmentID}
}

type unknownEquipmentError struct {
	id string
}

func (e *unknownEquipmentError) Error() string {
	return "unknown equipment " + e.id
}
