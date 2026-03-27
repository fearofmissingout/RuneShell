package engine

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	"cmdcards/internal/content"
)

func NewRun(lib *content.Library, profile Profile, mode GameMode, classID string, seed int64) (*RunState, error) {
	class, ok := lib.Classes[classID]
	if !ok {
		return nil, fmt.Errorf("unknown class %q", classID)
	}
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	player := PlayerState{
		ClassID:        class.ID,
		Name:           class.Name,
		MaxHP:          class.BaseHP + profile.Perks["bonus_max_hp"],
		HP:             class.BaseHP + profile.Perks["bonus_max_hp"],
		MaxEnergy:      class.MaxEnergy,
		Gold:           class.BaseGold + profile.Perks["bonus_start_gold"],
		Deck:           make([]DeckCard, 0, len(class.StartingDeck)),
		Relics:         slices.Clone(class.StartingRelics),
		Potions:        slices.Clone(class.StartingPotions),
		PotionCapacity: 2 + profile.Perks["extra_potion_slot"],
		PermanentStats: map[string]int{
			"combat_start_block": profile.Perks["bonus_start_block"],
			"rest_heal_bonus":    profile.Perks["bonus_rest_heal"],
		},
	}
	for _, id := range class.StartingDeck {
		player.Deck = append(player.Deck, DeckCard{CardID: id})
	}
	for _, eqID := range EffectiveStartingEquipment(lib, profile, class) {
		if _, err := equipItem(lib, &player, eqID); err != nil {
			return nil, err
		}
	}
	mapGraph := GenerateActMap(seed, 1, nil)
	if err := ValidateMapConstraints(mapGraph); err != nil {
		return nil, err
	}
	run := &RunState{
		Version:      1,
		Mode:         mode,
		Seed:         seed,
		Act:          1,
		CurrentFloor: 0,
		PartySize:    1,
		Status:       RunStatusActive,
		Player:       player,
		Map:          mapGraph,
		Reachable:    nodeIDs(mapGraph.Floors[0]),
		History:      []string{},
	}
	return run, nil
}

func nodeIDs(nodes []Node) []string {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.ID)
	}
	return out
}

func ReachableNodes(run *RunState) []Node {
	index := FlattenNodes(run.Map)
	out := make([]Node, 0, len(run.Reachable))
	for _, id := range run.Reachable {
		if node, ok := index[id]; ok {
			out = append(out, node)
		}
	}
	return out
}

func SelectNode(run *RunState, nodeID string) (Node, error) {
	index := FlattenNodes(run.Map)
	if _, ok := index[nodeID]; !ok {
		return Node{}, fmt.Errorf("unknown node %q", nodeID)
	}
	if !slices.Contains(run.Reachable, nodeID) {
		return Node{}, fmt.Errorf("node %q not reachable", nodeID)
	}
	return index[nodeID], nil
}

func StartEncounter(lib *content.Library, run *RunState, node Node) (*CombatState, error) {
	var kind string
	switch node.Kind {
	case NodeMonster:
		kind = "monster"
	case NodeElite:
		kind = "elite"
	case NodeBoss:
		kind = "boss"
	default:
		return nil, fmt.Errorf("node %s is not a combat node", node.ID)
	}
	encounters := buildEncounterGroup(lib, run, kind, node.Act, node.Floor, node.Index)
	if len(encounters) == 0 {
		return nil, fmt.Errorf("no encounters available for %s act %d", kind, node.Act)
	}
	rewardBasis := aggregateEncounterGroup(encounters, kind, node.Act)
	combatPlayer, persistedPlayer := PreparePlayerForCombat(run.Player)
	run.Player = persistedPlayer
	return NewCombatWithEnemies(lib, combatPlayer, encounters, rewardBasis, run.Seed+int64(node.Floor*101+node.Index)), nil
}

func randomEncounter(lib *content.Library, run *RunState, kind string, act int) content.EncounterDef {
	baseAct := act
	if baseAct > 3 {
		baseAct = 3
	}
	matches := make([]content.EncounterDef, 0)
	for _, encounter := range lib.Encounters {
		if encounter.Kind == kind && encounter.Act == baseAct {
			matches = append(matches, encounter)
		}
	}
	rng := rand.New(rand.NewSource(run.Seed + int64(act*1000+run.CurrentFloor*17)))
	if len(matches) == 0 {
		return content.EncounterDef{}
	}
	encounter := matches[rng.Intn(len(matches))]
	return scaleEncounterForAct(encounter, act)
}

