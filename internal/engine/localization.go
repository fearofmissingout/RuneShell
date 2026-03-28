package engine

import (
	"fmt"
	"sort"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/i18n"
)

func contentLanguage(lib *content.Library) string {
	if lib == nil {
		return i18n.DefaultLanguage
	}
	return i18n.NormalizeLanguage(lib.Language)
}

func isEnglishLanguage(lang string) bool {
	return i18n.NormalizeLanguage(lang) == i18n.LangEnUS
}

func NodeKindNameFor(lang string, kind NodeKind) string {
	if isEnglishLanguage(lang) {
		switch kind {
		case NodeMonster:
			return "Monster"
		case NodeEvent:
			return "Event"
		case NodeShop:
			return "Shop"
		case NodeElite:
			return "Elite"
		case NodeRest:
			return "Rest"
		case NodeBoss:
			return "Boss"
		default:
			return strings.ToUpper(string(kind))
		}
	}
	switch kind {
	case NodeMonster:
		return "怪物"
	case NodeEvent:
		return "事件"
	case NodeShop:
		return "商店"
	case NodeElite:
		return "精英"
	case NodeRest:
		return "篝火"
	case NodeBoss:
		return "Boss"
	default:
		return strings.ToUpper(string(kind))
	}
}

func EquipmentSlotNameFor(lang string, slot string) string {
	if isEnglishLanguage(lang) {
		switch slot {
		case "weapon":
			return "Weapon"
		case "armor":
			return "Armor"
		case "accessory":
			return "Accessory"
		default:
			return slot
		}
	}
	switch slot {
	case "weapon":
		return "武器"
	case "armor":
		return "护甲"
	case "accessory":
		return "饰品"
	default:
		return slot
	}
}

func CombatInspectPaneNameFor(lang string, index int) string {
	switch normalizeCombatInspectPane(index) {
	case 0:
		if isEnglishLanguage(lang) {
			return "Overview"
		}
		return "概览"
	case 1:
		if isEnglishLanguage(lang) {
			return "Draw Pile"
		}
		return "抽牌堆"
	case 2:
		if isEnglishLanguage(lang) {
			return "Discard Pile"
		}
		return "弃牌堆"
	case 3:
		if isEnglishLanguage(lang) {
			return "Exhaust Pile"
		}
		return "消耗堆"
	case 4:
		if isEnglishLanguage(lang) {
			return "Deck"
		}
		return "牌组"
	default:
		if isEnglishLanguage(lang) {
			return "Effects"
		}
		return "效果"
	}
}

func CardAugmentScopeLabelFor(lang string, scope CardEffectScope) string {
	if isEnglishLanguage(lang) {
		switch scope {
		case CardEffectScopeCombat:
			return "next combat"
		case CardEffectScopeTurn:
			return "next combat turn"
		default:
			return "this run"
		}
	}
	switch scope {
	case CardEffectScopeCombat:
		return "下场战斗"
	case CardEffectScopeTurn:
		return "下场战斗的本回合"
	default:
		return "本局"
	}
}

func StatusLabelFor(lang string, name string) string {
	if isEnglishLanguage(lang) {
		switch name {
		case "vulnerable":
			return "Vulnerable"
		case "weak":
			return "Weak"
		case "frail":
			return "Frail"
		case "burn":
			return "Burn"
		case "poison":
			return "Poison"
		case "strength":
			return "Strength"
		case "focus":
			return "Focus"
		case "regen":
			return "Regen"
		case "thorns":
			return "Thorns"
		case "guard":
			return "Guard"
		case "sheltered":
			return "Sheltered"
		default:
			return strings.Title(name)
		}
	}
	switch name {
	case "vulnerable":
		return "易伤"
	case "weak":
		return "虚弱"
	case "frail":
		return "脆弱"
	case "burn":
		return "燃烧"
	case "poison":
		return "中毒"
	case "strength":
		return "力量"
	case "focus":
		return "专注"
	case "regen":
		return "再生"
	case "thorns":
		return "荆棘"
	case "guard":
		return "壁守"
	case "sheltered":
		return "庇护"
	default:
		return name
	}
}

