package engine

import (
	"fmt"
	"slices"

	"cmdcards/internal/content"
)

func RunSmoke(lib *content.Library, profile Profile, mode GameMode, classID string, seed int64) (*SmokeResult, error) {
	run, err := NewRun(lib, profile, mode, classID, seed)
	if err != nil {
		return nil, err
	}
	result := &SmokeResult{
		Mode:    mode,
		ClassID: classID,
		Seed:    seed,
	}

	for steps := 0; steps < 300 && run.Status == RunStatusActive; steps++ {
		nodes := ReachableNodes(run)
		if len(nodes) == 0 {
			return nil, fmt.Errorf("no reachable nodes")
		}
		node := chooseNodeForSmoke(run, nodes)
		result.Log = append(result.Log, fmt.Sprintf("选择节点 A%dF%d %s", node.Act, node.Floor, NodeKindName(node.Kind)))

		switch node.Kind {
		case NodeMonster, NodeElite, NodeBoss:
			combat, err := StartEncounter(lib, run, node)
			if err != nil {
				return nil, err
			}
			if err := autoPlayCombat(lib, &run.Player, combat); err != nil {
				return nil, err
			}
			FinishCombat(lib, run, node, combat)
			rewardChoice := pickRewardCard(run, combat.Reward)
			takeEquipment := false
			if combat.Reward.EquipmentID != "" {
				offer, err := BuildEquipmentOffer(lib, run.Player, combat.Reward.EquipmentID, "reward", 0)
				if err != nil {
					return nil, err
				}
				takeEquipment = ShouldTakeEquipmentOffer(offer)
			}
			if err := ApplyCombatResultDecision(lib, run, node, combat, rewardChoice, takeEquipment); err != nil {
				return nil, err
			}
		case NodeEvent:
			eventState, err := StartEvent(lib, run, node)
			if err != nil {
				return nil, err
			}
			choice := chooseEventChoice(lib, run, eventState.Event)
			takeEquipment := true
			deckIndex := -1
			for _, item := range eventState.Event.Choices {
				if item.ID != choice {
					continue
				}
				if equipmentID := EventChoiceEquipmentID(item); equipmentID != "" {
					offer, err := BuildEquipmentOffer(lib, run.Player, equipmentID, "event", 0)
					if err != nil {
						return nil, err
					}
					takeEquipment = ShouldTakeEquipmentOffer(offer)
				}
				plan, err := EventChoiceDeckActionPlan(lib, run, item, takeEquipment)
				if err != nil {
					return nil, err
				}
				if plan != nil {
					deckIndex = chooseAugmentCardIndex(lib, run.Player.Deck, plan.Indexes)
					if deckIndex < 0 {
						return nil, fmt.Errorf("no valid deck card for event choice %s", choice)
					}
				}
				break
			}
			if err := ResolveEventDecisionWithDeckChoice(lib, run, &eventState, choice, takeEquipment, deckIndex); err != nil {
				return nil, err
			}
			if err := AdvanceNonCombatNode(run, node); err != nil {
				return nil, err
			}
		case NodeShop:
			shop := StartShop(lib, run)
			if run.Player.HP < run.Player.MaxHP*2/3 {
				for _, offer := range shop.Offers {
					if offer.Kind == "heal" && run.Player.Gold >= offer.Price {
						_ = ApplyShopPurchase(lib, run, &shop, offer.ID)
						break
					}
				}
			}
			for _, offer := range shop.Offers {
				if run.Player.Gold < offer.Price {
					continue
				}
				shouldBuy := offer.Kind == "heal" || (offer.Kind == "card" && len(run.Player.Deck) < 26)
				deckIndex := -1
				if offer.Kind == "equipment" {
					equipOffer, err := BuildEquipmentOffer(lib, run.Player, offer.ItemID, "shop", offer.Price)
					if err != nil {
						return nil, err
					}
					shouldBuy = ShouldTakeEquipmentOffer(equipOffer)
				}
				if offer.Kind == "remove" {
					shouldBuy = shouldBuyRemoval(lib, run)
				}
				if offer.Kind == "service" {
					plan, err := ShopOfferDeckActionPlan(lib, run, &shop, offer.ID)
					if err != nil {
						return nil, err
					}
					if plan != nil {
						shouldBuy, deckIndex = shouldBuyAugmentService(lib, run.Player.Deck, offer.Price, *plan)
					}
				}
				if shouldBuy {
					applyErr := error(nil)
					if offer.Kind == "equipment" {
						applyErr = ApplyShopEquipmentPurchase(lib, run, &shop, offer.ID, true)
					} else if offer.Kind == "remove" {
						applyErr = ApplyShopCardRemoval(lib, run, &shop, offer.ID, chooseRemovalCardIndex(lib, run.Player.Deck))
					} else if offer.Kind == "service" && deckIndex >= 0 {
						applyErr = ApplyShopServiceWithDeckChoice(lib, run, &shop, offer.ID, deckIndex)
					} else {
						applyErr = ApplyShopPurchase(lib, run, &shop, offer.ID)
					}
					if applyErr == nil {
						break
					}
				}
			}
			if err := AdvanceNonCombatNode(run, node); err != nil {
				return nil, err
			}
		case NodeRest:
			choice := "upgrade"
			if run.Player.HP < run.Player.MaxHP*2/3 {
				choice = "heal"
			}
			if choice == "upgrade" {
				if idx := chooseUpgradeCardIndex(lib, run.Player.Deck); idx >= 0 {
					if _, err := UpgradeDeckCard(lib, &run.Player, idx); err != nil {
						return nil, err
					}
				} else {
					if _, err := ResolveRest(lib, run, choice); err != nil {
						return nil, err
					}
				}
			} else {
				if _, err := ResolveRest(lib, run, choice); err != nil {
					return nil, err
				}
			}
			if err := AdvanceNonCombatNode(run, node); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported node kind %s", node.Kind)
		}
	}

	result.Result = run.Status
	result.ReachedAct = run.Act
	result.ClearedFloors = run.Stats.ClearedFloors
	result.FinalHP = run.Player.HP
	result.FinalGold = run.Player.Gold
	result.FinalDeckSize = len(run.Player.Deck)
	result.CombatsWon = run.Stats.CombatsWon
	return result, nil
}

