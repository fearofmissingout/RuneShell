package netplay

import (
	"strings"

	"cmdcards/internal/i18n"
)

func normalizeLanguage(lang string) string {
	return i18n.NormalizeLanguage(lang)
}

func textForLanguage(lang string, key string, args i18n.Args) string {
	return i18n.Text(normalizeLanguage(lang), key, args)
}

func (s *server) playerLanguageLocked(id string) string {
	if player := s.players[id]; player != nil {
		if strings.TrimSpace(player.Language) != "" {
			return normalizeLanguage(player.Language)
		}
		return i18n.LangEnUS
	}
	return i18n.DefaultLanguage
}

func (s *server) textLocked(id string, key string, args i18n.Args) string {
	return textForLanguage(s.playerLanguageLocked(id), key, args)
}

func snapshotLanguage(snapshot *roomSnapshot) string {
	if snapshot == nil {
		return i18n.DefaultLanguage
	}
	return normalizeLanguage(snapshot.Language)
}

func snapshotText(snapshot *roomSnapshot, key string, args i18n.Args) string {
	return textForLanguage(snapshotLanguage(snapshot), key, args)
}

func localizedModeName(lang string, mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "story":
		return textForLanguage(lang, "mode.story", nil)
	case "endless":
		return textForLanguage(lang, "mode.endless", nil)
	default:
		return mode
	}
}

func phaseDisplayNameFor(lang string, phase string) string {
	switch phase {
	case phaseLobby:
		return textForLanguage(lang, "net.phase.lobby", nil)
	case phaseMap:
		return textForLanguage(lang, "net.phase.map", nil)
	case phaseCombat:
		return textForLanguage(lang, "net.phase.combat", nil)
	case phaseReward:
		return textForLanguage(lang, "net.phase.reward", nil)
	case phaseEvent:
		return textForLanguage(lang, "net.phase.event", nil)
	case phaseShop:
		return textForLanguage(lang, "net.phase.shop", nil)
	case phaseRest:
		return textForLanguage(lang, "net.phase.rest", nil)
	case phaseEquipment:
		return textForLanguage(lang, "net.phase.equipment", nil)
	case phaseDeckAction:
		return textForLanguage(lang, "net.phase.deck_action", nil)
	case phaseSummary:
		return textForLanguage(lang, "net.phase.summary", nil)
	default:
		return phase
	}
}

func phaseDisplayName(phase string) string {
	return phaseDisplayNameFor(i18n.DefaultLanguage, phase)
}
