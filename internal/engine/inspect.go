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
	switch normalizeCombatInspectPane(pane) {
	case 0:
		return combatOverviewLines(lib, run, combat)
	case 1:
		return runtimeCardLines(lib, combat.DrawPile, "抽牌堆为空")
	case 2:
		return runtimeCardLines(lib, combat.Discard, "弃牌堆为空")
	case 3:
		return runtimeCardLines(lib, combat.Exhaust, "消耗堆为空")
	case 4:
		return deckCardLines(lib, run.Player.Deck, "牌组为空")
	default:
		return combatEffectLines(lib, run.Player, combat)
	}
}

func CardStateName(lib *content.Library, cardID string, upgraded bool) string {
	card := lib.Cards[cardID]
	if upgraded {
		return card.Name + "+"
	}
	return card.Name
}

func CardStateSummary(lib *content.Library, cardID string, upgraded bool) string {
	card := lib.Cards[cardID]
	effects := card.Effects
	if upgraded && len(card.UpgradeEffects) > 0 {
		effects = card.UpgradeEffects
	}
	return DescribeEffects(lib, effects)
}

func DescribeEffects(lib *content.Library, effects []content.Effect) string {
	parts := make([]string, 0, len(effects))
	for _, effect := range effects {
		parts = append(parts, DescribeEffect(lib, effect))
	}
	return strings.Join(parts, "；")
}

func DescribeEffect(lib *content.Library, effect content.Effect) string {
	switch effect.Op {
	case "damage":
		switch effect.Target {
		case "all_enemies":
			return fmt.Sprintf("对全体敌人造成 %d 伤害", effect.Value)
		case "all_allies":
			return fmt.Sprintf("对全体友军造成 %d 伤害", effect.Value)
		default:
			return fmt.Sprintf("造成 %d 伤害", effect.Value)
		}
	case "block":
		switch effect.Target {
		case "all_allies":
			return fmt.Sprintf("全体友军获得 %d 格挡", effect.Value)
		default:
			return fmt.Sprintf("获得 %d 格挡", effect.Value)
		}
	case "draw":
		return fmt.Sprintf("抽 %d 张牌", effect.Value)
	case "apply_status":
		return fmt.Sprintf("%s %s %d%s", effectStatusTargetLabel(effect.Target), statusLabel(effect.Status), effect.Value, durationSuffix(effect.Duration))
	case "cleanse_status":
		return fmt.Sprintf("净化%s的%s", effectOwnerLabel(effect.Target), statusLabel(effect.Status))
	case "modify_damage":
		return fmt.Sprintf("%s伤害 %+d", modifierTriggerLabel(effect.Trigger), effect.Value)
	case "modify_taken_damage":
		if effect.Value < 0 {
			return fmt.Sprintf("受到伤害 %d", effect.Value)
		}
		return fmt.Sprintf("受到伤害 +%d", effect.Value)
	case "gain_gold":
		return fmt.Sprintf("获得 %d 金币", effect.Value)
	case "lose_hp":
		return fmt.Sprintf("失去 %d 生命", effect.Value)
	case "heal":
		switch effect.Target {
		case "all_allies":
			return fmt.Sprintf("全体友军恢复 %d 生命", effect.Value)
		default:
			return fmt.Sprintf("恢复 %d 生命", effect.Value)
		}
	case "gain_max_hp":
		return fmt.Sprintf("最大生命 +%d", effect.Value)
	case "gain_relic":
		if relic, ok := lib.Relics[effect.ItemID]; ok {
			return "获得遗物 " + relic.Name
		}
		return "获得遗物"
	case "gain_equipment":
		if equipment, ok := lib.Equipments[effect.ItemID]; ok {
			return "获得装备 " + equipment.Name
		}
		return "获得装备"
	case "gain_potion":
		if potion, ok := lib.Potions[effect.ItemID]; ok {
			return "获得药水 " + potion.Name
		}
		return "获得药水"
	case "upgrade_relic":
		fromName := effect.ItemID
		if relic, ok := lib.Relics[effect.ItemID]; ok {
			fromName = relic.Name
		}
		toName := effect.ResultID
		if relic, ok := lib.Relics[effect.ResultID]; ok {
			toName = relic.Name
		}
		return "升级遗物 " + fromName + " -> " + toName
	case "add_card":
		if card, ok := lib.Cards[effect.CardID]; ok {
			return "获得卡牌 " + card.Name
		}
		return "获得卡牌"
	case "upgrade_card":
		return "强化一张可升级卡牌"
	case "gain_energy":
		return fmt.Sprintf("获得 %d 能量", effect.Value)
	default:
		return effect.Op
	}
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
	lines := make([]string, 0, len(pile))
	for i, card := range pile {
		lines = append(lines, fmt.Sprintf("%d. %s | %s", i+1, CardStateName(lib, card.ID, card.Upgraded), CardStateSummary(lib, card.ID, card.Upgraded)))
	}
	return lines
}

func deckCardLines(lib *content.Library, deck []DeckCard, empty string) []string {
	if len(deck) == 0 {
		return []string{empty}
	}
	lines := make([]string, 0, len(deck))
	for i, card := range deck {
		lines = append(lines, fmt.Sprintf("%d. %s | %s", i+1, CardStateName(lib, card.CardID, card.Upgraded), CardStateSummary(lib, card.CardID, card.Upgraded)))
	}
	return lines
}

func combatOverviewLines(lib *content.Library, run *RunState, combat *CombatState) []string {
	lines := []string{
		fmt.Sprintf("手牌 %d | 抽牌堆 %d | 弃牌堆 %d | 消耗堆 %d", len(combat.Hand), len(combat.DrawPile), len(combat.Discard), len(combat.Exhaust)),
		fmt.Sprintf("药水 %d/%d | 金币 %d | 牌组 %d", len(run.Player.Potions), run.Player.PotionCapacity, run.Player.Gold, len(run.Player.Deck)),
		"友方：",
	}
	lines = append(lines, DescribeParty(combat)...)
	lines = append(lines, "敌方：")
	for i, enemy := range combat.Enemies {
		line := fmt.Sprintf("%d. %s HP %d/%d Block %d | 意图: %s", i+1, enemy.Name, enemy.HP, enemy.MaxHP, enemy.Block, DescribeIntent(enemy.CurrentIntent))
		if status := DescribeStatuses(enemy.Statuses); status != "" {
			line += " | " + status
		}
		lines = append(lines, line)
	}
	if len(run.Player.Potions) > 0 {
		lines = append(lines, "药水：")
		for i, potionID := range run.Player.Potions {
			potion := lib.Potions[potionID]
			lines = append(lines, fmt.Sprintf("%d. %s | %s", i+1, potion.Name, potion.Description))
		}
	}
	return lines
}

func combatEffectLines(lib *content.Library, player PlayerState, combat *CombatState) []string {
	lines := []string{}
	lines = append(lines, passiveSourceLines(lib, player)...)
	lines = append(lines, encounterPassiveLines(lib, combat)...)
	if len(lines) == 0 {
		lines = append(lines, "当前没有常驻效果。")
	}
	return lines
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
