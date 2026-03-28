package engine

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strings"

	"cmdcards/internal/content"
)

type combatSide string

const (
	sidePlayer      combatSide = "player"
	sideEnemy       combatSide = "enemy"
	combatHandLimit            = 10
)

type combatSourceRef struct {
	side  combatSide
	index int
}

func buildCombatSeat(player PlayerState, seed int64) CombatSeatState {
	draw := make([]RuntimeCard, 0, len(player.Deck))
	for _, card := range player.Deck {
		draw = append(draw, RuntimeCard{ID: card.CardID, Upgraded: card.Upgraded, Augments: cloneCardAugments(card.Augments)})
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(draw), func(i, j int) {
		draw[i], draw[j] = draw[j], draw[i]
	})
	return CombatSeatState{
		DrawPile: draw,
		Potions:  append([]string{}, player.Potions...),
	}
}

func ensureCombatSeats(combat *CombatState) {
	if combat == nil || len(combat.Seats) > 0 {
		return
	}
	combat.Seats = []CombatSeatState{{
		DrawPile:    append([]RuntimeCard{}, combat.DrawPile...),
		Discard:     append([]RuntimeCard{}, combat.Discard...),
		Exhaust:     append([]RuntimeCard{}, combat.Exhaust...),
		Hand:        append([]RuntimeCard{}, combat.Hand...),
		PotionsUsed: append([]string{}, combat.PotionsUsed...),
	}}
}

func combatSeat(combat *CombatState, seatIndex int) *CombatSeatState {
	ensureCombatSeats(combat)
	if combat == nil || seatIndex < 0 || seatIndex >= len(combat.Seats) {
		return nil
	}
	return &combat.Seats[seatIndex]
}

func syncLegacySeat0(combat *CombatState) {
	ensureCombatSeats(combat)
	if combat == nil || len(combat.Seats) == 0 {
		return
	}
	seat := combat.Seats[0]
	combat.DrawPile = append([]RuntimeCard{}, seat.DrawPile...)
	combat.Discard = append([]RuntimeCard{}, seat.Discard...)
	combat.Exhaust = append([]RuntimeCard{}, seat.Exhaust...)
	combat.Hand = append([]RuntimeCard{}, seat.Hand...)
	combat.PotionsUsed = append([]string{}, seat.PotionsUsed...)
}

func refreshSeat0FromLegacy(combat *CombatState) {
	ensureCombatSeats(combat)
	if combat == nil || len(combat.Seats) == 0 {
		return
	}
	combat.Seats[0].DrawPile = append([]RuntimeCard{}, combat.DrawPile...)
	combat.Seats[0].Discard = append([]RuntimeCard{}, combat.Discard...)
	combat.Seats[0].Exhaust = append([]RuntimeCard{}, combat.Exhaust...)
	combat.Seats[0].Hand = append([]RuntimeCard{}, combat.Hand...)
	combat.Seats[0].PotionsUsed = append([]string{}, combat.PotionsUsed...)
}

func partyActorBySeat(combat *CombatState, seatIndex int) *CombatActor {
	if combat == nil {
		return nil
	}
	if seatIndex <= 0 {
		return &combat.Player
	}
	if seatIndex-1 >= 0 && seatIndex-1 < len(combat.Allies) {
		return &combat.Allies[seatIndex-1]
	}
	return nil
}

func combatPlayerStateForSeat(combat *CombatState, fallback PlayerState, seatIndex int) PlayerState {
	if combat != nil && seatIndex >= 0 && seatIndex < len(combat.SeatPlayers) {
		return combat.SeatPlayers[seatIndex]
	}
	return fallback
}

func NewCombat(lib *content.Library, player PlayerState, encounter content.EncounterDef, seed int64) *CombatState {
	return NewCombatWithEnemies(lib, player, []content.EncounterDef{encounter}, encounter, seed)
}

func NewCombatWithEnemies(lib *content.Library, player PlayerState, encounters []content.EncounterDef, rewardBasis content.EncounterDef, seed int64) *CombatState {
	if len(encounters) == 0 {
		encounters = []content.EncounterDef{rewardBasis}
	}
	seat := buildCombatSeat(player, seed)

	state := &CombatState{
		Player: CombatActor{
			Name:      player.Name,
			HP:        player.HP,
			MaxHP:     player.MaxHP,
			Energy:    player.MaxEnergy,
			MaxEnergy: player.MaxEnergy,
			Statuses:  map[string]Status{},
		},
		Encounter:   rewardBasis,
		RewardBasis: rewardBasis,
		SeatPlayers: []PlayerState{player},
		DrawPile:    append([]RuntimeCard{}, seat.DrawPile...),
		Seats:       []CombatSeatState{seat},
		Turn:        1,
	}

	state.Enemies = make([]CombatEnemy, 0, len(encounters))
	for i, encounter := range encounters {
		enemy := CombatEnemy{
			CombatActor: CombatActor{
				Name:     encounter.Name,
				HP:       encounter.HP,
				MaxHP:    encounter.HP,
				Statuses: map[string]Status{},
			},
			ID:            fmt.Sprintf("enemy-%d", i),
			EncounterID:   encounter.ID,
			Slot:          i,
			Passives:      slices.Clone(encounter.Passives),
			IntentCycle:   slices.Clone(encounter.IntentCycle),
			CurrentIntent: encounter.IntentCycle[0],
		}
		state.Enemies = append(state.Enemies, enemy)
	}
	syncPrimaryEnemy(state)

	triggerPassiveWindow(lib, player, state, sideEnemy, "combat_start")
	triggerPassiveWindow(lib, player, state, sidePlayer, "combat_start")
	if bonusBlock := player.PermanentStats["combat_start_block"]; bonusBlock > 0 {
		state.Player.Block += bonusBlock
		state.log(fmt.Sprintf("%s战斗开始时获得 %d 格挡", state.Player.Name, bonusBlock))
	}
	state.log("遭遇 " + EncounterGroupName(state))
	syncLegacySeat0(state)
	return state
}

func EncounterGroupName(combat *CombatState) string {
	names := make([]string, 0, len(combat.Enemies))
	for _, enemy := range combat.Enemies {
		if enemy.HP > 0 {
			names = append(names, enemy.Name)
		}
	}
	if len(names) == 0 {
		return "敌群"
	}
	return strings.Join(names, " / ")
}

func passiveEffectsForSide(lib *content.Library, player PlayerState, combat *CombatState, side combatSide, index int) []content.Effect {
	if side == sideEnemy {
		if index >= 0 && index < len(combat.Enemies) {
			return slices.Clone(combat.Enemies[index].Passives)
		}
		return nil
	}
	player = combatPlayerStateForSeat(combat, player, index)

	out := []content.Effect{}
	for _, id := range player.Relics {
		if relic, ok := lib.Relics[id]; ok {
			out = append(out, relic.Effects...)
		}
	}
	for _, id := range []string{player.Equipment.Weapon, player.Equipment.Armor, player.Equipment.Accessory} {
		if id == "" {
			continue
		}
		if equipment, ok := lib.Equipments[id]; ok {
			out = append(out, equipment.Effects...)
		}
	}
	return out
}

