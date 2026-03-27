package app

import (
	"fmt"
	"time"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/netplay"
	"cmdcards/internal/storage"
	"cmdcards/internal/ui"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type screen string

const (
	screenMenu              screen = "menu"
	screenClass             screen = "class"
	screenMap               screen = "map"
	screenCombat            screen = "combat"
	screenPotionReplace     screen = "potion_replace"
	screenReward            screen = "reward"
	screenEquip             screen = "equip"
	screenDeckAct           screen = "deck_action"
	screenEvent             screen = "event"
	screenShop              screen = "shop"
	screenRest              screen = "rest"
	screenCodex             screen = "codex"
	screenProfile           screen = "profile"
	screenMultiplayerMenu   screen = "multiplayer_menu"
	screenMultiplayerCreate screen = "multiplayer_create"
	screenMultiplayerJoin   screen = "multiplayer_join"
	screenMultiplayerRoom   screen = "multiplayer_room"
	screenSummary           screen = "summary"
)

const (
	menuContinue    = "继续冒险"
	menuAbandon     = "放弃本局"
	menuStory       = "主线模式"
	menuEndless     = "无尽模式"
	menuProfile     = "档案"
	menuMultiplayer = "多人模式"
	menuQuit        = "Quit"
)

type multiplayerLaunchKind string

const (
	multiplayerLaunchNone multiplayerLaunchKind = ""
	multiplayerLaunchHost multiplayerLaunchKind = "host"
	multiplayerLaunchJoin multiplayerLaunchKind = "join"
)

type multiplayerLaunchRequest struct {
	Kind     multiplayerLaunchKind
	Addr     string
	Name     string
	ClassID  string
	Port     int
	ForceNew bool
}

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Select  key.Binding
	Cycle   key.Binding
	Map     key.Binding
	Stats   key.Binding
	Back    key.Binding
	EndTurn key.Binding
	Potion  key.Binding
	Skip    key.Binding
	Leave   key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Select, k.Cycle, k.Map, k.Stats, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Select, k.Cycle, k.Map, k.Stats},
		{k.EndTurn, k.Potion, k.Skip, k.Leave},
		{k.Back, k.Help, k.Quit},
	}
}

type model struct {
	width                      int
	height                     int
	now                        func() time.Time
	keys                       keyMap
	help                       help.Model
	theme                      ui.Theme
	lib                        *content.Library
	store                      *storage.Store
	profile                    engine.Profile
	mode                       engine.GameMode
	run                        *engine.RunState
	screen                     screen
	index                      int
	message                    string
	err                        error
	menuItems                  []string
	classes                    []content.ClassDef
	combat                     *engine.CombatState
	combatPane                 int
	combatTopPage              int
	combatLogPage              int
	combatPotionIndex          int
	combatPotionMode           bool
	combatTarget               engine.CombatTarget
	reward                     *engine.RewardState
	equipOffer                 *engine.EquipmentOfferState
	currentNode                engine.Node
	eventState                 *engine.EventState
	shopState                  *engine.ShopState
	restLog                    []string
	equipFrom                  string
	rewardCard                 string
	shopOfferID                string
	eventChoice                string
	deckActionMode             string
	deckActionTitle            string
	deckActionSubtitle         string
	deckActionIndexes          []int
	deckActionPrice            int
	deckActionEffect           *content.Effect
	deckActionTakeEquip        bool
	codexTab                   int
	profileTab                 int
	multiplayerMenuItems       []string
	multiplayerCreateName      textinput.Model
	multiplayerCreatePort      textinput.Model
	multiplayerCreateClass     int
	multiplayerCreateForceNew  bool
	multiplayerJoinAddr        textinput.Model
	multiplayerJoinName        textinput.Model
	multiplayerJoinClass       int
	multiplayerSession         *netplay.Session
	multiplayerSnapshot        *netplay.Snapshot
	multiplayerCommandInput    textinput.Model
	multiplayerConnecting      bool
	multiplayerRoomFocus       int
	multiplayerActionIndex     int
	multiplayerCombatMode      int
	multiplayerCombatIndex     int
	multiplayerPotionIndex     int
	multiplayerCombatTarget    multiplayerTargetState
	multiplayerStructuredIndex int
	multiplayerInspectPane     int
	multiplayerCombatTopPage   int
	multiplayerInspectLogPage  int
	showMapOverlay             bool
	showStatsOverlay           bool
	pendingPotionID            string
	pendingPotionResume        screen
	pendingPotionAdvance       bool
	lastActionAt               time.Time
}

func Run(lib *content.Library, store *storage.Store) error {
	profile, err := store.LoadProfile(lib)
	if err != nil {
		return err
	}
	existingRun, err := store.LoadRun()
	if err != nil {
		return err
	}
	m := newModel(lib, store, profile, existingRun)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func newModel(lib *content.Library, store *storage.Store, profile engine.Profile, run *engine.RunState) model {
	menuItems := menuItemsForRun(run)
	createName := newMultiplayerInput("Host", "Choose a local display name", 24)
	createPort := newMultiplayerInput("7777", "LAN port, for example 7777", 5)
	joinAddr := newMultiplayerInput("127.0.0.1:7777", "Host address, for example 127.0.0.1:7777", 64)
	joinName := newMultiplayerInput("Guest", "Choose a local display name", 24)
	commandInput := newMultiplayerInput("", "Examples: ready, chat hello, node 1", 240)
	commandInput.Width = 64
	return model{
		now:                       time.Now,
		lib:                       lib,
		store:                     store,
		profile:                   profile,
		run:                       run,
		screen:                    screenMenu,
		theme:                     ui.DefaultTheme(),
		help:                      help.New(),
		menuItems:                 menuItems,
		classes:                   lib.ClassList(),
		multiplayerMenuItems:      []string{"Create Room", "Join Room", "Back"},
		multiplayerCreateName:     createName,
		multiplayerCreatePort:     createPort,
		multiplayerCreateClass:    classIndexByID(lib.ClassList(), "vanguard"),
		multiplayerCreateForceNew: true,
		multiplayerJoinAddr:       joinAddr,
		multiplayerJoinName:       joinName,
		multiplayerJoinClass:      classIndexByID(lib.ClassList(), "arcanist"),
		multiplayerCommandInput:   commandInput,
		keys: keyMap{
			Left:    key.NewBinding(key.WithKeys("left", "pgup"), key.WithHelp("left/PgUp", "prev page")),
			Right:   key.NewBinding(key.WithKeys("right", "pgdown"), key.WithHelp("right/PgDn", "next page")),
			Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "up")),
			Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "down")),
			Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
			Cycle:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "cycle")),
			Map:     key.NewBinding(key.WithKeys("m", "M"), key.WithHelp("m", "map")),
			Stats:   key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "stats")),
			Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
			EndTurn: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "end turn")),
			Potion:  key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "potion")),
			Skip:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "skip")),
			Leave:   key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "leave")),
			Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		},
	}
}

