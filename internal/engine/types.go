package engine

import (
	"fmt"
	"strings"
	"time"

	"cmdcards/internal/content"
)

type GameMode string

const (
	ModeStory   GameMode = "story"
	ModeEndless GameMode = "endless"
)

type RunStatus string

const (
	RunStatusActive RunStatus = "active"
	RunStatusWon    RunStatus = "won"
	RunStatusLost   RunStatus = "lost"
)

type NodeKind string

const (
	NodeMonster NodeKind = "monster"
	NodeEvent   NodeKind = "event"
	NodeShop    NodeKind = "shop"
	NodeElite   NodeKind = "elite"
	NodeRest    NodeKind = "rest"
	NodeBoss    NodeKind = "boss"
)

type Profile struct {
	Version            int                       `json:"version"`
	LastUpdated        time.Time                 `json:"last_updated"`
	Language           string                    `json:"language,omitempty"`
	UnlockedClasses    []string                  `json:"unlocked_classes"`
	MetaCurrency       int                       `json:"meta_currency"`
	Perks              map[string]int            `json:"perks"`
	ContentUnlocks     map[string][]string       `json:"content_unlocks"`
	UnlockedEquipments []string                  `json:"unlocked_equipments,omitempty"`
	ClassLoadouts      map[string]EquipmentSlots `json:"class_loadouts,omitempty"`
}

type EquipmentSlots struct {
	Weapon    string `json:"weapon"`
	Armor     string `json:"armor"`
	Accessory string `json:"accessory"`
}

type DeckCard struct {
	CardID   string        `json:"card_id"`
	Upgraded bool          `json:"upgraded"`
	Augments []CardAugment `json:"augments,omitempty"`
}

type PlayerState struct {
	ClassID        string         `json:"class_id"`
	Name           string         `json:"name"`
	MaxHP          int            `json:"max_hp"`
	HP             int            `json:"hp"`
	MaxEnergy      int            `json:"max_energy"`
	Gold           int            `json:"gold"`
	Deck           []DeckCard     `json:"deck"`
	Relics         []string       `json:"relics"`
	Potions        []string       `json:"potions"`
	PotionCapacity int            `json:"potion_capacity"`
	Equipment      EquipmentSlots `json:"equipment"`
	PermanentStats map[string]int `json:"permanent_stats"`
}

type Node struct {
	ID          string   `json:"id"`
	Act         int      `json:"act"`
	Floor       int      `json:"floor"`
	Index       int      `json:"index"`
	Kind        NodeKind `json:"kind"`
	EncounterID string   `json:"encounter_id,omitempty"`
	EventID     string   `json:"event_id,omitempty"`
	Edges       []string `json:"edges"`
}

func (n Node) Label() string {
	return fmt.Sprintf("第%d层 %s", n.Floor, NodeKindName(n.Kind))
}

type MapGraph struct {
	Act    int      `json:"act"`
	Floors [][]Node `json:"floors"`
}

type CombatMetrics struct {
	DamageDealt    int `json:"damage_dealt,omitempty"`
	StatusApplied  int `json:"status_applied,omitempty"`
	StatusReceived int `json:"status_received,omitempty"`
	DamageBlocked  int `json:"damage_blocked,omitempty"`
	DamageTaken    int `json:"damage_taken,omitempty"`
}

type RunStats struct {
	CombatsWon    int           `json:"combats_won"`
	ElitesWon     int           `json:"elites_won"`
	BossesWon     int           `json:"bosses_won"`
	ClearedFloors int           `json:"cleared_floors"`
	CombatTurns   int           `json:"combat_turns,omitempty"`
	Metrics       CombatMetrics `json:"metrics,omitempty"`
}

type RunCheckpoint struct {
	Screen               string               `json:"screen,omitempty"`
	CurrentNode          *Node                `json:"current_node,omitempty"`
	Combat               *CombatState         `json:"combat,omitempty"`
	Reward               *RewardState         `json:"reward,omitempty"`
	EquipOffer           *EquipmentOfferState `json:"equip_offer,omitempty"`
	EventState           *EventState          `json:"event_state,omitempty"`
	ShopState            *ShopState           `json:"shop_state,omitempty"`
	RestLog              []string             `json:"rest_log,omitempty"`
	EquipFrom            string               `json:"equip_from,omitempty"`
	RewardCard           string               `json:"reward_card,omitempty"`
	ShopOfferID          string               `json:"shop_offer_id,omitempty"`
	EventChoice          string               `json:"event_choice,omitempty"`
	DeckActionMode       string               `json:"deck_action_mode,omitempty"`
	DeckActionTitle      string               `json:"deck_action_title,omitempty"`
	DeckActionSubtitle   string               `json:"deck_action_subtitle,omitempty"`
	DeckActionIndexes    []int                `json:"deck_action_indexes,omitempty"`
	DeckActionPrice      int                  `json:"deck_action_price,omitempty"`
	DeckActionEffect     *content.Effect      `json:"deck_action_effect,omitempty"`
	DeckActionTakeEquip  bool                 `json:"deck_action_take_equip,omitempty"`
	CombatPane           int                  `json:"combat_pane,omitempty"`
	CombatTopPage        int                  `json:"combat_top_page,omitempty"`
	CombatLogPage        int                  `json:"combat_log_page,omitempty"`
	CombatPotionIndex    int                  `json:"combat_potion_index,omitempty"`
	CombatPotionMode     bool                 `json:"combat_potion_mode,omitempty"`
	CombatTarget         CombatTarget         `json:"combat_target,omitempty"`
	PendingPotionID      string               `json:"pending_potion_id,omitempty"`
	PendingPotionResume  string               `json:"pending_potion_resume,omitempty"`
	PendingPotionAdvance bool                 `json:"pending_potion_advance,omitempty"`
}