func chooseNodeForSmoke(run *RunState, nodes []Node) Node {
	best := nodes[0]
	scoreBest := nodeSmokeScore(run, best)
	for _, node := range nodes[1:] {
		score := nodeSmokeScore(run, node)
		if score > scoreBest {
			best = node
			scoreBest = score
		}
	}
	return best
}

func nodeSmokeScore(run *RunState, node Node) int {
	score := 0
	switch node.Kind {
	case NodeBoss:
		score += 100
	case NodeElite:
		score += 26
	case NodeMonster:
		score += 20
	case NodeRest:
		score += 18
	case NodeShop:
		score += 12
	case NodeEvent:
		score += 10
	}
	if run.Player.HP < run.Player.MaxHP*3/4 && node.Kind == NodeRest {
		score += 28
	}
	if run.Player.HP < run.Player.MaxHP/2 && node.Kind == NodeRest {
		score += 18
	}
	if run.Player.HP < run.Player.MaxHP*2/3 && node.Kind == NodeElite {
		score -= 20
	}
	if run.Player.HP < run.Player.MaxHP/2 && node.Kind == NodeElite {
		score -= 25
	}
	if run.Player.HP < run.Player.MaxHP/2 && node.Kind == NodeShop && run.Player.Gold >= 24 {
		score += 10
	}
	if run.Player.HP < run.Player.MaxHP/3 && node.Kind == NodeElite {
		score -= 25
	}
	return score
}

