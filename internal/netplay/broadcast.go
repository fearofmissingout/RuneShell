package netplay

import (
	"fmt"
	"strings"
)

func (s *server) setClientNoticeLocked(id, notice string) {
	if notice == "" {
		return
	}
	if client := s.clients[id]; client != nil {
		client.notice = notice
	}
}

func (s *server) setClientResumeLocked(id string, lines []string) {
	if len(lines) == 0 {
		return
	}
	if client := s.clients[id]; client != nil {
		client.resume = append([]string{}, lines...)
	}
}

func (s *server) announcePhaseLocked(reason string) {
	title := phaseDisplayName(s.phase)
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		notice := fmt.Sprintf("Phase changed: %s.", title)
		if reason != "" {
			notice = fmt.Sprintf("%s %s", notice, reason)
		}
		s.setClientNoticeLocked(id, notice)
		s.setClientResumeLocked(id, s.phaseAnnouncementLinesLocked(id))
	}
}

func (s *server) phaseAnnouncementLinesLocked(selfID string) []string {
	presentation := s.phasePresentationLocked(selfID, s.waitingOnLocked())
	lines := []string{}
	lines = append(lines, s.phaseResumeLinesLocked(selfID)...)
	if presentation.ControlLabel != "" {
		lines = append(lines, "Control: "+presentation.ControlLabel)
	}
	if presentation.RoleNote != "" {
		lines = append(lines, presentation.RoleNote)
	}
	examples := presentation.Examples
	if len(examples) > 0 {
		lines = append(lines, "Suggested next: "+strings.Join(examples, " | "))
	} else {
		lines = append(lines, "Suggested next: wait for the next room update, or use `help`, `status`, or `log` locally.")
	}
	return compactStrings(lines)
}

func (s *server) consumeClientNoticeLocked(id string) string {
	client := s.clients[id]
	if client == nil || client.notice == "" {
		return ""
	}
	notice := client.notice
	client.notice = ""
	return notice
}

func (s *server) consumeClientResumeLocked(id string) []string {
	client := s.clients[id]
	if client == nil || len(client.resume) == 0 {
		return nil
	}
	lines := append([]string{}, client.resume...)
	client.resume = nil
	return lines
}

func (s *server) broadcastLocked() {
	if s.closed {
		return
	}
	_ = s.persistLocked()
	for _, id := range s.order {
		client := s.clients[id]
		if client == nil {
			continue
		}
		_ = client.enc.Encode(message{Type: "snapshot", Snapshot: s.snapshotLocked(id)})
	}
}