func DescribeStatusesFor(lang string, statuses map[string]Status) string {
	if len(statuses) == 0 {
		return ""
	}
	names := make([]string, 0, len(statuses))
	for name := range statuses {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		status := statuses[name]
		part := fmt.Sprintf("%s:%d", StatusLabelFor(lang, name), status.Stacks)
		if status.Duration > 0 {
			if isEnglishLanguage(lang) {
				part = fmt.Sprintf("%s(%dT)", part, status.Duration)
			} else {
				part = fmt.Sprintf("%s(%d回合)", part, status.Duration)
			}
		}
		parts = append(parts, part)
	}
	if isEnglishLanguage(lang) {
		return strings.Join(parts, "  ")
	}
	return strings.Join(parts, "  ")
}

func DescribeIntentFor(lang string, intent content.EnemyIntentDef) string {
	lines := []string{}
	for _, effect := range intent.Effects {
		switch effect.Op {
		case "damage":
			if isEnglishLanguage(lang) {
				lines = append(lines, fmt.Sprintf("Attack %d", effect.Value))
			} else {
				lines = append(lines, fmt.Sprintf("攻击 %d", effect.Value))
			}
		case "block":
			if isEnglishLanguage(lang) {
				lines = append(lines, fmt.Sprintf("Block %d", effect.Value))
			} else {
				lines = append(lines, fmt.Sprintf("格挡 %d", effect.Value))
			}
		case "apply_status":
			target := effect.Target
			if target == "opponent" {
				if isEnglishLanguage(lang) {
					target = "Target"
				} else {
					target = "对手"
				}
			}
			lines = append(lines, fmt.Sprintf("%s %s %d", target, StatusLabelFor(lang, effect.Status), effect.Value))
		case "cleanse_status":
			if isEnglishLanguage(lang) {
				lines = append(lines, fmt.Sprintf("Cleanse %s", StatusLabelFor(lang, effect.Status)))
			} else {
				lines = append(lines, fmt.Sprintf("净化 %s", StatusLabelFor(lang, effect.Status)))
			}
		case "heal":
			if isEnglishLanguage(lang) {
				lines = append(lines, fmt.Sprintf("Heal %d", effect.Value))
			} else {
				lines = append(lines, fmt.Sprintf("治疗 %d", effect.Value))
			}
		case "lose_hp":
			if isEnglishLanguage(lang) {
				lines = append(lines, fmt.Sprintf("Lose %d HP", effect.Value))
			} else {
				lines = append(lines, fmt.Sprintf("失去 %d 生命", effect.Value))
			}
		case "add_combat_card":
			if isEnglishLanguage(lang) {
				lines = append(lines, "Add "+effect.CardID)
			} else {
				lines = append(lines, "加入"+effect.CardID)
			}
		}
	}
	return strings.Join(lines, " / ")
}

func PendingRepeatDescriptionFor(lang string, count int, tag string) string {
	if count <= 0 {
		count = 1
	}
	if isEnglishLanguage(lang) {
		switch strings.TrimSpace(tag) {
		case "attack":
			return fmt.Sprintf("Repeat next Attack %d time(s)", count)
		case "spell":
			return fmt.Sprintf("Repeat next Spell %d time(s)", count)
		case "":
			return fmt.Sprintf("Repeat next card %d time(s)", count)
		default:
			return fmt.Sprintf("Repeat next %s card %d time(s)", strings.Title(tag), count)
		}
	}
	switch strings.TrimSpace(tag) {
	case "attack":
		return fmt.Sprintf("下张攻击牌额外重复 %d 次", count)
	case "spell":
		return fmt.Sprintf("下张法术牌额外重复 %d 次", count)
	case "":
		return fmt.Sprintf("下张牌额外重复 %d 次", count)
	default:
		return fmt.Sprintf("下张%s牌额外重复 %d 次", tag, count)
	}
}