func drawCards(lib *content.Library, combat *CombatState, count int) {
	refreshSeat0FromLegacy(combat)
	drawCardsForSeat(lib, combat, 0, count)
}

func drawCardsForSeat(lib *content.Library, combat *CombatState, seatIndex int, count int) {
	if count <= 0 {
		return
	}
	seat := combatSeat(combat, seatIndex)
	if seat == nil {
		return
	}
	for i := 0; i < count; i++ {
		if len(seat.DrawPile) == 0 {
			if len(seat.Discard) == 0 {
				return
			}
			rng := rand.New(rand.NewSource(int64(combat.Turn*1000 + seatIndex*211 + len(seat.Discard)*37 + len(seat.Exhaust)*17 + len(seat.Hand))))
			rng.Shuffle(len(seat.Discard), func(i, j int) {
				seat.Discard[i], seat.Discard[j] = seat.Discard[j], seat.Discard[i]
			})
			seat.DrawPile = append(seat.DrawPile, seat.Discard...)
			seat.Discard = nil
		}
		card := seat.DrawPile[0]
		seat.DrawPile = seat.DrawPile[1:]
		if len(seat.Hand) >= combatHandLimit {
			seat.Discard = append(seat.Discard, card)
			combat.log("手牌已满，额外抽到的牌进入弃牌堆")
			continue
		}
		seat.Hand = append(seat.Hand, card)
	}
	syncLegacySeat0(combat)
}

func StartPlayerTurn(lib *content.Library, playerState PlayerState, combat *CombatState) {
	StartPartyTurn(lib, []PlayerState{playerState}, combat)
}

func StartPartyTurn(lib *content.Library, players []PlayerState, combat *CombatState) {
	if combat.Won || combat.Lost {
		return
	}
	normalizeLegacyEnemyState(combat)
	ensureCombatSeats(combat)
	ResetEndTurnVotes(combat)
	ResetCoopTurnTracking(combat)

	for _, actor := range PartyActors(combat) {
		actor.Block = 0
		readyStatuses(actor)
		processStatusStart(combat, actor, sidePlayer, -1)
		if combat.Won || combat.Lost {
			return
		}
	}

	for seatIndex, playerState := range players {
		actor := partyActorBySeat(combat, seatIndex)
		if actor == nil {
			continue
		}
		actor.Energy = actor.MaxEnergy
		drawCardsForSeat(lib, combat, seatIndex, 5)
		triggerPlayerPassiveWindow(lib, playerState, combat, seatIndex, turnStartWindow(sidePlayer))
	}
	checkCombatOutcome(combat)
	if combat.Won || combat.Lost {
		return
	}
	combat.log(fmt.Sprintf("第 %d 回合开始", combat.Turn))
}

func AvailableCards(lib *content.Library, combat *CombatState) []RuntimeCard {
	refreshSeat0FromLegacy(combat)
	out := []RuntimeCard{}
	seat := combatSeat(combat, 0)
	if seat == nil {
		return out
	}
	for _, card := range seat.Hand {
		def := lib.Cards[card.ID]
		if slices.Contains(def.Flags, "unplayable") {
			continue
		}
		if def.Cost <= combat.Player.Energy {
			out = append(out, card)
		}
	}
	return out
}

func PlayCard(lib *content.Library, playerState PlayerState, combat *CombatState, handIndex int) error {
	return PlayCardWithTarget(lib, playerState, combat, handIndex, CombatTarget{Kind: CombatTargetNone})
}

func PlayCardWithTarget(lib *content.Library, playerState PlayerState, combat *CombatState, handIndex int, target CombatTarget) error {
	refreshSeat0FromLegacy(combat)
	return PlaySeatCardWithTarget(lib, playerState, combat, 0, handIndex, target)
}

func PlaySeatCardWithTarget(lib *content.Library, playerState PlayerState, combat *CombatState, seatIndex, handIndex int, target CombatTarget) error {
	normalizeLegacyEnemyState(combat)
	seat := combatSeat(combat, seatIndex)
	actor := partyActorBySeat(combat, seatIndex)
	if seat == nil || actor == nil {
		return fmt.Errorf("invalid seat %d", seatIndex)
	}
	if handIndex < 0 || handIndex >= len(seat.Hand) {
		return fmt.Errorf("invalid hand index %d", handIndex)
	}

	card := seat.Hand[handIndex]
	def := lib.Cards[card.ID]
	if slices.Contains(def.Flags, "unplayable") {
		return fmt.Errorf("card %q cannot be played", def.Name)
	}
	if def.Cost > actor.Energy {
		return fmt.Errorf("not enough energy")
	}

	actor.Energy -= def.Cost
	seat.Hand = append(seat.Hand[:handIndex], seat.Hand[handIndex+1:]...)

	effects := activeEffectsForCard(lib, card)
	playEffects, selfReplies := splitCardPlayEffects(effects)
	repeatCount := consumeNextCardRepeats(combat, seatIndex, def.Tags)
	repeatCount += selfReplies
	for repeatIndex := 0; repeatIndex <= repeatCount; repeatIndex++ {
		if repeatIndex > 0 {
			combat.log(fmt.Sprintf("%s再次结算 %s (%d/%d)", actor.Name, def.Name, repeatIndex, repeatCount))
		}
		for _, effect := range playEffects {
			if err := resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sidePlayer, index: seatIndex}, effect, card, target); err != nil {
				return err
			}
			if combat.Won || combat.Lost {
				break
			}
		}
		if combat.Won || combat.Lost {
			break
		}
	}

	if def.Exhaust {
		seat.Exhaust = append(seat.Exhaust, card)
	} else {
		seat.Discard = append(seat.Discard, card)
	}
	combat.log("打出 " + def.Name)
	syncLegacySeat0(combat)
	return nil
}

func EndPlayerTurn(lib *content.Library, playerState PlayerState, combat *CombatState) {
	if combat.Won || combat.Lost {
		return
	}
	refreshSeat0FromLegacy(combat)
	normalizeLegacyEnemyState(combat)
	if !RequestEndTurnVote(combat, 0) {
		combat.log("已提交结束回合，等待其他队友。")
		return
	}
	endPartyTurn(lib, []PlayerState{playerState}, combat)
}

func EndPartyTurn(lib *content.Library, players []PlayerState, combat *CombatState) {
	endPartyTurn(lib, players, combat)
}

