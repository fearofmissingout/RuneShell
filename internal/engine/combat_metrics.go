package engine

import "fmt"

func (metrics *CombatMetrics) Add(other CombatMetrics) {
	if metrics == nil {
		return
	}
	metrics.DamageDealt += other.DamageDealt
	metrics.StatusApplied += other.StatusApplied
	metrics.StatusReceived += other.StatusReceived
	metrics.DamageBlocked += other.DamageBlocked
	metrics.DamageTaken += other.DamageTaken
}

func CombinedCombatMetrics(parts ...CombatMetrics) CombatMetrics {
	var total CombatMetrics
	for _, part := range parts {
		total.Add(part)
	}
	return total
}

func FormatCombatMetrics(metrics CombatMetrics, turns int) []string {
	lines := make([]string, 0, 5)
	lines = append(lines, formatCombatMetricLine("伤害输出", metrics.DamageDealt, turns))
	lines = append(lines, formatCombatMetricLine("给予异常", metrics.StatusApplied, turns))
	lines = append(lines, formatCombatMetricLine("受到异常", metrics.StatusReceived, turns))
	lines = append(lines, formatCombatMetricLine("格挡抵消", metrics.DamageBlocked, turns))
	lines = append(lines, formatCombatMetricLine("受到伤害", metrics.DamageTaken, turns))
	return lines
}

func formatCombatMetricLine(label string, total int, turns int) string {
	if turns <= 0 {
		return fmt.Sprintf("%s 0.0/每回合 | 总数 %d", label, total)
	}
	return fmt.Sprintf("%s %.1f/每回合 | 总数 %d", label, float64(total)/float64(turns), total)
}

func combatSeatMetrics(combat *CombatState, seatIndex int) *CombatMetrics {
	seat := combatSeat(combat, seatIndex)
	if seat == nil {
		return nil
	}
	return &seat.Metrics
}

func recordSeatDamageDealt(combat *CombatState, seatIndex int, amount int) {
	if amount <= 0 {
		return
	}
	if metrics := combatSeatMetrics(combat, seatIndex); metrics != nil {
		metrics.DamageDealt += amount
	}
}

func recordSeatStatusApplied(combat *CombatState, seatIndex int, amount int) {
	if amount <= 0 {
		return
	}
	if metrics := combatSeatMetrics(combat, seatIndex); metrics != nil {
		metrics.StatusApplied += amount
	}
}

func recordSeatStatusReceived(combat *CombatState, seatIndex int, amount int) {
	if amount <= 0 {
		return
	}
	if metrics := combatSeatMetrics(combat, seatIndex); metrics != nil {
		metrics.StatusReceived += amount
	}
}

func recordSeatDamageReceived(combat *CombatState, seatIndex int, taken int, blocked int) {
	if metrics := combatSeatMetrics(combat, seatIndex); metrics != nil {
		if taken > 0 {
			metrics.DamageTaken += taken
		}
		if blocked > 0 {
			metrics.DamageBlocked += blocked
		}
	}
}

func CombatMetricsForSeat(combat *CombatState, seatIndex int) CombatMetrics {
	if metrics := combatSeatMetrics(combat, seatIndex); metrics != nil {
		return *metrics
	}
	return CombatMetrics{}
}

func CombatTurns(combat *CombatState) int {
	if combat == nil || combat.Turn <= 0 {
		return 0
	}
	return combat.Turn
}

func CombatSeatIndexForRun(combat *CombatState, run *RunState) int {
	if combat == nil || run == nil || len(combat.SeatPlayers) == 0 {
		return 0
	}
	exactMatches := []int{}
	nameMatches := []int{}
	for i, player := range combat.SeatPlayers {
		if player.Name != run.Player.Name {
			continue
		}
		nameMatches = append(nameMatches, i)
		if player.ClassID == run.Player.ClassID {
			exactMatches = append(exactMatches, i)
		}
	}
	switch {
	case len(exactMatches) == 1:
		return exactMatches[0]
	case len(nameMatches) == 1:
		return nameMatches[0]
	case len(combat.SeatPlayers) == 1:
		return 0
	default:
		return 0
	}
}

func applyCombatMetricsToRun(run *RunState, metrics CombatMetrics, turns int) {
	if run == nil {
		return
	}
	run.Stats.Metrics.Add(metrics)
	if turns > 0 {
		run.Stats.CombatTurns += turns
	}
}
