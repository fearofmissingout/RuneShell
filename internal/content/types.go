package content

type Effect struct {
	Op       string   `json:"op"`
	Name     string   `json:"name,omitempty"`
	Value    int      `json:"value,omitempty"`
	Target   string   `json:"target,omitempty"`
	Scope    string   `json:"scope,omitempty"`
	Selector string   `json:"selector,omitempty"`
	Trigger  string   `json:"trigger,omitempty"`
	Status   string   `json:"status,omitempty"`
	Duration int      `json:"duration,omitempty"`
	Tag      string   `json:"tag,omitempty"`
	CardID   string   `json:"card_id,omitempty"`
	ItemID   string   `json:"item_id,omitempty"`
	ResultID string   `json:"result_id,omitempty"`
	ItemType string   `json:"item_type,omitempty"`
	Count    int      `json:"count,omitempty"`
	Flags    []string `json:"flags,omitempty"`
	Effects  []Effect `json:"effects,omitempty"`
}

type ClassDef struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	BaseHP            int      `json:"base_hp"`
	BaseGold          int      `json:"base_gold"`
	MaxEnergy         int      `json:"max_energy"`
	StartingDeck      []string `json:"starting_deck"`
	CardPool          []string `json:"card_pool"`
	StartingRelics    []string `json:"starting_relics"`
	StartingPotions   []string `json:"starting_potions"`
	StartingEquipment []string `json:"starting_equipment"`
}

type CardDef struct {
	ID             string   `json:"id"`
	ClassID        string   `json:"class_id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Rarity         string   `json:"rarity"`
	Cost           int      `json:"cost"`
	Tags           []string `json:"tags"`
	Flags          []string `json:"flags,omitempty"`
	Effects        []Effect `json:"effects"`
	UpgradeEffects []Effect `json:"upgrade_effects,omitempty"`
	Exhaust        bool     `json:"exhaust,omitempty"`
}

type RelicDef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rarity      string   `json:"rarity"`
	Flags       []string `json:"flags,omitempty"`
	Effects     []Effect `json:"effects"`
}

type PotionDef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rarity      string   `json:"rarity"`
	Effects     []Effect `json:"effects"`
}

type EquipmentDef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Slot        string   `json:"slot"`
	Rarity      string   `json:"rarity"`
	Effects     []Effect `json:"effects"`
}

type EnemyIntentDef struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Effects     []Effect `json:"effects"`
}

type EncounterDef struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Kind        string           `json:"kind"`
	Act         int              `json:"act"`
	HP          int              `json:"hp"`
	GoldReward  int              `json:"gold_reward"`
	CardReward  int              `json:"card_reward"`
	Passives    []Effect         `json:"passives,omitempty"`
	IntentCycle []EnemyIntentDef `json:"intent_cycle"`
}

type EventChoiceDef struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Description    string   `json:"description"`
	RequiresRelics []string `json:"requires_relics,omitempty"`
	Effects        []Effect `json:"effects"`
}

type EventDef struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Acts        []int            `json:"acts"`
	Flags       []string         `json:"flags,omitempty"`
	Choices     []EventChoiceDef `json:"choices"`
}

type Library struct {
	Classes        map[string]ClassDef
	Cards          map[string]CardDef
	Relics         map[string]RelicDef
	Potions        map[string]PotionDef
	Equipments     map[string]EquipmentDef
	Encounters     map[string]EncounterDef
	Events         map[string]EventDef
	classOrder     []string
	cardOrder      []string
	relicOrder     []string
	potionOrder    []string
	equipmentOrder []string
}

func (l *Library) ClassList() []ClassDef {
	out := make([]ClassDef, 0, len(l.classOrder))
	for _, id := range l.classOrder {
		out = append(out, l.Classes[id])
	}
	return out
}

func (l *Library) CardList() []CardDef {
	out := make([]CardDef, 0, len(l.cardOrder))
	for _, id := range l.cardOrder {
		out = append(out, l.Cards[id])
	}
	return out
}

func (l *Library) RelicList() []RelicDef {
	out := make([]RelicDef, 0, len(l.relicOrder))
	for _, id := range l.relicOrder {
		out = append(out, l.Relics[id])
	}
	return out
}

func (l *Library) PotionList() []PotionDef {
	out := make([]PotionDef, 0, len(l.potionOrder))
	for _, id := range l.potionOrder {
		out = append(out, l.Potions[id])
	}
	return out
}

func (l *Library) EquipmentList() []EquipmentDef {
	out := make([]EquipmentDef, 0, len(l.equipmentOrder))
	for _, id := range l.equipmentOrder {
		out = append(out, l.Equipments[id])
	}
	return out
}