func autoPlayCombat(lib *content.Library, player *PlayerState, combat *CombatState) error {
	for turns := 0; turns < 50 && !combat.Won && !combat.Lost; turns++ {
		StartPlayerTurn(lib, *player, combat)

		if potionIndex := choosePotionToUse(lib, *player, combat, turns); potionIndex >= 0 {
			potionID := player.Potions[potionIndex]
			if err := UsePotion(lib, *player, combat, potionID); err == nil {
				player.Potions = append(player.Potions[:potionIndex], player.Potions[potionIndex+1:]...)
			}
		}

		for {
			cardIndex := chooseCardToPlay(lib, combat)
			if cardIndex < 0 {
				break
			}
			if err := PlayCard(lib, *player, combat, cardIndex); err != nil {
				return err
			}
			if combat.Won || combat.Lost {
				break
			}
		}

		if combat.Won || combat.Lost {
			break
		}
		EndPlayerTurn(lib, *player, combat)
	}

	player.HP = combat.Player.HP
	if combat.Player.HP <= 0 {
		combat.Lost = true
	}
	return nil
}

func chooseCardToPlay(lib *content.Library, combat *CombatState) int {
	best := -1
	bestScore := -999
	for i, card := range combat.Hand {
		def := lib.Cards[card.ID]
		if slices.Contains(def.Flags, "unplayable") {
			continue
		}
		if def.Cost > combat.Player.Energy {
			continue
		}
		score := 0
		for _, effect := range activeEffectsForCard(lib, card) {
			switch effect.Op {
			case "damage":
				score += effect.Value * 3
			case "block":
				if nextEnemyAttackDamage(combat.Enemy.CurrentIntent) > 0 {
					score += effect.Value * 2
				} else {
					score += effect.Value
				}
			case "draw":
				score += effect.Value * 2
			case "apply_status":
				score += statusPlayScore(effect, combat)
			case "cleanse_status":
				score += cleanseScore(effect, combat)
			case "gain_energy":
				score += 4
			case "repeat_next_card":
				score += max(1, effect.Value) * 5
			case "reply":
				score += max(1, effect.Value) * 6
			}
		}
		if slices.Contains(def.Tags, "spell") {
			score++
			score += statusStacks(combat.Player.Statuses, "focus") * 2
		}
		if slices.Contains(def.Tags, "attack") {
			score += statusStacks(combat.Player.Statuses, "strength")
		}
		if score > bestScore {
			best = i
			bestScore = score
		}
	}
	return best
}

func activeEffectsForCard(lib *content.Library, card RuntimeCard) []content.Effect {
	return runtimeCardEffects(lib, card)
}

func nextEnemyAttackDamage(intent content.EnemyIntentDef) int {
	total := 0
	for _, effect := range intent.Effects {
		if effect.Op == "damage" {
			total += effect.Value
		}
	}
	return total
}

func pickRewardCard(run *RunState, reward RewardState) string {
	if len(reward.CardChoices) == 0 {
		return ""
	}
	best := reward.CardChoices[0]
	bestScore := rewardCardScore(best)
	for _, card := range reward.CardChoices[1:] {
		score := rewardCardScore(card)
		if score > bestScore {
			best = card
			bestScore = score
		}
	}
	return best.ID
}

func rewardCardScore(card content.CardDef) int {
	return cardEffectScore(card.Effects) - card.Cost
}

func cardEffectScore(effects []content.Effect) int {
	score := 0
	for _, effect := range effects {
		switch effect.Op {
		case "damage":
			score += effect.Value * 3
		case "block":
			score += effect.Value * 2
		case "draw":
			score += effect.Value * 2
		case "apply_status":
			score += statusRewardScore(effect)
		case "cleanse_status":
			score += 2
		case "gain_energy":
			score += 4
		case "lose_hp":
			score -= effect.Value * 3
		case "repeat_next_card":
			score += max(1, effect.Value) * 5
		case "reply":
			score += max(1, effect.Value) * 6
		}
	}
	return score
}

