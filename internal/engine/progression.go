package engine

import (
	"fmt"
	"slices"
	"sort"

	"cmdcards/internal/content"
	"cmdcards/internal/i18n"
)

const CurrentProfileVersion = 3

type PerkDef struct {
	ID          string
	Name        string
	Description string
	Step        int
	BaseCost    int
	MaxLevel    int
}

var perkDefs = []PerkDef{
	{
		ID:          "bonus_max_hp",
		Name:        "坚韧体魄",
		Description: "每级 +4 最大生命。",
		Step:        4,
		BaseCost:    2,
		MaxLevel:    5,
	},
	{
		ID:          "bonus_start_gold",
		Name:        "行囊储备",
		Description: "每级 +15 初始金币。",
		Step:        15,
		BaseCost:    2,
		MaxLevel:    6,
	},
	{
		ID:          "extra_potion_slot",
		Name:        "药剂腰带",
		Description: "每级 +1 药水栏位。",
		Step:        1,
		BaseCost:    4,
		MaxLevel:    3,
	},
	{
		ID:          "bonus_start_block",
		Name:        "先手护架",
		Description: "每级在战斗开始时获得 +2 格挡。",
		Step:        2,
		BaseCost:    3,
		MaxLevel:    4,
	},
	{
		ID:          "bonus_rest_heal",
		Name:        "篝火余温",
		Description: "每级让篝火休息额外恢复 +4 生命。",
		Step:        4,
		BaseCost:    3,
		MaxLevel:    4,
	},
}

func ProgressionPerks() []PerkDef {
	return slices.Clone(perkDefs)
}

func NormalizeProfile(lib *content.Library, profile *Profile) {
	if profile.Version < CurrentProfileVersion {
		profile.Version = CurrentProfileVersion
	}
	if profile.Perks == nil {
		profile.Perks = map[string]int{}
	}
	if profile.Language == "" {
		profile.Language = i18n.DefaultLanguage
	}
	if profile.ContentUnlocks == nil {
		profile.ContentUnlocks = map[string][]string{}
	}
	for key, value := range defaultPerkValues() {
		if _, ok := profile.Perks[key]; !ok {
			profile.Perks[key] = value
		}
	}
	if len(profile.UnlockedClasses) == 0 {
		for _, class := range lib.ClassList() {
			profile.UnlockedClasses = append(profile.UnlockedClasses, class.ID)
		}
	}
	if profile.UnlockedEquipments == nil {
		profile.UnlockedEquipments = []string{}
	}
	for _, id := range defaultUnlockedEquipmentIDs(lib) {
		if !slices.Contains(profile.UnlockedEquipments, id) {
			profile.UnlockedEquipments = append(profile.UnlockedEquipments, id)
		}
	}
	sort.Strings(profile.UnlockedEquipments)
	if profile.ClassLoadouts == nil {
		profile.ClassLoadouts = map[string]EquipmentSlots{}
	}
	for _, class := range lib.ClassList() {
		slots := profile.ClassLoadouts[class.ID]
		defaults := defaultEquipmentSlots(class, lib)
		if slots.Weapon == "" {
			slots.Weapon = defaults.Weapon
		}
		if slots.Armor == "" {
			slots.Armor = defaults.Armor
		}
		if slots.Accessory == "" {
			slots.Accessory = defaults.Accessory
		}
		if !IsEquipmentUnlocked(*profile, slots.Weapon) {
			slots.Weapon = defaults.Weapon
		}
		if !IsEquipmentUnlocked(*profile, slots.Armor) {
			slots.Armor = defaults.Armor
		}
		if slots.Accessory != "" && !IsEquipmentUnlocked(*profile, slots.Accessory) {
			slots.Accessory = defaults.Accessory
		}
		profile.ClassLoadouts[class.ID] = slots
	}
}

func defaultPerkValues() map[string]int {
	return map[string]int{
		"bonus_max_hp":      8,
		"bonus_start_gold":  30,
		"extra_potion_slot": 1,
		"bonus_start_block": 0,
		"bonus_rest_heal":   0,
	}
}