func buildEncounterGroup(lib *content.Library, run *RunState, kind string, act, floor, index int) []content.EncounterDef {
	groupSize := encounterGroupSize(run.Seed, kind, act, floor, index)
	encounters := make([]content.EncounterDef, 0, groupSize)
	for i := 0; i < groupSize; i++ {
		encounter := randomEncounter(lib, run, kind, act)
		if encounter.ID == "" {
			break
		}
		encounter.Name = encounterGroupName(encounter.Name, i)
		if kind == "boss" {
			encounters = append(encounters, encounter)
			break
		}
		if i > 0 {
			encounter = shrinkEncounterForGroupSlot(encounter, i)
		}
		encounters = append(encounters, encounter)
	}
	return encounters
}

func encounterGroupSize(seed int64, kind string, act, floor, index int) int {
	rng := rand.New(rand.NewSource(seed + int64(act*3001+floor*41+index*11)))
	switch kind {
	case "boss":
		return 1
	case "elite":
		if act >= 3 && rng.Intn(100) < 35 {
			return 2
		}
		return 1
	default:
		maxCount := 2
		if act >= 3 {
			maxCount = 3
		}
		return 1 + rng.Intn(maxCount)
	}
}

func shrinkEncounterForGroupSlot(encounter content.EncounterDef, slot int) content.EncounterDef {
	encounter.HP = max(1, encounter.HP-(slot*8+4))
	encounter.GoldReward = max(4, encounter.GoldReward-4)
	for i := range encounter.IntentCycle {
		for j := range encounter.IntentCycle[i].Effects {
			switch encounter.IntentCycle[i].Effects[j].Op {
			case "damage":
				encounter.IntentCycle[i].Effects[j].Value = max(1, encounter.IntentCycle[i].Effects[j].Value-(slot*2+1))
			case "block", "heal":
				encounter.IntentCycle[i].Effects[j].Value = max(1, encounter.IntentCycle[i].Effects[j].Value-(slot+1))
			}
		}
	}
	return encounter
}

func aggregateEncounterGroup(encounters []content.EncounterDef, kind string, act int) content.EncounterDef {
	if len(encounters) == 0 {
		return content.EncounterDef{}
	}
	basis := encounters[0]
	basis.Name = strings.Join(encounterNames(encounters), " / ")
	basis.HP = 0
	basis.GoldReward = 0
	basis.CardReward = 0
	basis.Passives = nil
	basis.IntentCycle = nil
	for _, encounter := range encounters {
		basis.HP += encounter.HP
		basis.GoldReward += encounter.GoldReward
		basis.CardReward += encounter.CardReward
	}
	switch kind {
	case "monster":
		basis.CardReward = min(3, max(1, basis.CardReward))
	case "elite":
		basis.CardReward = min(3, max(2, basis.CardReward))
	case "boss":
		basis.CardReward = max(3, basis.CardReward)
	}
	basis.Act = act
	return basis
}

func encounterNames(encounters []content.EncounterDef) []string {
	names := make([]string, 0, len(encounters))
	for _, encounter := range encounters {
		names = append(names, encounter.Name)
	}
	return names
}

func encounterGroupName(name string, slot int) string {
	if slot == 0 {
		return name
	}
	return fmt.Sprintf("%s #%d", name, slot+1)
}

func scaleEncounterForAct(encounter content.EncounterDef, act int) content.EncounterDef {
	if act <= encounter.Act {
		return encounter
	}
	extraActs := act - encounter.Act
	scaled := encounter
	scaled.Act = act
	scaled.HP += extraActs * 26
	scaled.GoldReward += extraActs * 18
	for i := range scaled.IntentCycle {
		for j := range scaled.IntentCycle[i].Effects {
			switch scaled.IntentCycle[i].Effects[j].Op {
			case "damage":
				scaled.IntentCycle[i].Effects[j].Value += extraActs * 3
			case "block":
				scaled.IntentCycle[i].Effects[j].Value += extraActs * 2
			case "heal":
				scaled.IntentCycle[i].Effects[j].Value += extraActs * 2
			}
		}
	}
	return scaled
}

func StartEvent(lib *content.Library, run *RunState, node Node) (EventState, error) {
	act := node.Act
	if act > 3 {
		act = 3
	}
	matches := make([]content.EventDef, 0)
	for _, event := range lib.Events {
		if !contentAvailableInRun(run, event.Flags) {
			continue
		}
		if slices.Contains(event.Acts, act) {
			filtered := filterEventChoices(run.Player, event)
			if len(filtered.Choices) > 0 {
				matches = append(matches, filtered)
			}
		}
	}
	if len(matches) == 0 {
		return EventState{}, errors.New("no events available")
	}
	rng := rand.New(rand.NewSource(run.Seed + int64(node.Floor*59+node.Index)))
	return EventState{Event: matches[rng.Intn(len(matches))]}, nil
}

