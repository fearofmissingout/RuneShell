package netplay

import (
	"fmt"
	"slices"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/i18n"
)

func (s *server) snapshotLocked(selfID string) *roomSnapshot {
	lang := s.playerLanguageLocked(selfID)
	players := make([]roomPlayer, 0, len(s.order))
	for seat, id := range s.order {
		if player := s.players[id]; player != nil {
			players = append(players, roomPlayer{
				ID:        player.ID,
				Seat:      seat + 1,
				Name:      player.Name,
				ClassID:   player.ClassID,
				Language:  normalizeLanguage(player.Language),
				Ready:     player.Ready,
				Connected: player.Connected,
			})
		}
	}
	waitingOn := s.waitingOnForLocked(selfID)
	presentation := s.phasePresentationLocked(selfID, waitingOn)
	snap := &roomSnapshot{
		SelfID:           selfID,
		Language:         lang,
		Seat:             s.playerSeatIndexLocked(selfID) + 1,
		HostID:           s.hostID,
		RoomAddr:         s.roomAddr,
		Phase:            s.phase,
		PhaseTitle:       phaseDisplayNameFor(lang, s.phase),
		PhaseHint:        presentation.PhaseHint,
		ControlLabel:     presentation.ControlLabel,
		RoleNote:         presentation.RoleNote,
		Banner:           s.consumeClientNoticeLocked(selfID),
		Players:          players,
		OfflineSeats:     s.offlineSeatSummariesLocked(),
		WaitingOn:        waitingOn,
		SeatStatus:       s.seatStatusLocked(selfID),
		Recovery:         s.recoveryActionsLocked(selfID),
		Reconnect:        s.reconnectCommandsLocked(selfID),
		Resume:           append(s.consumeClientResumeLocked(selfID), s.resumeSummaryLocked(selfID)...),
		Commands:         presentation.Commands,
		Examples:         presentation.Examples,
		CanStart:         selfID == s.hostID && s.phase == phaseLobby && s.canStartRunLocked(),
		ChatLog:          tailStrings(s.chatLog, 10),
		ChatUnread:       s.chatUnreadLocked(selfID),
		ChatUnreadInView: min(s.chatUnreadLocked(selfID), len(tailStrings(s.chatLog, 10))),
		TransferNote:     s.transferNoteLocked(selfID),
		RoomLog:          tailStrings(s.roomLog, 12),
	}
	if s.run != nil {
		snap.SharedMap = s.buildMapSnapshotLocked(selfID)
		snap.Stats = s.buildStatsSnapshotLocked(selfID)
	}
	s.buildPhaseSnapshotLocked(selfID, snap)
	return snap
}

func (s *server) buildPhaseSnapshotLocked(selfID string, snap *roomSnapshot) {
	if snap == nil {
		return
	}
	lib := s.localizedLibLocked(selfID)
	switch s.phase {
	case phaseLobby:
		snap.Lobby = s.buildLobbySnapshotLocked()
	case phaseMap:
		snap.Map = s.buildMapSnapshotLocked(selfID)
	case phaseCombat:
		snap.Combat = buildCombatSnapshot(lib, s.run, s.combat, selfID, s.order, s.players)
	case phaseReward:
		snap.Reward = s.buildRewardSnapshotLocked(selfID, lib)
	case phaseEvent:
		snap.Event = s.buildEventSnapshotLocked(selfID, lib)
	case phaseShop:
		snap.Shop = s.buildShopSnapshotLocked(selfID, lib)
	case phaseRest:
		snap.Rest = s.buildRestSnapshotLocked()
	case phaseEquipment:
		snap.Equipment = buildEquipmentSnapshot(lib, s.equipOffer)
	case phaseDeckAction:
		snap.Deck = s.buildDeckActionSnapshotLocked(selfID, lib)
	case phaseSummary:
		snap.Summary = s.buildSummarySnapshotLocked(selfID)
	}
}

func (s *server) buildLobbySnapshotLocked() *lobbySnapshot {
	return &lobbySnapshot{
		Mode:    string(s.mode),
		Seed:    s.seed,
		Classes: classIDs(s.lib),
	}
}