func menuItemsForRun(run *engine.RunState) []string {
	items := []string{menuStory, menuEndless, menuCodexLabel, menuProgressionLabel, menuMultiplayer, menuQuit}
	if run != nil && run.Status == engine.RunStatusActive {
		items = []string{menuContinue, menuAbandon, menuStory, menuEndless, menuCodexLabel, menuProgressionLabel, menuMultiplayer, menuQuit}
	}
	return items
}

func (m *model) saveCheckpoint() error {
	if m.run == nil || m.run.Status != engine.RunStatusActive {
		return nil
	}
	if checkpoint := m.buildCheckpoint(); checkpoint != nil {
		m.run.Checkpoint = checkpoint
	}
	return m.store.SaveRun(m.run)
}

func (m *model) buildCheckpoint() *engine.RunCheckpoint {
	switch m.screen {
	case screenMap, screenCombat, screenPotionReplace, screenReward, screenEquip, screenDeckAct, screenEvent, screenShop, screenRest:
	default:
		return m.run.Checkpoint
	}

	checkpoint := &engine.RunCheckpoint{
		Screen:               string(m.screen),
		RestLog:              append([]string{}, m.restLog...),
		EquipFrom:            m.equipFrom,
		RewardCard:           m.rewardCard,
		ShopOfferID:          m.shopOfferID,
		EventChoice:          m.eventChoice,
		DeckActionMode:       m.deckActionMode,
		DeckActionTitle:      m.deckActionTitle,
		DeckActionSubtitle:   m.deckActionSubtitle,
		DeckActionIndexes:    append([]int{}, m.deckActionIndexes...),
		DeckActionPrice:      m.deckActionPrice,
		DeckActionTakeEquip:  m.deckActionTakeEquip,
		CombatPane:           m.combatPane,
		CombatTopPage:        m.combatTopPage,
		CombatLogPage:        m.combatLogPage,
		CombatPotionIndex:    m.combatPotionIndex,
		CombatPotionMode:     m.combatPotionMode,
		CombatTarget:         m.combatTarget,
		PendingPotionID:      m.pendingPotionID,
		PendingPotionResume:  string(m.pendingPotionResume),
		PendingPotionAdvance: m.pendingPotionAdvance,
	}
	if m.currentNode.ID != "" {
		node := m.currentNode
		checkpoint.CurrentNode = &node
	}
	if m.combat != nil {
		combat := *m.combat
		checkpoint.Combat = &combat
	}
	if m.reward != nil {
		reward := *m.reward
		checkpoint.Reward = &reward
	}
	if m.equipOffer != nil {
		equipOffer := *m.equipOffer
		checkpoint.EquipOffer = &equipOffer
	}
	if m.eventState != nil {
		eventState := *m.eventState
		checkpoint.EventState = &eventState
	}
	if m.shopState != nil {
		shopState := *m.shopState
		checkpoint.ShopState = &shopState
	}
	if m.deckActionEffect != nil {
		effect := *m.deckActionEffect
		checkpoint.DeckActionEffect = &effect
	}
	return checkpoint
}

func (m *model) restoreCheckpoint() {
	m.index = 0
	m.resetNodeFlowState()
	if m.run == nil || m.run.Status != engine.RunStatusActive || m.run.Checkpoint == nil {
		m.screen = screenMap
		return
	}
	cp := m.run.Checkpoint
	if cp.CurrentNode != nil {
		m.currentNode = *cp.CurrentNode
	}
	m.combat = cp.Combat
	m.reward = cp.Reward
	m.equipOffer = cp.EquipOffer
	m.eventState = cp.EventState
	m.shopState = cp.ShopState
	m.restLog = append([]string{}, cp.RestLog...)
	m.equipFrom = cp.EquipFrom
	m.rewardCard = cp.RewardCard
	m.shopOfferID = cp.ShopOfferID
	m.eventChoice = cp.EventChoice
	m.deckActionMode = cp.DeckActionMode
	m.deckActionTitle = cp.DeckActionTitle
	m.deckActionSubtitle = cp.DeckActionSubtitle
	m.deckActionIndexes = append([]int{}, cp.DeckActionIndexes...)
	m.deckActionPrice = cp.DeckActionPrice
	m.deckActionEffect = cp.DeckActionEffect
	m.deckActionTakeEquip = cp.DeckActionTakeEquip
	m.combatPane = cp.CombatPane
	m.combatTopPage = cp.CombatTopPage
	m.combatLogPage = cp.CombatLogPage
	m.combatPotionIndex = cp.CombatPotionIndex
	m.combatPotionMode = cp.CombatPotionMode
	m.combatTarget = cp.CombatTarget
	m.pendingPotionID = cp.PendingPotionID
	m.pendingPotionResume = screen(cp.PendingPotionResume)
	m.pendingPotionAdvance = cp.PendingPotionAdvance
	switch screen(cp.Screen) {
	case screenCombat, screenPotionReplace, screenReward, screenEquip, screenDeckAct, screenEvent, screenShop, screenRest:
		m.screen = screen(cp.Screen)
	default:
		m.screen = screenMap
	}
}

func (m *model) abandonRun() error {
	m.run = nil
	m.resetNodeFlowState()
	m.screen = screenMenu
	m.menuItems = menuItemsForRun(nil)
	m.index = 0
	return m.store.ClearRun()
}

func (m *model) resetNodeFlowState() {
	m.combat = nil
	m.reward = nil
	m.eventState = nil
	m.shopState = nil
	m.restLog = nil
	m.currentNode = engine.Node{}
	m.combatPane = 0
	m.combatTopPage = 0
	m.combatLogPage = 0
	m.combatPotionIndex = 0
	m.combatPotionMode = false
	m.combatTarget = engine.CombatTarget{}
	m.showMapOverlay = false
	m.showStatsOverlay = false
	m.clearEquipmentFlowState()
	m.clearDeckActionState()
	m.clearPendingPotionState()
}

func (m *model) clearEquipmentFlowState() {
	m.equipOffer = nil
	m.equipFrom = ""
	m.rewardCard = ""
	m.shopOfferID = ""
	m.eventChoice = ""
}