func endPartyTurn(lib *content.Library, players []PlayerState, combat *CombatState) {
	if combat.Won || combat.Lost {
		return
	}
	normalizeLegacyEnemyState(combat)

	for seatIndex, playerState := range players {
		triggerPlayerPassiveWindow(lib, playerState, combat, seatIndex, turnEndWindow(sidePlayer))
	}
	checkCombatOutcome(combat)
	if combat.Won || combat.Lost {
		return
	}
	for _, actor := range PartyActors(combat) {
		processStatusEnd(combat, actor, sidePlayer, -1)
	}
	checkCombatOutcome(combat)
	if combat.Won || combat.Lost {
		return
	}
	triggerHandEndCards(lib, players, combat)
	checkCombatOutcome(combat)
	if combat.Won || combat.Lost {
		return
	}
	endTurnDiscardHands(lib, combat)
	pruneTurnCardAugments(combat)
	for _, actor := range PartyActors(combat) {
		decrementTimedStatuses(actor)
	}

	leader := PlayerState{}
	if len(players) > 0 {
		leader = players[0]
	}
	for i := range combat.Enemies {
		if combat.Enemies[i].HP <= 0 {
			continue
		}
		runEnemyTurn(lib, leader, combat, i)
		if combat.Won || combat.Lost {
			return
		}
	}
	combat.Turn++
	syncPrimaryEnemy(combat)
	syncLegacySeat0(combat)
}

func runEnemyTurn(lib *content.Library, playerState PlayerState, combat *CombatState, enemyIndex int) {
	enemy := &combat.Enemies[enemyIndex]
	enemy.Block = 0
	readyStatuses(&enemy.CombatActor)
	processStatusStart(combat, &enemy.CombatActor, sideEnemy, enemyIndex)
	checkCombatOutcome(combat)
	if combat.Won || combat.Lost {
		return
	}
	triggerEnemyPassiveWindow(lib, playerState, combat, enemyIndex, turnStartWindow(sideEnemy))
	checkCombatOutcome(combat)
	if combat.Won || combat.Lost {
		return
	}

	intent := combat.Enemies[enemyIndex].CurrentIntent
	for _, effect := range intent.Effects {
		if err := resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sideEnemy, index: enemyIndex}, effect, RuntimeCard{}, CombatTarget{Kind: CombatTargetAllies}); err != nil {
			combat.log("敌人行动异常: " + err.Error())
			combat.Lost = true
			return
		}
		if combat.Won || combat.Lost {
			return
		}
	}

	triggerEnemyPassiveWindow(lib, playerState, combat, enemyIndex, turnEndWindow(sideEnemy))
	checkCombatOutcome(combat)
	if combat.Won || combat.Lost {
		return
	}
	processStatusEnd(combat, &enemy.CombatActor, sideEnemy, enemyIndex)
	decrementTimedStatuses(&enemy.CombatActor)
	combat.Enemies[enemyIndex].IntentIndex = (combat.Enemies[enemyIndex].IntentIndex + 1) % len(combat.Enemies[enemyIndex].IntentCycle)
	combat.Enemies[enemyIndex].CurrentIntent = combat.Enemies[enemyIndex].IntentCycle[combat.Enemies[enemyIndex].IntentIndex]
	syncPrimaryEnemy(combat)
}

func UsePotion(lib *content.Library, playerState PlayerState, combat *CombatState, potionID string) error {
	return UsePotionWithTarget(lib, playerState, combat, potionID, CombatTarget{Kind: CombatTargetNone})
}

func UsePotionWithTarget(lib *content.Library, playerState PlayerState, combat *CombatState, potionID string, target CombatTarget) error {
	refreshSeat0FromLegacy(combat)
	if seat := combatSeat(combat, 0); seat != nil {
		if len(seat.Potions) == 0 {
			seat.Potions = append([]string{}, playerState.Potions...)
		}
		found := false
		for _, id := range seat.Potions {
			if id == potionID {
				found = true
				break
			}
		}
		if !found {
			def, ok := lib.Potions[potionID]
			if !ok {
				return fmt.Errorf("unknown potion %q", potionID)
			}
			for _, effect := range def.Effects {
				if err := resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sidePlayer, index: 0}, effect, RuntimeCard{}, target); err != nil {
					return err
				}
			}
			seat.PotionsUsed = append(seat.PotionsUsed, potionID)
			combat.log("使用药水 " + def.Name)
			syncLegacySeat0(combat)
			return nil
		}
	}
	return UseSeatPotionWithTarget(lib, playerState, combat, 0, potionID, target)
}

func UseSeatPotionWithTarget(lib *content.Library, playerState PlayerState, combat *CombatState, seatIndex int, potionID string, target CombatTarget) error {
	normalizeLegacyEnemyState(combat)
	seat := combatSeat(combat, seatIndex)
	if seat == nil {
		return fmt.Errorf("invalid seat %d", seatIndex)
	}
	found := -1
	for i, id := range seat.Potions {
		if id == potionID {
			found = i
			break
		}
	}
	if found < 0 {
		return fmt.Errorf("potion %q not available", potionID)
	}
	def, ok := lib.Potions[potionID]
	if !ok {
		return fmt.Errorf("unknown potion %q", potionID)
	}
	for _, effect := range def.Effects {
		if err := resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sidePlayer, index: seatIndex}, effect, RuntimeCard{}, target); err != nil {
			return err
		}
	}
	seat.Potions = append(seat.Potions[:found], seat.Potions[found+1:]...)
	seat.PotionsUsed = append(seat.PotionsUsed, potionID)
	combat.log("使用药水 " + def.Name)
	syncLegacySeat0(combat)
	return nil
}

func ApplyExternalCombatEffect(lib *content.Library, playerState PlayerState, combat *CombatState, effect content.Effect, target CombatTarget) error {
	if combat == nil {
		return fmt.Errorf("combat state is nil")
	}
	normalizeLegacyEnemyState(combat)
	return resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sidePlayer, index: 0}, effect, RuntimeCard{}, target)
}

func ApplySeatExternalCombatEffect(lib *content.Library, playerState PlayerState, combat *CombatState, seatIndex int, effect content.Effect, target CombatTarget) error {
	if combat == nil {
		return fmt.Errorf("combat state is nil")
	}
	normalizeLegacyEnemyState(combat)
	return resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sidePlayer, index: seatIndex}, effect, RuntimeCard{}, target)
}

func PendingNextCardRepeats(combat *CombatState, seatIndex int) int {
	total := 0
	for _, repeat := range pendingCardRepeats(combat, seatIndex) {
		total += repeat.Count
	}
	return total
}

func PendingNextCardRepeatDescriptions(combat *CombatState, seatIndex int) []string {
	repeats := pendingCardRepeats(combat, seatIndex)
	if len(repeats) == 0 {
		return nil
	}
	counts := map[string]int{}
	extraTags := []string{}
	for _, repeat := range repeats {
		if repeat.Count <= 0 {
			continue
		}
		counts[repeat.Tag] += repeat.Count
		switch repeat.Tag {
		case "", "attack", "spell":
		default:
			if !slices.Contains(extraTags, repeat.Tag) {
				extraTags = append(extraTags, repeat.Tag)
			}
		}
	}
	out := []string{}
	for _, tag := range []string{"", "attack", "spell"} {
		if counts[tag] > 0 {
			out = append(out, pendingRepeatDescription(counts[tag], tag))
		}
	}
	for _, tag := range extraTags {
		if counts[tag] > 0 {
			out = append(out, pendingRepeatDescription(counts[tag], tag))
		}
	}
	return out
}

