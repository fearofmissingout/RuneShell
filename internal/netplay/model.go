package netplay

import (
	"encoding/json"
	"net"
	"sync"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
)

type message struct {
	Type     string          `json:"type"`
	Hello    *helloPayload   `json:"hello,omitempty"`
	Command  *commandPayload `json:"command,omitempty"`
	Snapshot *roomSnapshot   `json:"snapshot,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type helloPayload struct {
	Name     string `json:"name"`
	ClassID  string `json:"class_id"`
	Language string `json:"language,omitempty"`
}

type commandPayload struct {
	Action      string `json:"action"`
	Value       string `json:"value,omitempty"`
	Seat        int    `json:"seat,omitempty"`
	CardIndex   int    `json:"card_index,omitempty"`
	ItemIndex   int    `json:"item_index,omitempty"`
	TargetKind  string `json:"target_kind,omitempty"`
	TargetIndex int    `json:"target_index,omitempty"`
}

type roomPlayer struct {
	ID        string `json:"id"`
	Seat      int    `json:"seat,omitempty"`
	Name      string `json:"name"`
	ClassID   string `json:"class_id"`
	Language  string `json:"language,omitempty"`
	Ready     bool   `json:"ready"`
	Connected bool   `json:"connected"`
}

type seatRunState struct {
	Run        *engine.RunState    `json:"run,omitempty"`
	Reward     *engine.RewardState `json:"reward,omitempty"`
	RewardDone bool                `json:"reward_done,omitempty"`
	Event      *engine.EventState  `json:"event,omitempty"`
	EventDone  bool                `json:"event_done,omitempty"`
	Shop       *engine.ShopState   `json:"shop,omitempty"`
	ShopDone   bool                `json:"shop_done,omitempty"`
	MapVote    int                 `json:"map_vote,omitempty"`
	RestLog    []string            `json:"rest_log,omitempty"`
}

type actorSnapshot struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	ClassID   string `json:"class_id,omitempty"`
	HP        int    `json:"hp"`
	MaxHP     int    `json:"max_hp"`
	Energy    int    `json:"energy,omitempty"`
	MaxEnergy int    `json:"max_energy,omitempty"`
	Block     int    `json:"block"`
	Status    string `json:"status"`
}

type enemySnapshot struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	HP     int    `json:"hp"`
	MaxHP  int    `json:"max_hp"`
	Block  int    `json:"block"`
	Status string `json:"status"`
	Intent string `json:"intent"`
}

type cardSnapshot struct {
	Index      int      `json:"index"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind,omitempty"`
	Cost       int      `json:"cost,omitempty"`
	Summary    string   `json:"summary"`
	TargetHint string   `json:"target_hint,omitempty"`
	Badges     []string `json:"badges,omitempty"`
}

type nodeSnapshot struct {
	ID    string `json:"id,omitempty"`
	Index int    `json:"index"`
	Floor int    `json:"floor"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type mapTreeNodeSnapshot struct {
	ID    string   `json:"id"`
	Index int      `json:"index"`
	Floor int      `json:"floor"`
	Kind  string   `json:"kind"`
	Edges []string `json:"edges,omitempty"`
}

type choiceSnapshot struct {
	Index       int      `json:"index"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Badges      []string `json:"badges,omitempty"`
}

