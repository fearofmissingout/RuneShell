package netplay

import (
	"fmt"
	"strings"

	"cmdcards/internal/engine"
)

type phasePresentation struct {
	PhaseHint    string
	ControlLabel string
	RoleNote     string
	Commands     []string
	Examples     []string
}

type personalPhasePresentationConfig struct {
	Pending         bool
	ActiveControl   string
	WaitingControl  string
	ActiveRole      string
	WaitingRole     string
	ActiveHint      string
	WaitingHint     string
	ActiveCommands  []string
	WaitingCommands []string
	ActiveExamples  []string
	WaitingExamples []string
}

type flowPhasePresentationConfig struct {
	Owns            bool
	OwnerControl    string
	WaitingControl  string
	OwnerRole       string
	WaitingRole     string
	OwnerHint       string
	WaitingHint     string
	OwnerCommands   []string
	WaitingCommands []string
	OwnerExamples   []string
	WaitingExamples []string
}

func personalPhaseText(active bool, activeText string, waitingText string) string {
	if active {
		return activeText
	}
	return waitingText
}

func flowOwnerPhaseText(active bool, ownerText string, waitingText string) string {
	if active {
		return ownerText
	}
	return waitingText
}

func applyPersonalPhasePresentation(presentation *phasePresentation, config personalPhasePresentationConfig, appendHint func(string)) {
	presentation.ControlLabel = personalPhaseText(config.Pending, config.ActiveControl, config.WaitingControl)
	if presentation.RoleNote == "" {
		presentation.RoleNote = personalPhaseText(config.Pending, config.ActiveRole, config.WaitingRole)
	}
	presentation.Commands = append([]string{}, config.WaitingCommands...)
	presentation.Examples = append([]string{}, config.WaitingExamples...)
	if config.Pending {
		presentation.Commands = append([]string{}, config.ActiveCommands...)
		presentation.Examples = append([]string{}, config.ActiveExamples...)
	}
	appendHint(personalPhaseText(config.Pending, config.ActiveHint, config.WaitingHint))
}

func applyFlowPhasePresentation(presentation *phasePresentation, config flowPhasePresentationConfig, appendHint func(string)) {
	presentation.ControlLabel = flowOwnerPhaseText(config.Owns, config.OwnerControl, config.WaitingControl)
	if presentation.RoleNote == "" {
		presentation.RoleNote = flowOwnerPhaseText(config.Owns, config.OwnerRole, config.WaitingRole)
	}
	presentation.Commands = append([]string{}, config.WaitingCommands...)
	presentation.Examples = append([]string{}, config.WaitingExamples...)
	if config.Owns {
		presentation.Commands = append([]string{}, config.OwnerCommands...)
		presentation.Examples = append([]string{}, config.OwnerExamples...)
	}
	appendHint(flowOwnerPhaseText(config.Owns, config.OwnerHint, config.WaitingHint))
}

func (s *server) buildLobbyPhasePresentationLocked(player *roomPlayer, isHost bool, offline []string, waitingOn []string, presentation *phasePresentation, appendHint func(string)) {
	if isHost {
		presentation.ControlLabel = "Host room setup"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "You control room settings, can start the next run, and can clear offline reserved seats."
		}
		presentation.Commands = []string{"class <id>", "ready", "mode <story|endless>", "seed <n>", "start", "drop <seat|all>", "chat <text>", "abandon", "quit"}
	} else {
		presentation.ControlLabel = "Seat setup"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "Pick a class, toggle ready, and wait for the host to launch the room."
		}
		presentation.Commands = []string{"class <id>", "ready", "chat <text>", "quit"}
	}
	classID := ""
	if player != nil {
		classID = player.ClassID
	}
	if classID == "" {
		classes := classIDs(s.lib)
		if len(classes) > 0 {
			classID = classes[0]
		}
	}
	if classID != "" {
		presentation.Examples = append(presentation.Examples, fmt.Sprintf("class %s", classID))
	}
	presentation.Examples = append(presentation.Examples, "ready")
	if isHost {
		presentation.Examples = append(presentation.Examples, fmt.Sprintf("seed %d", s.seed), fmt.Sprintf("mode %s", s.mode))
		for seat, id := range s.order {
			offlinePlayer := s.players[id]
			if offlinePlayer == nil || offlinePlayer.Connected {
				continue
			}
			presentation.Examples = append(presentation.Examples, fmt.Sprintf("drop %d", seat+1))
			break
		}
		if len(offline) > 1 {
			presentation.Examples = append(presentation.Examples, "drop all")
		}
		if s.canStartRunLocked() {
			presentation.Examples = append(presentation.Examples, "start")
		}
	}
	presentation.Examples = append(presentation.Examples, "chat ready when you are")
	if len(waitingOn) > 0 {
		appendHint("Waiting for connected players to ready.")
	}
	if len(offline) > 0 {
		if isHost {
			appendHint("Offline seats block a new run. Ask them to reconnect with the same names or use `drop <seat>` in the lobby.")
		} else {
			appendHint("Offline seats must reconnect before the host can start a new run.")
		}
	} else if isHost {
		appendHint("Everyone online is ready. Start when you want.")
	} else {
		appendHint("Choose a class and toggle ready.")
	}
}