func FinishCombat(lib *content.Library, run *RunState, node Node, combat *CombatState) {
	if combat.Won {
		basis := combat.RewardBasis
		if basis.ID == "" {
			basis = combat.Encounter
		}
		combat.Reward = BuildReward(lib, run, node, basis)
	}
}

func DescribeIntent(intent content.EnemyIntentDef) string {
	lines := []string{}
	for _, effect := range intent.Effects {
		switch effect.Op {
		case "damage":
			lines = append(lines, fmt.Sprintf("攻击 %d", effect.Value))
		case "block":
			lines = append(lines, fmt.Sprintf("格挡 %d", effect.Value))
		case "apply_status":
			target := effect.Target
			if target == "opponent" {
				target = "对手"
			}
			lines = append(lines, fmt.Sprintf("%s %s %d", target, statusLabel(effect.Status), effect.Value))
		case "cleanse_status":
			lines = append(lines, fmt.Sprintf("净化 %s", statusLabel(effect.Status)))
		case "heal":
			lines = append(lines, fmt.Sprintf("治疗 %d", effect.Value))
		case "lose_hp":
			lines = append(lines, fmt.Sprintf("失去 %d 生命", effect.Value))
		case "add_combat_card":
			lines = append(lines, fmt.Sprintf("加入%s", effect.CardID))
		}
	}
	return strings.Join(lines, " / ")
}

func DescribeStatuses(statuses map[string]Status) string {
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
		part := fmt.Sprintf("%s:%d", statusLabel(name), status.Stacks)
		if status.Duration > 0 {
			part = fmt.Sprintf("%s(%dT)", part, status.Duration)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "  ")
}

func resolveCombatEffect(lib *content.Library, playerState PlayerState, combat *CombatState, source combatSourceRef, effect content.Effect, card RuntimeCard, chosenTarget CombatTarget) error {
	switch effect.Op {
	case "damage":
		targets := resolveDamageTargets(combat, source, effect, chosenTarget)
		tags := effectTags(lib, card, effect)
		base := effect.Value
		totalDealt := 0
		for _, target := range targets {
			final := applyOutgoingDamageModifiers(lib, playerState, combat, source, base, tags)
			final = applyIncomingDamageModifiers(lib, playerState, combat, target, final)
			dealt := dealDamageToTarget(combat, source, target, final)
			totalDealt += dealt
			combat.log(fmt.Sprintf("%s对%s造成 %d 伤害", sourceLabel(combat, source), targetLabel(combat, target), dealt))
			checkCombatOutcome(combat)
			if combat.Won || combat.Lost {
				break
			}
		}
		if len(targets) > 1 && totalDealt > 0 {
			combat.log(fmt.Sprintf("%s的范围伤害总计 %d", sourceLabel(combat, source), totalDealt))
		}
	case "block":
		targets := resolveSupportTargets(combat, source, effect, chosenTarget)
		for _, target := range targets {
			amount := applyBlockModifiers(combat, target, effect.Value)
			targetActor(combat, target).Block += amount
			combat.log(fmt.Sprintf("%s获得 %d 格挡", targetLabel(combat, target), amount))
		}
	case "draw":
		drawCardsForSeat(lib, combat, source.index, effect.Value)
		combat.log(fmt.Sprintf("抽 %d 张牌", effect.Value))
	case "apply_status":
		targets := resolveSupportTargets(combat, source, effect, chosenTarget)
		if effectTargetsEnemies(effect, source) {
			targets = resolveDamageTargets(combat, source, effect, chosenTarget)
		}
		for _, target := range targets {
			applyStatus(targetActor(combat, target), effect.Status, effect.Value, effect.Duration)
			if source.side == sidePlayer {
				recordSeatStatusApplied(combat, source.index, effect.Value)
			}
			switch target.Kind {
			case CombatTargetAlly:
				recordSeatStatusReceived(combat, target.Index, effect.Value)
			case CombatTargetAllies:
				for _, allyTarget := range livingAllyTargets(combat) {
					recordSeatStatusReceived(combat, allyTarget.Index, effect.Value)
				}
			}
			combat.log(fmt.Sprintf("%s获得状态 %s %d", targetLabel(combat, target), statusLabel(effect.Status), effect.Value))
		}
	case "lose_hp":
		targets := resolveSupportTargets(combat, source, effect, chosenTarget)
		if effectTargetsEnemies(effect, source) {
			targets = resolveDamageTargets(combat, source, effect, chosenTarget)
		}
		for _, target := range targets {
			lost := loseHP(targetActor(combat, target), effect.Value)
			if target.Kind == CombatTargetAlly {
				recordSeatDamageReceived(combat, target.Index, lost, 0)
			}
			combat.log(fmt.Sprintf("%s失去 %d 生命", targetLabel(combat, target), lost))
		}
	case "cleanse_status":
		targets := resolveSupportTargets(combat, source, effect, chosenTarget)
		for _, target := range targets {
			removed := cleanseStatus(targetActor(combat, target), effect.Status, effect.Value)
			if removed > 0 {
				combat.log(fmt.Sprintf("%s净化了 %s", targetLabel(combat, target), statusLabel(effect.Status)))
			}
		}
	case "heal":
		targets := resolveSupportTargets(combat, source, effect, chosenTarget)
		for _, target := range targets {
			actor := targetActor(combat, target)
			before := actor.HP
			actor.HP = min(actor.MaxHP, actor.HP+effect.Value)
			combat.log(fmt.Sprintf("%s恢复 %d 生命", targetLabel(combat, target), actor.HP-before))
		}
	case "gain_energy":
		if actor := sourceActor(combat, source); actor != nil {
			actor.Energy += effect.Value
		}
		combat.log(fmt.Sprintf("获得 %d 点能量", effect.Value))
	case "repeat_next_card":
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		queueNextCardRepeats(combat, source.index, count, effect.Tag)
		combat.log(fmt.Sprintf("%s获得效果：%s", sourceLabel(combat, source), pendingRepeatDescription(count, effect.Tag)))
	case "add_combat_card":
		count := effect.Value
		if count <= 0 {
			count = 1
		}
		for _, seatIndex := range combatCardTargetSeatIndexes(combat, source, effect, chosenTarget) {
			if err := addCombatCardsToPile(lib, combat, seatIndex, effect.CardID, effect.ItemType, count); err != nil {
				return err
			}
			combat.log(fmt.Sprintf("%s向%s加入 %d 张%s", sourceLabel(combat, source), combatCardPileLabel(effect.ItemType), count, lib.Cards[effect.CardID].Name))
		}
	default:
		return fmt.Errorf("unsupported combat effect %s", effect.Op)
	}
	syncPrimaryEnemy(combat)
	return nil
}

