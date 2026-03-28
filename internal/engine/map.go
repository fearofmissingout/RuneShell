package engine

import (
	"fmt"
	"math/rand"
	"slices"
)

func GenerateActMap(seed int64, act int, _ []string) MapGraph {
	rng := rand.New(rand.NewSource(seed + int64(act*7919)))
	floors := make([][]Node, 15)
	shopLastFloor := false

	for floor := 1; floor <= 14; floor++ {
		count := rng.Intn(3) + 2
		if floor == 1 {
			count = 3
		}
		layer := make([]Node, 0, count)
		for idx := 0; idx < count; idx++ {
			kind := pickNodeKind(rng, floor, shopLastFloor)
			layer = append(layer, Node{
				ID:    fmt.Sprintf("a%d-f%d-n%d", act, floor, idx),
				Act:   act,
				Floor: floor,
				Index: idx,
				Kind:  kind,
			})
		}
		if floor == 1 {
			for i := range layer {
				layer[i].Kind = NodeMonster
			}
		}
		if floor == 14 {
			layer[0].Kind = NodeRest
		}
		shopLastFloor = false
		for _, node := range layer {
			if node.Kind == NodeShop {
				shopLastFloor = true
				break
			}
		}
		floors[floor-1] = layer
	}

	floors[14] = []Node{{
		ID:    fmt.Sprintf("a%d-f15-boss", act),
		Act:   act,
		Floor: 15,
		Index: 0,
		Kind:  NodeBoss,
	}}

	for floor := 0; floor < 14; floor++ {
		nextCount := len(floors[floor+1])
		for idx := range floors[floor] {
			targetCount := 2
			if nextCount <= 2 {
				targetCount = nextCount
			}
			targets := map[int]struct{}{}
			for len(targets) < targetCount {
				targets[rng.Intn(nextCount)] = struct{}{}
			}
			for target := range targets {
				floors[floor][idx].Edges = append(floors[floor][idx].Edges, floors[floor+1][target].ID)
			}
			slices.Sort(floors[floor][idx].Edges)
		}
	}

	return MapGraph{Act: act, Floors: floors}
}

func pickNodeKind(rng *rand.Rand, floor int, shopLastFloor bool) NodeKind {
	if floor < 5 {
		pool := []NodeKind{
			NodeMonster, NodeMonster, NodeMonster,
			NodeEvent,
			NodeRest,
		}
		if !shopLastFloor {
			pool = append(pool, NodeShop)
		}
		return pool[rng.Intn(len(pool))]
	}
	pool := []NodeKind{
		NodeMonster, NodeMonster, NodeMonster, NodeMonster,
		NodeEvent,
		NodeEvent,
		NodeRest,
		NodeElite,
	}
	if !shopLastFloor {
		pool = append(pool, NodeShop)
	}
	return pool[rng.Intn(len(pool))]
}

func ValidateMapConstraints(graph MapGraph) error {
	if len(graph.Floors) != 15 {
		return fmt.Errorf("expected 15 floors, got %d", len(graph.Floors))
	}
	previousHadShop := false
	for floorIndex, layer := range graph.Floors {
		floor := floorIndex + 1
		if len(layer) == 0 {
			return fmt.Errorf("floor %d is empty", floor)
		}
		hasShop := false
		for _, node := range layer {
			if node.Floor != floor {
				return fmt.Errorf("node %s has wrong floor %d", node.ID, node.Floor)
			}
			if floor == 1 && node.Kind != NodeMonster {
				return fmt.Errorf("floor 1 must be monster")
			}
			if floor < 5 && node.Kind == NodeElite {
				return fmt.Errorf("elite before floor 5")
			}
			if floor == 15 && node.Kind != NodeBoss {
				return fmt.Errorf("floor 15 must be boss")
			}
			if node.Kind == NodeShop {
				hasShop = true
			}
		}
		if previousHadShop && hasShop {
			return fmt.Errorf("consecutive shop floors")
		}
		previousHadShop = hasShop
	}
	if graph.Floors[13][0].Kind != NodeRest {
		return fmt.Errorf("floor 14 must contain a rest node at index 0")
	}
	return nil
}

func FlattenNodes(graph MapGraph) map[string]Node {
	out := map[string]Node{}
	for _, layer := range graph.Floors {
		for _, node := range layer {
			out[node.ID] = node
		}
	}
	return out
}
