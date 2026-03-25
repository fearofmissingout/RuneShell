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
	menuQuit        = "退出"
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
	Back    key.Binding
	EndTurn key.Binding
	Potion  key.Binding
	Skip    key.Binding
	Leave   key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Select, k.Cycle, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Select, k.Cycle},
		{k.EndTurn, k.Potion, k.Skip, k.Leave},
		{k.Back, k.Help, k.Quit},
	}
}

type model struct {
	width                      int
	height                     int
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
	createName := newMultiplayerInput("Host", "给自己起个房主名，例如 Host", 24)
	createPort := newMultiplayerInput("7777", "局域网端口，例如 7777", 5)
	joinAddr := newMultiplayerInput("127.0.0.1:7777", "输入房主地址，例如 127.0.0.1:7777", 64)
	joinName := newMultiplayerInput("Guest", "给自己起个名字，例如 Guest", 24)
	commandInput := newMultiplayerInput("", "输入联机指令，例如 ready、chat 大家好、node 1", 240)
	commandInput.Width = 64
	return model{
		lib:                       lib,
		store:                     store,
		profile:                   profile,
		run:                       run,
		screen:                    screenMenu,
		theme:                     ui.DefaultTheme(),
		help:                      help.New(),
		menuItems:                 menuItems,
		classes:                   lib.ClassList(),
		multiplayerMenuItems:      []string{"创建房间", "加入房间", "返回主菜单"},
		multiplayerCreateName:     createName,
		multiplayerCreatePort:     createPort,
		multiplayerCreateClass:    classIndexByID(lib.ClassList(), "vanguard"),
		multiplayerCreateForceNew: true,
		multiplayerJoinAddr:       joinAddr,
		multiplayerJoinName:       joinName,
		multiplayerJoinClass:      classIndexByID(lib.ClassList(), "arcanist"),
		multiplayerCommandInput:   commandInput,
		keys: keyMap{
			Left:    key.NewBinding(key.WithKeys("left", "pgup"), key.WithHelp("←/PgUp", "翻前页")),
			Right:   key.NewBinding(key.WithKeys("right", "pgdown"), key.WithHelp("→/PgDn", "翻后页")),
			Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "上移")),
			Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "下移")),
			Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认")),
			Cycle:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "切换")),
			Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "返回")),
			EndTurn: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "结束回合")),
			Potion:  key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "药水")),
			Skip:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "跳过")),
			Leave:   key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "离开")),
			Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "帮助")),
			Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "退出")),
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
	case screenMap, screenCombat, screenReward, screenEquip, screenDeckAct, screenEvent, screenShop, screenRest:
	default:
		return m.run.Checkpoint
	}

	checkpoint := &engine.RunCheckpoint{
		Screen:             string(m.screen),
		RestLog:            append([]string{}, m.restLog...),
		EquipFrom:          m.equipFrom,
		RewardCard:         m.rewardCard,
		ShopOfferID:        m.shopOfferID,
		EventChoice:        m.eventChoice,
		DeckActionMode:     m.deckActionMode,
		DeckActionTitle:    m.deckActionTitle,
		DeckActionSubtitle: m.deckActionSubtitle,
		DeckActionIndexes:  append([]int{}, m.deckActionIndexes...),
		DeckActionPrice:    m.deckActionPrice,
		CombatPane:         m.combatPane,
		CombatTarget:       m.combatTarget,
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
	m.combatPane = cp.CombatPane
	m.combatTarget = cp.CombatTarget
	switch screen(cp.Screen) {
	case screenCombat, screenReward, screenEquip, screenDeckAct, screenEvent, screenShop, screenRest:
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
	m.combatTarget = engine.CombatTarget{}
	m.clearEquipmentFlowState()
	m.clearDeckActionState()
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
		switch m.screen {
		case screenMenu:
			return m.updateMenu(msg)
		case screenClass:
			return m.updateClass(msg)
		case screenMap:
			return m.updateMap(msg)
		case screenCombat:
			return m.updateCombat(msg)
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
		m.message = "联机会话已结束。"
		return m, clearMessage()
	case tickMsg:
		m.message = ""
	}
	return m, nil
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
		if len(m.combat.Hand) > 0 {
			m.index = wrapIndex(m.index-1, len(m.combat.Hand))
			m.syncCombatTarget()
		}
	case key.Matches(msg, m.keys.Down):
		if len(m.combat.Hand) > 0 {
			m.index = wrapIndex(m.index+1, len(m.combat.Hand))
			m.syncCombatTarget()
		}
	case key.Matches(msg, m.keys.Left):
		m.cycleCombatTarget(-1)
	case key.Matches(msg, m.keys.Right):
		m.cycleCombatTarget(1)
	case key.Matches(msg, m.keys.Cycle):
		m.combatPane = (m.combatPane + 1) % engine.CombatInspectPaneCount
	case key.Matches(msg, m.keys.Potion):
		if len(m.run.Player.Potions) > 0 {
			potionID := m.run.Player.Potions[0]
			if err := engine.UsePotionWithTarget(m.lib, m.run.Player, m.combat, potionID, m.combatTarget); err == nil {
				m.run.Player.Potions = m.run.Player.Potions[1:]
				changed = true
			}
		}
	case key.Matches(msg, m.keys.EndTurn):
		engine.EndPlayerTurn(m.lib, m.run.Player, m.combat)
		if !m.combat.Won && !m.combat.Lost {
			engine.StartPlayerTurn(m.lib, m.run.Player, m.combat)
			m.syncCombatTarget()
		}
		changed = true
	case key.Matches(msg, m.keys.Select):
		if len(m.combat.Hand) > 0 {
			if err := engine.PlayCardWithTarget(m.lib, m.run.Player, m.combat, m.index, m.combatTarget); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			if len(m.combat.Hand) > 0 {
				m.index = min(m.index, len(m.combat.Hand)-1)
				m.syncCombatTarget()
			} else {
				m.index = 0
				m.combatTarget = engine.CombatTarget{}
			}
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
		if err := m.saveCheckpoint(); err != nil {
			m.err = err
			return m, nil
		}
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
		if m.reward.EquipmentID != "" {
			return m.startEquipmentFlow("reward", "", "")
		}
		if err := engine.ApplyCombatResultDecision(m.lib, m.run, m.currentNode, m.combat, "", false); err != nil {
			m.err = err
			return m, nil
		}
		return m.afterNodeAdvance()
	case key.Matches(msg, m.keys.Select):
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
			if err := engine.ApplyCombatResultDecision(m.lib, m.run, m.currentNode, m.combat, m.rewardCard, take); err != nil {
				m.err = err
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
			if err := engine.ResolveEventDecision(m.lib, m.run, m.eventState, m.eventChoice, take); err != nil {
				m.err = err
				return m, nil
			}
			if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
				m.err = err
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
		choice := m.eventState.Event.Choices[m.index]
		if engine.EventChoiceEquipmentID(choice) != "" {
			return m.startEquipmentFlow("event", choice.ID, "")
		}
		if err := engine.ResolveEventDecision(m.lib, m.run, m.eventState, choice.ID, true); err != nil {
			m.err = err
			return m, nil
		}
		if err := engine.AdvanceNonCombatNode(m.run, m.currentNode); err != nil {
			m.err = err
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
		if err := engine.ApplyShopPurchase(m.lib, m.run, m.shopState, offer.ID); err != nil {
			m.message = err.Error()
			return m, clearMessage()
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
			m.restLog = []string{"强化了卡牌 " + name}
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
	var (
		title    string
		subtitle string
		indexes  []int
	)
	switch mode {
	case "rest_upgrade":
		title = "选择要强化的卡牌"
		subtitle = "篝火锻造将应用该牌的升级数值"
		indexes = engine.UpgradableCardIndexes(m.lib, m.run.Player.Deck)
	case "shop_remove":
		title = "选择要移除的卡牌"
		subtitle = fmt.Sprintf("本次服务价格 %d 金币", price)
		indexes = make([]int, len(m.run.Player.Deck))
		for i := range m.run.Player.Deck {
			indexes[i] = i
		}
	}
	m.deckActionMode = mode
	m.deckActionTitle = title
	m.deckActionSubtitle = subtitle
	m.deckActionIndexes = indexes
	m.deckActionPrice = price
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
		body = ui.RenderChoiceScreen(m.theme, "RuneShell", "符令迷城 - 终端卡牌肉鸽", m.menuItems, m.index, "方向键选择，回车确认，q 退出", width)
	case screenClass:
		items := make([]string, 0, len(m.classes))
		for _, class := range m.classes {
			items = append(items, fmt.Sprintf("%s - %s", class.Name, class.Description))
		}
		body = ui.RenderChoiceScreen(m.theme, "选择职业", string(m.mode), items, m.index, "回车开始新局，esc 返回", width)
	case screenMap:
		body = ui.RenderMap(m.theme, m.run, engine.ReachableNodes(m.run), m.index, width)
	case screenCombat:
		body = ui.RenderCombat(m.theme, m.lib, m.run, m.combat, m.index, m.combatPane, m.combatTarget, width, height)
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
		body = ui.RenderChoiceScreen(m.theme, "多人模式", "把局域网联机入口放进菜单里。先选创建房间或加入房间，进入后按页面提示填写即可。", m.multiplayerMenuItems, m.index, "回车确认，esc 返回主菜单", width)
	case screenMultiplayerCreate:
		body = ui.RenderMultiplayerSetup(m.theme, "创建多人房间", "先确认你的名字、职业和端口。创建成功后，页面会切到联机会话，其他玩家按同一地址加入即可。", multiplayerCreateLines(m), m.index, multiplayerCreateHelp(m), width)
	case screenMultiplayerJoin:
		body = ui.RenderMultiplayerSetup(m.theme, "加入多人房间", "输入房主给你的地址，确认名字和职业后直接加入。第一次联机也只需要填这三项。", multiplayerJoinLines(m), m.index, multiplayerJoinHelp(), width)
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
	return body + "\n\n" + footer
}

func (m *model) syncCombatTarget() {
	if m.combat == nil || len(m.combat.Hand) == 0 || m.index < 0 || m.index >= len(m.combat.Hand) {
		m.combatTarget = engine.CombatTarget{}
		return
	}
	card := m.combat.Hand[m.index]
	kind := engine.CardTargetKindForCard(m.lib, card)
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
	m.combatTarget = engine.DefaultTargetForCard(m.lib, m.combat, card)
}

func (m *model) cycleCombatTarget(delta int) {
	if m.combat == nil || len(m.combat.Hand) == 0 || m.index < 0 || m.index >= len(m.combat.Hand) {
		return
	}
	card := m.combat.Hand[m.index]
	kind := engine.CardTargetKindForCard(m.lib, card)
	if kind == engine.CombatTargetNone {
		return
	}
	m.syncCombatTarget()
	m.combatTarget = engine.CycleCombatTarget(m.combat, m.combatTarget, kind, delta)
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