func (m *model) clearDeckActionState() {
	m.deckActionMode = ""
	m.deckActionTitle = ""
	m.deckActionSubtitle = ""
	m.deckActionIndexes = nil
	m.deckActionPrice = 0
	m.deckActionEffect = nil
	m.deckActionTakeEquip = false
	m.shopOfferID = ""
}

func (m *model) clearPendingPotionState() {
	m.pendingPotionID = ""
	m.pendingPotionResume = ""
	m.pendingPotionAdvance = false
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) && !m.shouldTreatQuitKeyAsText(msg) {
			m.closeMultiplayerSession()
			if err := m.saveCheckpoint(); err != nil {
				m.err = err
				return m, nil
			}
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Help) {
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
		if m.showMapOverlay {
			switch {
			case key.Matches(msg, m.keys.Map), key.Matches(msg, m.keys.Back):
				m.showMapOverlay = false
			case msg.String() == "K" && m.canToggleStatsOverlay():
				m.showMapOverlay = false
				m.showStatsOverlay = true
			}
			return m, nil
		}
		if m.showStatsOverlay {
			switch {
			case msg.String() == "K", key.Matches(msg, m.keys.Back):
				m.showStatsOverlay = false
			case m.canToggleMapOverlay() && key.Matches(msg, m.keys.Map):
				m.showStatsOverlay = false
				m.showMapOverlay = true
			}
			return m, nil
		}
		if m.canToggleMapOverlay() && key.Matches(msg, m.keys.Map) {
			m.showMapOverlay = true
			return m, nil
		}
		if m.canToggleStatsOverlay() && msg.String() == "K" {
			m.showStatsOverlay = true
			return m, nil
		}
		switch m.screen {
		case screenMenu:
			return m.updateMenu(msg)
		case screenClass:
			return m.updateClass(msg)
		case screenMap:
			return m.updateMap(msg)
		case screenCombat:
			return m.updateCombat(msg)
		case screenPotionReplace:
			return m.updatePotionReplace(msg)
		case screenReward:
			return m.updateReward(msg)
		case screenEquip:
			return m.updateEquip(msg)
		case screenDeckAct:
			return m.updateDeckAction(msg)
		case screenEvent:
			return m.updateEvent(msg)
		case screenShop:
			return m.updateShop(msg)
		case screenRest:
			return m.updateRest(msg)
		case screenCodex:
			return m.updateCodex(msg)
		case screenProfile:
			return m.updateProfile(msg)
		case screenMultiplayerMenu:
			return m.updateMultiplayerMenu(msg)
		case screenMultiplayerCreate:
			return m.updateMultiplayerCreate(msg)
		case screenMultiplayerJoin:
			return m.updateMultiplayerJoin(msg)
		case screenMultiplayerRoom:
			return m.updateMultiplayerRoom(msg)
		case screenSummary:
			if key.Matches(msg, m.keys.Quit) || key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Select) {
				return m, tea.Quit
			}
		}
	case multiplayerConnectedMsg:
		m.multiplayerConnecting = false
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, clearMessage()
		}
		m.multiplayerSession = msg.session
		m.multiplayerSnapshot = msg.snapshot
		m.multiplayerCommandInput.SetValue("")
		m.multiplayerCommandInput.Focus()
		m.syncMultiplayerRoomSelection()
		m.screen = screenMultiplayerRoom
		m.index = 0
		return m, waitForMultiplayerUpdate(msg.session)
	case multiplayerSnapshotMsg:
		m.multiplayerSnapshot = msg.snapshot
		m.syncMultiplayerRoomSelection()
		if m.multiplayerSession != nil {
			return m, waitForMultiplayerUpdate(m.multiplayerSession)
		}
		return m, nil
	case multiplayerErrorMsg:
		m.multiplayerConnecting = false
		m.closeMultiplayerSession()
		m.screen = screenMultiplayerMenu
		m.index = 0
		if msg.err != nil && !netplay.IsGracefulRoomClose(msg.err) {
			m.message = msg.err.Error()
			return m, clearMessage()
		}
		m.message = "Multiplayer session closed."
		return m, clearMessage()
	case tickMsg:
		m.message = ""
	}
	return m, nil
}

func (m model) canToggleMapOverlay() bool {
	switch m.screen {
	case screenMultiplayerRoom:
		return m.multiplayerSnapshot != nil && (m.multiplayerSnapshot.Map != nil || m.multiplayerSnapshot.SharedMap != nil)
	case screenMap, screenCombat, screenReward, screenEquip, screenDeckAct, screenEvent, screenShop, screenRest:
		if m.run == nil || m.run.Status != engine.RunStatusActive {
			return false
		}
		return true
	default:
		return false
	}
}

func (m model) canToggleStatsOverlay() bool {
	switch m.screen {
	case screenMultiplayerRoom:
		return m.multiplayerSnapshot != nil && m.multiplayerSnapshot.Stats != nil
	case screenMap, screenCombat, screenReward, screenEquip, screenDeckAct, screenEvent, screenShop, screenRest:
		return m.run != nil && m.run.Status == engine.RunStatusActive
	default:
		return false
	}
}