func (s *server) buildMapPhasePresentationLocked(selfID string, isHost bool, waitingOn []string, presentation *phasePresentation, appendHint func(string)) {
	presentation.ControlLabel = "Shared route vote"
	if presentation.RoleNote == "" {
		presentation.RoleNote = "Each connected seat votes on the next node. The room advances once all current seats have picked a route."
	}
	presentation.Commands = []string{"node <index>", "chat <text>", "quit"}
	if s.run == nil {
		presentation.Examples = []string{"chat waiting on route"}
	} else if reachable := engine.ReachableNodes(s.run); len(reachable) == 0 {
		presentation.Examples = []string{"chat route?"}
	} else {
		presentation.Examples = []string{"node 1", "chat taking node 1"}
	}
	if isHost {
		appendHint("Submit your route vote. The room advances once every connected seat has voted.")
	} else {
		appendHint("Submit your route vote and wait for the remaining connected seats.")
	}
	if state := s.seatStateLocked(selfID); state != nil && state.MapVote > 0 {
		appendHint("Your vote is in: " + s.mapVoteChoiceLabelLocked(state.MapVote) + ".")
		if len(waitingOn) > 0 {
			appendHint(fmt.Sprintf("Waiting for %d connected seat(s) to vote.", len(waitingOn)))
		}
		return
	}
	if len(waitingOn) > 1 {
		appendHint(fmt.Sprintf("%d connected seat(s) still need to vote.", len(waitingOn)))
	}
}

func (s *server) buildCombatPhasePresentationLocked(selfID string, offline []string, waitingOn []string, presentation *phasePresentation, appendHint func(string)) {
	presentation.ControlLabel = "Personal combat loadout"
	if presentation.RoleNote == "" {
		presentation.RoleNote = "Combat now uses seat-specific hands, piles, potions, and energy. The battlefield, enemies, and end-turn voting remain shared across the room."
	}
	presentation.Commands = []string{"play <card#> [enemy|ally <target#>]", "potion <slot#> [enemy|ally <target#>]", "end", "chat <text>", "quit"}
	if s.run == nil || s.combat == nil {
		presentation.Examples = []string{"chat ready to end", "end"}
	} else {
		view := buildCombatSnapshot(s.lib, s.run, s.combat, selfID, s.order, s.players)
		if view != nil && len(view.Hand) > 0 {
			presentation.Examples = append(presentation.Examples, combatCommandExample(view.Hand[0]))
		}
		if view != nil && len(view.Potions) > 0 {
			presentation.Examples = append(presentation.Examples, "potion 1")
		}
		presentation.Examples = append(presentation.Examples, "chat focus left enemy", "end")
	}
	if len(waitingOn) > 0 {
		appendHint("Play cards, then end turn. Enemy turn waits for every connected seat to vote.")
	} else {
		appendHint("All connected seats are ready to end the turn.")
	}
	if len(offline) > 0 {
		appendHint("Offline seats stay reserved and count as auto-ready for end-turn voting.")
	}
}

