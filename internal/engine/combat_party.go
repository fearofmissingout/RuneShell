package engine

import (
	"fmt"
	"slices"

	"cmdcards/internal/content"
)

func ScaleEncounterForParty(encounter content.EncounterDef, partySize int) content.EncounterDef {
	if partySize <= 1 {
		return encounter
	}
	scaled := encounter
	scaled.Passives = append([]content.Effect{}, encounter.Passives...)
	scaled.IntentCycle = make([]content.EnemyIntentDef, 0, len(encounter.IntentCycle))
	for _, intent := range encounter.IntentCycle {
		copied := intent
		copied.Effects = append([]content.Effect{}, intent.Effects...)
		scaled.IntentCycle = append(scaled.IntentCycle, copied)
	}
	scaled.HP *= 2
	for i := range scaled.Passives {
		if scaled.Passives[i].Op == "damage" {
			scaled.Passives[i].Value *= 2
		}
	}
	for i := range scaled.IntentCycle {
		for j := range scaled.IntentCycle[i].Effects {
			if scaled.IntentCycle[i].Effects[j].Op == "damage" {
				scaled.IntentCycle[i].Effects[j].Value *= 2
			}
		}
	}
	return scaled
}

func ScaleEncounterGroupForParty(encounters []content.EncounterDef, partySize int) []content.EncounterDef {
	scaled := make([]content.EncounterDef, 0, len(encounters))
	for _, encounter := range encounters {
		scaled = append(scaled, ScaleEncounterForParty(encounter, partySize))
	}
	return scaled
}

func NewCombatForParty(lib *content.Library, players []PlayerState, encounter content.EncounterDef, seed int64) *CombatState {
	return NewCombatForPartyWithEnemies(lib, players, []content.EncounterDef{encounter}, encounter, seed)
}

func NewCombatForPartyWithEnemies(lib *content.Library, players []PlayerState, encounters []content.EncounterDef, rewardBasis content.EncounterDef, seed int64) *CombatState {
	if len(players) == 0 {
		return NewCombatWithEnemies(lib, PlayerState{Name: "player", MaxHP: 1, HP: 1, MaxEnergy: 3}, encounters, rewardBasis, seed)
	}
	scaled := ScaleEncounterGroupForParty(encounters, len(players))
	scaledBasis := ScaleEncounterForParty(rewardBasis, len(players))
	state := NewCombatWithEnemies(lib, players[0], scaled, scaledBasis, seed)
	if len(players) > 1 {
		state.Coop = CombatCoopState{
			Enabled:       true,
			SharedTurnEnd: true,
			EndTurnVotes:  make([]bool, len(players)),
		}
		state.Allies = make([]CombatActor, 0, len(players)-1)
		for _, player := range players[1:] {
			state.Allies = append(state.Allies, CombatActor{
				Name:      player.Name,
				HP:        player.HP,
				MaxHP:     player.MaxHP,
				Energy:    player.MaxEnergy,
				MaxEnergy: player.MaxEnergy,
				Statuses:  map[string]Status{},
			})
		}
		applyLatePartyCombatStartEffects(lib, players[0], state)
	}
	return state
}

func applyLatePartyCombatStartEffects(lib *content.Library, player PlayerState, combat *CombatState) {
	for _, effect := range passiveEffectsForSide(lib, player, combat, sidePlayer, 0) {
		if !matchesPassiveWindow(effect.Trigger, sidePlayer, "combat_start") {
			continue
		}
		if effect.Target != "all_allies" {
			continue
		}
		for allyIndex := range combat.Allies {
			target := CombatTarget{Kind: CombatTargetAlly, Index: allyIndex + 1}
			switch effect.Op {
			case "block":
				amount := applyBlockModifiers(combat, target, effect.Value)
				combat.Allies[allyIndex].Block += amount
			case "heal":
				combat.Allies[allyIndex].HP = min(combat.Allies[allyIndex].MaxHP, combat.Allies[allyIndex].HP+effect.Value)
			case "apply_status":
				applyStatus(&combat.Allies[allyIndex], effect.Status, effect.Value, effect.Duration)
			case "cleanse_status":
				cleanseStatus(&combat.Allies[allyIndex], effect.Status, effect.Value)
			default:
				combat.log("被动补触发失败: unsupported all_allies late trigger " + effect.Op)
			}
		}
	}
}

func RequestEndTurnVote(combat *CombatState, memberIndex int) bool {
	if len(combat.Coop.EndTurnVotes) == 0 {
		return true
	}
	if memberIndex < 0 || memberIndex >= len(combat.Coop.EndTurnVotes) {
		return false
	}
	combat.Coop.EndTurnVotes[memberIndex] = true
	for _, ready := range combat.Coop.EndTurnVotes {
		if !ready {
			return false
		}
	}
	return true
}

func ResetEndTurnVotes(combat *CombatState) {
	if len(combat.Coop.EndTurnVotes) == 0 {
		return
	}
	clear(combat.Coop.EndTurnVotes)
}

func ResetCoopTurnTracking(combat *CombatState) {
	if combat == nil {
		return
	}
	combat.Coop.ActedSeats = nil
	combat.Coop.TeamComboDone = false
}

func RecordCoopAction(combat *CombatState, memberIndex int) (firstForSeat bool, uniqueActors int) {
	if combat == nil || !combat.Coop.Enabled || memberIndex < 0 {
		return false, 0
	}
	for _, seat := range combat.Coop.ActedSeats {
		if seat == memberIndex {
			return false, len(combat.Coop.ActedSeats)
		}
	}
	combat.Coop.ActedSeats = append(combat.Coop.ActedSeats, memberIndex)
	return true, len(combat.Coop.ActedSeats)
}