func (m model) statsOverlayLines() (string, []string, []string) {
	switch {
	case m.screen == screenMultiplayerRoom && m.multiplayerSnapshot != nil && m.multiplayerSnapshot.Stats != nil:
		stats := m.multiplayerSnapshot.Stats
		title := "Multiplayer Stats"
		if stats.SeatName != "" {
			title = fmt.Sprintf("%s | %s", title, stats.SeatName)
		}
		return title, engine.FormatCombatMetrics(stats.Combat, stats.CombatTurns), engine.FormatCombatMetrics(stats.Run, stats.RunTurns)
	case m.run != nil:
		combatLines := engine.FormatCombatMetrics(engine.CombatMetrics{}, 0)
		if m.combat != nil {
			combatLines = engine.FormatCombatMetrics(engine.CombatMetricsForSeat(m.combat, 0), engine.CombatTurns(m.combat))
		}
		return "Solo Stats", combatLines, engine.FormatCombatMetrics(m.run.Stats.Metrics, m.run.Stats.CombatTurns)
	default:
		return "Stats", nil, nil
	}
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, len(m.menuItems), -1)
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, len(m.menuItems), 1)
	case key.Matches(msg, m.keys.Left):
		m.index = flipPage(m.index, len(m.menuItems), -1)
	case key.Matches(msg, m.keys.Right):
		m.index = flipPage(m.index, len(m.menuItems), 1)
	case key.Matches(msg, m.keys.Select):
		choice := m.menuItems[m.index]
		switch choice {
		case menuContinue:
			m.restoreCheckpoint()
		case menuAbandon:
			if err := m.abandonRun(); err != nil {
				m.err = err
				return m, nil
			}
		case menuStory:
			m.mode = engine.ModeStory
			m.screen = screenClass
			m.index = 0
		case menuEndless:
			m.mode = engine.ModeEndless
			m.screen = screenClass
			m.index = 0
		case menuCodexLabel:
			m.screen = screenCodex
			m.codexTab = 0
			m.index = 0
		case menuProfile, menuProgressionLabel:
			m.screen = screenProfile
			m.profileTab = 0
			m.index = 0
		case menuMultiplayer:
			m.screen = screenMultiplayerMenu
			m.index = 0
		case menuQuit:
			if err := m.saveCheckpoint(); err != nil {
				m.err = err
				return m, nil
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) updateClass(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, len(m.classes), -1)
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, len(m.classes), 1)
	case key.Matches(msg, m.keys.Left):
		m.index = flipPage(m.index, len(m.classes), -1)
	case key.Matches(msg, m.keys.Right):
		m.index = flipPage(m.index, len(m.classes), 1)
	case key.Matches(msg, m.keys.Back):
		m.screen = screenMenu
		m.index = 0
	case key.Matches(msg, m.keys.Select):
		run, err := engine.NewRun(m.lib, m.profile, m.mode, m.classes[m.index].ID, time.Now().UnixNano())
		if err != nil {
			m.err = err
			return m, nil
		}
		m.run = run
		m.menuItems = menuItemsForRun(m.run)
		m.screen = screenMap
		m.index = 0
		if err := m.saveCheckpoint(); err != nil {
			m.err = err
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateMap(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	nodes := engine.ReachableNodes(m.run)
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = wrapIndex(m.index-1, len(nodes))
	case key.Matches(msg, m.keys.Down):
		m.index = wrapIndex(m.index+1, len(nodes))
	case key.Matches(msg, m.keys.Select):
		node := nodes[m.index]
		m.currentNode = node
		m.index = 0
		switch node.Kind {
		case engine.NodeMonster, engine.NodeElite, engine.NodeBoss:
			combat, err := engine.StartEncounter(m.lib, m.run, node)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.combat = combat
			m.combatPane = 0
			m.combatTopPage = 0
			m.combatLogPage = 0
			m.combatPotionIndex = 0
			m.combatPotionMode = false
			engine.StartPlayerTurn(m.lib, m.run.Player, m.combat)
			m.syncCombatTarget()
			m.screen = screenCombat
		case engine.NodeEvent:
			eventState, err := engine.StartEvent(m.lib, m.run, node)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.eventState = &eventState
			m.screen = screenEvent
		case engine.NodeShop:
			shop := engine.StartShop(m.lib, m.run)
			m.shopState = &shop
			m.screen = screenShop
		case engine.NodeRest:
			m.restLog = nil
			m.screen = screenRest
		}
		if err := m.saveCheckpoint(); err != nil {
			m.err = err
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateCombat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	changed := false
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.combatPotionMode {
			if len(m.run.Player.Potions) > 0 {
				m.combatPotionIndex = wrapIndex(m.combatPotionIndex-1, len(m.run.Player.Potions))
				m.syncCombatTarget()
			}
		} else if len(m.combat.Hand) > 0 {
			m.index = wrapIndex(m.index-1, len(m.combat.Hand))
			m.syncCombatTarget()
		}
	case key.Matches(msg, m.keys.Down):
		if m.combatPotionMode {
			if len(m.run.Player.Potions) > 0 {
				m.combatPotionIndex = wrapIndex(m.combatPotionIndex+1, len(m.run.Player.Potions))
				m.syncCombatTarget()
			}
		} else if len(m.combat.Hand) > 0 {
			m.index = wrapIndex(m.index+1, len(m.combat.Hand))
			m.syncCombatTarget()
		}
	case key.Matches(msg, m.keys.Left):
		m.cycleCombatTarget(-1)
	case key.Matches(msg, m.keys.Right):
		m.cycleCombatTarget(1)
	case msg.String() == "[":
		if m.combatTopPage > 0 {
			m.combatTopPage--
		}
	case msg.String() == "]":
		m.combatTopPage++
	case msg.String() >= "1" && msg.String() <= "6":
		m.combatPane = int(msg.String()[0] - '1')
		m.combatTopPage = 0
	case msg.String() == ",":
		m.combatLogPage++
	case msg.String() == ".":
		if m.combatLogPage > 0 {
			m.combatLogPage--
		}
	case key.Matches(msg, m.keys.Cycle):
		m.combatPane = (m.combatPane + 1) % engine.CombatInspectPaneCount
		m.combatTopPage = 0
	case key.Matches(msg, m.keys.Potion):
		if len(m.run.Player.Potions) == 0 {
			m.message = "No potion is available right now."
			return m, clearMessage()
		}
		m.combatPotionMode = !m.combatPotionMode
		m.combatPotionIndex = min(m.combatPotionIndex, len(m.run.Player.Potions)-1)
		m.syncCombatTarget()
	case key.Matches(msg, m.keys.EndTurn):
		if blocked, cmd := m.blockRapidAction("Action blocked: slow down a little before ending the turn."); blocked {
			return m, cmd
		}
		engine.EndPlayerTurn(m.lib, m.run.Player, m.combat)
		if !m.combat.Won && !m.combat.Lost {
			engine.StartPlayerTurn(m.lib, m.run.Player, m.combat)
			m.syncCombatTarget()
		}
		m.markActionDispatched()
		changed = true
	case key.Matches(msg, m.keys.Select):
		if blocked, cmd := m.blockRapidAction("Action blocked: slow down a little before playing another card."); blocked {
			return m, cmd
		}
		if m.combatPotionMode {
			if len(m.run.Player.Potions) == 0 {
				m.message = "No potion is available right now."
				return m, clearMessage()
			}
			potionIndex := min(m.combatPotionIndex, len(m.run.Player.Potions)-1)
			potionID := m.run.Player.Potions[potionIndex]
			if err := engine.UsePotionWithTarget(m.lib, m.run.Player, m.combat, potionID, m.combatTarget); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			m.run.Player.Potions = append(m.run.Player.Potions[:potionIndex], m.run.Player.Potions[potionIndex+1:]...)
			if len(m.run.Player.Potions) == 0 {
				m.combatPotionMode = false
				m.combatPotionIndex = 0
			} else {
				m.combatPotionIndex = min(m.combatPotionIndex, len(m.run.Player.Potions)-1)
			}
			m.syncCombatTarget()
			m.markActionDispatched()
			changed = true
		} else if len(m.combat.Hand) > 0 {
			if err := engine.PlayCardWithTarget(m.lib, m.run.Player, m.combat, m.index, m.combatTarget); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			if len(m.combat.Hand) > 0 {
				m.index = min(m.index, len(m.combat.Hand)-1)
			} else {
				m.index = 0
				m.combatPotionMode = len(m.run.Player.Potions) > 0
			}
			m.syncCombatTarget()
			m.markActionDispatched()
			changed = true
		}
	}
	if m.combat.Won || m.combat.Lost {
		engine.FinishCombat(m.lib, m.run, m.currentNode, m.combat)
		if m.combat.Lost {
			_ = engine.ApplyCombatResult(m.lib, m.run, m.currentNode, m.combat, "")
			_ = m.store.ClearRun()
			_ = m.store.SaveProfile(m.profile)
			m.screen = screenSummary
			return m, nil
		}
		reward := m.combat.Reward
		m.reward = &reward
		m.screen = screenReward
		m.index = 0
		changed = true
	}
	if changed {
		m.combatLogPage = 0
		if err := m.saveCheckpoint(); err != nil {
			m.err = err
			return m, nil
		}
	}
	return m, nil
}

func (m model) updatePotionReplace(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	optionCount := len(m.run.Player.Potions) + 1
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = wrapIndex(m.index-1, optionCount)
	case key.Matches(msg, m.keys.Down):
		m.index = wrapIndex(m.index+1, optionCount)
	case key.Matches(msg, m.keys.Back):
		m.index = len(m.run.Player.Potions)
		fallthrough
	case key.Matches(msg, m.keys.Select):
		if m.index < len(m.run.Player.Potions) {
			if err := engine.ReplacePotion(&m.run.Player, m.index, m.pendingPotionID); err != nil {
				m.err = err
				return m, nil
			}
		}
		return m.finishPendingPotionFlow()
	}
	return m, nil
}