func (s *server) buildRewardPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	pending := s.seatRewardPendingLocked(selfID)
	config := personalPhasePresentationConfig{
		Pending:         pending,
		ActiveControl:   "Personal reward choice",
		WaitingControl:  "Waiting on other rewards",
		ActiveRole:      "You resolve your own post-combat reward. The room advances after every connected seat finishes.",
		WaitingRole:     "Your reward is done. Waiting for the other connected seats to finish theirs.",
		ActiveHint:      "Resolve your personal reward choice.",
		WaitingHint:     "Waiting for the remaining seats to finish their rewards.",
		ActiveCommands:  []string{"take <card#>", "skip", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		WaitingExamples: []string{"chat take the first card"},
	}
	if pending {
		if reward := s.seatRewardStateLocked(selfID); reward != nil && len(reward.CardChoices) > 0 {
			config.ActiveExamples = []string{"take 1", "skip", "chat taking card 1"}
		} else {
			config.ActiveExamples = []string{"skip", "chat skipping reward"}
		}
	}
	applyPersonalPhasePresentation(presentation, config, appendHint)
}

func (s *server) buildEventPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	pending := s.seatEventPendingLocked(selfID)
	config := personalPhasePresentationConfig{
		Pending:         pending,
		ActiveControl:   "Personal event choice",
		WaitingControl:  "Waiting on other events",
		ActiveRole:      "You choose your own event outcome. The shared map advances after every connected seat resolves their event.",
		WaitingRole:     "Your event is resolved. Waiting for the other connected seats to finish theirs.",
		ActiveHint:      "Resolve your personal event choice.",
		WaitingHint:     "Waiting for the remaining seats to finish their events.",
		ActiveCommands:  []string{"choose <index>", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		ActiveExamples:  []string{"choose 1", "chat choosing option 1"},
		WaitingExamples: []string{"chat choose 1 if you agree"},
	}
	if pending {
		if eventState := s.seatEventStateLocked(selfID); eventState == nil || len(eventState.Event.Choices) == 0 {
			config.ActiveExamples = []string{"chat choose 1 if you agree"}
		}
	}
	applyPersonalPhasePresentation(presentation, config, appendHint)
}

func (s *server) buildShopPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	pending := s.seatShopPendingLocked(selfID)
	config := personalPhasePresentationConfig{
		Pending:         pending,
		ActiveControl:   "Personal shop",
		WaitingControl:  "Waiting on other shops",
		ActiveRole:      "You have your own shop inventory and gold state. Leave when you are done so the shared map can advance.",
		WaitingRole:     "Your shop is done. Waiting for the other connected seats to leave theirs.",
		ActiveHint:      "Buy from your personal shop or leave when finished.",
		WaitingHint:     "Waiting for the remaining seats to leave their shops.",
		ActiveCommands:  []string{"buy <index>", "leave", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		WaitingExamples: []string{"chat want anything from shop?"},
	}
	if pending {
		if shop := s.seatShopStateLocked(selfID); shop != nil && len(shop.Offers) > 0 {
			config.ActiveExamples = []string{"buy 1", "leave", "chat buying offer 1"}
		} else {
			config.ActiveExamples = []string{"leave", "chat leaving shop"}
		}
	}
	applyPersonalPhasePresentation(presentation, config, appendHint)
}

func (s *server) buildRestPhasePresentationLocked(isHost bool, presentation *phasePresentation, appendHint func(string)) {
	presentation.ControlLabel = phaseHostWaitHint(isHost, "Shared campfire choice")
	if !isHost {
		presentation.ControlLabel = "Waiting for campfire choice"
	}
	if presentation.RoleNote == "" {
		presentation.RoleNote = phaseHostWaitHint(isHost, "You choose the campfire action for the room.")
		if !isHost {
			presentation.RoleNote = "You are waiting for the host to resolve the campfire."
		}
	}
	if isHost {
		presentation.Commands = []string{"heal", "upgrade", "chat <text>", "quit"}
		presentation.Examples = []string{"heal", "upgrade", "chat upgrading at campfire"}
	} else {
		presentation.Commands = []string{"chat <text>", "quit"}
		presentation.Examples = []string{"chat heal or upgrade?"}
	}
	appendHint(phaseHostWaitHint(isHost, "Host chooses heal or upgrade."))
}

