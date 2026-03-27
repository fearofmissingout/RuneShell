package netplay

import (
	"fmt"
	"math/rand"

	"cmdcards/internal/engine"
)

func (s *server) seatStateLocked(playerID string) *seatRunState {
	if s.seatStates == nil {
		s.seatStates = map[string]*seatRunState{}
	}
	state := s.seatStates[playerID]
	if state == nil {
		state = &seatRunState{}
		s.seatStates[playerID] = state
	}
	return state
}

func (s *server) seatRunLocked(playerID string) *engine.RunState {
	state := s.seatStateLocked(playerID)
	return state.Run
}

func (s *server) seatContextLocked(playerID string) (*seatRunState, *engine.RunState) {
	state := s.seatStateLocked(playerID)
	if state == nil {
		return nil, nil
	}
	return state, state.Run
}

func (s *server) seatGoldLocked(playerID string) int {
	run := s.seatRunLocked(playerID)
	if run == nil {
		return 0
	}
	return run.Player.Gold
}

func (s *server) seatHistoryLocked(playerID string, limit int) []string {
	run := s.seatRunLocked(playerID)
	if run == nil {
		return nil
	}
	return tailStrings(run.History, limit)
}

func (s *server) seatRewardPendingLocked(playerID string) bool {
	state := s.seatStateLocked(playerID)
	return state != nil && state.Reward != nil && !state.RewardDone
}

func (s *server) seatRewardStateLocked(playerID string) *engine.RewardState {
	state := s.seatStateLocked(playerID)
	if state == nil {
		return nil
	}
	return state.Reward
}

func (s *server) seatEventPendingLocked(playerID string) bool {
	state := s.seatStateLocked(playerID)
	return state != nil && state.Event != nil && !state.EventDone
}

func (s *server) seatEventStateLocked(playerID string) *engine.EventState {
	state := s.seatStateLocked(playerID)
	if state == nil {
		return nil
	}
	return state.Event
}

func (s *server) seatShopPendingLocked(playerID string) bool {
	state := s.seatStateLocked(playerID)
	return state != nil && state.Shop != nil && !state.ShopDone
}

func (s *server) seatShopStateLocked(playerID string) *engine.ShopState {
	state := s.seatStateLocked(playerID)
	if state == nil {
		return nil
	}
	return state.Shop
}

func (s *server) seatMapVoteSubmittedLocked(playerID string) bool {
	state := s.seatStateLocked(playerID)
	return state != nil && state.MapVote > 0
}

func (s *server) seatOwnsFlowLocked(playerID string) bool {
	return playerID != "" && playerID == s.flowOwner
}

func (s *server) connectedSeatIDsLocked() []string {
	ids := make([]string, 0, len(s.order))
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func (s *server) connectedSeatRunsLocked() []engine.PlayerState {
	runs := make([]engine.PlayerState, 0, len(s.order))
	for _, id := range s.connectedSeatIDsLocked() {
		run := s.seatRunLocked(id)
		if run == nil {
			continue
		}
		runs = append(runs, run.Player)
	}
	return runs
}

func (s *server) seatSnapshotsLocked() []actorSnapshot {
	actors := make([]actorSnapshot, 0, len(s.order))
	for seat, id := range s.order {
		player := s.players[id]
		if player == nil {
			continue
		}
		run := s.seatRunLocked(id)
		actor := actorSnapshot{Index: seat + 1, Name: player.Name}
		if run != nil {
			actor.HP = run.Player.HP
			actor.MaxHP = run.Player.MaxHP
		}
		actors = append(actors, actor)
	}
	return actors
}

func (s *server) allConnectedSeatsDoneLocked(done func(*seatRunState) bool) bool {
	any := false
	for _, id := range s.connectedSeatIDsLocked() {
		state := s.seatStateLocked(id)
		any = true
		if !done(state) {
			return false
		}
	}
	return any
}

func (s *server) weightedSeatVoteLocked() int {
	votes := []int{}
	for _, id := range s.connectedSeatIDsLocked() {
		vote := s.seatStateLocked(id).MapVote
		if vote > 0 {
			votes = append(votes, vote)
		}
	}
	if len(votes) == 0 {
		return 0
	}
	return votes[rand.New(rand.NewSource(s.seed+int64(s.currentNode.Floor*97+len(votes)*13+len(s.roomLog)))).Intn(len(votes))]
}

func (s *server) clearMapVotesLocked() {
	for _, id := range s.order {
		if state := s.seatStates[id]; state != nil {
			state.MapVote = 0
		}
	}
}

func (s *server) advanceSharedRunLocked() error {
	if s.run == nil {
		return nil
	}
	return engine.AdvanceNonCombatNode(s.run, s.currentNode)
}

func (s *server) buildPartyStatesLocked() ([]engine.PlayerState, error) {
	players := make([]engine.PlayerState, 0, len(s.order))
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		state, err := buildBasePlayerState(s.lib, player.ClassID, player.Name)
		if err != nil {
			return nil, err
		}
		players = append(players, state)
	}
	return players, nil
}