func (m model) updateReward(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if len(m.reward.CardChoices) > 0 {
			m.index = wrapIndex(m.index-1, len(m.reward.CardChoices))
		}
	case key.Matches(msg, m.keys.Down):
		if len(m.reward.CardChoices) > 0 {
			m.index = wrapIndex(m.index+1, len(m.reward.CardChoices))
		}
	case key.Matches(msg, m.keys.Skip):
		before := m.run.Player
		if m.reward.EquipmentID != "" {
			return m.startEquipmentFlow("reward", "", "")
		}
		if err := engine.ApplyCombatResultDecision(m.lib, m.run, m.currentNode, m.combat, "", false); err != nil {
			m.err = err
			return m, nil
		}
		if m.startPendingPotionFlow(m.reward.PotionID, before, true, screenMap) {
			return m, nil
		}
		return m.afterNodeAdvance()
	case key.Matches(msg, m.keys.Select):
		before := m.run.Player
		choice := ""
		if len(m.reward.CardChoices) > 0 {
			choice = m.reward.CardChoices[m.index].ID
		}
		if m.reward.EquipmentID != "" {
			return m.startEquipmentFlow("reward", choice, "")
		}
		if err := engine.ApplyCombatResultDecision(m.lib, m.run, m.currentNode, m.combat, choice, false); err != nil {
			m.err = err
			return m, nil
		}
		if m.startPendingPotionFlow(m.reward.PotionID, before, true, screenMap) {
			return m, nil
		}
		return m.afterNodeAdvance()
	}
	return m, nil
}

func (m model) updateEquip(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = wrapIndex(m.index-1, 2)
	case key.Matches(msg, m.keys.Down):
		m.index = wrapIndex(m.index+1, 2)
	case key.Matches(msg, m.keys.Back):
		return m.cancelEquipmentFlow()
	case key.Matches(msg, m.keys.Select):
		take := m.index == 0
		switch m.equipFrom {
		case "reward":
			before := m.run.Player
			if err := engine.ApplyCombatResultDecision(m.lib, m.run, m.currentNode, m.combat, m.rewardCard, take); err != nil {
				m.err = err
				return m, nil
			}
			if m.startPendingPotionFlow(m.reward.PotionID, before, true, screenMap) {
				return m, nil
			}
			return m.afterNodeAdvance()
		case "shop":
			if err := engine.ApplyShopEquipmentPurchase(m.lib, m.run, m.shopState, m.shopOfferID, take); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			m.screen = screenShop
			m.index = 0
			m.clearEquipmentFlowState()
			if err := m.saveCheckpoint(); err != nil {
				m.err = err
				return m, nil
			}
			return m, nil
		case "event":
			choice := choiceByID(m.eventState.Event.Choices, m.eventChoice)
			if plan, err := engine.EventChoiceDeckActionPlan(m.lib, m.run, choice, take); err != nil {
				m.err = err
				return m, nil
			} else if plan != nil {
				return m.startDeckActionPlan(*plan)
			}
			before := m.run.Player
			if err := engine.ResolveEventDecision(m.lib, m.run, m.eventState, m.eventChoice, take); err != nil {
				m.err = err
				return m, nil
			}
			if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
				m.err = err
				return m, nil
			}
			if m.startPendingPotionFlow(engine.EventChoicePotionID(choice), before, true, screenMap) {
				return m, nil
			}
			return m.afterNodeAdvance()
		}
	}
	return m, nil
}

func (m model) updateEvent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = wrapIndex(m.index-1, len(m.eventState.Event.Choices))
	case key.Matches(msg, m.keys.Down):
		m.index = wrapIndex(m.index+1, len(m.eventState.Event.Choices))
	case key.Matches(msg, m.keys.Select):
		before := m.run.Player
		choice := m.eventState.Event.Choices[m.index]
		if engine.EventChoiceEquipmentID(choice) != "" {
			return m.startEquipmentFlow("event", choice.ID, "")
		}
		if plan, err := engine.EventChoiceDeckActionPlan(m.lib, m.run, choice, true); err != nil {
			m.err = err
			return m, nil
		} else if plan != nil {
			m.eventChoice = choice.ID
			return m.startDeckActionPlan(*plan)
		}
		if err := engine.ResolveEventDecision(m.lib, m.run, m.eventState, choice.ID, true); err != nil {
			m.err = err
			return m, nil
		}
		if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
			m.err = err
			return m, nil
		}
		if m.startPendingPotionFlow(engine.EventChoicePotionID(choice), before, true, screenMap) {
			return m, nil
		}
		return m.afterNodeAdvance()
	}
	return m, nil
}

