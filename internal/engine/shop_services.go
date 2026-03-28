package engine

import (
	"fmt"
	"slices"

	"cmdcards/internal/content"
)

type shopServiceDefinition struct {
	Offer  ShopOffer
	Effect *content.Effect
}

type augmentShopServiceSpec struct {
	ID          string
	ItemID      string
	Name        string
	Description string
	Price       int
	Effect      content.Effect
}

func availableShopServices(lib *content.Library, run *RunState) []ShopOffer {
	defs := shopServiceDefinitions(lib, run)
	offers := make([]ShopOffer, 0, len(defs))
	for _, def := range defs {
		offers = append(offers, def.Offer)
	}
	return offers
}

func ShopOfferDeckActionPlan(lib *content.Library, run *RunState, shop *ShopState, offerID string) (*DeckActionPlan, error) {
	offer, err := findShopOffer(shop, offerID)
	if err != nil {
		return nil, err
	}
	if offer.Kind != "service" {
		return nil, nil
	}
	if run.Player.Gold < offer.Price {
		return nil, fmt.Errorf("金币不足")
	}
	def, ok := shopServiceDefinitionByItemID(lib, run, offer.ItemID)
	if !ok || def.Effect == nil {
		return nil, nil
	}
	plan, err := BuildAugmentDeckActionPlan(lib, run.Player, *def.Effect, offer.Name, false)
	if err != nil || plan == nil {
		return plan, err
	}
	plan.Mode = "shop_augment_card"
	plan.Price = offer.Price
	if plan.Subtitle != "" {
		plan.Subtitle += fmt.Sprintf(" | 价格 %d 金币", offer.Price)
	}
	return plan, nil
}

func ApplyShopServiceWithDeckChoice(lib *content.Library, run *RunState, shop *ShopState, offerID string, deckIndex int) error {
	offer, err := findShopOffer(shop, offerID)
	if err != nil {
		return err
	}
	if offer.Kind != "service" {
		return fmt.Errorf("offer %q is not a service", offerID)
	}
	if run.Player.Gold < offer.Price {
		return fmt.Errorf("金币不足")
	}
	def, ok := shopServiceDefinitionByItemID(lib, run, offer.ItemID)
	if !ok || def.Effect == nil {
		return fmt.Errorf("service %q does not support card selection", offer.ItemID)
	}
	run.Player.Gold -= offer.Price
	lines, err := ApplyDeckCardAugmentEffect(lib, &run.Player, *def.Effect, deckIndex)
	if err != nil {
		run.Player.Gold += offer.Price
		return err
	}
	for _, line := range lines {
		shop.Log = append(shop.Log, "工坊服务 "+offer.Name+"："+line)
	}
	consumeShopOffer(shop, offer.ID)
	return nil
}

func shopServiceDefinitionByItemID(lib *content.Library, run *RunState, itemID string) (shopServiceDefinition, bool) {
	for _, def := range shopServiceDefinitions(lib, run) {
		if def.Offer.ItemID == itemID {
			return def, true
		}
	}
	return shopServiceDefinition{}, false
}

func appendAvailableAugmentShopService(defs []shopServiceDefinition, lib *content.Library, player PlayerState, spec augmentShopServiceSpec) []shopServiceDefinition {
	indexes, err := augmentCardCandidateIndexes(lib, player, spec.Effect)
	if err != nil || len(indexes) == 0 {
		return defs
	}
	return append(defs, shopServiceDefinition{
		Offer: ShopOffer{
			ID:          spec.ID,
			Kind:        "service",
			Name:        spec.Name,
			Description: spec.Description,
			Price:       spec.Price,
			ItemID:      spec.ItemID,
		},
		Effect: cloneEffectPtr(spec.Effect),
	})
}