func (s *server) buildEquipmentPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	applyFlowPhasePresentation(presentation, flowPhasePresentationConfig{
		Owns:            s.seatOwnsFlowLocked(selfID),
		OwnerControl:    "Equipment replacement",
		WaitingControl:  "Waiting on equipment choice",
		OwnerRole:       "You decide whether the offered equipment replaces the current slot item.",
		WaitingRole:     "Another seat is resolving an equipment replacement prompt.",
		OwnerHint:       "Confirm whether to equip the new item.",
		WaitingHint:     "Waiting for another seat to resolve an equipment choice.",
		OwnerCommands:   []string{"take", "skip", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		OwnerExamples:   []string{"take", "skip", "chat equipping this"},
		WaitingExamples: []string{"chat take or skip?"},
	}, appendHint)
}

func (s *server) buildDeckActionPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	ownerExamples := []string{"back", "chat backing out"}
	if len(s.deckActionIndexes) > 0 {
		ownerExamples = []string{"choose 1", "back", "chat picking card 1"}
	}
	applyFlowPhasePresentation(presentation, flowPhasePresentationConfig{
		Owns:            s.seatOwnsFlowLocked(selfID),
		OwnerControl:    "Deck action",
		WaitingControl:  "Waiting on deck action",
		OwnerRole:       "You choose the card affected by the current deck action.",
		WaitingRole:     "Another seat is resolving a deck action.",
		OwnerHint:       "Choose the card for the current deck action.",
		WaitingHint:     "Waiting for another seat to resolve a deck action.",
		OwnerCommands:   []string{"choose <index>", "back", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		OwnerExamples:   ownerExamples,
		WaitingExamples: []string{"chat choose 1?"},
	}, appendHint)
}

func (s *server) buildSummaryPhasePresentationLocked(isHost bool, presentation *phasePresentation, appendHint func(string)) {
	if isHost {
		presentation.ControlLabel = "Host summary decision"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "You can start the next run or abandon the room save from here."
		}
		presentation.Commands = []string{"new", "abandon", "chat <text>", "quit"}
		presentation.Examples = []string{"new", "abandon", "chat reset?"}
	} else {
		presentation.ControlLabel = "Waiting for summary decision"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "You are waiting for the host to continue or close the room."
		}
		presentation.Commands = []string{"chat <text>", "quit"}
		presentation.Examples = []string{"chat gg"}
	}
	appendHint(phaseHostWaitHint(isHost, "Host can start a new run or abandon the room."))
}

func (s *server) phasePresentationLocked(selfID string, waitingOn []string) phasePresentation {
	isHost := selfID == s.hostID
	offline := s.offlineSeatSummariesLocked()
	presentation := phasePresentation{}

	if note := s.transferNoteLocked(selfID); note != "" {
		presentation.PhaseHint = note
	}
	if s.hostTransfer != nil {
		switch selfID {
		case s.hostTransfer.FromID:
			presentation.RoleNote = "Host transfer is pending. Wait for the requested seat to accept, or use cancel-host."
			presentation.Examples = []string{"chat host transfer pending", "cancel-host"}
		case s.hostTransfer.ToID:
			presentation.RoleNote = "You have a pending host transfer request. Accept to take room control, or deny to keep the current host."
			presentation.Examples = []string{"accept-host", "deny-host", "chat taking host"}
		default:
			presentation.Examples = []string{"chat waiting on host transfer"}
		}
	}

	appendHint := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		if presentation.PhaseHint == "" {
			presentation.PhaseHint = text
			return
		}
		presentation.PhaseHint += " " + text
	}

	if s.restoredFromSave && len(offline) > 0 {
		if isHost {
			appendHint("Recovered room loaded from disk. Wait for offline seats to reconnect with the same names, or keep playing with reserved seats offline.")
		} else {
			appendHint("This room was restored from disk. Offline seats can reclaim their spots by reconnecting with the same names.")
		}
	}

	player := s.players[selfID]
	switch s.phase {
	case phaseLobby:
		s.buildLobbyPhasePresentationLocked(player, isHost, offline, waitingOn, &presentation, appendHint)
	case phaseMap:
		s.buildMapPhasePresentationLocked(selfID, isHost, waitingOn, &presentation, appendHint)
		presentation.ControlLabel = personalPhaseText(s.seatMapVoteSubmittedLocked(selfID), "Route vote submitted", "Shared route vote")
		if s.seatMapVoteSubmittedLocked(selfID) {
			if state := s.seatStateLocked(selfID); state != nil && state.MapVote > 0 {
				appendHint("Your route vote is in: " + s.mapVoteChoiceLabelLocked(state.MapVote) + ".")
			}
			appendHint("Waiting for the remaining connected seats.")
		}
	case phaseCombat:
		s.buildCombatPhasePresentationLocked(selfID, offline, waitingOn, &presentation, appendHint)
	case phaseReward:
		s.buildRewardPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseEvent:
		s.buildEventPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseShop:
		s.buildShopPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseRest:
		s.buildRestPhasePresentationLocked(isHost, &presentation, appendHint)
	case phaseEquipment:
		s.buildEquipmentPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseDeckAction:
		s.buildDeckActionPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseSummary:
		s.buildSummaryPhasePresentationLocked(isHost, &presentation, appendHint)
	default:
		presentation.Commands = []string{"chat <text>", "quit"}
	}

	presentation.Commands = s.appendTransferCommandsLocked(selfID, presentation.Commands)
	if s.hostTransfer == nil {
		presentation.Examples = compactStrings(s.appendTransferExamplesLocked(selfID, presentation.Examples))
	}
	return presentation
}

