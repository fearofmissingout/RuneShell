package engine

import (
	"fmt"
	"slices"
	"strings"

	"cmdcards/internal/content"
)

type CardEffectScope string

const (
	CardEffectScopeRun    CardEffectScope = "run"
	CardEffectScopeCombat CardEffectScope = "combat"
	CardEffectScopeTurn   CardEffectScope = "turn"
)

type CombatCardPile string

const (
	CombatCardPileHand    CombatCardPile = "hand"
	CombatCardPileDraw    CombatCardPile = "draw"
	CombatCardPileDiscard CombatCardPile = "discard"
	CombatCardPileExhaust CombatCardPile = "exhaust"
)

type CardAugment struct {
	Name    string           `json:"name,omitempty"`
	Scope   CardEffectScope  `json:"scope,omitempty"`
	Effects []content.Effect `json:"effects,omitempty"`
}

const (
	CardAugmentSelectorChoose           = "choose"
	CardAugmentSelectorAll              = "all"
	CardAugmentSelectorChooseUpgradable = "choose_upgradable"
	CardAugmentSelectorAllUpgradable    = "all_upgradable"
)

func NewCardAugment(name string, scope CardEffectScope, effects ...content.Effect) CardAugment {
	return CardAugment{
		Name:    name,
		Scope:   scope,
		Effects: cloneEffects(effects),
	}
}

func ReplyCardAugment(scope CardEffectScope, count int) CardAugment {
	if count <= 0 {
		count = 1
	}
	return NewCardAugment("reply", scope, content.Effect{
		Op:    "reply",
		Value: count,
	})
}

func CardAugmentFromEffect(effect content.Effect) (CardAugment, error) {
	augment := NewCardAugment(effect.Name, CardEffectScope(effect.Scope), effect.Effects...)
	augment = normalizeCardAugment(augment)
	if augment.Name == "" {
		augment.Name = effect.Op
	}
	switch augment.Scope {
	case CardEffectScopeRun, CardEffectScopeCombat, CardEffectScopeTurn:
	default:
		return CardAugment{}, fmt.Errorf("invalid augment scope %q", augment.Scope)
	}
	return augment, nil
}

func AddRunCardAugment(player *PlayerState, deckIndex int, augment CardAugment) error {
	if player == nil {
		return fmt.Errorf("player is nil")
	}
	if deckIndex < 0 || deckIndex >= len(player.Deck) {
		return fmt.Errorf("invalid deck index %d", deckIndex)
	}
	augment = normalizeCardAugment(augment)
	player.Deck[deckIndex].Augments = append(player.Deck[deckIndex].Augments, augment)
	return nil
}

func AddCombatCardAugment(combat *CombatState, seatIndex int, pile CombatCardPile, cardIndex int, augment CardAugment) error {
	card := combatCardAt(combat, seatIndex, pile, cardIndex)
	if card == nil {
		return fmt.Errorf("invalid combat card target %s[%d]", pile, cardIndex)
	}
	augment = normalizeCardAugment(augment)
	card.Augments = append(card.Augments, augment)
	syncLegacySeat0(combat)
	return nil
}

func normalizeCardAugment(augment CardAugment) CardAugment {
	if augment.Scope == "" {
		augment.Scope = CardEffectScopeRun
	}
	augment.Effects = cloneEffects(augment.Effects)
	return augment
}

func cloneDeckCards(cards []DeckCard) []DeckCard {
	if len(cards) == 0 {
		return nil
	}
	out := make([]DeckCard, 0, len(cards))
	for _, card := range cards {
		out = append(out, DeckCard{
			CardID:   card.CardID,
			Upgraded: card.Upgraded,
			Augments: cloneCardAugments(card.Augments),
		})
	}
	return out
}

func cloneCardAugments(augments []CardAugment) []CardAugment {
	if len(augments) == 0 {
		return nil
	}
	out := make([]CardAugment, 0, len(augments))
	for _, augment := range augments {
		out = append(out, CardAugment{
			Name:    augment.Name,
			Scope:   augment.Scope,
			Effects: cloneEffects(augment.Effects),
		})
	}
	return out
}

func cloneEffects(effects []content.Effect) []content.Effect {
	if len(effects) == 0 {
		return nil
	}
	out := make([]content.Effect, 0, len(effects))
	for _, effect := range effects {
		copied := effect
		copied.Flags = append([]string{}, effect.Flags...)
		out = append(out, copied)
	}
	return out
}