func triggerHandEndCards(lib *content.Library, players []PlayerState, combat *CombatState) {
	ensureCombatSeats(combat)
	for seatIndex := range combat.Seats {
		seat := combatSeat(combat, seatIndex)
		if seat == nil {
			continue
		}
		playerState := combatPlayerStateForSeat(combat, PlayerState{}, seatIndex)
		if seatIndex < len(players) {
			playerState = players[seatIndex]
		}
		for _, card := range append([]RuntimeCard{}, seat.Hand...) {
			def, ok := lib.Cards[card.ID]
			if !ok || !slices.Contains(def.Flags, "end_turn_trigger") {
				continue
			}
			combat.log("滞留状态牌触发：" + def.Name)
			effects, _ := splitCardPlayEffects(activeEffectsForCard(lib, card))
			for _, effect := range effects {
				if err := resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sidePlayer, index: seatIndex}, effect, card, CombatTarget{Kind: CombatTargetNone}); err != nil {
					combat.log("状态牌触发失败: " + err.Error())
				}
				if combat.Won || combat.Lost {
					return
				}
			}
		}
	}
}

func queueNextCardRepeats(combat *CombatState, seatIndex int, count int, tag string) {
	if count <= 0 {
		return
	}
	seat := combatSeat(combat, seatIndex)
	if seat == nil {
		return
	}
	for i := range seat.PendingCardRepeats {
		if seat.PendingCardRepeats[i].Tag != tag {
			continue
		}
		seat.PendingCardRepeats[i].Count += count
		return
	}
	seat.PendingCardRepeats = append(seat.PendingCardRepeats, PendingCardRepeat{Count: count, Tag: tag})
}

func consumeNextCardRepeats(combat *CombatState, seatIndex int, cardTags []string) int {
	seat := combatSeat(combat, seatIndex)
	if seat == nil {
		return 0
	}
	count := seat.NextCardRepeats
	seat.NextCardRepeats = 0
	remaining := make([]PendingCardRepeat, 0, len(seat.PendingCardRepeats))
	for _, repeat := range seat.PendingCardRepeats {
		if repeat.Count <= 0 {
			continue
		}
		if repeat.Tag == "" || slices.Contains(cardTags, repeat.Tag) {
			count += repeat.Count
			continue
		}
		remaining = append(remaining, repeat)
	}
	seat.PendingCardRepeats = remaining
	return count
}

func splitCardPlayEffects(effects []content.Effect) ([]content.Effect, int) {
	playEffects := make([]content.Effect, 0, len(effects))
	replies := 0
	for _, effect := range effects {
		if effect.Op == "reply" {
			replies += max(1, effect.Value)
			continue
		}
		playEffects = append(playEffects, effect)
	}
	return playEffects, replies
}

func pendingCardRepeats(combat *CombatState, seatIndex int) []PendingCardRepeat {
	seat := combatSeat(combat, seatIndex)
	if seat == nil {
		return nil
	}
	out := make([]PendingCardRepeat, 0, len(seat.PendingCardRepeats)+1)
	if seat.NextCardRepeats > 0 {
		out = append(out, PendingCardRepeat{Count: seat.NextCardRepeats})
	}
	for _, repeat := range seat.PendingCardRepeats {
		if repeat.Count > 0 {
			out = append(out, repeat)
		}
	}
	return out
}

func triggerPassiveWindow(lib *content.Library, playerState PlayerState, combat *CombatState, side combatSide, window string) {
	if side == sideEnemy {
		for i := range combat.Enemies {
			if combat.Enemies[i].HP <= 0 {
				continue
			}
			triggerEnemyPassiveWindow(lib, playerState, combat, i, window)
			if combat.Won || combat.Lost {
				return
			}
		}
		return
	}
	triggerPlayerPassiveWindow(lib, playerState, combat, 0, window)
}

func triggerPlayerPassiveWindow(lib *content.Library, playerState PlayerState, combat *CombatState, seatIndex int, window string) {
	for _, effect := range passiveEffectsForSide(lib, playerState, combat, sidePlayer, seatIndex) {
		if !matchesPassiveWindow(effect.Trigger, sidePlayer, window) {
			continue
		}
		if err := resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sidePlayer, index: seatIndex}, effect, RuntimeCard{}, CombatTarget{}); err != nil {
			combat.log("被动触发失败: " + err.Error())
		}
		if combat.Won || combat.Lost {
			return
		}
	}
}

func triggerEnemyPassiveWindow(lib *content.Library, playerState PlayerState, combat *CombatState, enemyIndex int, window string) {
	for _, effect := range passiveEffectsForSide(lib, playerState, combat, sideEnemy, enemyIndex) {
		if !matchesPassiveWindow(effect.Trigger, sideEnemy, window) {
			continue
		}
		if err := resolveCombatEffect(lib, playerState, combat, combatSourceRef{side: sideEnemy, index: enemyIndex}, effect, RuntimeCard{}, CombatTarget{}); err != nil {
			combat.log("敌方被动触发失败: " + err.Error())
		}
		if combat.Won || combat.Lost {
			return
		}
	}
}

func processStatusStart(combat *CombatState, actor *CombatActor, side combatSide, index int) {
	for _, name := range sortedStatusNames(actor.Statuses) {
		status := actor.Statuses[name]
		switch name {
		case "burn":
			lost := loseHP(actor, status.Stacks)
			if side == sidePlayer {
				recordSeatDamageReceived(combat, index, lost, 0)
			}
			combat.log(fmt.Sprintf("%s受到燃烧 %d 伤害", actorLabel(combat, side, index, actor), lost))
		case "poison":
			lost := loseHP(actor, status.Stacks)
			if side == sidePlayer {
				recordSeatDamageReceived(combat, index, lost, 0)
			}
			combat.log(fmt.Sprintf("%s受到中毒 %d 伤害", actorLabel(combat, side, index, actor), lost))
		}
	}
}

func processStatusEnd(combat *CombatState, actor *CombatActor, side combatSide, index int) {
	for _, name := range sortedStatusNames(actor.Statuses) {
		status := actor.Statuses[name]
		switch name {
		case "regen":
			before := actor.HP
			actor.HP = min(actor.MaxHP, actor.HP+status.Stacks)
			combat.log(fmt.Sprintf("%s恢复 %d 生命", actorLabel(combat, side, index, actor), actor.HP-before))
		}
	}
}

func readyStatuses(actor *CombatActor) {
	for name, status := range actor.Statuses {
		if status.Fresh {
			status.Fresh = false
			actor.Statuses[name] = status
		}
	}
}

func decrementTimedStatuses(actor *CombatActor) {
	for name, status := range actor.Statuses {
		if status.Duration <= 0 || status.Fresh {
			continue
		}
		status.Duration--
		if status.Duration <= 0 {
			delete(actor.Statuses, name)
			continue
		}
		actor.Statuses[name] = status
	}
}

func discardHand(combat *CombatState) {
	discardHandForSeat(combat, 0)
}

