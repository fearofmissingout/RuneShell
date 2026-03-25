package app

import (
	"bytes"
	"testing"
	"time"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
)

func TestProgramNavigationCanReachCardCodexThirdPage(t *testing.T) {
	lib, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}

	store := storage.NewStore(t.TempDir())
	start := newModel(lib, store, engine.DefaultProfile(lib), nil)

	var output bytes.Buffer
	program := tea.NewProgram(
		start,
		tea.WithInput(nil),
		tea.WithOutput(&output),
		tea.WithoutRenderer(),
		tea.WithoutSignals(),
	)

	type runResult struct {
		model tea.Model
		err   error
	}
	done := make(chan runResult, 1)
	go func() {
		finalModel, err := program.Run()
		done <- runResult{model: finalModel, err: err}
	}()

	send := func(msg tea.Msg) {
		program.Send(msg)
		time.Sleep(20 * time.Millisecond)
	}

	send(tea.KeyMsg{Type: tea.KeyDown})
	send(tea.KeyMsg{Type: tea.KeyDown})
	send(tea.KeyMsg{Type: tea.KeyEnter})
	send(tea.KeyMsg{Type: tea.KeyRight})
	send(tea.KeyMsg{Type: tea.KeyRight})
	send(tea.KeyMsg{Type: tea.KeyDown})
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("program.Run() error = %v", result.err)
		}
		got := result.model.(model)
		if got.screen != screenCodex {
			t.Fatalf("expected to remain on codex screen, got %q", got.screen)
		}
		if got.codexItemCount() < 46 {
			t.Fatalf("expected at least 46 card codex items after real program navigation, got %d (tab=%d index=%d)", got.codexItemCount(), got.codexTab, got.index)
		}
		if got.index != 21 {
			t.Fatalf("expected navigation to land on card index 21, got %d (tab=%d count=%d)", got.index, got.codexTab, got.codexItemCount())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("program did not exit after simulated codex navigation")
	}
}