func defaultUnlockedEquipmentIDs(lib *content.Library) []string {
	ids := []string{}
	for _, class := range lib.ClassList() {
		for _, id := range class.StartingEquipment {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func defaultEquipmentSlots(class content.ClassDef, lib *content.Library) EquipmentSlots {
	slots := EquipmentSlots{}
	for _, id := range class.StartingEquipment {
		item, ok := lib.Equipments[id]
		if !ok {
			continue
		}
		switch item.Slot {
		case "weapon":
			slots.Weapon = id
		case "armor":
			slots.Armor = id
		case "accessory":
			slots.Accessory = id
		}
	}
	return slots
}

func IsEquipmentUnlocked(profile Profile, equipmentID string) bool {
	if equipmentID == "" {
		return false
	}
	return slices.Contains(profile.UnlockedEquipments, equipmentID)
}

func EquipmentUnlockCost(rarity string) int {
	switch rarity {
	case "legendary":
		return 12
	case "rare":
		return 8
	case "uncommon":
		return 5
	case "starter":
		return 0
	default:
		return 3
	}
}

func UnlockEquipment(lib *content.Library, profile *Profile, equipmentID string) error {
	item, ok := lib.Equipments[equipmentID]
	if !ok {
		return fmt.Errorf("unknown equipment %q", equipmentID)
	}
	if IsEquipmentUnlocked(*profile, equipmentID) {
		return fmt.Errorf("装备已解锁")
	}
	cost := EquipmentUnlockCost(item.Rarity)
	if profile.MetaCurrency < cost {
		return fmt.Errorf("局外货币不足")
	}
	profile.MetaCurrency -= cost
	profile.UnlockedEquipments = append(profile.UnlockedEquipments, equipmentID)
	sort.Strings(profile.UnlockedEquipments)
	return nil
}

func UnlockedEquipmentOptions(lib *content.Library, profile Profile, slot string) []content.EquipmentDef {
	out := []content.EquipmentDef{}
	for _, equipment := range lib.EquipmentList() {
		if equipment.Slot != slot || !IsEquipmentUnlocked(profile, equipment.ID) {
			continue
		}
		out = append(out, equipment)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if rarityScore(out[i].Rarity) == rarityScore(out[j].Rarity) {
			return out[i].Name < out[j].Name
		}
		return rarityScore(out[i].Rarity) < rarityScore(out[j].Rarity)
	})
	return out
}

func EffectiveLoadout(lib *content.Library, profile Profile, class content.ClassDef) EquipmentSlots {
	slots := defaultEquipmentSlots(class, lib)
	if custom, ok := profile.ClassLoadouts[class.ID]; ok {
		if custom.Weapon != "" && IsEquipmentUnlocked(profile, custom.Weapon) {
			slots.Weapon = custom.Weapon
		}
		if custom.Armor != "" && IsEquipmentUnlocked(profile, custom.Armor) {
			slots.Armor = custom.Armor
		}
		if custom.Accessory != "" && IsEquipmentUnlocked(profile, custom.Accessory) {
			slots.Accessory = custom.Accessory
		}
	}
	return slots
}

func EffectiveStartingEquipment(lib *content.Library, profile Profile, class content.ClassDef) []string {
	slots := EffectiveLoadout(lib, profile, class)
	out := []string{}
	for _, id := range []string{slots.Weapon, slots.Armor, slots.Accessory} {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func CycleLoadoutEquipment(lib *content.Library, profile *Profile, classID, slot string) (string, error) {
	class, ok := lib.Classes[classID]
	if !ok {
		return "", fmt.Errorf("unknown class %q", classID)
	}
	options := UnlockedEquipmentOptions(lib, *profile, slot)
	if slot == "accessory" {
		options = append([]content.EquipmentDef{{ID: "", Name: "空置", Slot: "accessory", Rarity: "starter"}}, options...)
	}
	if len(options) == 0 {
		return "", fmt.Errorf("当前没有可用装备")
	}
	current := CurrentEquipmentID(PlayerState{Equipment: EffectiveLoadout(lib, *profile, class)}, slot)
	nextIndex := 0
	for i, option := range options {
		if option.ID == current {
			nextIndex = (i + 1) % len(options)
			break
		}
	}
	slots := profile.ClassLoadouts[classID]
	switch slot {
	case "weapon":
		slots.Weapon = options[nextIndex].ID
	case "armor":
		slots.Armor = options[nextIndex].ID
	case "accessory":
		slots.Accessory = options[nextIndex].ID
	default:
		return "", fmt.Errorf("unknown slot %q", slot)
	}
	profile.ClassLoadouts[classID] = slots
	return options[nextIndex].ID, nil
}

func FindPerkDef(id string) (PerkDef, bool) {
	for _, def := range perkDefs {
		if def.ID == id {
			return def, true
		}
	}
	return PerkDef{}, false
}

func PerkLevel(def PerkDef, value int) int {
	if def.Step <= 0 {
		return 0
	}
	return value / def.Step
}

func NextPerkCost(profile Profile, perkID string) (int, bool) {
	def, ok := FindPerkDef(perkID)
	if !ok {
		return 0, false
	}
	level := PerkLevel(def, profile.Perks[perkID])
	if level >= def.MaxLevel {
		return 0, false
	}
	return def.BaseCost + level, true
}

func UpgradePerk(profile *Profile, perkID string) error {
	def, ok := FindPerkDef(perkID)
	if !ok {
		return fmt.Errorf("unknown perk %q", perkID)
	}
	cost, canUpgrade := NextPerkCost(*profile, perkID)
	if !canUpgrade {
		return fmt.Errorf("该成长已到上限")
	}
	if profile.MetaCurrency < cost {
		return fmt.Errorf("局外货币不足")
	}
	profile.MetaCurrency -= cost
	profile.Perks[perkID] += def.Step
	return nil
}