func discardHandForSeat(combat *CombatState, seatIndex int) {
	seat := combatSeat(combat, seatIndex)
	if seat == nil || len(seat.Hand) == 0 {
		return
	}
	seat.Discard = append(seat.Discard, seat.Hand...)
	seat.Hand = nil
	syncLegacySeat0(combat)
}

func discardHands(combat *CombatState) {
	ensureCombatSeats(combat)
	for seatIndex := range combat.Seats {
		discardHandForSeat(combat, seatIndex)
	}
}

func endTurnDiscardHands(lib *content.Library, combat *CombatState) {
	ensureCombatSeats(combat)
	for seatIndex := range combat.Seats {
		endTurnDiscardHandForSeat(lib, combat, seatIndex)
	}
}

func endTurnDiscardHandForSeat(lib *content.Library, combat *CombatState, seatIndex int) {
	seat := combatSeat(combat, seatIndex)
	if seat == nil || len(seat.Hand) == 0 {
		return
	}
	for _, card := range seat.Hand {
		discardRuntimeCard(lib, seat, card)
	}
	seat.Hand = nil
	syncLegacySeat0(combat)
}

func discardRuntimeCard(lib *content.Library, seat *CombatSeatState, card RuntimeCard) {
	if seat == nil {
		return
	}
	if def, ok := lib.Cards[card.ID]; ok && slices.Contains(def.Flags, "purge_on_discard") {
		seat.Exhaust = append(seat.Exhaust, card)
		return
	}
	seat.Discard = append(seat.Discard, card)
}

func combatCardTargetSeatIndexes(combat *CombatState, source combatSourceRef, effect content.Effect, chosenTarget CombatTarget) []int {
	ensureCombatSeats(combat)
	switch effect.Target {
	case "all_allies":
		indexes := make([]int, 0, len(combat.Seats))
		for i := range combat.Seats {
			indexes = append(indexes, i)
		}
		return indexes
	case "ally":
		if chosenTarget.Kind == CombatTargetAlly && validAllyTarget(combat, chosenTarget.Index) {
			return []int{chosenTarget.Index}
		}
	}
	if source.side == sidePlayer {
		if source.index >= 0 && source.index < len(combat.Seats) {
			return []int{source.index}
		}
		return []int{0}
	}
	if len(combat.Seats) == 0 {
		return nil
	}
	return []int{0}
}

func addCombatCardsToPile(lib *content.Library, combat *CombatState, seatIndex int, cardID string, pile string, count int) error {
	seat := combatSeat(combat, seatIndex)
	if seat == nil {
		return fmt.Errorf("invalid seat %d", seatIndex)
	}
	if _, ok := lib.Cards[cardID]; !ok {
		return fmt.Errorf("unknown combat card %q", cardID)
	}
	if count <= 0 {
		count = 1
	}
	for i := 0; i < count; i++ {
		card := RuntimeCard{ID: cardID}
		switch strings.TrimSpace(pile) {
		case "hand":
			if len(seat.Hand) >= combatHandLimit {
				seat.Discard = append(seat.Discard, card)
				combat.log("手牌已满，额外加入的牌进入弃牌堆")
				continue
			}
			seat.Hand = append(seat.Hand, card)
		case "discard":
			seat.Discard = append(seat.Discard, card)
		default:
			seat.DrawPile = append(seat.DrawPile, card)
		}
	}
	syncLegacySeat0(combat)
	return nil
}

func combatCardPileLabel(pile string) string {
	switch strings.TrimSpace(pile) {
	case "hand":
		return "手牌"
	case "discard":
		return "弃牌堆"
	default:
		return "抽牌堆"
	}
}

func applyOutgoingDamageModifiers(lib *content.Library, playerState PlayerState, combat *CombatState, source combatSourceRef, base int, tags []string) int {
	actor := sourceActor(combat, source)
	total := base

	total += statusStacks(actor.Statuses, "strength")
	if slices.Contains(tags, "spell") {
		total += statusStacks(actor.Statuses, "focus")
	}
	if statusStacks(actor.Statuses, "weak") > 0 {
		total -= 2
	}

	for _, effect := range passiveEffectsForSide(lib, playerState, combat, source.side, source.index) {
		if effect.Op != "modify_damage" {
			continue
		}
		if matchesModifierTrigger(effect.Trigger, tags) {
			total += effect.Value
		}
	}

	return max(0, total)
}

func applyIncomingDamageModifiers(lib *content.Library, playerState PlayerState, combat *CombatState, target CombatTarget, amount int) int {
	switch target.Kind {
	case CombatTargetEnemy:
		actor := targetActor(combat, target)
		total := amount
		if statusStacks(actor.Statuses, "vulnerable") > 0 {
			total += 2
		}
		if target.Index >= 0 && target.Index < len(combat.Enemies) {
			for _, effect := range combat.Enemies[target.Index].Passives {
				if effect.Op == "modify_taken_damage" && (effect.Trigger == "" || effect.Trigger == "incoming_damage") {
					total += effect.Value
				}
			}
		}
		return max(0, total)
	case CombatTargetEnemies:
		return amount
	default:
		total := amount
		seatIndex := 0
		if target.Kind == CombatTargetAlly {
			seatIndex = target.Index
		}
		actor := partyActorBySeat(combat, seatIndex)
		if actor != nil && statusStacks(actor.Statuses, "vulnerable") > 0 {
			total += 2
		}
		targetPlayer := combatPlayerStateForSeat(combat, playerState, seatIndex)
		for _, effect := range passiveEffectsForSide(lib, targetPlayer, combat, sidePlayer, seatIndex) {
			if effect.Op == "modify_taken_damage" && (effect.Trigger == "" || effect.Trigger == "incoming_damage") {
				total += effect.Value
			}
		}
		return max(0, total)
	}
}

func applyBlockModifiers(combat *CombatState, target CombatTarget, base int) int {
	actor := targetActor(combat, target)
	total := base + statusStacks(actor.Statuses, "guard")
	if statusStacks(actor.Statuses, "frail") > 0 {
		total -= 2
	}
	return max(0, total)
}

func applyStatus(actor *CombatActor, name string, value int, duration int) {
	if value == 0 {
		return
	}
	status := actor.Statuses[name]
	status.Name = name
	status.Stacks += value
	if duration > 0 && duration > status.Duration {
		status.Duration = duration
	}
	status.Fresh = true
	actor.Statuses[name] = status
}

func cleanseStatus(actor *CombatActor, name string, value int) int {
	status, ok := actor.Statuses[name]
	if !ok {
		return 0
	}
	if value <= 0 || value >= status.Stacks {
		delete(actor.Statuses, name)
		return status.Stacks
	}
	status.Stacks -= value
	actor.Statuses[name] = status
	return value
}

func statusStacks(statuses map[string]Status, name string) int {
	return statuses[name].Stacks
}

func sourceActor(combat *CombatState, source combatSourceRef) *CombatActor {
	if source.side == sideEnemy {
		return &combat.Enemies[source.index].CombatActor
	}
	return partyActorBySeat(combat, source.index)
}

