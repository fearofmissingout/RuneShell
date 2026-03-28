package engine

import (
	"fmt"
	"strings"

	"cmdcards/internal/content"
)

const CombatInspectPaneCount = 6

func CombatInspectPaneName(index int) string {
	switch normalizeCombatInspectPane(index) {
	case 0:
		return "概览"
	case 1:
		return "抽牌堆"
	case 2:
		return "弃牌堆"
	case 3:
		return "消耗堆"
	case 4:
		return "牌组"
	default:
		return "效果"
	}
}

func CombatInspectLines(lib *content.Library, run *RunState, combat *CombatState, pane int) []string {
	return CombatInspectLinesForSeat(lib, run, combat, 0, pane)
}

func CombatInspectLinesForSeat(lib *content.Library, run *RunState, combat *CombatState, seatIndex int, pane int) []string {
	seat := CombatSeatView(combat, seatIndex)
	player := run.Player
	if actor := ActorForSeat(combat, seatIndex); actor != nil {
		player.Name = actor.Name
		player.HP = actor.HP
		player.MaxHP = actor.MaxHP
		player.MaxEnergy = actor.MaxEnergy
	}
	if state := SeatPlayerForInspect(combat, run.Player, seatIndex); state.Name != "" {
		player = state
	}
	if seat != nil {
		player.Potions = append([]string{}, seat.Potions...)
	}
	switch normalizeCombatInspectPane(pane) {
	case 0:
		return combatOverviewLines(lib, player, combat, seatIndex)
	case 1:
		if seat == nil {
			return []string{"抽牌堆为空"}
		}
		return runtimeCardLines(lib, seat.DrawPile, "抽牌堆为空")
	case 2:
		if seat == nil {
			return []string{"弃牌堆为空"}
		}
		return runtimeCardLines(lib, seat.Discard, "弃牌堆为空")
	case 3:
		if seat == nil {
			return []string{"消耗堆为空"}
		}
		return runtimeCardLines(lib, seat.Exhaust, "消耗堆为空")
	case 4:
		return deckCardLines(lib, player.Deck, "牌组为空")
	default:
		return combatEffectLines(lib, player, combat, seatIndex)
	}
}

func CombatSeatView(combat *CombatState, seatIndex int) *CombatSeatState {
	if combat == nil || seatIndex < 0 || seatIndex >= len(combat.Seats) {
		return nil
	}
	return &combat.Seats[seatIndex]
}

func SeatPlayerForInspect(combat *CombatState, fallback PlayerState, seatIndex int) PlayerState {
	if combat != nil && seatIndex >= 0 && seatIndex < len(combat.SeatPlayers) {
		return combat.SeatPlayers[seatIndex]
	}
	return fallback
}

func CardStateName(lib *content.Library, cardID string, upgraded bool) string {
	card := lib.Cards[cardID]
	if upgraded {
		return card.Name + "+"
	}
	return card.Name
}

func CardStateSummary(lib *content.Library, cardID string, upgraded bool) string {
	return DescribeEffects(lib, baseCardEffects(lib, cardID, upgraded))
}

func DescribeEffects(lib *content.Library, effects []content.Effect) string {
	return content.DescribeEffects(lib, effects)
}

func DescribeEffect(lib *content.Library, effect content.Effect) string {
	return content.DescribeEffect(lib, effect)
}

func passiveSourceLines(lib *content.Library, player PlayerState) []string {
	lines := []string{}
	for _, relicID := range player.Relics {
		if relic, ok := lib.Relics[relicID]; ok {
			lines = append(lines, "遗物 | "+relic.Name+" | "+relic.Description)
		}
	}
	for _, equipmentID := range []string{player.Equipment.Weapon, player.Equipment.Armor, player.Equipment.Accessory} {
		if equipmentID == "" {
			continue
		}
		if equipment, ok := lib.Equipments[equipmentID]; ok {
			lines = append(lines, "装备 | "+equipment.Name+" | "+equipment.Description)
		}
	}
	return lines
}

func encounterPassiveLines(lib *content.Library, combat *CombatState) []string {
	lines := []string{}
	for _, enemy := range combat.Enemies {
		for _, effect := range enemy.Passives {
			lines = append(lines, enemy.Name+" 被动 | "+DescribeEffect(lib, effect))
		}
	}
	return lines
}

func normalizeCombatInspectPane(index int) int {
	if index < 0 {
		return 0
	}
	return index % CombatInspectPaneCount
}

