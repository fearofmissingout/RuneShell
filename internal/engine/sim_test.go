package engine

import (
	"reflect"
	"testing"

	"cmdcards/internal/content"
)

func TestRunSmokeStoryWins(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	result, err := RunSmoke(lib, DefaultProfile(lib), ModeStory, "vanguard", 1)
	if err != nil {
		t.Fatalf("RunSmoke() error = %v", err)
	}
	if result.Result != RunStatusWon {
		t.Fatalf("expected story smoke to win, got %s", result.Result)
	}
	if result.ReachedAct != 3 {
		t.Fatalf("expected to finish act 3, got act %d", result.ReachedAct)
	}
}

func TestRunSmokeEndlessReachesActFour(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	result, err := RunSmoke(lib, DefaultProfile(lib), ModeEndless, "vanguard", 1)
	if err != nil {
		t.Fatalf("RunSmoke() error = %v", err)
	}
	if result.ReachedAct < 4 {
		t.Fatalf("expected endless smoke to reach act 4+, got act %d", result.ReachedAct)
	}
}

func TestRunSmokeDeterministicForSameSeed(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	first, err := RunSmoke(lib, DefaultProfile(lib), ModeEndless, "vanguard", 1)
	if err != nil {
		t.Fatalf("first RunSmoke() error = %v", err)
	}
	second, err := RunSmoke(lib, DefaultProfile(lib), ModeEndless, "vanguard", 1)
	if err != nil {
		t.Fatalf("second RunSmoke() error = %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected smoke results to be deterministic for same seed,\nfirst=%+v\nsecond=%+v", first, second)
	}
}
