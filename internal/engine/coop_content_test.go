package engine

import (
	"slices"
	"strings"
	"testing"

	"cmdcards/internal/content"
)

func TestCoopContentPoolsRespectPartySize(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	soloRun := &RunState{
		Mode:         ModeStory,
		Seed:         11,
		Act:          1,
		CurrentFloor: 1,
		PartySize:    1,
		Player: PlayerState{
			ClassID: "vanguard",
			Relics:  []string{"war_banner"},
		},
	}
	coopRun := &RunState{
		Mode:         ModeStory,
		Seed:         11,
		Act:          1,
		CurrentFloor: 1,
		PartySize:    2,
		Player: PlayerState{
			ClassID: "vanguard",
			Relics:  []string{"war_banner"},
		},
	}

	if got := len(coopCardsForRun(soloRun, lib.NeutralCards())); got != 0 {
		t.Fatalf("expected no coop-only cards for solo run, got %d", got)
	}
	if got := len(coopCardsForRun(coopRun, lib.NeutralCards())); got == 0 {
		t.Fatal("expected coop-only cards to be available for coop run")
	}

	soloShop := StartShop(lib, soloRun)
	for _, offer := range soloShop.Offers {
		if offer.CardID != "" && slices.Contains(lib.Cards[offer.CardID].Flags, "coop_only") {
			t.Fatalf("expected solo shop to exclude coop-only cards, got %q", offer.CardID)
		}
		if offer.ItemID != "" {
			if relic, ok := lib.Relics[offer.ItemID]; ok && slices.Contains(relic.Flags, "coop_only") {
				t.Fatalf("expected solo shop to exclude coop-only relics, got %q", offer.ItemID)
			}
		}
	}

	coopShop := StartShop(lib, coopRun)
	foundCoopCard := false
	foundCoopRelic := false
	for _, offer := range coopShop.Offers {
		if offer.CardID != "" && slices.Contains(lib.Cards[offer.CardID].Flags, "coop_only") {
			foundCoopCard = true
		}
		if offer.ItemID != "" {
			if relic, ok := lib.Relics[offer.ItemID]; ok && slices.Contains(relic.Flags, "coop_only") {
				foundCoopRelic = true
			}
		}
	}
	if !foundCoopCard {
		t.Fatal("expected coop shop to include at least one coop-only card")
	}
	if !foundCoopRelic {
		t.Fatal("expected coop shop to include at least one coop-only relic")
	}
	foundCoopService := false
	for _, offer := range coopShop.Offers {
		if offer.Kind == "service" {
			foundCoopService = true
			break
		}
	}
	if !foundCoopService {
		t.Fatal("expected coop shop to include at least one coop-only service")
	}

	node := Node{ID: "a1f1m1", Act: 1, Floor: 1, Index: 1, Kind: NodeMonster}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 3,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	soloReward := BuildReward(lib, soloRun, node, encounter)
	for _, card := range soloReward.CardChoices {
		if slices.Contains(card.Flags, "coop_only") {
			t.Fatalf("expected solo reward to exclude coop-only cards, got %q", card.ID)
		}
	}

	coopReward := BuildReward(lib, coopRun, node, encounter)
	foundCoopRewardCard := false
	for _, card := range coopReward.CardChoices {
		if slices.Contains(card.Flags, "coop_only") {
			foundCoopRewardCard = true
			break
		}
	}
	if !foundCoopRewardCard {
		t.Fatal("expected coop reward to include a coop-only card choice")
	}

	eventNode := Node{ID: "a1f2e1", Act: 1, Floor: 2, Index: 1, Kind: NodeEvent}
	sawCoopEventSolo := false
	for i := 0; i < 12; i++ {
		soloRun.Seed = int64(100 + i)
		event, err := StartEvent(lib, soloRun, eventNode)
		if err != nil {
			t.Fatalf("StartEvent(solo) error = %v", err)
		}
		if slices.Contains(event.Event.Flags, "coop_only") {
			sawCoopEventSolo = true
			break
		}
	}
	if sawCoopEventSolo {
		t.Fatal("expected solo events to exclude coop-only events")
	}

	sawCoopEvent := false
	for i := 0; i < 16; i++ {
		coopRun.Seed = int64(200 + i)
		event, err := StartEvent(lib, coopRun, eventNode)
		if err != nil {
			t.Fatalf("StartEvent(coop) error = %v", err)
		}
		if slices.Contains(event.Event.Flags, "coop_only") {
			sawCoopEvent = true
			break
		}
	}
	if !sawCoopEvent {
		t.Fatal("expected coop event pool to include coop-only events")
	}
}

