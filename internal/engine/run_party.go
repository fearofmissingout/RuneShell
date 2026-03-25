package engine

import (
	"fmt"

	"cmdcards/internal/content"
)

func StartEncounterForParty(lib *content.Library, run *RunState, party []PlayerState, node Node) (*CombatState, error) {
	var kind string
	switch node.Kind {
	case NodeMonster:
		kind = "monster"
	case NodeElite:
		kind = "elite"
	case NodeBoss:
		kind = "boss"
	default:
		return nil, fmt.Errorf("node %s is not a combat node", node.ID)
	}

	encounters := buildEncounterGroup(lib, run, kind, node.Act, node.Floor, node.Index)
	if len(encounters) == 0 {
		return nil, fmt.Errorf("no encounters available for %s act %d", kind, node.Act)
	}
	rewardBasis := aggregateEncounterGroup(encounters, kind, node.Act)
	if len(party) == 0 {
		party = []PlayerState{run.Player}
	}
	return NewCombatForPartyWithEnemies(lib, party, encounters, rewardBasis, run.Seed+int64(node.Floor*101+node.Index)), nil
}