func (s *server) combatPartyLocked() []engine.PlayerState {
	party := s.seatPartyLocked(false)
	if len(party) == 0 && s.run != nil {
		leader := s.run.Player
		party = append(party, leader)
	}
	return party
}

func (s *server) seatPartyLocked(includeDisconnected bool) []engine.PlayerState {
	party := make([]engine.PlayerState, 0, len(s.order))
	for _, id := range s.order {
		if !includeDisconnected {
			player := s.players[id]
			if player == nil || !player.Connected {
				continue
			}
		}
		run := s.seatRunLocked(id)
		if run == nil {
			continue
		}
		party = append(party, run.Player)
	}
	return party
}

func (s *server) rebuildSharedRunPlayerLocked() {
	if s.run == nil {
		return
	}
	players := s.seatPartyLocked(true)
	if len(players) == 0 {
		return
	}
	s.run.Player = buildSharedLeader(players)
}

func (s *server) syncPartyFromCombatLocked() {
	if s.combat == nil {
		return
	}
	ids := s.connectedSeatIDsLocked()
	if len(ids) == 0 {
		return
	}
	for seatIndex, id := range ids {
		run := s.seatRunLocked(id)
		actor := engine.ActorForSeat(s.combat, seatIndex)
		if run == nil || actor == nil {
			continue
		}
		run.Player.HP = actor.HP
		run.Player.MaxHP = actor.MaxHP
		if seat := engine.CombatSeatView(s.combat, seatIndex); seat != nil {
			run.Player.Potions = append([]string{}, seat.Potions...)
		}
		if seatIndex < len(s.combat.SeatPlayers) {
			run.Player.Relics = append([]string{}, s.combat.SeatPlayers[seatIndex].Relics...)
			run.Player.Equipment = s.combat.SeatPlayers[seatIndex].Equipment
			run.Player.Deck = append([]engine.DeckCard{}, s.combat.SeatPlayers[seatIndex].Deck...)
			if stats := s.combat.SeatPlayers[seatIndex].PermanentStats; stats != nil {
				run.Player.PermanentStats = map[string]int{}
				for key, value := range stats {
					run.Player.PermanentStats[key] = value
				}
			} else {
				run.Player.PermanentStats = nil
			}
		}
	}
	s.rebuildSharedRunPlayerLocked()
}

func (s *server) connectedPlayerIDsLocked() []string {
	ids := []string{}
	for _, id := range s.order {
		player := s.players[id]
		if player != nil && player.Connected {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *server) playerSeatIndexLocked(playerID string) int {
	for index, id := range s.order {
		if id == playerID {
			return index
		}
	}
	return -1
}

func (s *server) allReadyLocked() bool {
	connected := 0
	for _, id := range s.order {
		player := s.players[id]
		if player == nil || !player.Connected {
			continue
		}
		connected++
		if !player.Ready {
			return false
		}
	}
	return connected > 0
}

func (s *server) canStartRunLocked() bool {
	if !s.allReadyLocked() {
		return false
	}
	return len(s.offlineSeatSummariesLocked()) == 0
}

func (s *server) dropOfflineSeatLocked(seat int) error {
	if seat <= 0 || seat > len(s.order) {
		return fmt.Errorf("invalid seat %d", seat)
	}
	id := s.order[seat-1]
	if id == s.hostID {
		return fmt.Errorf("cannot drop the host seat")
	}
	player := s.players[id]
	if player == nil {
		return fmt.Errorf("seat %d is empty", seat)
	}
	if player.Connected {
		return fmt.Errorf("seat %d is still connected", seat)
	}
	delete(s.players, id)
	delete(s.clients, id)
	s.order = append(s.order[:seat-1], s.order[seat:]...)
	s.appendRoomLogLocked(fmt.Sprintf("Dropped offline seat %d (%s).", seat, player.Name))
	if len(s.offlineSeatSummariesLocked()) == 0 {
		s.restoredFromSave = false
	}
	return nil
}

func (s *server) dropAllOfflineSeatsLocked() int {
	dropped := 0
	for seat := len(s.order); seat >= 1; seat-- {
		id := s.order[seat-1]
		if id == s.hostID {
			continue
		}
		player := s.players[id]
		if player == nil || player.Connected {
			continue
		}
		if err := s.dropOfflineSeatLocked(seat); err == nil {
			dropped++
		}
	}
	return dropped
}