func (s *server) buildMapSnapshotLocked(selfID string) *mapSnapshot {
	if s.run == nil {
		return nil
	}
	lang := s.playerLanguageLocked(selfID)
	reachable := engine.ReachableNodes(s.run)
	nodes := make([]nodeSnapshot, 0, len(reachable))
	for i, node := range reachable {
		nodes = append(nodes, nodeSnapshot{
			ID:    node.ID,
			Index: i + 1,
			Floor: node.Floor,
			Kind:  string(node.Kind),
			Label: fmt.Sprintf("A%d F%d %s", node.Act, node.Floor, engine.NodeKindNameFor(lang, node.Kind)),
		})
	}
	return &mapSnapshot{
		Mode:          string(s.run.Mode),
		Act:           s.run.Act,
		NextFloor:     s.run.CurrentFloor + 1,
		CurrentFloor:  currentMapFloor(s.run, s.currentNode),
		CurrentNodeID: currentMapNodeID(s.currentNode),
		Gold:          s.seatGoldLocked(selfID),
		Party:         s.seatSnapshotsLocked(),
		Reachable:     nodes,
		VoteStatus:    s.mapVoteStatusLocked(selfID, reachable),
		VoteSummary:   s.mapVoteSummaryForLocked(selfID, reachable),
		Graph:         buildMapTreeSnapshot(s.run.Map),
		History:       s.seatHistoryLocked(selfID, 8),
	}
}

func currentMapFloor(run *engine.RunState, currentNode engine.Node) int {
	if currentNode.ID != "" {
		return currentNode.Floor
	}
	if run == nil {
		return 0
	}
	return run.CurrentFloor
}

func currentMapNodeID(currentNode engine.Node) string {
	return currentNode.ID
}

func buildMapTreeSnapshot(graph engine.MapGraph) [][]mapTreeNodeSnapshot {
	out := make([][]mapTreeNodeSnapshot, 0, len(graph.Floors))
	for _, layer := range graph.Floors {
		row := make([]mapTreeNodeSnapshot, 0, len(layer))
		for _, node := range layer {
			row = append(row, mapTreeNodeSnapshot{
				ID:    node.ID,
				Index: node.Index,
				Floor: node.Floor,
				Kind:  string(node.Kind),
				Edges: append([]string{}, node.Edges...),
			})
		}
		out = append(out, row)
	}
	return out
}

func (s *server) buildCombatSnapshotLocked(selfID string) *combatSnapshot {
	return buildCombatSnapshot(s.localizedLibLocked(selfID), s.run, s.combat, selfID, s.order, s.players)
}

func combatSnapshotSeatIndex(selfID string, order []string) int {
	for i, id := range order {
		if id == selfID {
			return i
		}
	}
	return -1
}

func buildCombatPartySnapshots(lib *content.Library, combat *engine.CombatState) []actorSnapshot {
	party := make([]actorSnapshot, 0, 1+len(combat.Allies))
	lang := content.LanguageOf(lib)
	for i, actor := range engine.PartyMembersView(combat) {
		classID := ""
		if i >= 0 && i < len(combat.SeatPlayers) {
			classID = combat.SeatPlayers[i].ClassID
		}
		party = append(party, actorSnapshot{
			Index:     i + 1,
			Name:      actor.Name,
			ClassID:   classID,
			HP:        actor.HP,
			MaxHP:     actor.MaxHP,
			Energy:    actor.Energy,
			MaxEnergy: actor.MaxEnergy,
			Block:     actor.Block,
			Status:    engine.DescribeStatusesFor(lang, actor.Statuses),
		})
	}
	return party
}

func buildCombatEnemySnapshots(lib *content.Library, combat *engine.CombatState) []enemySnapshot {
	enemies := make([]enemySnapshot, 0, len(combat.Enemies))
	lang := content.LanguageOf(lib)
	for i, enemy := range combat.Enemies {
		name := enemy.Name
		intent := enemy.CurrentIntent
		if def, ok := lib.Encounters[enemy.EncounterID]; ok {
			slotName := def.Name
			if enemy.Slot > 0 {
				slotName = fmt.Sprintf("%s #%d", slotName, enemy.Slot+1)
			}
			name = slotName
			if enemy.IntentIndex >= 0 && enemy.IntentIndex < len(def.IntentCycle) {
				intent = def.IntentCycle[enemy.IntentIndex]
			}
		}
		intentSummary := intent.Description
		if strings.TrimSpace(intentSummary) == "" {
			intentSummary = content.DescribeEffects(lib, intent.Effects)
		}
		enemies = append(enemies, enemySnapshot{
			Index:  i + 1,
			Name:   name,
			HP:     enemy.HP,
			MaxHP:  enemy.MaxHP,
			Block:  enemy.Block,
			Status: engine.DescribeStatusesFor(lang, enemy.Statuses),
			Intent: intentSummary,
		})
	}
	return enemies
}