func statusPlayScore(effect content.Effect, combat *CombatState) int {
	switch effect.Status {
	case "vulnerable":
		return 6
	case "weak":
		return 5
	case "frail":
		if combat.Enemy.Block > 0 {
			return 8
		}
		return 5
	case "thorns":
		if nextEnemyAttackDamage(combat.Enemy.CurrentIntent) > 0 {
			return 8
		}
		return 4
	case "guard":
		return 6
	case "focus":
		return 7
	case "burn":
		return 7
	case "poison":
		return 7
	case "regen":
		if combat.Player.HP < combat.Player.MaxHP {
			return 6
		}
		return 3
	case "strength":
		return 6
	default:
		return 4
	}
}

func cleanseScore(effect content.Effect, combat *CombatState) int {
	if effect.Target != "self" && effect.Target != "" {
		return 1
	}
	switch effect.Status {
	case "weak", "vulnerable", "frail", "burn", "poison":
		if statusStacks(combat.Player.Statuses, effect.Status) > 0 {
			return 9
		}
	}
	return 1
}

func statusRewardScore(effect content.Effect) int {
	switch effect.Status {
	case "vulnerable":
		return 4
	case "weak":
		return 4
	case "frail":
		return 4
	case "thorns":
		return 5
	case "guard":
		return 5
	case "focus":
		return 6
	case "burn":
		return 6
	case "poison":
		return 6
	case "regen":
		return 5
	case "strength":
		return 5
	default:
		return 3
	}
}

func chooseUpgradeCardIndex(lib *content.Library, deck []DeckCard) int {
	best := -1
	bestGain := -999
	for _, idx := range UpgradableCardIndexes(lib, deck) {
		card := deck[idx]
		def := lib.Cards[card.CardID]
		gain := cardEffectScore(def.UpgradeEffects) - cardEffectScore(def.Effects)
		if def.Rarity == "starter" {
			gain++
		}
		if gain > bestGain {
			best = idx
			bestGain = gain
		}
	}
	return best
}

func chooseAugmentCardIndex(lib *content.Library, deck []DeckCard, candidateIndexes []int) int {
	best := -1
	bestScore := -999
	for _, idx := range candidateIndexes {
		if idx < 0 || idx >= len(deck) {
			continue
		}
		card := deck[idx]
		def, ok := lib.Cards[card.CardID]
		if !ok {
			continue
		}
		score := cardEffectScore(deckCardEffects(lib, card))*2 - def.Cost
		if card.Upgraded {
			score += 2
		}
		if slices.Contains(def.Tags, "attack") {
			score++
		}
		if score > bestScore {
			best = idx
			bestScore = score
		}
	}
	return best
}

func shouldBuyAugmentService(lib *content.Library, deck []DeckCard, price int, plan DeckActionPlan) (bool, int) {
	if plan.Effect == nil {
		return false, -1
	}
	bestIndex := chooseAugmentCardIndex(lib, deck, plan.Indexes)
	if bestIndex < 0 || bestIndex >= len(deck) {
		return false, -1
	}
	augment, err := CardAugmentFromEffect(*plan.Effect)
	if err != nil {
		return false, -1
	}
	score := max(4, cardEffectScore(deckCardEffects(lib, deck[bestIndex]))/2)
	score += cardEffectScore(augment.Effects)
	if augment.Scope == CardEffectScopeRun {
		score += 4
	} else if augment.Scope == CardEffectScopeCombat {
		score += 2
	}
	return score*8 >= price, bestIndex
}

func shouldBuyRemoval(lib *content.Library, run *RunState) bool {
	if len(run.Player.Deck) <= 12 {
		return false
	}
	idx := chooseRemovalCardIndex(lib, run.Player.Deck)
	if idx < 0 {
		return false
	}
	card := run.Player.Deck[idx]
	def := lib.Cards[card.CardID]
	return def.Rarity == "starter" || def.ClassID == "neutral"
}