func targetActor(combat *CombatState, target CombatTarget) *CombatActor {
	switch target.Kind {
	case CombatTargetEnemy:
		return &combat.Enemies[target.Index].CombatActor
	case CombatTargetAlly:
		if target.Index <= 0 {
			return &combat.Player
		}
		return &combat.Allies[target.Index-1]
	default:
		return &combat.Player
	}
}

func CardTargetKindForCard(lib *content.Library, card RuntimeCard) CombatTargetKind {
	needEnemy := false
	needAlly := false
	for _, effect := range activeEffectsForCard(lib, card) {
		switch effect.Target {
		case "all_enemies":
			return CombatTargetEnemies
		case "all_allies":
			return CombatTargetAllies
		}
		if effectTargetsEnemies(effect, combatSourceRef{side: sidePlayer, index: 0}) {
			needEnemy = true
		}
		if effectSupportsFriendlyTarget(effect) {
			needAlly = true
		}
	}
	switch {
	case needEnemy:
		return CombatTargetEnemy
	case needAlly:
		return CombatTargetAlly
	default:
		return CombatTargetNone
	}
}

func PotionTargetKind(lib *content.Library, potionID string) CombatTargetKind {
	def, ok := lib.Potions[potionID]
	if !ok {
		return CombatTargetNone
	}
	needEnemy := false
	needAlly := false
	for _, effect := range def.Effects {
		switch effect.Target {
		case "all_enemies":
			return CombatTargetEnemies
		case "all_allies":
			return CombatTargetAllies
		}
		if effectTargetsEnemies(effect, combatSourceRef{side: sidePlayer, index: 0}) {
			needEnemy = true
		}
		if effectSupportsFriendlyTarget(effect) {
			needAlly = true
		}
	}
	switch {
	case needEnemy:
		return CombatTargetEnemy
	case needAlly:
		return CombatTargetAlly
	default:
		return CombatTargetNone
	}
}

func DefaultTargetForCard(lib *content.Library, combat *CombatState, card RuntimeCard) CombatTarget {
	return DefaultTargetForSeat(lib, combat, 0, card)
}

func DefaultTargetForSeat(lib *content.Library, combat *CombatState, seatIndex int, card RuntimeCard) CombatTarget {
	switch CardTargetKindForCard(lib, card) {
	case CombatTargetEnemy:
		return CombatTarget{Kind: CombatTargetEnemy, Index: firstLivingEnemyIndex(combat)}
	case CombatTargetEnemies:
		return CombatTarget{Kind: CombatTargetEnemies}
	case CombatTargetAlly:
		if validAllyTarget(combat, seatIndex) {
			return CombatTarget{Kind: CombatTargetAlly, Index: seatIndex}
		}
		return CombatTarget{Kind: CombatTargetAlly, Index: 0}
	case CombatTargetAllies:
		return CombatTarget{Kind: CombatTargetAllies}
	default:
		return CombatTarget{Kind: CombatTargetNone}
	}
}

func DefaultTargetForPotion(lib *content.Library, combat *CombatState, seatIndex int, potionID string) CombatTarget {
	switch PotionTargetKind(lib, potionID) {
	case CombatTargetEnemy:
		return CombatTarget{Kind: CombatTargetEnemy, Index: firstLivingEnemyIndex(combat)}
	case CombatTargetEnemies:
		return CombatTarget{Kind: CombatTargetEnemies}
	case CombatTargetAlly:
		if validAllyTarget(combat, seatIndex) {
			return CombatTarget{Kind: CombatTargetAlly, Index: seatIndex}
		}
		return CombatTarget{Kind: CombatTargetAlly, Index: 0}
	case CombatTargetAllies:
		return CombatTarget{Kind: CombatTargetAllies}
	default:
		return CombatTarget{Kind: CombatTargetNone}
	}
}

func resolveDamageTargets(combat *CombatState, source combatSourceRef, effect content.Effect, chosenTarget CombatTarget) []CombatTarget {
	switch effect.Target {
	case "all_enemies":
		targets := []CombatTarget{}
		for i := range combat.Enemies {
			if combat.Enemies[i].HP > 0 {
				targets = append(targets, CombatTarget{Kind: CombatTargetEnemy, Index: i})
			}
		}
		return targets
	case "all_allies":
		return livingAllyTargets(combat)
	}

	if source.side == sideEnemy {
		return []CombatTarget{{Kind: CombatTargetAllies}}
	}
	if chosenTarget.Kind == CombatTargetEnemy && validLivingEnemyTarget(combat, chosenTarget.Index) {
		return []CombatTarget{{Kind: CombatTargetEnemy, Index: chosenTarget.Index}}
	}
	return []CombatTarget{{Kind: CombatTargetEnemy, Index: firstLivingEnemyIndex(combat)}}
}

func resolveSupportTargets(combat *CombatState, source combatSourceRef, effect content.Effect, chosenTarget CombatTarget) []CombatTarget {
	switch effect.Target {
	case "all_allies":
		return livingAllyTargets(combat)
	case "all_enemies":
		targets := []CombatTarget{}
		for i := range combat.Enemies {
			if combat.Enemies[i].HP > 0 {
				targets = append(targets, CombatTarget{Kind: CombatTargetEnemy, Index: i})
			}
		}
		return targets
	}

	if source.side == sideEnemy {
		return []CombatTarget{{Kind: CombatTargetEnemy, Index: source.index}}
	}
	if effectSupportsFriendlyTarget(effect) {
		if chosenTarget.Kind == CombatTargetAlly && validAllyTarget(combat, chosenTarget.Index) {
			return []CombatTarget{{Kind: CombatTargetAlly, Index: chosenTarget.Index}}
		}
		if validAllyTarget(combat, source.index) {
			return []CombatTarget{{Kind: CombatTargetAlly, Index: source.index}}
		}
		return []CombatTarget{{Kind: CombatTargetAlly, Index: 0}}
	}
	if validAllyTarget(combat, source.index) {
		return []CombatTarget{{Kind: CombatTargetAlly, Index: source.index}}
	}
	return []CombatTarget{{Kind: CombatTargetAlly, Index: 0}}
}

func effectTargetsEnemies(effect content.Effect, source combatSourceRef) bool {
	if effect.Target == "all_enemies" {
		return true
	}
	switch effect.Op {
	case "damage":
		return source.side == sidePlayer
	case "apply_status":
		return effect.Target == "enemy" || effect.Target == "opponent"
	default:
		return false
	}
}

func effectSupportsFriendlyTarget(effect content.Effect) bool {
	switch effect.Op {
	case "block", "heal", "cleanse_status":
		return true
	case "apply_status":
		switch effect.Status {
		case "guard", "regen", "taunt", "sheltered", "strength", "focus":
			return true
		}
	}
	return false
}

func validLivingEnemyTarget(combat *CombatState, index int) bool {
	return index >= 0 && index < len(combat.Enemies) && combat.Enemies[index].HP > 0
}