func PartyActors(combat *CombatState) []*CombatActor {
	actors := []*CombatActor{&combat.Player}
	for i := range combat.Allies {
		actors = append(actors, &combat.Allies[i])
	}
	return actors
}

func PartyMembersView(combat *CombatState) []CombatActor {
	members := make([]CombatActor, 0, 1+len(combat.Allies))
	members = append(members, combat.Player)
	members = append(members, combat.Allies...)
	return members
}

func LivingPartyActors(combat *CombatState) []*CombatActor {
	living := []*CombatActor{}
	for _, actor := range PartyActors(combat) {
		if actor.HP > 0 {
			living = append(living, actor)
		}
	}
	return living
}

func DealPlayerSideDamage(combat *CombatState, amount int) int {
	if amount <= 0 {
		return 0
	}

	living := LivingPartyActors(combat)
	if len(living) == 0 {
		return 0
	}
	if len(living) == 1 {
		return dealDamage(living[0], amount)
	}

	priority := slices.Clone(living)
	slices.SortFunc(priority, func(a, b *CombatActor) int {
		if cmp := compareInt(statusStacks(b.Statuses, "taunt"), statusStacks(a.Statuses, "taunt")); cmp != 0 {
			return cmp
		}
		if cmp := compareInt(b.Block, a.Block); cmp != 0 {
			return cmp
		}
		return compareInt(b.HP, a.HP)
	})

	remaining := amount
	for _, actor := range priority {
		if remaining <= 0 {
			break
		}
		if actor.Block <= 0 {
			continue
		}
		absorbed := min(actor.Block, remaining)
		actor.Block -= absorbed
		remaining -= absorbed
	}
	if remaining <= 0 {
		return 0
	}

	weights := make([]int, 0, len(living))
	totalWeight := 0
	for _, actor := range living {
		weight := 1 + statusStacks(actor.Statuses, "taunt") - statusStacks(actor.Statuses, "sheltered")
		if weight < 1 {
			weight = 1
		}
		weights = append(weights, weight)
		totalWeight += weight
	}

	damageTaken := 0
	leftover := remaining
	for i, actor := range living {
		share := remaining * weights[i] / totalWeight
		if i == len(living)-1 {
			share = leftover
		}
		leftover -= share
		damageTaken += loseHP(actor, share)
	}
	return damageTaken
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func DescribeParty(combat *CombatState) []string {
	lines := make([]string, 0, 1+len(combat.Allies))
	for i, actor := range PartyMembersView(combat) {
		label := fmt.Sprintf("%d. %s HP %d/%d Block %d", i+1, actor.Name, actor.HP, actor.MaxHP, actor.Block)
		if status := DescribeStatuses(actor.Statuses); status != "" {
			label += " | " + status
		}
		lines = append(lines, label)
	}
	return lines
}

func EnemyMembersView(combat *CombatState) []CombatEnemy {
	return slices.Clone(combat.Enemies)
}

func LivingEnemyTargets(combat *CombatState) []CombatTarget {
	targets := []CombatTarget{}
	for i, enemy := range combat.Enemies {
		if enemy.HP > 0 {
			targets = append(targets, CombatTarget{Kind: CombatTargetEnemy, Index: i})
		}
	}
	return targets
}

func LivingFriendlyTargets(combat *CombatState) []CombatTarget {
	return livingAllyTargets(combat)
}

func CycleCombatTarget(combat *CombatState, current CombatTarget, kind CombatTargetKind, delta int) CombatTarget {
	switch kind {
	case CombatTargetEnemy:
		targets := LivingEnemyTargets(combat)
		if len(targets) == 0 {
			return CombatTarget{Kind: CombatTargetEnemy, Index: 0}
		}
		return cycleTargetList(targets, current, delta)
	case CombatTargetAlly:
		targets := LivingFriendlyTargets(combat)
		if len(targets) == 0 {
			return CombatTarget{Kind: CombatTargetAlly, Index: 0}
		}
		return cycleTargetList(targets, current, delta)
	case CombatTargetEnemies:
		return CombatTarget{Kind: CombatTargetEnemies}
	case CombatTargetAllies:
		return CombatTarget{Kind: CombatTargetAllies}
	default:
		return CombatTarget{Kind: CombatTargetNone}
	}
}

func cycleTargetList(targets []CombatTarget, current CombatTarget, delta int) CombatTarget {
	if len(targets) == 0 {
		return CombatTarget{Kind: CombatTargetNone}
	}
	position := 0
	for i, target := range targets {
		if target.Kind == current.Kind && target.Index == current.Index {
			position = i
			break
		}
	}
	position = wrapTargetIndex(position+delta, len(targets))
	return targets[position]
}

func wrapTargetIndex(index, length int) int {
	if length == 0 {
		return 0
	}
	if index < 0 {
		return length - 1
	}
	if index >= length {
		return 0
	}
	return index
}

func DescribeCombatTarget(combat *CombatState, target CombatTarget) string {
	switch target.Kind {
	case CombatTargetEnemy:
		if target.Index >= 0 && target.Index < len(combat.Enemies) {
			return "敌方目标：" + combat.Enemies[target.Index].Name
		}
		return "敌方目标"
	case CombatTargetEnemies:
		return "目标：全体敌人"
	case CombatTargetAllies:
		return "目标：全体友军"
	case CombatTargetAlly:
		if target.Index == 0 {
			return "友方目标：" + combat.Player.Name
		}
		if target.Index-1 >= 0 && target.Index-1 < len(combat.Allies) {
			return "友方目标：" + combat.Allies[target.Index-1].Name
		}
		return "友方目标"
	default:
		return "当前卡牌无需选目标"
	}
}