func (m model) updateShop(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, len(m.shopState.Offers), -1)
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, len(m.shopState.Offers), 1)
	case key.Matches(msg, m.keys.Left):
		m.index = flipPage(m.index, len(m.shopState.Offers), -1)
	case key.Matches(msg, m.keys.Right):
		m.index = flipPage(m.index, len(m.shopState.Offers), 1)
	case key.Matches(msg, m.keys.Leave), key.Matches(msg, m.keys.Back):
		if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
			m.err = err
			return m, nil
		}
		return m.afterNodeAdvance()
	case key.Matches(msg, m.keys.Select):
		offer := m.shopState.Offers[m.index]
		if offer.Kind == "equipment" {
			return m.startEquipmentFlow("shop", "", offer.ID)
		}
		if offer.Kind == "remove" {
			if m.run.Player.Gold < offer.Price {
				m.message = "金币不足"
				return m, clearMessage()
			}
			return m.startDeckActionFlow("shop_remove", offer.Price)
		}
		if offer.Kind == "service" {
			plan, err := engine.ShopOfferDeckActionPlan(m.lib, m.run, m.shopState, offer.ID)
			if err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			if plan != nil {
				m.shopOfferID = offer.ID
				return m.startDeckActionPlan(*plan)
			}
		}
		before := m.run.Player
		if err := engine.ApplyShopPurchase(m.lib, m.run, m.shopState, offer.ID); err != nil {
			m.message = err.Error()
			return m, clearMessage()
		}
		if offer.Kind == "potion" && m.startPendingPotionFlow(offer.ItemID, before, false, screenShop) {
			return m, nil
		}
		if len(m.shopState.Offers) == 0 {
			m.index = 0
		} else {
			m.index = min(m.index, len(m.shopState.Offers)-1)
		}
		if err := m.saveCheckpoint(); err != nil {
			m.err = err
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateRest(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = wrapIndex(m.index-1, 2)
	case key.Matches(msg, m.keys.Down):
		m.index = wrapIndex(m.index+1, 2)
	case key.Matches(msg, m.keys.Select):
		choice := "heal"
		if m.index == 1 {
			choice = "upgrade"
		}
		if choice == "upgrade" {
			indexes := engine.UpgradableCardIndexes(m.lib, m.run.Player.Deck)
			if len(indexes) > 0 {
				return m.startDeckActionFlow("rest_upgrade", 0)
			}
		}
		restState, err := engine.ResolveRest(m.lib, m.run, choice)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.restLog = restState.Log
		if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
			m.err = err
			return m, nil
		}
		return m.afterNodeAdvance()
	}
	return m, nil
}

func (m model) updateDeckAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, len(m.deckActionIndexes), -1)
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, len(m.deckActionIndexes), 1)
	case key.Matches(msg, m.keys.Left):
		m.index = flipPage(m.index, len(m.deckActionIndexes), -1)
	case key.Matches(msg, m.keys.Right):
		m.index = flipPage(m.index, len(m.deckActionIndexes), 1)
	case key.Matches(msg, m.keys.Back):
		return m.cancelDeckActionFlow()
	case key.Matches(msg, m.keys.Select):
		if len(m.deckActionIndexes) == 0 {
			return m.cancelDeckActionFlow()
		}
		deckIndex := m.deckActionIndexes[m.index]
		switch m.deckActionMode {
		case "rest_upgrade":
			name, err := engine.UpgradeDeckCard(m.lib, &m.run.Player, deckIndex)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.restLog = []string{"Upgraded card: " + name}
			if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
				m.err = err
				return m, nil
			}
			return m.afterNodeAdvance()
		case "shop_remove":
			if err := engine.ApplyShopCardRemoval(m.lib, m.run, m.shopState, "remove-card", deckIndex); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			m.screen = screenShop
			m.index = 0
			m.clearDeckActionState()
			if err := m.saveCheckpoint(); err != nil {
				m.err = err
				return m, nil
			}
			return m, nil
		case "shop_augment_card":
			if err := engine.ApplyShopServiceWithDeckChoice(m.lib, m.run, m.shopState, m.shopOfferID, deckIndex); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			m.screen = screenShop
			m.index = 0
			m.clearDeckActionState()
			if err := m.saveCheckpoint(); err != nil {
				m.err = err
				return m, nil
			}
			return m, nil
		case "event_augment_card":
			before := m.run.Player
			if err := engine.ResolveEventDecisionWithDeckChoice(m.lib, m.run, m.eventState, m.eventChoice, m.deckActionTakeEquip, deckIndex); err != nil {
				m.err = err
				return m, nil
			}
			if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
				m.err = err
				return m, nil
			}
			m.clearDeckActionState()
			if m.startPendingPotionFlow(engine.EventChoicePotionID(choiceByID(m.eventState.Event.Choices, m.eventChoice)), before, true, screenMap) {
				return m, nil
			}
			return m.afterNodeAdvance()
		}
	}
	return m, nil
}

func (m model) startEquipmentFlow(source, rewardCard, shopOfferID string) (tea.Model, tea.Cmd) {
	var equipmentID string
	price := 0
	switch source {
	case "reward":
		equipmentID = m.reward.EquipmentID
	case "shop":
		for _, offer := range m.shopState.Offers {
			if offer.ID == shopOfferID {
				equipmentID = offer.ItemID
				price = offer.Price
				break
			}
		}
	case "event":
		for _, choice := range m.eventState.Event.Choices {
			if choice.ID == rewardCard {
				equipmentID = engine.EventChoiceEquipmentID(choice)
				break
			}
		}
	}
	offer, err := engine.BuildEquipmentOffer(m.lib, m.run.Player, equipmentID, source, price)
	if err != nil {
		m.err = err
		return m, nil
	}
	m.equipOffer = &offer
	m.equipFrom = source
	m.rewardCard = rewardCard
	m.shopOfferID = shopOfferID
	m.eventChoice = rewardCard
	m.screen = screenEquip
	m.index = 0
	if err := m.saveCheckpoint(); err != nil {
		m.err = err
		return m, nil
	}
	return m, nil
}

func (m model) startDeckActionFlow(mode string, price int) (tea.Model, tea.Cmd) {
	plan := engine.DeckActionPlan{Mode: mode, Price: price}
	switch mode {
	case "rest_upgrade":
		plan.Title = "Choose a card to upgrade"
		plan.Subtitle = "The campfire upgrade applies that card's upgrade values."
		plan.Indexes = engine.UpgradableCardIndexes(m.lib, m.run.Player.Deck)
	case "shop_remove":
		plan.Title = "Choose a card to remove"
		plan.Subtitle = fmt.Sprintf("Service price: %d gold", price)
		plan.Indexes = make([]int, len(m.run.Player.Deck))
		for i := range m.run.Player.Deck {
			plan.Indexes[i] = i
		}
	}
	return m.startDeckActionPlan(plan)
}