func PreparePlayerForCombat(player PlayerState) (PlayerState, PlayerState) {
	combatPlayer := player
	persistedPlayer := player
	combatPlayer.Deck = cloneDeckCards(player.Deck)
	persistedPlayer.Deck = cloneDeckCards(player.Deck)
	for i := range persistedPlayer.Deck {
		persistedPlayer.Deck[i].Augments = filterPersistentDeckAugments(persistedPlayer.Deck[i].Augments)
	}
	return combatPlayer, persistedPlayer
}

func filterPersistentDeckAugments(augments []CardAugment) []CardAugment {
	if len(augments) == 0 {
		return nil
	}
	filtered := make([]CardAugment, 0, len(augments))
	for _, augment := range augments {
		if augment.Scope != CardEffectScopeRun {
			continue
		}
		filtered = append(filtered, CardAugment{
			Name:    augment.Name,
			Scope:   augment.Scope,
			Effects: cloneEffects(augment.Effects),
		})
	}
	return filtered
}

func BuildAugmentDeckActionPlan(lib *content.Library, player PlayerState, effect content.Effect, sourceLabel string, takeEquipment bool) (*DeckActionPlan, error) {
	selector := normalizeAugmentSelector(effect.Selector)
	if selector != CardAugmentSelectorChoose && selector != CardAugmentSelectorChooseUpgradable {
		return nil, nil
	}
	indexes, err := augmentCardCandidateIndexes(lib, player, effect)
	if err != nil {
		return nil, err
	}
	if len(indexes) == 0 {
		return nil, fmt.Errorf("no valid cards for augment selection")
	}
	augment, err := CardAugmentFromEffect(effect)
	if err != nil {
		return nil, err
	}
	summary := DescribeEffects(lib, augment.Effects)
	title := "选择要附加效果的卡牌"
	subtitle := fmt.Sprintf("%s -> %s（%s）", sourceLabel, summary, cardAugmentScopeLabel(augment.Scope))
	if effect.Tag != "" {
		subtitle += fmt.Sprintf("，仅限 %s 牌", effect.Tag)
	}
	return &DeckActionPlan{
		Mode:          "event_augment_card",
		Title:         title,
		Subtitle:      subtitle,
		Indexes:       indexes,
		Effect:        cloneEffectPtr(effect),
		TakeEquipment: takeEquipment,
	}, nil
}

func ApplyDeckCardAugmentEffect(lib *content.Library, player *PlayerState, effect content.Effect, deckIndex int) ([]string, error) {
	if player == nil {
		return nil, fmt.Errorf("player is nil")
	}
	augment, err := CardAugmentFromEffect(effect)
	if err != nil {
		return nil, err
	}
	indexes, err := augmentCardCandidateIndexes(lib, *player, effect)
	if err != nil {
		return nil, err
	}
	if len(indexes) == 0 {
		return nil, fmt.Errorf("no valid cards for augment")
	}
	selector := normalizeAugmentSelector(effect.Selector)
	if selector == CardAugmentSelectorChoose || selector == CardAugmentSelectorChooseUpgradable {
		if len(indexes) == 1 && deckIndex < 0 {
			deckIndex = indexes[0]
		}
		if !containsInt(indexes, deckIndex) {
			return nil, fmt.Errorf("invalid augment deck index %d", deckIndex)
		}
		if err := AddRunCardAugment(player, deckIndex, augment); err != nil {
			return nil, err
		}
		return []string{fmt.Sprintf("%s 获得效果：%s（%s）", CardStateName(lib, player.Deck[deckIndex].CardID, player.Deck[deckIndex].Upgraded), DescribeEffects(lib, augment.Effects), cardAugmentScopeLabel(augment.Scope))}, nil
	}
	logs := make([]string, 0, len(indexes))
	for _, index := range indexes {
		if err := AddRunCardAugment(player, index, augment); err != nil {
			return nil, err
		}
		logs = append(logs, fmt.Sprintf("%s 获得效果：%s（%s）", CardStateName(lib, player.Deck[index].CardID, player.Deck[index].Upgraded), DescribeEffects(lib, augment.Effects), cardAugmentScopeLabel(augment.Scope)))
	}
	return logs, nil
}