func runtimeCardLines(lib *content.Library, pile []RuntimeCard, empty string) []string {
	if len(pile) == 0 {
		return []string{empty}
	}
	lines := make([]string, 0, len(pile)*4)
	for i, card := range pile {
		def := lib.Cards[card.ID]
		lines = append(lines, fmt.Sprintf("%d. %s | %s %d | %s", i+1, CardStateName(lib, card.ID, card.Upgraded), inspectCostLabel(lib), def.Cost, inspectTargetHint(lib, RuntimeCard{ID: card.ID, Upgraded: card.Upgraded, Augments: cloneCardAugments(card.Augments)})))
		lines = append(lines, "   "+inspectCurrentLabel(lib)+": "+RuntimeCardStateSummary(lib, card))
		if !card.Upgraded && len(def.UpgradeEffects) > 0 {
			lines = append(lines, "   "+inspectUpgradeLabel(lib)+": "+DescribeEffects(lib, def.UpgradeEffects))
		}
		if len(card.Augments) > 0 {
			lines = append(lines, "   "+inspectAugmentsLabel(lib)+": "+runtimeAugmentSummary(lib, card.Augments))
		}
	}
	return lines
}

func deckCardLines(lib *content.Library, deck []DeckCard, empty string) []string {
	if len(deck) == 0 {
		return []string{empty}
	}
	lines := make([]string, 0, len(deck)*4)
	for i, card := range deck {
		def := lib.Cards[card.CardID]
		lines = append(lines, fmt.Sprintf("%d. %s | %s %d | %s", i+1, CardStateName(lib, card.CardID, card.Upgraded), inspectCostLabel(lib), def.Cost, inspectTargetHint(lib, RuntimeCard{ID: card.CardID, Upgraded: card.Upgraded})))
		lines = append(lines, "   "+inspectCurrentLabel(lib)+": "+DeckCardStateSummary(lib, card))
		if !card.Upgraded && len(def.UpgradeEffects) > 0 {
			lines = append(lines, "   "+inspectUpgradeLabel(lib)+": "+DescribeEffects(lib, def.UpgradeEffects))
		}
		if len(card.Augments) > 0 {
			lines = append(lines, "   "+inspectAugmentsLabel(lib)+": "+runtimeAugmentSummary(lib, card.Augments))
		}
	}
	return lines
}

func inspectTargetHint(lib *content.Library, card RuntimeCard) string {
	switch CardTargetKindForCard(lib, card) {
	case CombatTargetEnemy:
		return inspectTargetSingleEnemy(lib)
	case CombatTargetEnemies:
		return inspectTargetAllEnemies(lib)
	case CombatTargetAlly:
		return inspectTargetSingleAlly(lib)
	case CombatTargetAllies:
		return inspectTargetAllAllies(lib)
	default:
		return inspectTargetNone(lib)
	}
}

func runtimeAugmentSummary(lib *content.Library, augments []CardAugment) string {
	parts := make([]string, 0, len(augments))
	for _, augment := range augments {
		summary := DescribeEffects(lib, augment.Effects)
		if summary == "" {
			continue
		}
		scope := cardAugmentScopeLabelFor(lib, augment.Scope)
		if augment.Name != "" {
			parts = append(parts, augment.Name+": "+summary+" ("+scope+")")
			continue
		}
		parts = append(parts, summary+" ("+scope+")")
	}
	return strings.Join(parts, " | ")
}