func StartShop(lib *content.Library, run *RunState) ShopState {
	rng := rand.New(rand.NewSource(run.Seed + int64(run.Act*701+run.CurrentFloor*13)))
	classCards := standardCardsForRun(run, lib.CardsForClass(run.Player.ClassID))
	neutralCards := standardCardsForRun(run, lib.NeutralCards())
	coopCards := coopCardsForRun(run, lib.NeutralCards())

	offers := []ShopOffer{
		{
			ID:          "heal",
			Kind:        "heal",
			Name:        "补给餐",
			Description: "回复 18 生命。",
			Price:       24,
		},
		{
			ID:          "remove-card",
			Kind:        "remove",
			Name:        "精简牌组",
			Description: "移除牌组中的一张牌。",
			Price:       ShopRemovePrice(run),
		},
	}
	offers = append(offers, availableShopServices(lib, run)...)

	for i := 0; i < min(2, len(classCards)); i++ {
		idx := rng.Intn(len(classCards))
		card := classCards[idx]
		classCards = append(classCards[:idx], classCards[idx+1:]...)
		offers = append(offers, ShopOffer{
			ID:          fmt.Sprintf("class-card-%d", i),
			Kind:        "card",
			Name:        card.Name,
			Description: card.Description,
			Price:       55 + rng.Intn(16),
			CardID:      card.ID,
		})
	}

	if len(coopCards) > 0 {
		card := coopCards[rng.Intn(len(coopCards))]
		offers = append(offers, ShopOffer{
			ID:          "coop-card",
			Kind:        "card",
			Name:        card.Name,
			Description: card.Description,
			Price:       52,
			CardID:      card.ID,
		})
	}
	if len(neutralCards) > 0 {
		card := neutralCards[rng.Intn(len(neutralCards))]
		offers = append(offers, ShopOffer{
			ID:          "neutral-card",
			Kind:        "card",
			Name:        card.Name,
			Description: card.Description,
			Price:       45,
			CardID:      card.ID,
		})
	}

	relics := shopRelicPool(lib, run)
	if len(relics) > 0 {
		relic := relics[rng.Intn(len(relics))]
		offers = append(offers, ShopOffer{
			ID:          "relic-" + relic.ID,
			Kind:        "relic",
			Name:        relic.Name,
			Description: relic.Description,
			Price:       shopRelicPrice(relic.Rarity),
			ItemID:      relic.ID,
		})
	}

	equipments := shopEquipmentPool(lib)
	for i := 0; i < min(2, len(equipments)); i++ {
		idx := rng.Intn(len(equipments))
		equipment := equipments[idx]
		equipments = append(equipments[:idx], equipments[idx+1:]...)
		offers = append(offers, ShopOffer{
			ID:          "equipment-" + equipment.ID,
			Kind:        "equipment",
			Name:        equipment.Name,
			Description: equipment.Description,
			Price:       shopEquipmentPrice(equipment.Rarity),
			ItemID:      equipment.ID,
		})
	}

	for _, potion := range lib.PotionList() {
		offers = append(offers, ShopOffer{
			ID:          "potion-" + potion.ID,
			Kind:        "potion",
			Name:        potion.Name,
			Description: potion.Description,
			Price:       40,
			ItemID:      potion.ID,
		})
		break
	}

	slices.SortStableFunc(offers, func(a, b ShopOffer) int {
		if shopKindOrder(a.Kind) != shopKindOrder(b.Kind) {
			return shopKindOrder(a.Kind) - shopKindOrder(b.Kind)
		}
		return compareStrings(a.Name, b.Name)
	})
	return ShopState{Offers: offers}
}

