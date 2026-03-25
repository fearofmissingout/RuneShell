package netplay

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
)

const roomStateVersion = 1

type persistedRoomState struct {
	Version            int                         `json:"version"`
	HostID             string                      `json:"host_id"`
	NextID             int                         `json:"next_id"`
	Order              []string                    `json:"order"`
	Players            map[string]roomPlayer       `json:"players"`
	Phase              string                      `json:"phase"`
	Mode               engine.GameMode             `json:"mode"`
	Seed               int64                       `json:"seed"`
	ChatLog            []string                    `json:"chat_log,omitempty"`
	RoomLog            []string                    `json:"room_log"`
	HostTransfer       *hostTransferRequest        `json:"host_transfer,omitempty"`
	Run                *engine.RunState            `json:"run,omitempty"`
	PartyMembers       []engine.PlayerState        `json:"party_members,omitempty"`
	CurrentNode        engine.Node                 `json:"current_node,omitempty"`
	Combat             *engine.CombatState         `json:"combat,omitempty"`
	Reward             *engine.RewardState         `json:"reward,omitempty"`
	EventState         *engine.EventState          `json:"event_state,omitempty"`
	ShopState          *engine.ShopState           `json:"shop_state,omitempty"`
	RestLog            []string                    `json:"rest_log,omitempty"`
	EquipOffer         *engine.EquipmentOfferState `json:"equip_offer,omitempty"`
	EquipFrom          string                      `json:"equip_from,omitempty"`
	RewardCard         string                      `json:"reward_card,omitempty"`
	ShopOfferID        string                      `json:"shop_offer_id,omitempty"`
	EventChoice        string                      `json:"event_choice,omitempty"`
	DeckActionMode     string                      `json:"deck_action_mode,omitempty"`
	DeckActionTitle    string                      `json:"deck_action_title,omitempty"`
	DeckActionSubtitle string                      `json:"deck_action_subtitle,omitempty"`
	DeckActionIndexes  []int                       `json:"deck_action_indexes,omitempty"`
	DeckActionPrice    int                         `json:"deck_action_price,omitempty"`
}

func roomSavePath(baseDir string) string {
	return filepath.Join(baseDir, "netplay-room.json")
}

func defaultRoomSavePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return roomSavePath(filepath.Join(dir, "cmdcards")), nil
}

func newServerWithSavePath(lib *content.Library, addr, savePath string) (*server, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &server{
		lib:      lib,
		roomAddr: listener.Addr().String(),
		listener: listener,
		savePath: savePath,
		players:  map[string]*roomPlayer{},
		clients:  map[string]*clientConn{},
		phase:    phaseLobby,
		mode:     engine.ModeStory,
		seed:     time.Now().UnixNano(),
		chatLog:  []string{},
		roomLog: []string{
			formatChannelEntry("System", "", "Room created. Players can use `class <id>` and `ready`."),
			formatChannelEntry("System", "", "Host can use `mode story|endless`, `seed <n>`, and `start`."),
		},
	}, nil
}

func loadServerFromSavePath(lib *content.Library, addr, savePath string) (*server, bool, error) {
	if savePath == "" {
		srv, err := newServerWithSavePath(lib, addr, "")
		return srv, false, err
	}
	data, err := os.ReadFile(savePath)
	if errors.Is(err, os.ErrNotExist) {
		srv, err := newServerWithSavePath(lib, addr, savePath)
		return srv, false, err
	}
	if err != nil {
		return nil, false, err
	}

	var state persistedRoomState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, err
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, false, err
	}

	srv := &server{
		lib:                lib,
		roomAddr:           listener.Addr().String(),
		listener:           listener,
		savePath:           savePath,
		nextID:             state.NextID,
		hostID:             state.HostID,
		order:              append([]string{}, state.Order...),
		players:            map[string]*roomPlayer{},
		clients:            map[string]*clientConn{},
		phase:              state.Phase,
		mode:               state.Mode,
		seed:               state.Seed,
		chatLog:            append([]string{}, state.ChatLog...),
		roomLog:            append([]string{}, state.RoomLog...),
		hostTransfer:       state.HostTransfer,
		run:                state.Run,
		partyMembers:       clonePartyStates(state.PartyMembers),
		currentNode:        state.CurrentNode,
		combat:             state.Combat,
		reward:             state.Reward,
		eventState:         state.EventState,
		shopState:          state.ShopState,
		restLog:            append([]string{}, state.RestLog...),
		equipOffer:         state.EquipOffer,
		equipFrom:          state.EquipFrom,
		rewardCard:         state.RewardCard,
		shopOfferID:        state.ShopOfferID,
		eventChoice:        state.EventChoice,
		deckActionMode:     state.DeckActionMode,
		deckActionTitle:    state.DeckActionTitle,
		deckActionSubtitle: state.DeckActionSubtitle,
		deckActionIndexes:  append([]int{}, state.DeckActionIndexes...),
		deckActionPrice:    state.DeckActionPrice,
	}
	for id, player := range state.Players {
		player.Connected = false
		copied := player
		srv.players[id] = &copied
	}
	return srv, true, nil
}

func (s *server) persistLocked() error {
	if s.savePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.savePath), 0o755); err != nil {
		return err
	}

	players := map[string]roomPlayer{}
	for id, player := range s.players {
		if player == nil {
			continue
		}
		copied := *player
		players[id] = copied
	}
	state := persistedRoomState{
		Version:            roomStateVersion,
		HostID:             s.hostID,
		NextID:             s.nextID,
		Order:              append([]string{}, s.order...),
		Players:            players,
		Phase:              s.phase,
		Mode:               s.mode,
		Seed:               s.seed,
		ChatLog:            append([]string{}, s.chatLog...),
		RoomLog:            append([]string{}, s.roomLog...),
		HostTransfer:       s.hostTransfer,
		Run:                s.run,
		PartyMembers:       clonePartyStates(s.partyMembers),
		CurrentNode:        s.currentNode,
		Combat:             s.combat,
		Reward:             s.reward,
		EventState:         s.eventState,
		ShopState:          s.shopState,
		RestLog:            append([]string{}, s.restLog...),
		EquipOffer:         s.equipOffer,
		EquipFrom:          s.equipFrom,
		RewardCard:         s.rewardCard,
		ShopOfferID:        s.shopOfferID,
		EventChoice:        s.eventChoice,
		DeckActionMode:     s.deckActionMode,
		DeckActionTitle:    s.deckActionTitle,
		DeckActionSubtitle: s.deckActionSubtitle,
		DeckActionIndexes:  append([]int{}, s.deckActionIndexes...),
		DeckActionPrice:    s.deckActionPrice,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.savePath, data, 0o644)
}

func clearSavedRoom(savePath string) error {
	if savePath == "" {
		return nil
	}
	err := os.Remove(savePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *server) reconnectCandidateLocked(name string) (string, *roomPlayer, bool) {
	for _, id := range s.order {
		player := s.players[id]
		if player == nil {
			continue
		}
		if strings.EqualFold(player.Name, name) {
			return id, player, true
		}
	}
	return "", nil, false
}