func (m model) startDeckActionPlan(plan engine.DeckActionPlan) (tea.Model, tea.Cmd) {
	m.deckActionMode = plan.Mode
	m.deckActionTitle = plan.Title
	m.deckActionSubtitle = plan.Subtitle
	m.deckActionIndexes = append([]int{}, plan.Indexes...)
	m.deckActionPrice = plan.Price
	m.deckActionTakeEquip = plan.TakeEquipment
	m.deckActionEffect = nil
	if plan.Effect != nil {
		effect := *plan.Effect
		m.deckActionEffect = &effect
	}
	m.screen = screenDeckAct
	m.index = 0
	if err := m.saveCheckpoint(); err != nil {
		m.err = err
		return m, nil
	}
	return m, nil
}

func (m model) cancelEquipmentFlow() (tea.Model, tea.Cmd) {
	switch m.equipFrom {
	case "reward":
		m.screen = screenReward
	case "shop":
		m.screen = screenShop
	case "event":
		m.screen = screenEvent
	}
	m.index = 0
	m.clearEquipmentFlowState()
	if err := m.saveCheckpoint(); err != nil {
		m.err = err
		return m, nil
	}
	return m, nil
}

func (m model) cancelDeckActionFlow() (tea.Model, tea.Cmd) {
	switch m.deckActionMode {
	case "rest_upgrade":
		m.screen = screenRest
	case "shop_remove":
		m.screen = screenShop
	case "shop_augment_card":
		m.screen = screenShop
	case "event_augment_card":
		m.screen = screenEvent
	default:
		m.screen = screenMap
	}
	m.index = 0
	m.clearDeckActionState()
	if err := m.saveCheckpoint(); err != nil {
		m.err = err
		return m, nil
	}
	return m, nil
}

func (m model) afterNodeAdvance() (tea.Model, tea.Cmd) {
	if m.run.Status != engine.RunStatusActive {
		m.profile.MetaCurrency += 1
		if err := m.store.SaveProfile(m.profile); err != nil {
			m.err = err
			return m, nil
		}
		if err := m.store.ClearRun(); err != nil {
			m.err = err
			return m, nil
		}
		m.menuItems = menuItemsForRun(nil)
		m.screen = screenSummary
		return m, nil
	}
	m.resetNodeFlowState()
	m.screen = screenMap
	m.index = 0
	if err := m.saveCheckpoint(); err != nil {
		m.err = err
		return m, nil
	}
	return m, nil
}

func (m *model) startPendingPotionFlow(potionID string, before engine.PlayerState, advance bool, resume screen) bool {
	if potionID == "" || m.run == nil {
		return false
	}
	if len(before.Potions) < engine.EffectivePotionCapacity(m.lib, before) {
		return false
	}
	if len(m.run.Player.Potions) > len(before.Potions) {
		return false
	}
	m.pendingPotionID = potionID
	m.pendingPotionAdvance = advance
	m.pendingPotionResume = resume
	m.screen = screenPotionReplace
	m.index = 0
	if err := m.saveCheckpoint(); err != nil {
		m.err = err
	}
	return true
}

func (m model) finishPendingPotionFlow() (tea.Model, tea.Cmd) {
	advance := m.pendingPotionAdvance
	resume := m.pendingPotionResume
	m.clearPendingPotionState()
	m.index = 0
	if advance {
		return m.afterNodeAdvance()
	}
	m.screen = resume
	if err := m.saveCheckpoint(); err != nil {
		m.err = err
		return m, nil
	}
	return m, nil
}

func choiceByID(choices []content.EventChoiceDef, choiceID string) content.EventChoiceDef {
	for _, choice := range choices {
		if choice.ID == choiceID {
			return choice
		}
	}
	return content.EventChoiceDef{}
}

func (m model) View() string {
	if m.err != nil {
		return m.theme.Panel.Render("错误: " + m.err.Error())
	}
	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 32
	}
	var body string

	switch m.screen {
	case screenMenu:
		body = ui.RenderChoiceScreen(m.theme, "RuneShell", "Terminal deckbuilder", m.menuItems, m.index, "Use arrows to choose, Enter to confirm, q to quit", width)
	case screenClass:
		items := make([]string, 0, len(m.classes))
		for _, class := range m.classes {
			items = append(items, fmt.Sprintf("%s - %s", class.Name, class.Description))
		}
		body = ui.RenderChoiceScreen(m.theme, "Choose Class", string(m.mode), items, m.index, "Enter to start, Esc to go back", width)
	case screenMap:
		body = ui.RenderMap(m.theme, m.run, engine.ReachableNodes(m.run), m.index, width)
	case screenCombat:
		body = ui.RenderCombat(m.theme, m.lib, m.run, m.combat, m.index, m.combatPotionIndex, m.combatPotionMode, m.combatPane, m.combatTopPage, m.combatLogPage, m.combatTarget, width, height)
	case screenPotionReplace:
		body = ui.RenderPotionReplace(m.theme, m.lib, m.run.Player, m.pendingPotionID, m.index, width)
	case screenReward:
		body = ui.RenderReward(m.theme, m.lib, *m.reward, m.index, width)
	case screenEquip:
		body = ui.RenderEquipment(m.theme, m.lib, *m.equipOffer, m.index, width)
	case screenDeckAct:
		body = ui.RenderDeckAction(m.theme, m.lib, m.run, m.deckActionTitle, m.deckActionSubtitle, m.deckActionIndexes, m.index, width)
	case screenEvent:
		body = ui.RenderEvent(m.theme, *m.eventState, m.index, width)
	case screenShop:
		body = ui.RenderShopGrouped(m.theme, m.run, *m.shopState, m.index, width)
	case screenRest:
		body = ui.RenderRest(m.theme, m.run, m.index, m.restLog, width)
	case screenCodex:
		body = ui.RenderCodex(m.theme, m.lib, m.codexTab, m.index, width)
	case screenProfile:
		body = ui.RenderProgression(m.theme, m.lib, m.profile, m.profileTab, m.index, width)
	case screenMultiplayerMenu:
		body = ui.RenderChoiceScreen(m.theme, "Multiplayer", "Create or join a LAN room.", m.multiplayerMenuItems, m.index, "Enter to confirm, Esc to go back", width)
	case screenMultiplayerCreate:
		body = ui.RenderMultiplayerSetup(m.theme, "Create Multiplayer Room", "Confirm your name, class, and port before hosting.", multiplayerCreateLines(m), m.index, multiplayerCreateHelp(m), width)
	case screenMultiplayerJoin:
		body = ui.RenderMultiplayerSetup(m.theme, "Join Multiplayer Room", "Enter the host address, then confirm your name and class.", multiplayerJoinLines(m), m.index, multiplayerJoinHelp(), width)
	case screenMultiplayerRoom:
		actions := multiplayerQuickActions(m.multiplayerSnapshot)
		actionLabels := multiplayerQuickActionLabels(actions)
		if m.multiplayerUsesCombatControls() {
			actionLabels = nil
		}
		body = ui.RenderMultiplayerRoom(m.theme, m.multiplayerSnapshot, actionLabels, m.multiplayerActionIndex, m.multiplayerRoomFocus == multiplayerRoomFocusActions, m.multiplayerCommandInput.Value(), m.multiplayerCombatRenderState(), width, height)
	case screenSummary:
		body = ui.RenderSummary(m.theme, m.run, width)
	}

	footer := m.help.View(m.keys)
	if m.message != "" {
		footer = m.theme.Bad.Render(m.message) + "\n" + footer
	}
	if m.showMapOverlay {
		if m.screen == screenMultiplayerRoom {
			body = ui.RenderMultiplayerMapTreeOverlay(m.theme, m.multiplayerSnapshot, width, height)
		} else {
			body = ui.RenderMapTreeOverlay(m.theme, m.run, m.currentNode, width, height)
		}
	}
	if m.showStatsOverlay {
		title, combatLines, runLines := m.statsOverlayLines()
		body = ui.RenderStatsOverlay(m.theme, title, combatLines, runLines, width, height)
	}
	return body + "\n\n" + footer
}