func TestPlayCardWithTargetCanApplyCoopSupportStatusToAlly(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	players := []PlayerState{
		{
			ClassID:   "vanguard",
			Name:      "lead",
			MaxHP:     80,
			HP:        80,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "cover_formation"}},
		},
		{
			ClassID:   "arcanist",
			Name:      "ally",
			MaxHP:     70,
			HP:        70,
			MaxEnergy: 3,
		},
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 13)
	combat.Hand = []RuntimeCard{{ID: "cover_formation"}}
	combat.DrawPile = nil

	if err := PlayCardWithTarget(lib, players[0], combat, 0, CombatTarget{Kind: CombatTargetAlly, Index: 1}); err != nil {
		t.Fatalf("PlayCardWithTarget() error = %v", err)
	}

	if combat.Player.Block != 0 {
		t.Fatalf("expected leader block to stay 0, got %d", combat.Player.Block)
	}
	if combat.Allies[0].Block != 9 {
		t.Fatalf("expected ally block 9, got %d", combat.Allies[0].Block)
	}
	if got := statusStacks(combat.Allies[0].Statuses, "sheltered"); got != 1 {
		t.Fatalf("expected ally sheltered 1, got %d", got)
	}
}

func TestPlayCardWithTargetCanGrantRelayBulwarkToAlly(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	players := []PlayerState{
		{
			ClassID:   "vanguard",
			Name:      "lead",
			MaxHP:     80,
			HP:        80,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "relay_bulwark"}},
		},
		{
			ClassID:   "arcanist",
			Name:      "ally",
			MaxHP:     70,
			HP:        58,
			MaxEnergy: 3,
		},
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 17)
	combat.Hand = []RuntimeCard{{ID: "relay_bulwark"}}
	combat.DrawPile = nil

	if err := PlayCardWithTarget(lib, players[0], combat, 0, CombatTarget{Kind: CombatTargetAlly, Index: 1}); err != nil {
		t.Fatalf("PlayCardWithTarget() error = %v", err)
	}

	if combat.Allies[0].Block != 7 {
		t.Fatalf("expected ally block 7, got %d", combat.Allies[0].Block)
	}
	if got := statusStacks(combat.Allies[0].Statuses, "regen"); got != 2 {
		t.Fatalf("expected ally regen 2, got %d", got)
	}
}

func TestApplyShopPurchaseTagsCoopLogs(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run := &RunState{
		Mode:      ModeStory,
		Seed:      5,
		Act:       1,
		PartySize: 2,
		Player: PlayerState{
			ClassID:        "vanguard",
			HP:             60,
			MaxHP:          60,
			Gold:           200,
			Deck:           []DeckCard{{CardID: "slash"}},
			PotionCapacity: 2,
		},
	}
	shop := &ShopState{Offers: []ShopOffer{
		{ID: "card-pack", Kind: "card", Name: lib.Cards["pack_tactics"].Name, Price: 30, CardID: "pack_tactics"},
		{ID: "service-coop", Kind: "service", Name: "协同简报", Price: 20, ItemID: "service_coop_card"},
	}}

	if err := ApplyShopPurchase(lib, run, shop, "card-pack"); err != nil {
		t.Fatalf("ApplyShopPurchase(card) error = %v", err)
	}
	if err := ApplyShopPurchase(lib, run, shop, "service-coop"); err != nil {
		t.Fatalf("ApplyShopPurchase(service) error = %v", err)
	}

	joined := strings.Join(shop.Log, "\n")
	if !strings.Contains(joined, "[CO-OP] 购入卡牌 "+lib.Cards["pack_tactics"].Name) {
		t.Fatalf("expected coop card tag in shop log, got %q", joined)
	}
	if !strings.Contains(joined, "[CO-OP] 参加协同简报，获得协作牌") {
		t.Fatalf("expected coop service tag in shop log, got %q", joined)
	}
}

