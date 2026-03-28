package content

import (
	"fmt"
	"regexp"
	"strings"

	"cmdcards/internal/i18n"
)

var actPrefixPattern = regexp.MustCompile(`^act\d+_`)

func LanguageOf(lib *Library) string {
	if lib == nil {
		return i18n.DefaultLanguage
	}
	return i18n.NormalizeLanguage(lib.Language)
}

func DescribeEffects(lib *Library, effects []Effect) string {
	if len(effects) == 0 {
		return ""
	}
	parts := make([]string, 0, len(effects))
	for _, effect := range effects {
		parts = append(parts, DescribeEffect(lib, effect))
	}
	if LanguageOf(lib) == i18n.LangEnUS {
		return strings.Join(parts, ", ")
	}
	return strings.Join(parts, "，")
}

func DescribeEffect(lib *Library, effect Effect) string {
	if LanguageOf(lib) == i18n.LangEnUS {
		return describeEffectEN(lib, effect)
	}
	return describeEffectZH(lib, effect)
}

func AugmentScopeLabel(lang string, scope string) string {
	switch i18n.NormalizeLanguage(lang) {
	case i18n.LangEnUS:
		switch strings.TrimSpace(scope) {
		case "combat":
			return "this combat"
		case "turn":
			return "this turn of the next combat"
		default:
			return "this run"
		}
	default:
		switch strings.TrimSpace(scope) {
		case "combat":
			return "下场战斗"
		case "turn":
			return "下场战斗的本回合"
		default:
			return "本局"
		}
	}
}

func HumanizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	id = actPrefixPattern.ReplaceAllString(id, "")
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i, part := range parts {
		lower := strings.ToLower(part)
		switch lower {
		case "hp":
			parts[i] = "HP"
		case "ui":
			parts[i] = "UI"
		case "coop":
			parts[i] = "Co-op"
		default:
			parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
		}
	}
	return strings.Join(parts, " ")
}