func ApplyShopPurchase(lib *content.Library, run *RunState, shop *ShopState, offerID string) error {
	offer, err := findShopOffer(shop, offerID)
	if err != nil {
		return err
	}
	if offer.Kind == "service" {
		plan, err := ShopOfferDeckActionPlan(lib, run, shop, offerID)
		if err != nil {
			return err
		}
		if plan != nil {
			return fmt.Errorf("service %q requires card selection", offer.Name)
		}
	}
	if run.Player.Gold < offer.Price {
		return fmt.Errorf("金币不足")
	}
	run.Player.Gold -= offer.Price
	switch offer.Kind {
	case "heal":
		run.Player.HP = min(run.Player.MaxHP, run.Player.HP+18)
		shop.Log = append(shop.Log, "购买补给餐，回复了 18 生命。")
	case "card":
		run.Player.Deck = append(run.Player.Deck, DeckCard{CardID: offer.CardID})
		card := lib.Cards[offer.CardID]
		shop.Log = append(shop.Log, taggedLogLine(slices.Contains(card.Flags, "coop_only"), "购入卡牌 "+card.Name))
	case "relic":
		if addRelic(&run.Player, offer.ItemID) {
			relic := lib.Relics[offer.ItemID]
			shop.Log = append(shop.Log, taggedLogLine(slices.Contains(relic.Flags, "coop_only"), "购入遗物 "+relic.Name))
		} else {
			relic := lib.Relics[offer.ItemID]
			shop.Log = append(shop.Log, taggedLogLine(slices.Contains(relic.Flags, "coop_only"), "已持有遗物 "+relic.Name))
		}
	case "equipment":
		replacedID, err := equipItem(lib, &run.Player, offer.ItemID)
		if err != nil {
			return err
		}
		equipment := lib.Equipments[offer.ItemID]
		logLine := "购入装备 " + equipment.Name
		if replacedID != "" {
			logLine += "，替换 " + lib.Equipments[replacedID].Name
		}
		shop.Log = append(shop.Log, logLine)
	case "potion":
		addPotion(lib, &run.Player, offer.ItemID)
		shop.Log = append(shop.Log, "购入药水 "+lib.Potions[offer.ItemID].Name)
	case "service":
		if err := applyShopService(lib, run, shop, offer.ItemID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown shop offer type %s", offer.Kind)
	}
	consumeShopOffer(shop, offer.ID)
	return nil
}

func ApplyShopEquipmentPurchase(lib *content.Library, run *RunState, shop *ShopState, offerID string, take bool) error {
	offer, err := findShopOffer(shop, offerID)
	if err != nil {
		return err
	}
	if offer.Kind != "equipment" {
		return fmt.Errorf("offer %q is not equipment", offerID)
	}
	if !take {
		shop.Log = append(shop.Log, "放弃购买装备 "+offer.Name)
		return nil
	}
	return ApplyShopPurchase(lib, run, shop, offerID)
}

func ResolveEvent(lib *content.Library, run *RunState, state *EventState, choiceID string) error {
	return ResolveEventDecisionWithDeckChoice(lib, run, state, choiceID, true, -1)
}

func ResolveEventDecision(lib *content.Library, run *RunState, state *EventState, choiceID string, takeEquipment bool) error {
	return ResolveEventDecisionWithDeckChoice(lib, run, state, choiceID, takeEquipment, -1)
}

func ResolveEventDecisionWithDeckChoice(lib *content.Library, run *RunState, state *EventState, choiceID string, takeEquipment bool, deckIndex int) error {
	for _, choice := range state.Event.Choices {
		if choice.ID != choiceID {
			continue
		}
		lines, err := applyOutOfCombatEffectsDecision(lib, &run.Player, choice.Effects, takeEquipment, deckIndex)
		if err != nil {
			return err
		}
		state.Log = append(state.Log, choice.Label)
		state.Log = append(state.Log, lines...)
		state.Done = true
		return nil
	}
	return fmt.Errorf("event choice %q not found", choiceID)
}

func EventChoiceDeckActionPlan(lib *content.Library, run *RunState, choice content.EventChoiceDef, takeEquipment bool) (*DeckActionPlan, error) {
	for _, effect := range choice.Effects {
		if effect.Op != "augment_card" {
			continue
		}
		plan, err := BuildAugmentDeckActionPlan(lib, run.Player, effect, choice.Label, takeEquipment)
		if err != nil || plan != nil {
			return plan, err
		}
	}
	return nil, nil
}

func EventChoiceEquipmentID(choice content.EventChoiceDef) string {
	for _, effect := range choice.Effects {
		if effect.Op == "gain_equipment" {
			return effect.ItemID
		}
	}
	return ""
}

func EventChoicePotionID(choice content.EventChoiceDef) string {
	for _, effect := range choice.Effects {
		if effect.Op == "gain_potion" {
			return effect.ItemID
		}
	}
	return ""
}

func ResolveRest(lib *content.Library, run *RunState, choice string) (RestState, error) {
	state := RestState{}
	switch choice {
	case "heal":
		heal := max(16, run.Player.MaxHP/3+4) + run.Player.PermanentStats["rest_heal_bonus"]
		run.Player.HP = min(run.Player.MaxHP, run.Player.HP+heal)
		state.Log = append(state.Log, fmt.Sprintf("在篝火边休息，回复 %d 生命。", heal))
	case "upgrade":
		idx := firstUpgradableCard(lib, run.Player.Deck)
		if idx < 0 {
			run.Player.HP = min(run.Player.MaxHP, run.Player.HP+10)
			state.Log = append(state.Log, "没有可强化卡牌，转而冥想并回复 10 生命。")
			return state, nil
		}
		name, err := UpgradeDeckCard(lib, &run.Player, idx)
		if err != nil {
			return state, err
		}
		state.Log = append(state.Log, "强化了卡牌 "+name)
	default:
		return state, fmt.Errorf("unknown rest choice %q", choice)
	}
	return state, nil
}

func firstUpgradableCard(lib *content.Library, deck []DeckCard) int {
	indexes := UpgradableCardIndexes(lib, deck)
	if len(indexes) == 0 {
		return -1
	}
	return indexes[0]
}

func ApplyCombatResult(lib *content.Library, run *RunState, node Node, combat *CombatState, rewardChoice string) error {
	return ApplyCombatResultDecision(lib, run, node, combat, rewardChoice, combat.Reward.EquipmentID != "")
}

func ApplyCombatResultDecision(lib *content.Library, run *RunState, node Node, combat *CombatState, rewardChoice string, takeEquipment bool) error {
	seatIndex := CombatSeatIndexForRun(combat, run)
	applyCombatMetricsToRun(run, CombatMetricsForSeat(combat, seatIndex), CombatTurns(combat))
	if combat.Lost {
		run.Status = RunStatusLost
		run.Player.HP = 0
		return nil
	}
	run.Player.HP = combat.Player.HP
	run.Player.Gold += combat.Reward.Gold
	if rewardChoice != "" {
		run.Player.Deck = append(run.Player.Deck, DeckCard{CardID: rewardChoice})
	}
	if combat.Reward.PotionID != "" {
		addPotion(lib, &run.Player, combat.Reward.PotionID)
	}
	if combat.Reward.RelicID != "" {
		addRelic(&run.Player, combat.Reward.RelicID)
	}
	if takeEquipment && combat.Reward.EquipmentID != "" {
		if _, err := equipItem(lib, &run.Player, combat.Reward.EquipmentID); err != nil {
			return err
		}
	}
	run.Stats.CombatsWon++
	switch node.Kind {
	case NodeElite:
		run.Stats.ElitesWon++
	case NodeBoss:
		run.Stats.BossesWon++
	}
	return advanceAfterNode(run, node)
}

func AdvanceNonCombatNode(run *RunState, node Node) error {
	return advanceAfterNode(run, node)
}

func advanceAfterNode(run *RunState, node Node) error {
	run.CurrentFloor = node.Floor
	run.Stats.ClearedFloors++
	run.History = append(run.History, node.ID)

	if node.Kind == NodeBoss {
		if run.Mode == ModeStory && run.Act >= 3 {
			run.Status = RunStatusWon
			return nil
		}
		run.Act++
		run.Map = GenerateActMap(run.Seed, run.Act, nil)
		if err := ValidateMapConstraints(run.Map); err != nil {
			return err
		}
		run.Reachable = nodeIDs(run.Map.Floors[0])
		run.CurrentFloor = 0
		run.Player.HP = min(run.Player.MaxHP, run.Player.HP+run.Player.MaxHP/3+6)
		return nil
	}

	if len(node.Edges) == 0 {
		return fmt.Errorf("node %s has no outgoing edges", node.ID)
	}
	run.Reachable = slices.Clone(node.Edges)
	return nil
}

func BuildReward(lib *content.Library, run *RunState, node Node, encounter content.EncounterDef) RewardState {
	rng := rand.New(rand.NewSource(run.Seed + int64(run.Act*1001+node.Floor*31+node.Index)))
	cardChoices := make([]content.CardDef, 0, encounter.CardReward)

	coopPool := coopCardsForRun(run, lib.NeutralCards())
	if len(coopPool) > 0 && len(cardChoices) < encounter.CardReward {
		choice := coopPool[rng.Intn(len(coopPool))]
		cardChoices = append(cardChoices, choice)
	}

	classPool := append([]content.CardDef{}, standardCardsForRun(run, lib.CardsForClass(run.Player.ClassID))...)
	classPool = append(classPool, standardCardsForRun(run, lib.NeutralCards())...)
	for len(cardChoices) < encounter.CardReward && len(classPool) > 0 {
		idx := rng.Intn(len(classPool))
		cardChoices = append(cardChoices, classPool[idx])
		classPool = append(classPool[:idx], classPool[idx+1:]...)
	}

	reward := RewardState{
		Gold:           encounter.GoldReward + rng.Intn(12),
		CardChoices:    cardChoices,
		SourceNodeKind: node.Kind,
	}

	switch node.Kind {
	case NodeMonster:
		if rng.Intn(100) < 35 {
			reward.PotionID = lib.PotionList()[rng.Intn(len(lib.PotionList()))].ID
		}
	case NodeElite:
		if rng.Intn(2) == 0 {
			pool := rewardRelicPool(lib, run)
			if len(pool) > 0 {
				reward.RelicID = pool[rng.Intn(len(pool))].ID
			}
		} else {
			reward.EquipmentID = rewardEquipmentID(lib, rng, "elite")
		}
	case NodeBoss:
		reward.Gold += 80
		pool := rewardRelicPool(lib, run)
		if len(pool) > 0 {
			reward.RelicID = pool[rng.Intn(len(pool))].ID
		}
		reward.EquipmentID = rewardEquipmentID(lib, rng, "boss")
	}
	return reward
}

func EffectivePotionCapacity(lib *content.Library, player PlayerState) int {
	capacity := max(0, player.PotionCapacity)
	for _, relicID := range player.Relics {
		relic, ok := lib.Relics[relicID]
		if !ok {
			continue
		}
		for _, effect := range relic.Effects {
			if effect.Op == "potion_capacity" {
				capacity += max(0, effect.Value)
			}
		}
	}
	for _, equipmentID := range []string{player.Equipment.Weapon, player.Equipment.Armor, player.Equipment.Accessory} {
		if equipmentID == "" {
			continue
		}
		equipment, ok := lib.Equipments[equipmentID]
		if !ok {
			continue
		}
		for _, effect := range equipment.Effects {
			if effect.Op == "potion_capacity" {
				capacity += max(0, effect.Value)
			}
		}
	}
	return max(0, capacity)
}

func addPotion(lib *content.Library, player *PlayerState, potionID string) bool {
	if len(player.Potions) >= EffectivePotionCapacity(lib, *player) {
		return false
	}
	player.Potions = append(player.Potions, potionID)
	return true
}

func ReplacePotion(player *PlayerState, index int, potionID string) error {
	if index < 0 || index >= len(player.Potions) {
		return fmt.Errorf("invalid potion slot %d", index)
	}
	player.Potions[index] = potionID
	return nil
}

func equipItem(lib *content.Library, player *PlayerState, equipmentID string) (string, error) {
	item, ok := lib.Equipments[equipmentID]
	if !ok {
		return "", fmt.Errorf("unknown equipment %q", equipmentID)
	}
	replacedID := CurrentEquipmentID(*player, item.Slot)
	switch item.Slot {
	case "weapon":
		player.Equipment.Weapon = equipmentID
	case "armor":
		player.Equipment.Armor = equipmentID
	case "accessory":
		player.Equipment.Accessory = equipmentID
	default:
		return "", fmt.Errorf("unknown equipment slot %q", item.Slot)
	}
	return replacedID, nil
}

func applyOutOfCombatEffects(lib *content.Library, player *PlayerState, effects []content.Effect) ([]string, error) {
	return applyOutOfCombatEffectsDecision(lib, player, effects, true, -1)
}

func applyOutOfCombatEffectsDecision(lib *content.Library, player *PlayerState, effects []content.Effect, takeEquipment bool, deckIndex int) ([]string, error) {
	logs := []string{}
	for _, effect := range effects {
		switch effect.Op {
		case "gain_gold":
			player.Gold += effect.Value
			logs = append(logs, fmt.Sprintf("获得 %d 金币。", effect.Value))
		case "lose_hp":
			before := player.HP
			player.HP = max(1, player.HP-effect.Value)
			logs = append(logs, fmt.Sprintf("失去 %d 生命。", before-player.HP))
		case "heal":
			before := player.HP
			player.HP = min(player.MaxHP, player.HP+effect.Value)
			logs = append(logs, fmt.Sprintf("恢复 %d 生命。", player.HP-before))
		case "gain_max_hp":
			player.MaxHP += effect.Value
			player.HP = min(player.MaxHP, player.HP+effect.Value)
			logs = append(logs, fmt.Sprintf("最大生命提升 %d。", effect.Value))
		case "gain_relic":
			if addRelic(player, effect.ItemID) {
				relic := lib.Relics[effect.ItemID]
				logs = append(logs, taggedLogLine(slices.Contains(relic.Flags, "coop_only"), "获得遗物 "+relic.Name))
			}
		case "upgrade_relic":
			if err := upgradeRelic(player, effect.ItemID, effect.ResultID); err != nil {
				return nil, err
			}
			relic := lib.Relics[effect.ResultID]
			logs = append(logs, taggedLogLine(slices.Contains(relic.Flags, "coop_only"), "遗物升级为 "+relic.Name))
		case "gain_equipment":
			if !takeEquipment {
				logs = append(logs, "放弃装备 "+lib.Equipments[effect.ItemID].Name)
				continue
			}
			if _, err := equipItem(lib, player, effect.ItemID); err != nil {
				return nil, err
			}
			logs = append(logs, "获得装备 "+lib.Equipments[effect.ItemID].Name)
		case "gain_potion":
			addPotion(lib, player, effect.ItemID)
			logs = append(logs, "获得药水 "+lib.Potions[effect.ItemID].Name)
		case "add_card":
			player.Deck = append(player.Deck, DeckCard{CardID: effect.CardID})
			card := lib.Cards[effect.CardID]
			logs = append(logs, taggedLogLine(slices.Contains(card.Flags, "coop_only"), "获得卡牌 "+card.Name))
		case "upgrade_card":
			idx := firstUpgradableCard(lib, player.Deck)
			if idx >= 0 {
				player.Deck[idx].Upgraded = true
				logs = append(logs, "随机强化了 "+lib.Cards[player.Deck[idx].CardID].Name)
			}
		case "augment_card":
			lines, err := ApplyDeckCardAugmentEffect(lib, player, effect, deckIndex)
			if err != nil {
				return nil, err
			}
			logs = append(logs, lines...)
		default:
			return nil, fmt.Errorf("unsupported out-of-combat effect %s", effect.Op)
		}
	}
	return logs, nil
}

func findShopOffer(shop *ShopState, offerID string) (ShopOffer, error) {
	for _, offer := range shop.Offers {
		if offer.ID == offerID {
			return offer, nil
		}
	}
	return ShopOffer{}, fmt.Errorf("offer %q not found", offerID)
}

func shopEquipmentPool(lib *content.Library) []content.EquipmentDef {
	pool := []content.EquipmentDef{}
	for _, equipment := range lib.EquipmentList() {
		if equipment.Rarity == "starter" {
			continue
		}
		pool = append(pool, equipment)
	}
	return pool
}

func rewardRelicPool(lib *content.Library, run *RunState) []content.RelicDef {
	pool := []content.RelicDef{}
	for _, relic := range lib.RelicList() {
		if relic.Rarity == "starter" || relic.Rarity == "upgraded" {
			continue
		}
		if slices.Contains(run.Player.Relics, relic.ID) {
			continue
		}
		if !contentAvailableInRun(run, relic.Flags) {
			continue
		}
		pool = append(pool, relic)
	}
	if coopPool := coopRelicPool(lib, run); len(coopPool) > 0 {
		return coopPool
	}
	return pool
}

func shopRelicPool(lib *content.Library, run *RunState) []content.RelicDef {
	return rewardRelicPool(lib, run)
}

func rewardEquipmentID(lib *content.Library, rng *rand.Rand, source string) string {
	pool := []content.EquipmentDef{}
	for _, equipment := range shopEquipmentPool(lib) {
		if source == "boss" && equipment.Rarity == "common" {
			continue
		}
		pool = append(pool, equipment)
	}
	if len(pool) == 0 {
		return ""
	}
	return pool[rng.Intn(len(pool))].ID
}

func shopEquipmentPrice(rarity string) int {
	switch rarity {
	case "legendary":
		return 132
	case "rare":
		return 108
	case "uncommon":
		return 90
	default:
		return 76
	}
}

func shopRelicPrice(rarity string) int {
	switch rarity {
	case "legendary":
		return 138
	case "rare":
		return 112
	case "uncommon":
		return 94
	default:
		return 80
	}
}

func effectivePartySize(run *RunState) int {
	if run == nil || run.PartySize <= 1 {
		return 1
	}
	return run.PartySize
}

func contentAvailableInRun(run *RunState, flags []string) bool {
	if !slices.Contains(flags, "coop_only") {
		return true
	}
	return effectivePartySize(run) > 1
}

func taggedLogLine(coop bool, text string) string {
	if !coop {
		return text
	}
	return "[CO-OP] " + text
}

func standardCardsForRun(run *RunState, cards []content.CardDef) []content.CardDef {
	out := make([]content.CardDef, 0, len(cards))
	for _, card := range cards {
		if !contentAvailableInRun(run, card.Flags) {
			continue
		}
		if slices.Contains(card.Flags, "coop_only") {
			continue
		}
		out = append(out, card)
	}
	return out
}

func coopCardsForRun(run *RunState, cards []content.CardDef) []content.CardDef {
	if effectivePartySize(run) <= 1 {
		return nil
	}
	out := []content.CardDef{}
	for _, card := range cards {
		if slices.Contains(card.Flags, "coop_only") {
			out = append(out, card)
		}
	}
	return out
}

func coopRelicPool(lib *content.Library, run *RunState) []content.RelicDef {
	if effectivePartySize(run) <= 1 {
		return nil
	}
	out := []content.RelicDef{}
	for _, relic := range lib.RelicList() {
		if relic.Rarity == "starter" || relic.Rarity == "upgraded" {
			continue
		}
		if slices.Contains(run.Player.Relics, relic.ID) {
			continue
		}
		if slices.Contains(relic.Flags, "coop_only") {
			out = append(out, relic)
		}
	}
	return out
}

func ShopRemovePrice(run *RunState) int {
	return 68 + max(0, run.Act-1)*10
}

func addRelic(player *PlayerState, relicID string) bool {
	if relicID == "" || slices.Contains(player.Relics, relicID) {
		return false
	}
	player.Relics = append(player.Relics, relicID)
	return true
}

func upgradeRelic(player *PlayerState, currentID, nextID string) error {
	for i, relicID := range player.Relics {
		if relicID == currentID {
			player.Relics[i] = nextID
			return nil
		}
	}
	return fmt.Errorf("relic %q not owned", currentID)
}

func UpgradableCardIndexes(lib *content.Library, deck []DeckCard) []int {
	indexes := []int{}
	for i, card := range deck {
		def := lib.Cards[card.CardID]
		if !card.Upgraded && len(def.UpgradeEffects) > 0 {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func UpgradeDeckCard(lib *content.Library, player *PlayerState, deckIndex int) (string, error) {
	if deckIndex < 0 || deckIndex >= len(player.Deck) {
		return "", fmt.Errorf("invalid deck index %d", deckIndex)
	}
	card := player.Deck[deckIndex]
	def := lib.Cards[card.CardID]
	if card.Upgraded || len(def.UpgradeEffects) == 0 {
		return "", fmt.Errorf("card %q cannot be upgraded", def.Name)
	}
	player.Deck[deckIndex].Upgraded = true
	return def.Name, nil
}

func RemoveDeckCard(lib *content.Library, player *PlayerState, deckIndex int) (string, error) {
	if deckIndex < 0 || deckIndex >= len(player.Deck) {
		return "", fmt.Errorf("invalid deck index %d", deckIndex)
	}
	card := player.Deck[deckIndex]
	name := lib.Cards[card.CardID].Name
	player.Deck = append(player.Deck[:deckIndex], player.Deck[deckIndex+1:]...)
	return name, nil
}

func ApplyShopCardRemoval(lib *content.Library, run *RunState, shop *ShopState, offerID string, deckIndex int) error {
	offer, err := findShopOffer(shop, offerID)
	if err != nil {
		return err
	}
	if offer.Kind != "remove" {
		return fmt.Errorf("offer %q is not a removal service", offerID)
	}
	if run.Player.Gold < offer.Price {
		return fmt.Errorf("金币不足")
	}
	name, err := RemoveDeckCard(lib, &run.Player, deckIndex)
	if err != nil {
		return err
	}
	run.Player.Gold -= offer.Price
	shop.Log = append(shop.Log, fmt.Sprintf("花费 %d 金币移除了 %s", offer.Price, name))
	consumeShopOffer(shop, offer.ID)
	return nil
}

func consumeShopOffer(shop *ShopState, offerID string) {
	for i, offer := range shop.Offers {
		if offer.ID == offerID {
			shop.Offers = append(shop.Offers[:i], shop.Offers[i+1:]...)
			return
		}
	}
}

func filterEventChoices(player PlayerState, event content.EventDef) content.EventDef {
	filtered := event
	filtered.Choices = nil
	for _, choice := range event.Choices {
		if canTakeEventChoice(player, choice) {
			filtered.Choices = append(filtered.Choices, choice)
		}
	}
	return filtered
}

func canTakeEventChoice(player PlayerState, choice content.EventChoiceDef) bool {
	for _, relicID := range choice.RequiresRelics {
		if !slices.Contains(player.Relics, relicID) {
			return false
		}
	}
	return true
}

func shopKindOrder(kind string) int {
	switch kind {
	case "card":
		return 0
	case "relic":
		return 1
	case "equipment":
		return 2
	case "potion":
		return 3
	case "service":
		return 4
	default:
		return 5
	}
}

func applyShopService(lib *content.Library, run *RunState, shop *ShopState, serviceID string) error {
	switch serviceID {
	case "service_coop_card":
		pool := coopCardsForRun(run, lib.NeutralCards())
		if len(pool) == 0 {
			return fmt.Errorf("no coop cards available")
		}
		idx := len(shop.Log) % len(pool)
		card := pool[idx]
		run.Player.Deck = append(run.Player.Deck, DeckCard{CardID: card.ID})
		shop.Log = append(shop.Log, taggedLogLine(true, "参加协同简报，获得协作牌 "+card.Name))
		return nil
	case "service_combo_drill":
		healedBefore := run.Player.HP
		run.Player.HP = min(run.Player.MaxHP, run.Player.HP+10)
		if idx := firstUpgradableCard(lib, run.Player.Deck); idx >= 0 {
			name, err := UpgradeDeckCard(lib, &run.Player, idx)
			if err != nil {
				return err
			}
			shop.Log = append(shop.Log, taggedLogLine(true, "完成连携操练，强化了 "+name))
		} else {
			shop.Log = append(shop.Log, taggedLogLine(true, "完成连携操练，但没有可强化的牌。"))
		}
		if healed := run.Player.HP - healedBefore; healed > 0 {
			shop.Log = append(shop.Log, taggedLogLine(true, fmt.Sprintf("恢复了 %d 生命。", healed)))
		}
		return nil
	default:
		return fmt.Errorf("unknown shop service %q", serviceID)
	}
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