func shopServiceDefinitions(lib *content.Library, run *RunState) []shopServiceDefinition {
	defs := []shopServiceDefinition{}

	augmentSpecs := []augmentShopServiceSpec{
		{
			ID:          "service-echo-workshop",
			ItemID:      "service_echo_workshop",
			Name:        "回响工坊",
			Description: "选择一张攻击牌，本局使其使用时额外抽 1 张牌。",
			Price:       66,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "echo_script",
				Scope:    "run",
				Selector: "choose",
				Tag:      "attack",
				Effects: []content.Effect{
					{Op: "draw", Value: 1},
				},
			},
		},
		{
			ID:          "service-flash-workshop",
			ItemID:      "service_flash_workshop",
			Name:        "瞬充工坊",
			Description: "选择一张可升级牌，下场战斗中它使用时额外获得 1 点能量。",
			Price:       72,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "flash_charge",
				Scope:    "combat",
				Selector: "choose_upgradable",
				Effects: []content.Effect{
					{Op: "gain_energy", Value: 1},
				},
			},
		},
		{
			ID:          "service-ember-workshop",
			ItemID:      "service_ember_workshop",
			Name:        "余烬工坊",
			Description: "选择一张攻击牌，本局使其命中时额外施加 2 层燃烧。",
			Price:       68,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "ember_brand",
				Scope:    "run",
				Selector: "choose",
				Tag:      "attack",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "enemy", Status: "burn", Value: 2, Duration: 2},
				},
			},
		},
		{
			ID:          "service-bastion-workshop",
			ItemID:      "service_bastion_workshop",
			Name:        "壁垒工坊",
			Description: "选择一张技能牌，本局使其使用时额外获得 4 点格挡。",
			Price:       64,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "bastion_trace",
				Scope:    "run",
				Selector: "choose",
				Tag:      "skill",
				Effects: []content.Effect{
					{Op: "block", Value: 4},
				},
			},
		},
		{
			ID:          "service-opening-workshop",
			ItemID:      "service_opening_workshop",
			Name:        "先机工坊",
			Description: "选择一张可升级牌，下场战斗的本回合里它使用时额外抽 1 张牌。",
			Price:       58,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "opening_spark",
				Scope:    "turn",
				Selector: "choose_upgradable",
				Effects: []content.Effect{
					{Op: "draw", Value: 1},
				},
			},
		},
		{
			ID:          "service-mirror-workshop",
			ItemID:      "service_mirror_workshop",
			Name:        "镜刻工坊",
			Description: "选择一张可升级牌，下场战斗中它会额外重复 1 次。",
			Price:       78,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "mirror_reply",
				Scope:    "combat",
				Selector: "choose_upgradable",
				Effects: []content.Effect{
					{Op: "reply", Value: 1},
				},
			},
		},
		{
			ID:          "service-renew-workshop",
			ItemID:      "service_renew_workshop",
			Name:        "回生工坊",
			Description: "选择一张技能牌，本局使其使用时额外获得 2 层再生。",
			Price:       67,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "renew_moss",
				Scope:    "run",
				Selector: "choose",
				Tag:      "skill",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "self", Status: "regen", Value: 2, Duration: 2},
				},
			},
		},
		{
			ID:          "service-resonance-workshop",
			ItemID:      "service_resonance_workshop",
			Name:        "共鸣工坊",
			Description: "选择一张法术牌，本局使其使用时额外获得 1 层聚能。",
			Price:       74,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "resonance_ink",
				Scope:    "run",
				Selector: "choose",
				Tag:      "spell",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "self", Status: "focus", Value: 1, Duration: 2},
				},
			},
		},
		{
			ID:          "service-sunder-workshop",
			ItemID:      "service_sunder_workshop",
			Name:        "裂甲工坊",
			Description: "选择一张攻击牌，本局使其命中时额外施加 1 层易伤。",
			Price:       70,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "sunder_mark",
				Scope:    "run",
				Selector: "choose",
				Tag:      "attack",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "enemy", Status: "vulnerable", Value: 1, Duration: 2},
				},
			},
		},
	}
	for _, spec := range augmentSpecs {
		defs = appendAvailableAugmentShopService(defs, lib, run.Player, spec)
	}

	if run.Player.ClassID == "vanguard" {
		defs = appendAvailableAugmentShopService(defs, lib, run.Player, augmentShopServiceSpec{
			ID:          "service-oathguard-workshop",
			ItemID:      "service_oathguard_workshop",
			Name:        "誓甲工坊",
			Description: "选择一张技能牌，本局使其使用时额外获得 1 层壁守。",
			Price:       71,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "oathguard_mark",
				Scope:    "run",
				Selector: "choose",
				Tag:      "skill",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "self", Status: "guard", Value: 1, Duration: 2},
				},
			},
		})
		defs = appendAvailableAugmentShopService(defs, lib, run.Player, augmentShopServiceSpec{
			ID:          "service-breach-workshop",
			ItemID:      "service_breach_workshop",
			Name:        "破阵工坊",
			Description: "选择一张攻击牌，下场战斗中它命中时额外施加 1 层脆弱。",
			Price:       73,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "breach_stamp",
				Scope:    "combat",
				Selector: "choose",
				Tag:      "attack",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "enemy", Status: "frail", Value: 1, Duration: 2},
				},
			},
		})
	}

	if run.Player.ClassID == "arcanist" {
		defs = appendAvailableAugmentShopService(defs, lib, run.Player, augmentShopServiceSpec{
			ID:          "service-cinderweave-workshop",
			ItemID:      "service_cinderweave_workshop",
			Name:        "烬纹工坊",
			Description: "选择一张法术牌，本局使其命中时额外施加 2 层燃烧。",
			Price:       75,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "cinderweave_brand",
				Scope:    "run",
				Selector: "choose",
				Tag:      "spell",
				Effects: []content.Effect{
					{Op: "apply_status", Target: "enemy", Status: "burn", Value: 2, Duration: 2},
				},
			},
		})
		defs = appendAvailableAugmentShopService(defs, lib, run.Player, augmentShopServiceSpec{
			ID:          "service-prismdraft-workshop",
			ItemID:      "service_prismdraft_workshop",
			Name:        "棱写工坊",
			Description: "选择一张法术牌，下场战斗中它使用时额外抽 1 张牌。",
			Price:       69,
			Effect: content.Effect{
				Op:       "augment_card",
				Name:     "prismdraft_echo",
				Scope:    "combat",
				Selector: "choose",
				Tag:      "spell",
				Effects: []content.Effect{
					{Op: "draw", Value: 1},
				},
			},
		})
	}

	if effectivePartySize(run) > 1 {
		defs = append(defs,
			shopServiceDefinition{
				Offer: ShopOffer{
					ID:          "service-coop-card",
					Kind:        "service",
					Name:        "协同简报",
					Description: "获得一张随机联机协作牌。",
					Price:       64,
					ItemID:      "service_coop_card",
				},
			},
			shopServiceDefinition{
				Offer: ShopOffer{
					ID:          "service-combo-drill",
					Kind:        "service",
					Name:        "连携操练",
					Description: "随机强化一张可升级牌并恢复 10 生命。",
					Price:       72,
					ItemID:      "service_combo_drill",
				},
			},
		)
	}

	slices.SortStableFunc(defs, func(a, b shopServiceDefinition) int {
		return compareStrings(a.Offer.Name, b.Offer.Name)
	})
	return defs
}
