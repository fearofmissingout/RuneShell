package engine

import (
	"reflect"
	"testing"
)

func TestGenerateActMapDeterministicAndValid(t *testing.T) {
	first := GenerateActMap(42, 1, nil)
	second := GenerateActMap(42, 1, nil)

	if !reflect.DeepEqual(first, second) {
		t.Fatal("expected map generation to be deterministic for same seed")
	}
	if err := ValidateMapConstraints(first); err != nil {
		t.Fatalf("ValidateMapConstraints() error = %v", err)
	}
	if got := first.Floors[14][0].Kind; got != NodeBoss {
		t.Fatalf("expected final floor to be boss, got %s", got)
	}
}

func TestGenerateActMapFavorsCombatNodes(t *testing.T) {
	combatNodes := 0
	nonCombatNodes := 0
	earlyCombatNodes := 0
	earlyNonCombatNodes := 0

	for seed := int64(1); seed <= 200; seed++ {
		graph := GenerateActMap(seed, 1, nil)
		for floorIndex, layer := range graph.Floors[:14] {
			for _, node := range layer {
				switch node.Kind {
				case NodeMonster, NodeElite:
					combatNodes++
					if floorIndex < 4 {
						earlyCombatNodes++
					}
				case NodeEvent, NodeShop, NodeRest:
					nonCombatNodes++
					if floorIndex < 4 {
						earlyNonCombatNodes++
					}
				}
			}
		}
	}

	if combatNodes <= nonCombatNodes {
		t.Fatalf("expected combat nodes to outnumber non-combat nodes, got combat=%d nonCombat=%d", combatNodes, nonCombatNodes)
	}
	if earlyCombatNodes <= earlyNonCombatNodes {
		t.Fatalf("expected early floors to lean toward combat nodes, got combat=%d nonCombat=%d", earlyCombatNodes, earlyNonCombatNodes)
	}
}