func (s *server) phaseResumeLinesLocked(selfID string) []string {
	switch s.phase {
	case phaseLobby:
		return []string{fmt.Sprintf("Mode %s, seed %d.", s.mode, s.seed)}
	case phaseMap:
		if s.run != nil {
			return []string{fmt.Sprintf("Act %d, next floor %d, gold %d.", s.run.Act, s.run.CurrentFloor+1, s.seatGoldLocked(selfID))}
		}
	case phaseCombat:
		if s.combat != nil {
			energy := s.combat.Player.Energy
			maxEnergy := s.combat.Player.MaxEnergy
			if seatIndex := s.playerSeatIndexLocked(selfID); seatIndex >= 0 {
				if actor := engine.ActorForSeat(s.combat, seatIndex); actor != nil {
					energy = actor.Energy
					maxEnergy = actor.MaxEnergy
				}
			}
			return []string{fmt.Sprintf("Combat turn %d, energy %d/%d.", s.combat.Turn, energy, maxEnergy)}
		}
	case phaseReward:
		return []string{"Reward choice is still pending."}
	case phaseEvent:
		return []string{"Event choice is still pending."}
	case phaseShop:
		if s.run != nil {
			return []string{fmt.Sprintf("Shop is open with %d gold available.", s.seatGoldLocked(selfID))}
		}
	case phaseRest:
		return []string{"Campfire action is still pending."}
	case phaseEquipment:
		return []string{"Equipment replacement prompt is still pending."}
	case phaseDeckAction:
		return []string{"Deck action prompt is still pending."}
	case phaseSummary:
		return []string{"Run summary is waiting for the host's next decision."}
	}
	return nil
}

func (s *server) commandHintsLocked(selfID string) []string {
	return s.phasePresentationLocked(selfID, s.waitingOnLocked()).Commands
}

func (s *server) roleNoteLocked(selfID string) string {
	return s.phasePresentationLocked(selfID, s.waitingOnLocked()).RoleNote
}

func (s *server) controlLabelLocked(selfID string) string {
	return s.phasePresentationLocked(selfID, s.waitingOnLocked()).ControlLabel
}

func (s *server) exampleCommandsLocked(selfID string) []string {
	return s.phasePresentationLocked(selfID, s.waitingOnLocked()).Examples
}

func (s *server) phaseHintLocked(selfID string, waitingOn []string) string {
	return s.phasePresentationLocked(selfID, waitingOn).PhaseHint
}

func phaseHostWaitHint(isHost bool, hostText string) string {
	if isHost {
		return hostText
	}
	return "Waiting for the host."
}