func describeEffectEN(lib *Library, effect Effect) string {
	switch effect.Op {
	case "damage":
		switch effect.Target {
		case "all_enemies":
			return fmt.Sprintf("Deal %d damage to all enemies", effect.Value)
		case "all_allies":
			return fmt.Sprintf("Deal %d damage to all allies", effect.Value)
		default:
			return fmt.Sprintf("Deal %d damage", effect.Value)
		}
	case "block":
		switch effect.Target {
		case "all_allies":
			return fmt.Sprintf("Gain %d Block for all allies", effect.Value)
		default:
			return fmt.Sprintf("Gain %d Block", effect.Value)
		}
	case "draw":
		return fmt.Sprintf("Draw %d cards", effect.Value)
	case "apply_status":
		return fmt.Sprintf("%s %s %d%s", effectStatusTargetLabelEN(effect.Target), statusLabelEN(effect.Status), effect.Value, durationSuffixEN(effect.Duration))
	case "cleanse_status":
		return fmt.Sprintf("Cleanse %s from %s", statusLabelEN(effect.Status), effectOwnerLabelEN(effect.Target))
	case "modify_damage":
		return fmt.Sprintf("%s damage %+d", modifierTriggerLabelEN(effect.Trigger), effect.Value)
	case "modify_taken_damage":
		if effect.Value < 0 {
			return fmt.Sprintf("Take %d less damage", -effect.Value)
		}
		return fmt.Sprintf("Take %d more damage", effect.Value)
	case "gain_gold":
		return fmt.Sprintf("Gain %d gold", effect.Value)
	case "lose_hp":
		return fmt.Sprintf("Lose %d HP", effect.Value)
	case "heal":
		switch effect.Target {
		case "all_allies":
			return fmt.Sprintf("Heal all allies for %d HP", effect.Value)
		default:
			return fmt.Sprintf("Heal %d HP", effect.Value)
		}
	case "gain_max_hp":
		return fmt.Sprintf("Max HP +%d", effect.Value)
	case "gain_relic":
		if relic, ok := lib.Relics[effect.ItemID]; ok {
			return "Gain relic " + relic.Name
		}
		return "Gain a relic"
	case "gain_equipment":
		if equipment, ok := lib.Equipments[effect.ItemID]; ok {
			return "Gain equipment " + equipment.Name
		}
		return "Gain equipment"
	case "gain_potion":
		if potion, ok := lib.Potions[effect.ItemID]; ok {
			return "Gain potion " + potion.Name
		}
		return "Gain a potion"
	case "upgrade_relic":
		fromName := HumanizeID(effect.ItemID)
		if relic, ok := lib.Relics[effect.ItemID]; ok {
			fromName = relic.Name
		}
		toName := HumanizeID(effect.ResultID)
		if relic, ok := lib.Relics[effect.ResultID]; ok {
			toName = relic.Name
		}
		return "Upgrade relic " + fromName + " -> " + toName
	case "add_card":
		if card, ok := lib.Cards[effect.CardID]; ok {
			return "Gain card " + card.Name
		}
		return "Gain a card"
	case "add_combat_card":
		name := HumanizeID(effect.CardID)
		if card, ok := lib.Cards[effect.CardID]; ok {
			name = card.Name
		}
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		pile := "draw pile"
		switch effect.ItemType {
		case "hand":
			pile = "hand"
		case "discard":
			pile = "discard pile"
		}
		if count == 1 {
			return fmt.Sprintf("Add %s to %s", name, pile)
		}
		return fmt.Sprintf("Add %d %s to %s", count, name, pile)
	case "upgrade_card":
		return "Upgrade a card"
	case "augment_card":
		effectName := DescribeEffects(lib, effect.Effects)
		selector := augmentSelectorLabelEN(effect.Selector)
		if effect.Tag != "" {
			selector += " (" + HumanizeID(effect.Tag) + ")"
		}
		return selector + " gains: " + effectName + " (" + AugmentScopeLabel(i18n.LangEnUS, effect.Scope) + ")"
	case "gain_energy":
		return fmt.Sprintf("Gain %d Energy", effect.Value)
	case "potion_capacity":
		return fmt.Sprintf("Potion slots +%d", effect.Value)
	case "repeat_next_card":
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		return pendingRepeatDescriptionEN(count, effect.Tag)
	case "reply":
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		return fmt.Sprintf("Repeat this card %d extra times", count)
	default:
		return effect.Op
	}
}

func describeEffectZH(lib *Library, effect Effect) string {
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
		return fmt.Sprintf("%s %s %d%s", effectStatusTargetLabelZH(effect.Target), statusLabelZH(effect.Status), effect.Value, durationSuffixZH(effect.Duration))
	case "cleanse_status":
		return fmt.Sprintf("净化%s的%s", effectOwnerLabelZH(effect.Target), statusLabelZH(effect.Status))
	case "modify_damage":
		return fmt.Sprintf("%s伤害 %+d", modifierTriggerLabelZH(effect.Trigger), effect.Value)
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
	case "add_combat_card":
		name := effect.CardID
		if card, ok := lib.Cards[effect.CardID]; ok {
			name = card.Name
		}
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		pile := "抽牌堆"
		switch effect.ItemType {
		case "hand":
			pile = "手牌"
		case "discard":
			pile = "弃牌堆"
		}
		if count == 1 {
			return fmt.Sprintf("向%s加入 %s", pile, name)
		}
		return fmt.Sprintf("向%s加入 %d 张%s", pile, count, name)
	case "upgrade_card":
		return "强化一张可升级卡牌"
	case "augment_card":
		effectName := DescribeEffects(lib, effect.Effects)
		selector := augmentSelectorLabelZH(effect.Selector)
		if effect.Tag != "" {
			selector += "（" + effect.Tag + "）"
		}
		return selector + "附加效果：" + effectName + "（" + AugmentScopeLabel(i18n.LangZhCN, effect.Scope) + "）"
	case "gain_energy":
		return fmt.Sprintf("获得 %d 能量", effect.Value)
	case "potion_capacity":
		return fmt.Sprintf("药水栏位 +%d", effect.Value)
	case "repeat_next_card":
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		return pendingRepeatDescriptionZH(count, effect.Tag)
	case "reply":
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		return fmt.Sprintf("本牌额外重复 %d 次", count)
	default:
		return effect.Op
	}
}