type RunState struct {
	Version      int            `json:"version"`
	Mode         GameMode       `json:"mode"`
	Seed         int64          `json:"seed"`
	Act          int            `json:"act"`
	CurrentFloor int            `json:"current_floor"`
	PartySize    int            `json:"party_size,omitempty"`
	Status       RunStatus      `json:"status"`
	Player       PlayerState    `json:"player"`
	Map          MapGraph       `json:"map"`
	Reachable    []string       `json:"reachable"`
	History      []string       `json:"history"`
	Stats        RunStats       `json:"stats"`
	Checkpoint   *RunCheckpoint `json:"checkpoint,omitempty"`
}

type Status struct {
	Name     string
	Stacks   int
	Duration int
	Fresh    bool
}

type RuntimeCard struct {
	ID       string
	Upgraded bool
	Augments []CardAugment
}

type CombatActor struct {
	Name      string
	HP        int
	MaxHP     int
	Block     int
	Energy    int
	MaxEnergy int
	Statuses  map[string]Status
}

type CombatCoopState struct {
	Enabled       bool
	SharedTurnEnd bool
	EndTurnVotes  []bool
	ActedSeats    []int
	TeamComboDone bool
}

type CombatTargetKind string

const (
	CombatTargetNone    CombatTargetKind = "none"
	CombatTargetAlly    CombatTargetKind = "ally"
	CombatTargetEnemy   CombatTargetKind = "enemy"
	CombatTargetAllies  CombatTargetKind = "all_allies"
	CombatTargetEnemies CombatTargetKind = "all_enemies"
)

type CombatTarget struct {
	Kind  CombatTargetKind `json:"kind"`
	Index int              `json:"index,omitempty"`
}

type CombatEnemy struct {
	CombatActor
	ID            string
	EncounterID   string
	Slot          int
	Passives      []content.Effect
	IntentIndex   int
	CurrentIntent content.EnemyIntentDef
	IntentCycle   []content.EnemyIntentDef
}

type CombatLogEntry struct {
	Turn int
	Text string
}

type PendingCardRepeat struct {
	Count int    `json:"count"`
	Tag   string `json:"tag,omitempty"`
}

type CombatSeatState struct {
	DrawPile           []RuntimeCard       `json:"draw_pile,omitempty"`
	Discard            []RuntimeCard       `json:"discard,omitempty"`
	Exhaust            []RuntimeCard       `json:"exhaust,omitempty"`
	Hand               []RuntimeCard       `json:"hand,omitempty"`
	Potions            []string            `json:"potions,omitempty"`
	PotionsUsed        []string            `json:"potions_used,omitempty"`
	NextCardRepeats    int                 `json:"next_card_repeats,omitempty"`
	PendingCardRepeats []PendingCardRepeat `json:"pending_card_repeats,omitempty"`
	Metrics            CombatMetrics       `json:"metrics,omitempty"`
}

type RewardState struct {
	Gold           int
	CardChoices    []content.CardDef
	PotionID       string
	RelicID        string
	EquipmentID    string
	SourceNodeKind NodeKind
}

type DeckActionPlan struct {
	Mode          string
	Title         string
	Subtitle      string
	Indexes       []int
	Price         int
	Effect        *content.Effect
	TakeEquipment bool
}

type EquipmentOfferState struct {
	Source             string `json:"source,omitempty"`
	EquipmentID        string `json:"equipment_id"`
	Slot               string `json:"slot"`
	CurrentEquipmentID string `json:"current_equipment_id,omitempty"`
	Price              int    `json:"price,omitempty"`
	CandidateScore     int    `json:"candidate_score,omitempty"`
	CurrentScore       int    `json:"current_score,omitempty"`
}

type CombatState struct {
	Player      CombatActor
	Allies      []CombatActor
	SeatPlayers []PlayerState
	Enemy       CombatEnemy
	Enemies     []CombatEnemy
	Encounter   content.EncounterDef
	RewardBasis content.EncounterDef
	DrawPile    []RuntimeCard
	Discard     []RuntimeCard
	Exhaust     []RuntimeCard
	Hand        []RuntimeCard
	Turn        int
	Won         bool
	Lost        bool
	Log         []CombatLogEntry
	Reward      RewardState
	PotionsUsed []string
	Seats       []CombatSeatState
	Coop        CombatCoopState
}

type ShopOffer struct {
	ID          string
	Kind        string
	Name        string
	Description string
	Price       int
	CardID      string
	ItemID      string
}

type ShopState struct {
	Offers []ShopOffer
	Log    []string
}

type EventState struct {
	Event content.EventDef
	Log   []string
	Done  bool
}

type RestState struct {
	Log []string
}

type SmokeResult struct {
	Mode          GameMode
	ClassID       string
	Seed          int64
	Result        RunStatus
	ReachedAct    int
	ClearedFloors int
	FinalHP       int
	FinalGold     int
	FinalDeckSize int
	CombatsWon    int
	Log           []string
}

func DefaultProfile(lib *content.Library) Profile {
	profile := Profile{
		Version:        CurrentProfileVersion,
		LastUpdated:    time.Now(),
		MetaCurrency:   0,
		Perks:          map[string]int{},
		ContentUnlocks: map[string][]string{},
	}
	NormalizeProfile(lib, &profile)
	return profile
}

func NodeKindName(kind NodeKind) string {
	switch kind {
	case NodeMonster:
		return "怪物"
	case NodeEvent:
		return "事件"
	case NodeShop:
		return "商店"
	case NodeElite:
		return "精英"
	case NodeRest:
		return "篝火"
	case NodeBoss:
		return "Boss"
	default:
		return strings.ToUpper(string(kind))
	}
}