func augmentCardCandidateIndexes(lib *content.Library, player PlayerState, effect content.Effect) ([]int, error) {
	selector := normalizeAugmentSelector(effect.Selector)
	indexes := make([]int, 0, len(player.Deck))
	for i, card := range player.Deck {
		def, ok := lib.Cards[card.CardID]
		if !ok {
			continue
		}
		if effect.Tag != "" && !slices.Contains(def.Tags, effect.Tag) {
			continue
		}
		if selector == CardAugmentSelectorChooseUpgradable || selector == CardAugmentSelectorAllUpgradable {
			if card.Upgraded || len(def.UpgradeEffects) == 0 {
				continue
			}
		}
		indexes = append(indexes, i)
	}
	if selector == CardAugmentSelectorChoose || selector == CardAugmentSelectorAll || selector == CardAugmentSelectorChooseUpgradable || selector == CardAugmentSelectorAllUpgradable {
		return indexes, nil
	}
	return nil, fmt.Errorf("unsupported augment selector %q", effect.Selector)
}

func normalizeAugmentSelector(selector string) string {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return CardAugmentSelectorChoose
	}
	return selector
}

func cardAugmentScopeLabel(scope CardEffectScope) string {
	switch scope {
	case CardEffectScopeCombat:
		return "下场战斗"
	case CardEffectScopeTurn:
		return "下场战斗的本回合"
	default:
		return "本局"
	}
}

func cloneEffectPtr(effect content.Effect) *content.Effect {
	cloned := effect
	cloned.Flags = append([]string{}, effect.Flags...)
	cloned.Effects = cloneEffects(effect.Effects)
	return &cloned
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func runtimeCardEffects(lib *content.Library, card RuntimeCard) []content.Effect {
	base := baseCardEffects(lib, card.ID, card.Upgraded)
	for _, augment := range card.Augments {
		base = append(base, augment.Effects...)
	}
	return base
}

func deckCardEffects(lib *content.Library, card DeckCard) []content.Effect {
	base := baseCardEffects(lib, card.CardID, card.Upgraded)
	for _, augment := range card.Augments {
		base = append(base, augment.Effects...)
	}
	return base
}

func RuntimeCardStateSummary(lib *content.Library, card RuntimeCard) string {
	return DescribeEffects(lib, runtimeCardEffects(lib, card))
}

func DeckCardStateSummary(lib *content.Library, card DeckCard) string {
	return DescribeEffects(lib, deckCardEffects(lib, card))
}

func baseCardEffects(lib *content.Library, cardID string, upgraded bool) []content.Effect {
	def := lib.Cards[cardID]
	if upgraded && len(def.UpgradeEffects) > 0 {
		return cloneEffects(def.UpgradeEffects)
	}
	return cloneEffects(def.Effects)
}

func pruneTurnCardAugments(combat *CombatState) {
	ensureCombatSeats(combat)
	if combat == nil {
		return
	}
	for i := range combat.Seats {
		pruneTurnAugmentsFromPile(combat.Seats[i].Hand)
		pruneTurnAugmentsFromPile(combat.Seats[i].DrawPile)
		pruneTurnAugmentsFromPile(combat.Seats[i].Discard)
		pruneTurnAugmentsFromPile(combat.Seats[i].Exhaust)
	}
	syncLegacySeat0(combat)
}

func pruneTurnAugmentsFromPile(pile []RuntimeCard) {
	for i := range pile {
		filtered := pile[i].Augments[:0]
		for _, augment := range pile[i].Augments {
			if augment.Scope == CardEffectScopeTurn {
				continue
			}
			filtered = append(filtered, augment)
		}
		pile[i].Augments = filtered
	}
}

func combatCardAt(combat *CombatState, seatIndex int, pile CombatCardPile, cardIndex int) *RuntimeCard {
	seat := combatSeat(combat, seatIndex)
	if seat == nil || cardIndex < 0 {
		return nil
	}
	switch pile {
	case CombatCardPileHand:
		if cardIndex >= len(seat.Hand) {
			return nil
		}
		return &seat.Hand[cardIndex]
	case CombatCardPileDraw:
		if cardIndex >= len(seat.DrawPile) {
			return nil
		}
		return &seat.DrawPile[cardIndex]
	case CombatCardPileDiscard:
		if cardIndex >= len(seat.Discard) {
			return nil
		}
		return &seat.Discard[cardIndex]
	case CombatCardPileExhaust:
		if cardIndex >= len(seat.Exhaust) {
			return nil
		}
		return &seat.Exhaust[cardIndex]
	default:
		return nil
	}
}