func (m *model) syncCombatTarget() {
	if m.combat == nil {
		m.combatTarget = engine.CombatTarget{}
		return
	}
	kind := m.currentCombatTargetKind()
	if kind == engine.CombatTargetNone {
		m.combatTarget = engine.CombatTarget{}
		return
	}
	if m.combatTarget.Kind == kind {
		switch kind {
		case engine.CombatTargetEnemy:
			if containsTarget(engine.LivingEnemyTargets(m.combat), m.combatTarget) {
				return
			}
		case engine.CombatTargetAlly:
			if containsTarget(engine.LivingFriendlyTargets(m.combat), m.combatTarget) {
				return
			}
		default:
			return
		}
	}
	m.combatTarget = m.defaultCombatTarget()
}

func (m *model) cycleCombatTarget(delta int) {
	if m.combat == nil {
		return
	}
	kind := m.currentCombatTargetKind()
	if kind == engine.CombatTargetNone {
		return
	}
	m.syncCombatTarget()
	m.combatTarget = engine.CycleCombatTarget(m.combat, m.combatTarget, kind, delta)
}

func (m *model) currentCombatTargetKind() engine.CombatTargetKind {
	if m.combat == nil {
		return engine.CombatTargetNone
	}
	if m.combatPotionMode {
		if len(m.run.Player.Potions) == 0 {
			return engine.CombatTargetNone
		}
		potionIndex := min(m.combatPotionIndex, len(m.run.Player.Potions)-1)
		return engine.PotionTargetKind(m.lib, m.run.Player.Potions[potionIndex])
	}
	if len(m.combat.Hand) == 0 || m.index < 0 || m.index >= len(m.combat.Hand) {
		return engine.CombatTargetNone
	}
	return engine.CardTargetKindForCard(m.lib, m.combat.Hand[m.index])
}

func (m *model) defaultCombatTarget() engine.CombatTarget {
	if m.combat == nil {
		return engine.CombatTarget{}
	}
	if m.combatPotionMode {
		if len(m.run.Player.Potions) == 0 {
			return engine.CombatTarget{}
		}
		potionIndex := min(m.combatPotionIndex, len(m.run.Player.Potions)-1)
		return engine.DefaultTargetForPotion(m.lib, m.combat, 0, m.run.Player.Potions[potionIndex])
	}
	if len(m.combat.Hand) == 0 || m.index < 0 || m.index >= len(m.combat.Hand) {
		return engine.CombatTarget{}
	}
	return engine.DefaultTargetForCard(m.lib, m.combat, m.combat.Hand[m.index])
}

func containsTarget(list []engine.CombatTarget, target engine.CombatTarget) bool {
	for _, item := range list {
		if item.Kind == target.Kind && item.Index == target.Index {
			return true
		}
	}
	return false
}

type tickMsg struct{}

func clearMessage() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

const actionDispatchCooldown = 180 * time.Millisecond

func (m model) nowTime() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m *model) markActionDispatched() {
	m.lastActionAt = m.nowTime()
}

func (m model) actionDispatchBlocked() bool {
	if m.lastActionAt.IsZero() {
		return false
	}
	return m.nowTime().Sub(m.lastActionAt) < actionDispatchCooldown
}

func (m *model) blockRapidAction(message string) (bool, tea.Cmd) {
	if !m.actionDispatchBlocked() {
		return false, nil
	}
	m.message = message
	return true, clearMessage()
}

const pagedListSize = 10

func pageBounds(length, index int) (int, int) {
	if length <= 0 {
		return 0, 0
	}
	index = clampIndex(index, length)
	start := (index / pagedListSize) * pagedListSize
	end := min(length, start+pagedListSize)
	return start, end
}

func moveWithinPage(index, length, delta int) int {
	if length <= 0 {
		return 0
	}
	index = clampIndex(index, length)
	start, end := pageBounds(length, index)
	next := index + delta
	if next < start {
		return start
	}
	if next >= end {
		return end - 1
	}
	return next
}

func moveClamped(index, length, delta int) int {
	if length <= 0 {
		return 0
	}
	return clampIndex(index+delta, length)
}

func flipPage(index, length, direction int) int {
	if length <= 0 {
		return 0
	}
	index = clampIndex(index, length)
	currentPage := index / pagedListSize
	lastPage := (length - 1) / pagedListSize
	nextPage := currentPage + direction
	if nextPage < 0 {
		nextPage = 0
	}
	if nextPage > lastPage {
		nextPage = lastPage
	}
	return clampIndex(nextPage*pagedListSize, length)
}

func clampIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func wrapIndex(index, length int) int {
	if length == 0 {
		return 0
	}
	if index < 0 {
		return length - 1
	}
	if index >= length {
		return 0
	}
	return index
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
