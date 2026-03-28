package netplay

import "cmdcards/internal/content"

func (s *server) localizedLibLocked(playerID string) *content.Library {
	if s == nil {
		return nil
	}
	base := s.baseLib
	if base == nil {
		base = s.lib
	}
	return content.LocalizeLibrary(base, s.playerLanguageLocked(playerID))
}