func buildCombatHandSnapshots(lib *content.Library, handCards []engine.RuntimeCard) []cardSnapshot {
	hand := make([]cardSnapshot, 0, len(handCards))
	for i, card := range handCards {
		def := lib.Cards[card.ID]
		hand = append(hand, cardSnapshot{
			Index:      i + 1,
			Name:       engine.CardStateName(lib, card.ID, card.Upgraded),
			Kind:       combatSnapshotCardKind(def.Tags),
			Cost:       def.Cost,
			Summary:    engine.RuntimeCardStateSummary(lib, card),
			TargetHint: combatSnapshotTargetHint(lib, engine.CardTargetKindForCard(lib, card)),
			Badges:     flagBadges(def.Flags),
		})
	}
	return hand
}

func combatSnapshotTargetHint(lib *content.Library, kind engine.CombatTargetKind) string {
	if content.LanguageOf(lib) == i18n.LangEnUS {
		switch kind {
		case engine.CombatTargetEnemy:
			return "Single enemy"
		case engine.CombatTargetEnemies:
			return "All enemies"
		case engine.CombatTargetAlly:
			return "Single ally"
		case engine.CombatTargetAllies:
			return "All allies"
		default:
			return "No target"
		}
	}
	switch kind {
	case engine.CombatTargetEnemy:
		return "单体敌人"
	case engine.CombatTargetEnemies:
		return "全体敌人"
	case engine.CombatTargetAlly:
		return "单体友军"
	case engine.CombatTargetAllies:
		return "全体友军"
	default:
		return "无目标"
	}
}

func buildCombatPotionSnapshots(lib *content.Library, potions []string) []string {
	lines := make([]string, 0, len(potions))
	for i, potionID := range potions {
		if potion, ok := lib.Potions[potionID]; ok {
			lines = append(lines, fmt.Sprintf("%d. %s | %s", i+1, potion.Name, potion.Description))
		}
	}
	return lines
}

func buildCombatSeatDetailSnapshot(lib *content.Library, run *engine.RunState, combat *engine.CombatState, seatIndex int) (int, int, int, int, int, []string, []string, []string, []string) {
	energy := combat.Player.Energy
	maxEnergy := combat.Player.MaxEnergy
	if actor := engine.ActorForSeat(combat, seatIndex); actor != nil {
		energy = actor.Energy
		maxEnergy = actor.MaxEnergy
	}
	if seatIndex < 0 {
		return energy, maxEnergy, 0, 0, 0, nil, nil, nil, nil
	}
	seat := engine.CombatSeatView(combat, seatIndex)
	if seat == nil {
		return energy, maxEnergy, 0, 0, 0, nil, nil, nil, nil
	}
	drawLines := engine.CombatInspectLinesForSeat(lib, run, combat, seatIndex, 1)
	discardLines := engine.CombatInspectLinesForSeat(lib, run, combat, seatIndex, 2)
	exhaustLines := engine.CombatInspectLinesForSeat(lib, run, combat, seatIndex, 3)
	effectLines := engine.CombatInspectLinesForSeat(lib, run, combat, seatIndex, 5)
	return energy, maxEnergy, len(seat.DrawPile), len(seat.Discard), len(seat.Exhaust), drawLines, discardLines, exhaustLines, effectLines
}

func combatSeatDeckSize(combat *engine.CombatState, run *engine.RunState, seatIndex int) int {
	if seatIndex < 0 {
		return 0
	}
	player := engine.SeatPlayerForInspect(combat, run.Player, seatIndex)
	return len(player.Deck)
}