func TestResolveEventDecisionTagsCoopLogs(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	run := &RunState{
		Mode:      ModeStory,
		Seed:      9,
		Act:       1,
		PartySize: 2,
		Player: PlayerState{
			ClassID:   "vanguard",
			HP:        60,
			MaxHP:     60,
			MaxEnergy: 3,
		},
	}
	state := &EventState{Event: lib.Events["war_council"]}

	if err := ResolveEventDecision(lib, run, state, "share_plan", true); err != nil {
		t.Fatalf("ResolveEventDecision() error = %v", err)
	}

	joined := strings.Join(state.Log, "\n")
	if !strings.Contains(joined, "[CO-OP] 获得卡牌 "+lib.Cards["pack_tactics"].Name) {
		t.Fatalf("expected coop card tag in event log, got %q", joined)
	}
	if !state.Done {
		t.Fatal("expected event state to be marked done")
	}
}

func TestCoopRelicAppliesToWholePartyAtCombatStart(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	players := []PlayerState{
		{
			ClassID:   "vanguard",
			Name:      "lead",
			MaxHP:     80,
			HP:        80,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "slash"}},
			Relics:    []string{"formation_beacon"},
		},
		{
			ClassID:   "arcanist",
			Name:      "ally",
			MaxHP:     70,
			HP:        70,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "guard"}},
		},
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 21)
	if combat.Player.Block != 4 {
		t.Fatalf("expected leader block 4 from formation beacon, got %d", combat.Player.Block)
	}
	if combat.Allies[0].Block != 4 {
		t.Fatalf("expected ally block 4 from formation beacon, got %d", combat.Allies[0].Block)
	}
}

func TestTeamComboRelicsCanDrawAndHeal(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	players := []PlayerState{
		{
			ClassID:   "vanguard",
			Name:      "lead",
			MaxHP:     80,
			HP:        74,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "slash"}, {CardID: "guard"}},
			Relics:    []string{"battlefield_manual", "relay_rations"},
		},
		{
			ClassID:   "arcanist",
			Name:      "ally",
			MaxHP:     70,
			HP:        61,
			MaxEnergy: 3,
			Deck:      []DeckCard{{CardID: "spark"}},
		},
	}
	encounter := content.EncounterDef{
		ID:         "dummy",
		Name:       "dummy",
		Kind:       "monster",
		Act:        1,
		HP:         20,
		GoldReward: 0,
		CardReward: 0,
		IntentCycle: []content.EnemyIntentDef{
			{Name: "wait", Effects: []content.Effect{{Op: "damage", Value: 0}}},
		},
	}

	combat := NewCombatForParty(lib, players, encounter, 29)
	combat.Hand = nil
	combat.DrawPile = []RuntimeCard{{ID: "slash"}}

	if first, unique := RecordCoopAction(combat, 0); !first || unique != 1 {
		t.Fatalf("expected first actor tracking, got first=%v unique=%d", first, unique)
	}
	if first, unique := RecordCoopAction(combat, 1); !first || unique != 2 {
		t.Fatalf("expected second actor tracking, got first=%v unique=%d", first, unique)
	}
	for _, relicID := range players[0].Relics {
		relic := lib.Relics[relicID]
		for _, effect := range relic.Effects {
			if effect.Trigger != "team_combo" {
				continue
			}
			if err := ApplyExternalCombatEffect(lib, players[0], combat, effect, CombatTarget{}); err != nil {
				t.Fatalf("ApplyExternalCombatEffect(%s) error = %v", relicID, err)
			}
		}
	}

	if len(combat.Hand) != 1 {
		t.Fatalf("expected team combo relic to draw 1 card, got hand size %d", len(combat.Hand))
	}
	if combat.Player.HP != 76 {
		t.Fatalf("expected leader heal to 76, got %d", combat.Player.HP)
	}
	if combat.Allies[0].HP != 63 {
		t.Fatalf("expected ally heal to 63, got %d", combat.Allies[0].HP)
	}
}
