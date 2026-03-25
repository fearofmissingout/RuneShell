package content

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadEmbedded(t *testing.T) {
	lib, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	if len(lib.Classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(lib.Classes))
	}
	if len(lib.Cards) < 20 {
		t.Fatalf("expected card library to be populated, got %d cards", len(lib.Cards))
	}
}

func TestLoadFSRejectsInvalidEffect(t *testing.T) {
	files := fstest.MapFS{}
	for _, name := range []string{
		"assets/classes.json",
		"assets/cards.json",
		"assets/relics.json",
		"assets/potions.json",
		"assets/equipments.json",
		"assets/encounters.json",
		"assets/events.json",
	} {
		data, err := fs.ReadFile(embeddedFiles, name)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		if name == "assets/cards.json" {
			data = []byte(`[{"id":"bad","class_id":"neutral","name":"坏牌","description":"坏","rarity":"starter","cost":1,"effects":[{"op":"explode"}]}]`)
		}
		files[name] = &fstest.MapFile{Data: data}
	}

	_, err := LoadFS(files)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported op") {
		t.Fatalf("expected unsupported op error, got %v", err)
	}
}