func chooseRemovalCardIndex(lib *content.Library, deck []DeckCard) int {
	best := -1
	bestScore := 1 << 30
	for i, card := range deck {
		def := lib.Cards[card.CardID]
		score := cardEffectScore(def.Effects) + rarityRemovalBias(def.Rarity)
		if card.Upgraded {
			score += 8
		}
		if def.ClassID != "neutral" {
			score += 2
		}
		if score < bestScore {
			best = i
			bestScore = score
		}
	}
	return best
}

func rarityRemovalBias(rarity string) int {
	switch rarity {
	case "starter":
		return -8
	case "common":
		return 0
	case "uncommon":
		return 4
	case "rare":
		return 8
	default:
		return 10
	}
}

func chooseEventChoice(lib *content.Library, run *RunState, event content.EventDef) string {
	if run.Player.HP < run.Player.MaxHP/2 {
		for _, choice := range event.Choices {
			for _, effect := range choice.Effects {
				if effect.Op == "heal" {
					return choice.ID
				}
			}
		}
	}
	best := event.Choices[0]
	bestScore := -999
	for _, choice := range event.Choices {
		score := 0
		for _, effect := range choice.Effects {
			switch effect.Op {
			case "gain_gold":
				score += effect.Value / 8
			case "heal":
				score += effect.Value / 4
			case "gain_max_hp":
				score += effect.Value * 2
			case "gain_relic":
				score += 10
			case "gain_potion":
				score += 6
			case "upgrade_card":
				score += 7
			case "add_card":
				if card, ok := lib.Cards[effect.CardID]; ok {
					score += rewardCardScore(card)
				}
			case "gain_equipment":
				offer, err := BuildEquipmentOffer(lib, run.Player, effect.ItemID, "event", 0)
				if err == nil {
					score += offer.CandidateScore / 4
					if !ShouldTakeEquipmentOffer(offer) {
						score -= 4
					}
				}
			case "augment_card":
				indexes, err := augmentCardCandidateIndexes(lib, run.Player, effect)
				if err == nil && len(indexes) > 0 {
					augment, augmentErr := CardAugmentFromEffect(effect)
					bestIndex := chooseAugmentCardIndex(lib, run.Player.Deck, indexes)
					if augmentErr == nil && bestIndex >= 0 && bestIndex < len(run.Player.Deck) {
						score += max(4, cardEffectScore(deckCardEffects(lib, run.Player.Deck[bestIndex]))/2)
						score += cardEffectScore(augment.Effects)
						if augment.Scope == CardEffectScopeRun {
							score += 4
						}
					}
				}
			case "lose_hp":
				score -= effect.Value / 2
			}
		}
		if score > bestScore {
			best = choice
			bestScore = score
		}
	}
	return best.ID
}

func choosePotionToUse(lib *content.Library, player PlayerState, combat *CombatState, turns int) int {
	for i, potionID := range player.Potions {
		switch potionID {
		case "tonic_minor":
			if combat.Player.HP < combat.Player.MaxHP*3/5 {
				return i
			}
		case "potion_echo":
			if turns == 0 && len(combat.Hand) > 0 && combat.Enemy.HP > combat.Enemy.MaxHP/2 {
				return i
			}
		case "potion_battle_echo":
			if turns == 0 && handContainsTag(lib, combat.Hand, "attack") {
				return i
			}
		case "potion_arcane_echo":
			if turns == 0 && handContainsTag(lib, combat.Hand, "spell") {
				return i
			}
		case "potion_focus", "potion_fury":
			if turns == 0 && combat.Enemy.HP > combat.Enemy.MaxHP/2 {
				return i
			}
		default:
			if turns == 0 {
				return i
			}
		}
	}
	return -1
}

func handContainsTag(lib *content.Library, hand []RuntimeCard, tag string) bool {
	for _, card := range hand {
		def, ok := lib.Cards[card.ID]
		if !ok {
			continue
		}
		if slices.Contains(def.Tags, tag) {
			return true
		}
	}
	return false
}