func buildCombatSnapshot(lib *content.Library, run *engine.RunState, combat *engine.CombatState, selfID string, order []string, players map[string]*roomPlayer) *combatSnapshot {
	if run == nil || combat == nil {
		return nil
	}
	lang := i18n.DefaultLanguage
	if player := players[selfID]; player != nil {
		lang = normalizeLanguage(player.Language)
	}
	seatIndex := combatSnapshotSeatIndex(selfID, order)
	party := buildCombatPartySnapshots(lib, combat)
	enemies := buildCombatEnemySnapshots(lib, combat)
	handCards := []engine.RuntimeCard{}
	if seatIndex >= 0 && seatIndex < len(combat.Seats) {
		handCards = combat.Seats[seatIndex].Hand
	}
	hand := buildCombatHandSnapshots(lib, handCards)
	potions := []string{}
	if seatIndex >= 0 && seatIndex < len(combat.Seats) {
		potions = buildCombatPotionSnapshots(lib, combat.Seats[seatIndex].Potions)
	}
	logs := []string{}
	for _, entry := range tailCombatLogs(combat.Log, 24) {
		logs = append(logs, fmt.Sprintf("T%d %s", entry.Turn, entry.Text))
	}
	voteStatus := buildVoteStatus(lang, order, players, combat.Coop.EndTurnVotes)
	highlights := []string{}
	if coopCards := countSnapshotsWithBadge(hand, "CO-OP"); coopCards > 0 {
		highlights = append(highlights, fmt.Sprintf("当前手牌中有 %d 张协作牌。", coopCards))
	}
	energy, maxEnergy, drawCount, discardCount, exhaustCount, drawLines, discardLines, exhaustLines, effectLines := buildCombatSeatDetailSnapshot(lib, run, combat, seatIndex)
	deckSize := combatSeatDeckSize(combat, run, seatIndex)
	pendingRepeats := engine.PendingNextCardRepeatDescriptions(combat, seatIndex)
	return &combatSnapshot{
		Turn:           combat.Turn,
		Energy:         energy,
		MaxEnergy:      maxEnergy,
		DeckSize:       deckSize,
		DrawCount:      drawCount,
		DiscardCount:   discardCount,
		ExhaustCount:   exhaustCount,
		Party:          party,
		Enemies:        enemies,
		Hand:           hand,
		Potions:        potions,
		DrawPile:       drawLines,
		DiscardPile:    discardLines,
		ExhaustPile:    exhaustLines,
		Effects:        effectLines,
		PendingRepeats: pendingRepeats,
		EndTurnVotes:   append([]bool{}, combat.Coop.EndTurnVotes...),
		VoteStatus:     voteStatus,
		Logs:           logs,
		Highlights:     highlights,
	}
}

func (s *server) buildStatsSnapshotLocked(selfID string) *statsSnapshot {
	_, run := s.seatContextLocked(selfID)
	if run == nil {
		return nil
	}
	seatIndex := s.playerSeatIndexLocked(selfID)
	combatMetrics := engine.CombatMetrics{}
	combatTurns := 0
	if s.combat != nil && seatIndex >= 0 {
		combatMetrics = engine.CombatMetricsForSeat(s.combat, seatIndex)
		combatTurns = engine.CombatTurns(s.combat)
	}
	seatName := run.Player.Name
	if player := s.players[selfID]; player != nil && strings.TrimSpace(player.Name) != "" {
		seatName = player.Name
	}
	return &statsSnapshot{
		SeatName:    seatName,
		CombatTurns: combatTurns,
		Combat:      combatMetrics,
		RunTurns:    run.Stats.CombatTurns,
		Run:         run.Stats.Metrics,
	}
}

