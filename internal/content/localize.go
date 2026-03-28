package content

import (
	"encoding/json"
	"fmt"
	"path"
	"sync"

	"cmdcards/internal/i18n"
)

type localizedText struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type localizedChoice struct {
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

type localizedIntent struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type localizedEncounter struct {
	Name    string            `json:"name,omitempty"`
	Intents []localizedIntent `json:"intents,omitempty"`
}

type localizedEvent struct {
	Name        string                     `json:"name,omitempty"`
	Description string                     `json:"description,omitempty"`
	Choices     map[string]localizedChoice `json:"choices,omitempty"`
}

type localeBundle struct {
	Classes    map[string]localizedText      `json:"classes,omitempty"`
	Cards      map[string]localizedText      `json:"cards,omitempty"`
	Relics     map[string]localizedText      `json:"relics,omitempty"`
	Potions    map[string]localizedText      `json:"potions,omitempty"`
	Equipments map[string]localizedText      `json:"equipments,omitempty"`
	Encounters map[string]localizedEncounter `json:"encounters,omitempty"`
	Events     map[string]localizedEvent     `json:"events,omitempty"`
}

var (
	localeCacheMu sync.Mutex
	localeCache   = map[string]*localeBundle{}
)

func LocalizeLibrary(base *Library, lang string) *Library {
	if base == nil {
		return nil
	}
	normalized := i18n.NormalizeLanguage(lang)
	clone := cloneLibrary(base)
	clone.Language = normalized
	if normalized == i18n.DefaultLanguage {
		return clone
	}
	bundle, err := loadLocaleBundle(normalized)
	if err != nil || bundle == nil {
		return clone
	}
	localizeNames(clone, bundle)
	localizeDescriptions(clone, bundle)
	return clone
}

func loadLocaleBundle(lang string) (*localeBundle, error) {
	localeCacheMu.Lock()
	if bundle, ok := localeCache[lang]; ok {
		localeCacheMu.Unlock()
		return bundle, nil
	}
	localeCacheMu.Unlock()

	filename := path.Join("locales", lang+".json")
	data, err := embeddedFiles.ReadFile(filename)
	if err != nil {
		return nil, nil
	}
	var bundle localeBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("decode %s: %w", filename, err)
	}

	localeCacheMu.Lock()
	localeCache[lang] = &bundle
	localeCacheMu.Unlock()
	return &bundle, nil
}

func localizeNames(lib *Library, bundle *localeBundle) {
	for id, def := range lib.Classes {
		override := bundle.Classes[id]
		if override.Name != "" {
			def.Name = override.Name
		} else {
			def.Name = HumanizeID(id)
		}
		lib.Classes[id] = def
	}
	for id, def := range lib.Cards {
		if override := bundle.Cards[id]; override.Name != "" {
			def.Name = override.Name
		} else {
			def.Name = HumanizeID(id)
		}
		lib.Cards[id] = def
	}
	for id, def := range lib.Relics {
		if override := bundle.Relics[id]; override.Name != "" {
			def.Name = override.Name
		} else {
			def.Name = HumanizeID(id)
		}
		lib.Relics[id] = def
	}
	for id, def := range lib.Potions {
		if override := bundle.Potions[id]; override.Name != "" {
			def.Name = override.Name
		} else {
			def.Name = HumanizeID(id)
		}
		lib.Potions[id] = def
	}
	for id, def := range lib.Equipments {
		if override := bundle.Equipments[id]; override.Name != "" {
			def.Name = override.Name
		} else {
			def.Name = HumanizeID(id)
		}
		lib.Equipments[id] = def
	}
	for id, def := range lib.Encounters {
		override := bundle.Encounters[id]
		if override.Name != "" {
			def.Name = override.Name
		} else {
			def.Name = HumanizeID(id)
		}
		for i := range def.IntentCycle {
			if i < len(override.Intents) && override.Intents[i].Name != "" {
				def.IntentCycle[i].Name = override.Intents[i].Name
			}
		}
		lib.Encounters[id] = def
	}
	for id, def := range lib.Events {
		override := bundle.Events[id]
		if override.Name != "" {
			def.Name = override.Name
		} else {
			def.Name = HumanizeID(id)
		}
		for i := range def.Choices {
			if choiceOverride, ok := override.Choices[def.Choices[i].ID]; ok && choiceOverride.Label != "" {
				def.Choices[i].Label = choiceOverride.Label
			} else {
				def.Choices[i].Label = HumanizeID(def.Choices[i].ID)
			}
		}
		lib.Events[id] = def
	}
}

