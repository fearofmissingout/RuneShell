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