func (s *server) buildRewardSnapshotLocked(selfID string, lib *content.Library) *rewardSnapshot {
	state, run := s.seatContextLocked(selfID)
	if state == nil || state.Reward == nil {
		return nil
	}
	reward := state.Reward
	cards := make([]cardSnapshot, 0, len(reward.CardChoices))
	for i, card := range reward.CardChoices {
		if localized, ok := lib.Cards[card.ID]; ok {
			card = localized
		}
		cards = append(cards, cardSnapshot{
			Index:   i + 1,
			Name:    card.Name,
			Kind:    combatSnapshotCardKind(card.Tags),
			Cost:    card.Cost,
			Summary: engine.DescribeEffects(lib, card.Effects),
			Badges:  flagBadges(card.Flags),
		})
	}
	snap := &rewardSnapshot{
		Gold:   reward.Gold,
		Source: string(reward.SourceNodeKind),
		Cards:  cards,
	}
	if reward.PotionID != "" {
		snap.Potion = lib.Potions[reward.PotionID].Name
	}
	if reward.RelicID != "" {
		relic := lib.Relics[reward.RelicID]
		snap.Relic = relic.Name
		snap.RelicBadges = flagBadges(relic.Flags)
	}
	if reward.EquipmentID != "" && run != nil {
		if offer, err := engine.BuildEquipmentOffer(s.lib, run.Player, reward.EquipmentID, "reward", 0); err == nil {
			snap.Equipment = buildEquipmentSnapshot(lib, &offer)
		}
	}
	if coopCards := countSnapshotsWithBadge(cards, "CO-OP"); coopCards > 0 {
		snap.Highlights = append(snap.Highlights, fmt.Sprintf("奖励池中有 %d 张协作牌可选。", coopCards))
	}
	if slices.Contains(snap.RelicBadges, "CO-OP") {
		snap.Highlights = append(snap.Highlights, "该奖励遗物仅在多人模式生效。")
	}
	return snap
}

func (s *server) buildEventSnapshotLocked(selfID string, lib *content.Library) *eventSnapshot {
	state, _ := s.seatContextLocked(selfID)
	if state == nil || state.Event == nil {
		return nil
	}
	eventDef := state.Event.Event
	if localized, ok := lib.Events[eventDef.ID]; ok {
		eventDef = localized
		if len(state.Event.Event.Choices) > 0 && len(localized.Choices) > 0 {
			filtered := make([]content.EventChoiceDef, 0, len(state.Event.Event.Choices))
			for _, current := range state.Event.Event.Choices {
				selected := current
				for _, candidate := range localized.Choices {
					if candidate.ID == current.ID {
						selected = candidate
						break
					}
				}
				filtered = append(filtered, selected)
			}
			eventDef.Choices = filtered
		}
	}
	choices := make([]choiceSnapshot, 0, len(eventDef.Choices))
	for i, choice := range eventDef.Choices {
		choices = append(choices, choiceSnapshot{
			Index:       i + 1,
			Label:       choice.Label,
			Description: choice.Description,
			Badges:      eventChoiceBadges(lib, choice),
		})
	}
	return &eventSnapshot{
		Name:        eventDef.Name,
		Description: eventDef.Description,
		Badges:      flagBadges(eventDef.Flags),
		Choices:     choices,
		Log:         append([]string{}, state.Event.Log...),
		Highlights:  eventHighlights(eventDef.Flags, choices),
	}
}

func (s *server) buildShopSnapshotLocked(selfID string, lib *content.Library) *shopSnapshot {
	state, _ := s.seatContextLocked(selfID)
	if state == nil || state.Shop == nil {
		return nil
	}
	offers := make([]shopOfferSnapshot, 0, len(state.Shop.Offers))
	for i, offer := range state.Shop.Offers {
		name, description := localizedShopOfferDisplay(lib, offer)
		offers = append(offers, shopOfferSnapshot{
			Index:       i + 1,
			Kind:        offer.Kind,
			Category:    shopOfferCategory(offer.Kind),
			Name:        name,
			Description: description,
			Price:       offer.Price,
			Badges:      shopOfferBadges(lib, offer),
		})
	}
	return &shopSnapshot{
		Gold:       s.seatGoldLocked(selfID),
		Offers:     offers,
		Log:        append([]string{}, state.Shop.Log...),
		Highlights: shopHighlights(offers),
	}
}