func validAllyTarget(combat *CombatState, index int) bool {
	if index == 0 {
		return combat.Player.HP > 0
	}
	index--
	return index >= 0 && index < len(combat.Allies) && combat.Allies[index].HP > 0
}

func livingAllyTargets(combat *CombatState) []CombatTarget {
	targets := []CombatTarget{}
	if combat.Player.HP > 0 {
		targets = append(targets, CombatTarget{Kind: CombatTargetAlly, Index: 0})
	}
	for i, ally := range combat.Allies {
		if ally.HP > 0 {
			targets = append(targets, CombatTarget{Kind: CombatTargetAlly, Index: i + 1})
		}
	}
	return targets
}

func firstLivingEnemyIndex(combat *CombatState) int {
	for i, enemy := range combat.Enemies {
		if enemy.HP > 0 {
			return i
		}
	}
	if len(combat.Enemies) == 0 {
		return 0
	}
	return 0
}

func matchesModifierTrigger(trigger string, tags []string) bool {
	if trigger == "" || trigger == "any" || trigger == "any_attack" {
		return true
	}
	return slices.Contains(tags, trigger)
}

func matchesPassiveWindow(trigger string, side combatSide, window string) bool {
	if trigger == window {
		return true
	}
	switch window {
	case "player_turn_start":
		return side == sidePlayer && trigger == "turn_start"
	case "player_turn_end":
		return side == sidePlayer && trigger == "turn_end"
	case "enemy_turn_start":
		return side == sideEnemy && trigger == "turn_start"
	case "enemy_turn_end":
		return side == sideEnemy && trigger == "turn_end"
	default:
		return false
	}
}

func turnStartWindow(side combatSide) string {
	if side == sideEnemy {
		return "enemy_turn_start"
	}
	return "player_turn_start"
}

func turnEndWindow(side combatSide) string {
	if side == sideEnemy {
		return "enemy_turn_end"
	}
	return "player_turn_end"
}

func sourceLabel(combat *CombatState, source combatSourceRef) string {
	if source.side == sideEnemy {
		if source.index >= 0 && source.index < len(combat.Enemies) {
			return combat.Enemies[source.index].Name
		}
		return "敌人"
	}
	if actor := partyActorBySeat(combat, source.index); actor != nil && actor.Name != "" {
		return actor.Name
	}
	return "队伍"
}

func actorLabel(combat *CombatState, side combatSide, index int, actor *CombatActor) string {
	if side == sideEnemy {
		if index >= 0 && index < len(combat.Enemies) {
			return combat.Enemies[index].Name
		}
	}
	if actor != nil && actor.Name != "" {
		return actor.Name
	}
	return "队伍"
}

func targetLabel(combat *CombatState, target CombatTarget) string {
	switch target.Kind {
	case CombatTargetEnemy:
		if target.Index >= 0 && target.Index < len(combat.Enemies) {
			return combat.Enemies[target.Index].Name
		}
		return "敌人"
	case CombatTargetEnemies:
		return "全体敌人"
	case CombatTargetAllies:
		return "队伍"
	case CombatTargetAlly:
		if target.Index == 0 {
			return combat.Player.Name
		}
		if target.Index-1 >= 0 && target.Index-1 < len(combat.Allies) {
			return combat.Allies[target.Index-1].Name
		}
		return "队友"
	default:
		return "目标"
	}
}

func effectTags(lib *content.Library, card RuntimeCard, effect content.Effect) []string {
	tags := []string{}
	if card.ID != "" {
		tags = append(tags, lib.Cards[card.ID].Tags...)
	}
	if effect.Tag != "" && !slices.Contains(tags, effect.Tag) {
		tags = append(tags, effect.Tag)
	}
	return tags
}

func sortedStatusNames(statuses map[string]Status) []string {
	names := make([]string, 0, len(statuses))
	for name := range statuses {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func statusLabel(name string) string {
	switch name {
	case "strength":
		return "力量"
	case "weak":
		return "虚弱"
	case "vulnerable":
		return "易伤"
	case "thorns":
		return "荆棘"
	case "guard":
		return "壁守"
	case "focus":
		return "聚能"
	case "frail":
		return "脆弱"
	case "burn":
		return "燃烧"
	case "poison":
		return "中毒"
	case "regen":
		return "再生"
	case "taunt":
		return "护卫"
	case "sheltered":
		return "庇护"
	default:
		return name
	}
}

func dealDamage(actor *CombatActor, amount int) int {
	if amount <= 0 {
		return 0
	}
	if actor.Block > 0 {
		absorbed := min(actor.Block, amount)
		actor.Block -= absorbed
		amount -= absorbed
	}
	actor.HP -= amount
	if actor.HP < 0 {
		actor.HP = 0
	}
	return amount
}

func loseHP(actor *CombatActor, amount int) int {
	if amount <= 0 {
		return 0
	}
	actor.HP -= amount
	if actor.HP < 0 {
		actor.HP = 0
	}
	return amount
}

func checkCombatOutcome(combat *CombatState) {
	aliveEnemies := false
	for _, enemy := range combat.Enemies {
		if enemy.HP > 0 {
			aliveEnemies = true
			break
		}
	}
	combat.Won = !aliveEnemies

	partyAlive := false
	for _, actor := range PartyMembersView(combat) {
		if actor.HP > 0 {
			partyAlive = true
			break
		}
	}
	combat.Lost = !partyAlive
	syncPrimaryEnemy(combat)
}

func (c *CombatState) log(text string) {
	c.Log = append(c.Log, CombatLogEntry{Turn: c.Turn, Text: text})
}

func dealDamageToTarget(combat *CombatState, source combatSourceRef, target CombatTarget, amount int) int {
	switch target.Kind {
	case CombatTargetEnemy:
		dealt := dealDamage(targetActor(combat, target), amount)
		if source.side == sidePlayer {
			recordSeatDamageDealt(combat, source.index, dealt)
		}
		syncPrimaryEnemy(combat)
		return dealt
	case CombatTargetEnemies:
		total := 0
		for i := range combat.Enemies {
			if combat.Enemies[i].HP > 0 {
				dealt := dealDamage(&combat.Enemies[i].CombatActor, amount)
				total += dealt
				if source.side == sidePlayer {
					recordSeatDamageDealt(combat, source.index, dealt)
				}
			}
		}
		syncPrimaryEnemy(combat)
		return total
	default:
		result := dealPlayerSideDamageDetailed(combat, amount)
		for seatIndex := range result.SeatTaken {
			recordSeatDamageReceived(combat, seatIndex, result.SeatTaken[seatIndex], result.SeatBlocked[seatIndex])
		}
		return result.Taken
	}
}

func syncPrimaryEnemy(combat *CombatState) {
	if len(combat.Enemies) == 0 {
		combat.Enemy = CombatEnemy{}
		return
	}
	index := firstLivingEnemyIndex(combat)
	combat.Enemy = combat.Enemies[index]
}

func normalizeLegacyEnemyState(combat *CombatState) {
	if len(combat.Enemies) == 1 {
		combat.Enemies[0] = combat.Enemy
	}
}