func effectStatusTargetLabelEN(target string) string {
	switch target {
	case "self":
		return "Gain"
	case "player":
		return "Player gains"
	case "enemy":
		return "Enemy gains"
	case "opponent":
		return "Opponent gains"
	case "all_allies":
		return "All allies gain"
	case "all_enemies":
		return "All enemies gain"
	default:
		return "Apply"
	}
}

func effectStatusTargetLabelZH(target string) string {
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

func effectOwnerLabelEN(target string) string {
	switch target {
	case "self":
		return "self"
	case "player":
		return "the player"
	case "enemy":
		return "the enemy"
	case "opponent":
		return "the opponent"
	case "all_allies":
		return "all allies"
	case "all_enemies":
		return "all enemies"
	default:
		return "the target"
	}
}

func effectOwnerLabelZH(target string) string {
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

func modifierTriggerLabelEN(trigger string) string {
	switch trigger {
	case "attack":
		return "Attack"
	case "spell":
		return "Spell"
	default:
		return "All"
	}
}

func modifierTriggerLabelZH(trigger string) string {
	switch trigger {
	case "attack":
		return "攻击"
	case "spell":
		return "法术"
	default:
		return "所有"
	}
}

func statusLabelEN(name string) string {
	switch name {
	case "strength":
		return "Strength"
	case "weak":
		return "Weak"
	case "vulnerable":
		return "Vulnerable"
	case "burn":
		return "Burn"
	case "poison":
		return "Poison"
	case "thorns":
		return "Thorns"
	case "regen":
		return "Regen"
	case "focus":
		return "Focus"
	case "taunt":
		return "Taunt"
	case "sheltered":
		return "Sheltered"
	default:
		return HumanizeID(name)
	}
}

func statusLabelZH(name string) string {
	switch name {
	case "strength":
		return "力量"
	case "weak":
		return "虚弱"
	case "vulnerable":
		return "易伤"
	case "burn":
		return "燃烧"
	case "poison":
		return "中毒"
	case "thorns":
		return "荆棘"
	case "regen":
		return "再生"
	case "focus":
		return "聚能"
	case "taunt":
		return "嘲讽"
	case "sheltered":
		return "庇护"
	default:
		return name
	}
}

func durationSuffixEN(duration int) string {
	if duration <= 0 {
		return ""
	}
	return fmt.Sprintf(" (%d turns)", duration)
}

func durationSuffixZH(duration int) string {
	if duration <= 0 {
		return ""
	}
	return fmt.Sprintf("(%d 回合)", duration)
}

func pendingRepeatDescriptionEN(count int, tag string) string {
	count = max(1, count)
	switch tag {
	case "attack":
		return fmt.Sprintf("Repeat the next Attack card %d extra times", count)
	case "spell":
		return fmt.Sprintf("Repeat the next Spell card %d extra times", count)
	default:
		return fmt.Sprintf("Repeat the next card %d extra times", count)
	}
}

func pendingRepeatDescriptionZH(count int, tag string) string {
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

func augmentSelectorLabelEN(selector string) string {
	switch strings.TrimSpace(selector) {
	case "all":
		return "All cards"
	case "choose_upgradable":
		return "Choose an upgradable card"
	case "all_upgradable":
		return "All upgradable cards"
	default:
		return "Choose a card"
	}
}

func augmentSelectorLabelZH(selector string) string {
	switch strings.TrimSpace(selector) {
	case "all":
		return "所有卡牌"
	case "choose_upgradable":
		return "选择一张可升级卡牌"
	case "all_upgradable":
		return "所有可升级卡牌"
	default:
		return "选择一张卡牌"
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