type shopOfferSnapshot struct {
	Index       int      `json:"index"`
	Kind        string   `json:"kind"`
	Category    string   `json:"category,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int      `json:"price"`
	Badges      []string `json:"badges,omitempty"`
}

type equipmentSnapshot struct {
	Source               string `json:"source"`
	Slot                 string `json:"slot"`
	CandidateName        string `json:"candidate_name"`
	CandidateDescription string `json:"candidate_description"`
	CurrentName          string `json:"current_name,omitempty"`
	CurrentDescription   string `json:"current_description,omitempty"`
	Price                int    `json:"price,omitempty"`
	CandidateScore       int    `json:"candidate_score"`
	CurrentScore         int    `json:"current_score,omitempty"`
}

type lobbySnapshot struct {
	Mode    string   `json:"mode"`
	Seed    int64    `json:"seed"`
	Classes []string `json:"classes"`
}

type mapSnapshot struct {
	Mode          string                  `json:"mode"`
	Act           int                     `json:"act"`
	NextFloor     int                     `json:"next_floor"`
	CurrentFloor  int                     `json:"current_floor,omitempty"`
	CurrentNodeID string                  `json:"current_node_id,omitempty"`
	Gold          int                     `json:"gold"`
	Party         []actorSnapshot         `json:"party"`
	Reachable     []nodeSnapshot          `json:"reachable"`
	VoteStatus    []string                `json:"vote_status,omitempty"`
	VoteSummary   []string                `json:"vote_summary,omitempty"`
	Graph         [][]mapTreeNodeSnapshot `json:"graph,omitempty"`
	History       []string                `json:"history"`
}

type statsSnapshot struct {
	SeatName    string               `json:"seat_name,omitempty"`
	CombatTurns int                  `json:"combat_turns,omitempty"`
	Combat      engine.CombatMetrics `json:"combat,omitempty"`
	RunTurns    int                  `json:"run_turns,omitempty"`
	Run         engine.CombatMetrics `json:"run,omitempty"`
}

type combatSnapshot struct {
	Turn           int             `json:"turn"`
	Energy         int             `json:"energy"`
	MaxEnergy      int             `json:"max_energy"`
	DeckSize       int             `json:"deck_size,omitempty"`
	DrawCount      int             `json:"draw_count,omitempty"`
	DiscardCount   int             `json:"discard_count,omitempty"`
	ExhaustCount   int             `json:"exhaust_count,omitempty"`
	Party          []actorSnapshot `json:"party"`
	Enemies        []enemySnapshot `json:"enemies"`
	Hand           []cardSnapshot  `json:"hand"`
	Potions        []string        `json:"potions"`
	DrawPile       []string        `json:"draw_pile,omitempty"`
	DiscardPile    []string        `json:"discard_pile,omitempty"`
	ExhaustPile    []string        `json:"exhaust_pile,omitempty"`
	Effects        []string        `json:"effects,omitempty"`
	PendingRepeats []string        `json:"pending_repeats,omitempty"`
	EndTurnVotes   []bool          `json:"end_turn_votes"`
	VoteStatus     []string        `json:"vote_status,omitempty"`
	Logs           []string        `json:"logs"`
	Highlights     []string        `json:"highlights,omitempty"`
}

type rewardSnapshot struct {
	Gold        int                `json:"gold"`
	Source      string             `json:"source"`
	Cards       []cardSnapshot     `json:"cards"`
	Potion      string             `json:"potion,omitempty"`
	Relic       string             `json:"relic,omitempty"`
	RelicBadges []string           `json:"relic_badges,omitempty"`
	Equipment   *equipmentSnapshot `json:"equipment,omitempty"`
	Highlights  []string           `json:"highlights,omitempty"`
}

type eventSnapshot struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Badges      []string         `json:"badges,omitempty"`
	Choices     []choiceSnapshot `json:"choices"`
	Log         []string         `json:"log"`
	Highlights  []string         `json:"highlights,omitempty"`
}

type shopSnapshot struct {
	Gold       int                 `json:"gold"`
	Offers     []shopOfferSnapshot `json:"offers"`
	Log        []string            `json:"log"`
	Highlights []string            `json:"highlights,omitempty"`
}

type restSnapshot struct {
	Party []actorSnapshot `json:"party"`
	Log   []string        `json:"log"`
}

type deckActionSnapshot struct {
	Mode     string         `json:"mode"`
	Title    string         `json:"title"`
	Subtitle string         `json:"subtitle"`
	Cards    []cardSnapshot `json:"cards"`
}

type summarySnapshot struct {
	Result   string          `json:"result"`
	Mode     string          `json:"mode"`
	Act      int             `json:"act"`
	Floors   int             `json:"floors"`
	Gold     int             `json:"gold"`
	DeckSize int             `json:"deck_size"`
	Party    []actorSnapshot `json:"party"`
	History  []string        `json:"history"`
}

type roomSnapshot struct {
	SelfID           string              `json:"self_id"`
	Language         string              `json:"language,omitempty"`
	Seat             int                 `json:"seat,omitempty"`
	HostID           string              `json:"host_id"`
	RoomAddr         string              `json:"room_addr"`
	Phase            string              `json:"phase"`
	PhaseTitle       string              `json:"phase_title,omitempty"`
	PhaseHint        string              `json:"phase_hint,omitempty"`
	ControlLabel     string              `json:"control_label,omitempty"`
	RoleNote         string              `json:"role_note,omitempty"`
	Banner           string              `json:"banner,omitempty"`
	Players          []roomPlayer        `json:"players"`
	OfflineSeats     []string            `json:"offline_seats,omitempty"`
	WaitingOn        []string            `json:"waiting_on,omitempty"`
	SeatStatus       []string            `json:"seat_status,omitempty"`
	Recovery         []string            `json:"recovery,omitempty"`
	Reconnect        []string            `json:"reconnect,omitempty"`
	Resume           []string            `json:"resume,omitempty"`
	Commands         []string            `json:"commands,omitempty"`
	Examples         []string            `json:"examples,omitempty"`
	CanStart         bool                `json:"can_start"`
	Lobby            *lobbySnapshot      `json:"lobby,omitempty"`
	Map              *mapSnapshot        `json:"map,omitempty"`
	SharedMap        *mapSnapshot        `json:"shared_map,omitempty"`
	Combat           *combatSnapshot     `json:"combat,omitempty"`
	Stats            *statsSnapshot      `json:"stats,omitempty"`
	Reward           *rewardSnapshot     `json:"reward,omitempty"`
	Event            *eventSnapshot      `json:"event,omitempty"`
	Shop             *shopSnapshot       `json:"shop,omitempty"`
	Rest             *restSnapshot       `json:"rest,omitempty"`
	Equipment        *equipmentSnapshot  `json:"equipment,omitempty"`
	Deck             *deckActionSnapshot `json:"deck,omitempty"`
	Summary          *summarySnapshot    `json:"summary,omitempty"`
	ChatLog          []string            `json:"chat_log,omitempty"`
	ChatUnread       int                 `json:"chat_unread,omitempty"`
	ChatUnreadInView int                 `json:"chat_unread_in_view,omitempty"`
	TransferNote     string              `json:"transfer_note,omitempty"`
	RoomLog          []string            `json:"room_log,omitempty"`
}

type clientConn struct {
	id       string
	conn     net.Conn
	enc      *json.Encoder
	notice   string
	resume   []string
	chatSeen int
}

type hostTransferRequest struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Seat   int    `json:"seat"`
}

type server struct {
	lib      *content.Library
	roomAddr string
	listener net.Listener
	savePath string

	mu      sync.Mutex
	nextID  int
	hostID  string
	order   []string
	players map[string]*roomPlayer
	clients map[string]*clientConn
	closed  bool

	phase            string
	mode             engine.GameMode
	seed             int64
	chatLog          []string
	roomLog          []string
	hostTransfer     *hostTransferRequest
	restoredFromSave bool

	run         *engine.RunState
	seatStates  map[string]*seatRunState
	currentNode engine.Node
	flowOwner   string

	combat  *engine.CombatState
	restLog []string

	equipOffer  *engine.EquipmentOfferState
	equipFrom   string
	rewardCard  string
	shopOfferID string
	eventChoice string

	deckActionMode      string
	deckActionTitle     string
	deckActionSubtitle  string
	deckActionIndexes   []int
	deckActionPrice     int
	deckActionEffect    *content.Effect
	deckActionTakeEquip bool
}