func localizeDescriptions(lib *Library, bundle *localeBundle) {
	for id, def := range lib.Classes {
		if override := bundle.Classes[id]; override.Description != "" {
			def.Description = override.Description
			lib.Classes[id] = def
		}
	}
	for id, def := range lib.Cards {
		if override := bundle.Cards[id]; override.Description != "" {
			def.Description = override.Description
		} else {
			def.Description = DescribeEffects(lib, def.Effects)
		}
		lib.Cards[id] = def
	}
	for id, def := range lib.Relics {
		if override := bundle.Relics[id]; override.Description != "" {
			def.Description = override.Description
		} else {
			def.Description = DescribeEffects(lib, def.Effects)
		}
		lib.Relics[id] = def
	}
	for id, def := range lib.Potions {
		if override := bundle.Potions[id]; override.Description != "" {
			def.Description = override.Description
		} else {
			def.Description = DescribeEffects(lib, def.Effects)
		}
		lib.Potions[id] = def
	}
	for id, def := range lib.Equipments {
		if override := bundle.Equipments[id]; override.Description != "" {
			def.Description = override.Description
		} else {
			def.Description = DescribeEffects(lib, def.Effects)
		}
		lib.Equipments[id] = def
	}
	for id, def := range lib.Encounters {
		override := bundle.Encounters[id]
		for i := range def.IntentCycle {
			if i < len(override.Intents) && override.Intents[i].Description != "" {
				def.IntentCycle[i].Description = override.Intents[i].Description
			} else {
				def.IntentCycle[i].Description = DescribeEffects(lib, def.IntentCycle[i].Effects)
			}
		}
		lib.Encounters[id] = def
	}
	for id, def := range lib.Events {
		override := bundle.Events[id]
		if override.Description != "" {
			def.Description = override.Description
		}
		for i := range def.Choices {
			if choiceOverride, ok := override.Choices[def.Choices[i].ID]; ok && choiceOverride.Description != "" {
				def.Choices[i].Description = choiceOverride.Description
			} else {
				def.Choices[i].Description = DescribeEffects(lib, def.Choices[i].Effects)
			}
		}
		lib.Events[id] = def
	}
}

func cloneLibrary(base *Library) *Library {
	lib := &Library{
		Language:       base.Language,
		Classes:        map[string]ClassDef{},
		Cards:          map[string]CardDef{},
		Relics:         map[string]RelicDef{},
		Potions:        map[string]PotionDef{},
		Equipments:     map[string]EquipmentDef{},
		Encounters:     map[string]EncounterDef{},
		Events:         map[string]EventDef{},
		classOrder:     append([]string{}, base.classOrder...),
		cardOrder:      append([]string{}, base.cardOrder...),
		relicOrder:     append([]string{}, base.relicOrder...),
		potionOrder:    append([]string{}, base.potionOrder...),
		equipmentOrder: append([]string{}, base.equipmentOrder...),
		encounterOrder: append([]string{}, base.encounterOrder...),
		eventOrder:     append([]string{}, base.eventOrder...),
	}
	for id, def := range base.Classes {
		cloned := def
		cloned.StartingDeck = append([]string{}, def.StartingDeck...)
		cloned.CardPool = append([]string{}, def.CardPool...)
		cloned.StartingRelics = append([]string{}, def.StartingRelics...)
		cloned.StartingPotions = append([]string{}, def.StartingPotions...)
		cloned.StartingEquipment = append([]string{}, def.StartingEquipment...)
		lib.Classes[id] = cloned
	}
	for id, def := range base.Cards {
		cloned := def
		cloned.Tags = append([]string{}, def.Tags...)
		cloned.Flags = append([]string{}, def.Flags...)
		cloned.Effects = cloneEffects(def.Effects)
		cloned.UpgradeEffects = cloneEffects(def.UpgradeEffects)
		lib.Cards[id] = cloned
	}
	for id, def := range base.Relics {
		cloned := def
		cloned.Flags = append([]string{}, def.Flags...)
		cloned.Effects = cloneEffects(def.Effects)
		lib.Relics[id] = cloned
	}
	for id, def := range base.Potions {
		cloned := def
		cloned.Effects = cloneEffects(def.Effects)
		lib.Potions[id] = cloned
	}
	for id, def := range base.Equipments {
		cloned := def
		cloned.Effects = cloneEffects(def.Effects)
		lib.Equipments[id] = cloned
	}
	for id, def := range base.Encounters {
		cloned := def
		cloned.Passives = cloneEffects(def.Passives)
		cloned.IntentCycle = make([]EnemyIntentDef, 0, len(def.IntentCycle))
		for _, intent := range def.IntentCycle {
			cloned.IntentCycle = append(cloned.IntentCycle, EnemyIntentDef{
				Name:        intent.Name,
				Description: intent.Description,
				Effects:     cloneEffects(intent.Effects),
			})
		}
		lib.Encounters[id] = cloned
	}
	for id, def := range base.Events {
		cloned := def
		cloned.Acts = append([]int{}, def.Acts...)
		cloned.Flags = append([]string{}, def.Flags...)
		cloned.Choices = make([]EventChoiceDef, 0, len(def.Choices))
		for _, choice := range def.Choices {
			cloned.Choices = append(cloned.Choices, EventChoiceDef{
				ID:             choice.ID,
				Label:          choice.Label,
				Description:    choice.Description,
				RequiresRelics: append([]string{}, choice.RequiresRelics...),
				Effects:        cloneEffects(choice.Effects),
			})
		}
		lib.Events[id] = cloned
	}
	return lib
}

func cloneEffects(effects []Effect) []Effect {
	if len(effects) == 0 {
		return nil
	}
	cloned := make([]Effect, 0, len(effects))
	for _, effect := range effects {
		next := effect
		next.Flags = append([]string{}, effect.Flags...)
		next.Effects = cloneEffects(effect.Effects)
		cloned = append(cloned, next)
	}
	return cloned
}
