package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
)

type Store struct {
	baseDir string
}

func NewDefaultStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{baseDir: filepath.Join(dir, "cmdcards")}, nil
}

func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) BaseDir() string {
	return s.baseDir
}

func (s *Store) Ensure() error {
	return os.MkdirAll(s.baseDir, 0o755)
}

func (s *Store) profilePath() string {
	return filepath.Join(s.baseDir, "profile.json")
}

func (s *Store) runPath() string {
	return filepath.Join(s.baseDir, "run.json")
}

func (s *Store) LoadProfile(lib *content.Library) (engine.Profile, error) {
	if err := s.Ensure(); err != nil {
		return engine.Profile{}, err
	}
	data, err := os.ReadFile(s.profilePath())
	if errors.Is(err, os.ErrNotExist) {
		profile := engine.DefaultProfile(lib)
		if err := s.SaveProfile(profile); err != nil {
			return engine.Profile{}, err
		}
		return profile, nil
	}
	if err != nil {
		return engine.Profile{}, err
	}
	var profile engine.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return engine.Profile{}, err
	}
	engine.NormalizeProfile(lib, &profile)
	return profile, nil
}

func (s *Store) SaveProfile(profile engine.Profile) error {
	if err := s.Ensure(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.profilePath(), data, 0o644)
}

func (s *Store) LoadRun() (*engine.RunState, error) {
	if err := s.Ensure(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.runPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var run engine.RunState
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, err
	}
	if run.Version == 0 {
		run.Version = 1
	}
	return &run, nil
}

func (s *Store) SaveRun(run *engine.RunState) error {
	if err := s.Ensure(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.runPath(), data, 0o644)
}

func (s *Store) ClearRun() error {
	if err := s.Ensure(); err != nil {
		return err
	}
	err := os.Remove(s.runPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