func localizedShopOfferDisplay(lib *content.Library, offer engine.ShopOffer) (string, string) {
	name := offer.Name
	description := offer.Description
	switch strings.ToLower(strings.TrimSpace(offer.Kind)) {
	case "heal":
		if content.LanguageOf(lib) == i18n.LangEnUS {
			return "Supplies", "Heal 18 HP."
		}
	case "remove":
		if content.LanguageOf(lib) == i18n.LangEnUS {
			return "Trim the Deck", "Remove a card from the deck."
		}
	case "service":
		if content.LanguageOf(lib) == i18n.LangEnUS {
			switch offer.ItemID {
			case "service_echo_workshop":
				return "Echo Workshop", "Choose an Attack card. It draws 1 extra card when played this run."
			case "service_flash_workshop":
				return "Flash Workshop", "Choose an upgradable card. It gains 1 extra Energy when played in the next combat."
			case "service_ember_workshop":
				return "Ember Workshop", "Choose an Attack card. It applies 2 extra Burn when it hits this run."
			case "service_bastion_workshop":
				return "Bastion Workshop", "Choose a Skill card. It gains 4 extra Block when played this run."
			case "service_opening_workshop":
				return "Opening Workshop", "Choose an upgradable card. It draws 1 extra card when played this turn of the next combat."
			case "service_coop_card":
				return "Co-op Briefing", "Gain a random Co-op card."
			case "service_combo_drill":
				return "Combo Drill", "Upgrade a random upgradable card and heal 10 HP."
			}
		}
	case "card":
		if card, ok := lib.Cards[offer.CardID]; ok {
			name = card.Name
			description = card.Description
		}
	case "relic":
		if relic, ok := lib.Relics[offer.ItemID]; ok {
			name = relic.Name
			description = relic.Description
		}
	case "equipment":
		if equipment, ok := lib.Equipments[offer.ItemID]; ok {
			name = equipment.Name
			description = equipment.Description
		}
	case "potion":
		if potion, ok := lib.Potions[offer.ItemID]; ok {
			name = potion.Name
			description = potion.Description
		}
	}
	return name, description
}

func (s *server) buildRestSnapshotLocked() *restSnapshot {
	return &restSnapshot{
		Party: s.seatSnapshotsLocked(),
		Log:   append([]string{}, s.restLog...),
	}
}

func buildEquipmentSnapshot(lib *content.Library, offer *engine.EquipmentOfferState) *equipmentSnapshot {
	if offer == nil {
		return nil
	}
	candidate := lib.Equipments[offer.EquipmentID]
	snap := &equipmentSnapshot{
		Source:               offer.Source,
		Slot:                 engine.EquipmentSlotNameFor(content.LanguageOf(lib), offer.Slot),
		CandidateName:        candidate.Name,
		CandidateDescription: candidate.Description,
		Price:                offer.Price,
		CandidateScore:       offer.CandidateScore,
		CurrentScore:         offer.CurrentScore,
	}
	if offer.CurrentEquipmentID != "" {
		current := lib.Equipments[offer.CurrentEquipmentID]
		snap.CurrentName = current.Name
		snap.CurrentDescription = current.Description
	}
	return snap
}

func (s *server) buildDeckActionSnapshotLocked(selfID string, lib *content.Library) *deckActionSnapshot {
	if selfID != s.flowOwner {
		return nil
	}
	_, run := s.seatContextLocked(selfID)
	if run == nil {
		return nil
	}
	cards := make([]cardSnapshot, 0, len(s.deckActionIndexes))
	for _, deckIndex := range s.deckActionIndexes {
		if deckIndex < 0 || deckIndex >= len(run.Player.Deck) {
			continue
		}
		card := run.Player.Deck[deckIndex]
		cards = append(cards, cardSnapshot{
			Index:   len(cards) + 1,
			Name:    engine.CardStateName(lib, card.CardID, card.Upgraded),
			Kind:    combatSnapshotCardKind(lib.Cards[card.CardID].Tags),
			Summary: engine.DeckCardStateSummary(lib, card),
		})
	}
	return &deckActionSnapshot{
		Mode:     s.deckActionMode,
		Title:    s.deckActionTitle,
		Subtitle: s.deckActionSubtitle,
		Cards:    cards,
	}
}

func combatSnapshotCardKind(tags []string) string {
	for _, tag := range tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "attack":
			return "attack"
		case "skill":
			return "skill"
		case "power", "ability", "spell":
			return "ability"
		}
	}
	return "ability"
}

func (s *server) buildSummarySnapshotLocked(selfID string) *summarySnapshot {
	_, run := s.seatContextLocked(selfID)
	if run == nil {
		return nil
	}
	return &summarySnapshot{
		Result:   string(run.Status),
		Mode:     string(run.Mode),
		Act:      run.Act,
		Floors:   run.Stats.ClearedFloors,
		Gold:     run.Player.Gold,
		DeckSize: len(run.Player.Deck),
		Party:    s.seatSnapshotsLocked(),
		History:  tailStrings(run.History, 12),
	}
}