func combatOverviewLines(lib *content.Library, player PlayerState, combat *CombatState, seatIndex int) []string {
	seat := CombatSeatView(combat, seatIndex)
	handSize, drawSize, discardSize, exhaustSize := len(combat.Hand), len(combat.DrawPile), len(combat.Discard), len(combat.Exhaust)
	if seat != nil {
		handSize = len(seat.Hand)
		drawSize = len(seat.DrawPile)
		discardSize = len(seat.Discard)
		exhaustSize = len(seat.Exhaust)
	}
	lines := []string{
		fmt.Sprintf("手牌 %d | 抽牌堆 %d | 弃牌堆 %d | 消耗堆 %d", handSize, drawSize, discardSize, exhaustSize),
		fmt.Sprintf("药水 %d/%d | 金币 %d | 牌组 %d", len(player.Potions), EffectivePotionCapacity(lib, player), player.Gold, len(player.Deck)),
		"友方：",
	}
	lines = append(lines, DescribeParty(combat)...)
	lines = append(lines, "敌方：")
	for i, enemy := range combat.Enemies {
		line := fmt.Sprintf("%d. %s HP %d/%d Block %d | %s: %s", i+1, enemy.Name, enemy.HP, enemy.MaxHP, enemy.Block, inspectIntentLabel(lib), DescribeIntentFor(content.LanguageOf(lib), enemy.CurrentIntent))
		if status := DescribeStatusesFor(content.LanguageOf(lib), enemy.Statuses); status != "" {
			line += " | " + status
		}
		lines = append(lines, line)
	}
	if len(player.Potions) > 0 {
		lines = append(lines, "药水：")
		for i, potionID := range player.Potions {
			potion := lib.Potions[potionID]
			lines = append(lines, fmt.Sprintf("%d. %s | %s", i+1, potion.Name, potion.Description))
		}
	}
	return lines
}

func combatEffectLines(lib *content.Library, player PlayerState, combat *CombatState, seatIndex int) []string {
	lines := []string{}
	if actor := ActorForSeat(combat, seatIndex); actor != nil {
		status := DescribeStatusesFor(content.LanguageOf(lib), actor.Statuses)
		if status != "" {
			lines = append(lines, inspectSelfStatusLabel(lib)+" | "+status)
		}
	}
	for _, pending := range PendingNextCardRepeatDescriptions(combat, seatIndex) {
		lines = append(lines, inspectPendingLabel(lib)+" | "+pending)
	}
	lines = append(lines, passiveSourceLines(lib, player)...)
	lines = append(lines, encounterPassiveLines(lib, combat)...)
	if len(lines) == 0 {
		lines = append(lines, inspectNoEffectsLabel(lib))
	}
	return lines
}

func inspectCostLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Cost"
	}
	return "费用"
}

func inspectCurrentLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Current"
	}
	return "当前"
}

func inspectUpgradeLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Upgrade"
	}
	return "升级"
}

func inspectAugmentsLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Augments"
	}
	return "附加效果"
}

func inspectTargetSingleEnemy(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Single enemy"
	}
	return "单体敌人"
}

func inspectTargetAllEnemies(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "All enemies"
	}
	return "全体敌人"
}

func inspectTargetSingleAlly(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Single ally"
	}
	return "单体友军"
}

func inspectTargetAllAllies(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "All allies"
	}
	return "全体友军"
}

func inspectTargetNone(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "No target"
	}
	return "无目标"
}

func inspectIntentLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Intent"
	}
	return "意图"
}

func inspectSelfStatusLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Self Status"
	}
	return "自身状态"
}

func inspectPendingLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "Pending"
	}
	return "待触发"
}

func inspectNoEffectsLabel(lib *content.Library) string {
	if content.LanguageOf(lib) == "en-US" {
		return "There are no persistent effects right now."
	}
	return "当前没有常驻效果。"
}

func effectStatusTargetLabel(target string) string {
	switch target {
	case "self":
		return "自己获得"
	case "player":
		return "玩家获得"
	case "enemy":
		return "敌人获得"
	case "opponent":
		return "对手获得"
	case "all_allies":
		return "全体友军获得"
	case "all_enemies":
		return "全体敌人获得"
	default:
		return "施加"
	}
}

func effectOwnerLabel(target string) string {
	switch target {
	case "self":
		return "自己"
	case "player":
		return "玩家"
	case "enemy":
		return "敌人"
	case "opponent":
		return "对手"
	case "all_allies":
		return "全体友军"
	case "all_enemies":
		return "全体敌人"
	default:
		return "目标"
	}
}

func modifierTriggerLabel(trigger string) string {
	switch trigger {
	case "attack":
		return "攻击"
	case "spell":
		return "法术"
	default:
		return "所有"
	}
}

func durationSuffix(duration int) string {
	if duration <= 0 {
		return ""
	}
	return fmt.Sprintf("(%d 回合)", duration)
}

func pendingRepeatDescription(count int, tag string) string {
	count = max(1, count)
	switch tag {
	case "attack":
		return fmt.Sprintf("下一张攻击牌额外重复 %d 次", count)
	case "spell":
		return fmt.Sprintf("下一张法术牌额外重复 %d 次", count)
	default:
		return fmt.Sprintf("下一张牌额外重复 %d 次", count)
	}
}
