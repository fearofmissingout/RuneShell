package app

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/i18n"
	"cmdcards/internal/netplay"
	"cmdcards/internal/ui"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	multiplayerCreateFieldName = iota
	multiplayerCreateFieldClass
	multiplayerCreateFieldPort
	multiplayerCreateFieldForceNew
	multiplayerCreateFieldLaunch
	multiplayerCreateFieldBack
	multiplayerCreateFieldCount
)

const (
	multiplayerJoinFieldAddr = iota
	multiplayerJoinFieldName
	multiplayerJoinFieldClass
	multiplayerJoinFieldLaunch
	multiplayerJoinFieldBack
	multiplayerJoinFieldCount
)

const (
	multiplayerRoomFocusActions = iota
	multiplayerRoomFocusInspect
	multiplayerRoomFocusInput
)

const (
	multiplayerCombatModeHand = iota
	multiplayerCombatModePotion
)

func newMultiplayerInput(value, placeholder string, limit int) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.SetValue(value)
	input.Placeholder = placeholder
	input.CharLimit = limit
	input.Width = max(18, min(48, max(len(value), len(placeholder))))
	return input
}

func classIndexByID(classes []content.ClassDef, classID string) int {
	for i, class := range classes {
		if class.ID == classID {
			return i
		}
	}
	return 0
}

func (m model) executePendingMultiplayer(lib *content.Library) error {
	_ = lib
	return nil
}

func (m model) shouldTreatQuitKeyAsText(msg tea.KeyMsg) bool {
	return msg.String() == "q" && (m.multiplayerSelectedTextField() || m.screen == screenMultiplayerRoom)
}

func (m model) multiplayerSelectedTextField() bool {
	switch m.screen {
	case screenMultiplayerCreate:
		return m.index == multiplayerCreateFieldName || m.index == multiplayerCreateFieldPort
	case screenMultiplayerJoin:
		return m.index == multiplayerJoinFieldAddr || m.index == multiplayerJoinFieldName
	default:
		return false
	}
}

type multiplayerConnectedMsg struct {
	session  *netplay.Session
	snapshot *netplay.Snapshot
	err      error
}

type multiplayerSnapshotMsg struct {
	snapshot *netplay.Snapshot
}

type multiplayerErrorMsg struct {
	err error
}

type multiplayerQuickAction struct {
	Label    string
	Command  string
	Template bool
	HostOnly bool
}

type multiplayerTargetState struct {
	Kind  engine.CombatTargetKind
	Index int
}

func (m *model) setMultiplayerCreateFocus() {
	m.multiplayerCreateName.Blur()
	m.multiplayerCreatePort.Blur()
	switch m.index {
	case multiplayerCreateFieldName:
		m.multiplayerCreateName.Focus()
	case multiplayerCreateFieldPort:
		m.multiplayerCreatePort.Focus()
	}
}

func (m *model) setMultiplayerJoinFocus() {
	m.multiplayerJoinAddr.Blur()
	m.multiplayerJoinName.Blur()
	switch m.index {
	case multiplayerJoinFieldAddr:
		m.multiplayerJoinAddr.Focus()
	case multiplayerJoinFieldName:
		m.multiplayerJoinName.Focus()
	}
}

func (m model) updateMultiplayerMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, len(m.multiplayerMenuItems), -1)
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, len(m.multiplayerMenuItems), 1)
	case key.Matches(msg, m.keys.Back):
		m.screen = screenMenu
		m.index = 0
	case key.Matches(msg, m.keys.Select):
		switch m.multiplayerMenuItems[m.index] {
		case multiplayerMenuCreate:
			m.screen = screenMultiplayerCreate
			m.index = 0
			m.setMultiplayerCreateFocus()
		case multiplayerMenuJoin:
			m.screen = screenMultiplayerJoin
			m.index = 0
			m.setMultiplayerJoinFocus()
		default:
			m.screen = screenMenu
			m.index = 0
		}
	}
	return m, nil
}

func (m model) updateMultiplayerCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, multiplayerCreateFieldCount, -1)
		m.setMultiplayerCreateFocus()
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, multiplayerCreateFieldCount, 1)
		m.setMultiplayerCreateFocus()
	case key.Matches(msg, m.keys.Back):
		m.screen = screenMultiplayerMenu
		m.index = 0
		m.setMultiplayerCreateFocus()
	case key.Matches(msg, m.keys.Left):
		switch m.index {
		case multiplayerCreateFieldClass:
			m.multiplayerCreateClass = wrapIndex(m.multiplayerCreateClass-1, len(m.classes))
		case multiplayerCreateFieldForceNew:
			m.multiplayerCreateForceNew = !m.multiplayerCreateForceNew
		}
	case key.Matches(msg, m.keys.Right):
		switch m.index {
		case multiplayerCreateFieldClass:
			m.multiplayerCreateClass = wrapIndex(m.multiplayerCreateClass+1, len(m.classes))
		case multiplayerCreateFieldForceNew:
			m.multiplayerCreateForceNew = !m.multiplayerCreateForceNew
		}
	case key.Matches(msg, m.keys.Select):
		switch m.index {
		case multiplayerCreateFieldName, multiplayerCreateFieldPort:
			m.index = min(multiplayerCreateFieldCount-1, m.index+1)
			m.setMultiplayerCreateFocus()
		case multiplayerCreateFieldClass:
			m.multiplayerCreateClass = wrapIndex(m.multiplayerCreateClass+1, len(m.classes))
		case multiplayerCreateFieldForceNew:
			m.multiplayerCreateForceNew = !m.multiplayerCreateForceNew
		case multiplayerCreateFieldLaunch:
			return m.launchMultiplayerHost()
		case multiplayerCreateFieldBack:
			m.screen = screenMultiplayerMenu
			m.index = 0
		}
	default:
		var cmd tea.Cmd
		switch m.index {
		case multiplayerCreateFieldName:
			m.multiplayerCreateName, cmd = m.multiplayerCreateName.Update(msg)
			return m, cmd
		case multiplayerCreateFieldPort:
			m.multiplayerCreatePort, cmd = m.multiplayerCreatePort.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) updateMultiplayerJoin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, multiplayerJoinFieldCount, -1)
		m.setMultiplayerJoinFocus()
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, multiplayerJoinFieldCount, 1)
		m.setMultiplayerJoinFocus()
	case key.Matches(msg, m.keys.Back):
		m.screen = screenMultiplayerMenu
		m.index = 0
		m.setMultiplayerJoinFocus()
	case key.Matches(msg, m.keys.Left):
		if m.index == multiplayerJoinFieldClass {
			m.multiplayerJoinClass = wrapIndex(m.multiplayerJoinClass-1, len(m.classes))
		}
	case key.Matches(msg, m.keys.Right):
		if m.index == multiplayerJoinFieldClass {
			m.multiplayerJoinClass = wrapIndex(m.multiplayerJoinClass+1, len(m.classes))
		}
	case key.Matches(msg, m.keys.Select):
		switch m.index {
		case multiplayerJoinFieldAddr, multiplayerJoinFieldName:
			m.index = min(multiplayerJoinFieldCount-1, m.index+1)
			m.setMultiplayerJoinFocus()
		case multiplayerJoinFieldClass:
			m.multiplayerJoinClass = wrapIndex(m.multiplayerJoinClass+1, len(m.classes))
		case multiplayerJoinFieldLaunch:
			return m.launchMultiplayerJoin()
		case multiplayerJoinFieldBack:
			m.screen = screenMultiplayerMenu
			m.index = 0
		}
	default:
		var cmd tea.Cmd
		switch m.index {
		case multiplayerJoinFieldAddr:
			m.multiplayerJoinAddr, cmd = m.multiplayerJoinAddr.Update(msg)
			return m, cmd
		case multiplayerJoinFieldName:
			m.multiplayerJoinName, cmd = m.multiplayerJoinName.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) launchMultiplayerHost() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.multiplayerCreateName.Value())
	if name == "" {
		m.message = "Please enter your name first so other players can recognize you in the room."
		return m, clearMessage()
	}
	portText := strings.TrimSpace(m.multiplayerCreatePort.Value())
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		m.message = "Port must be a number between 1 and 65535, for example 7777."
		return m, clearMessage()
	}
	class := m.classes[m.multiplayerCreateClass]
	m.multiplayerConnecting = true
	m.message = "Creating the room and connecting..."
	return m, startHostedSessionCmd(m.lib, port, name, class.ID, m.multiplayerCreateForceNew)
}

func (m model) launchMultiplayerJoin() (tea.Model, tea.Cmd) {
	addr := strings.TrimSpace(m.multiplayerJoinAddr.Value())
	if addr == "" {
		m.message = "Please enter a room address first, for example 127.0.0.1:7777."
		return m, clearMessage()
	}
	name := strings.TrimSpace(m.multiplayerJoinName.Value())
	if name == "" {
		m.message = "Please enter your name first. Use the same name again if you reconnect."
		return m, clearMessage()
	}
	class := m.classes[m.multiplayerJoinClass]
	m.multiplayerConnecting = true
	m.message = "Joining the room..."
	return m, startJoinedSessionCmd(addr, name, class.ID)
}

func multiplayerCreateLines(m model) []string {
	class := m.classes[m.multiplayerCreateClass]
	forceNew := m.theme.Text("multiplayer.create.force_new.yes")
	if !m.multiplayerCreateForceNew {
		forceNew = m.theme.Text("multiplayer.create.force_new.no")
	}
	return []string{
		m.theme.Textf("multiplayer.create.name", i18n.Args{"value": displayInputValue(m.theme, m.multiplayerCreateName)}),
		m.theme.Textf("multiplayer.create.class", i18n.Args{"name": class.Name, "id": class.ID}),
		m.theme.Textf("multiplayer.create.port", i18n.Args{"value": displayInputValue(m.theme, m.multiplayerCreatePort)}),
		m.theme.Textf("multiplayer.create.force_new", i18n.Args{"value": forceNew}),
		m.theme.Text("multiplayer.create.launch"),
		m.theme.Text("multiplayer.back"),
	}
}

func multiplayerJoinLines(m model) []string {
	class := m.classes[m.multiplayerJoinClass]
	return []string{
		m.theme.Textf("multiplayer.join.addr", i18n.Args{"value": displayInputValue(m.theme, m.multiplayerJoinAddr)}),
		m.theme.Textf("multiplayer.join.name", i18n.Args{"value": displayInputValue(m.theme, m.multiplayerJoinName)}),
		m.theme.Textf("multiplayer.join.class", i18n.Args{"name": class.Name, "id": class.ID}),
		m.theme.Text("multiplayer.join.launch"),
		m.theme.Text("multiplayer.back"),
	}
}

func displayInputValue(theme ui.Theme, input textinput.Model) string {
	value := strings.TrimSpace(input.Value())
	if value != "" {
		return value
	}
	if input.Placeholder != "" {
		return theme.Textf("multiplayer.value.placeholder", i18n.Args{"value": input.Placeholder})
	}
	return theme.Text("multiplayer.value.empty")
}

func multiplayerCreateHelp(m model) []string {
	return []string{
		m.theme.Text("multiplayer.create.help.1"),
		m.theme.Text("multiplayer.create.help.2"),
		m.theme.Text("multiplayer.create.help.3"),
		m.theme.Textf("multiplayer.create.help.4", i18n.Args{"value": multiplayerAddressSummary(strings.TrimSpace(m.multiplayerCreatePort.Value()))}),
		m.theme.Text("multiplayer.create.help.5"),
	}
}

func multiplayerJoinHelp(m model) []string {
	return []string{
		m.theme.Text("multiplayer.join.help.1"),
		m.theme.Text("multiplayer.join.help.2"),
		m.theme.Text("multiplayer.join.help.3"),
		m.theme.Text("multiplayer.join.help.4"),
	}
}

func startHostedSessionCmd(lib *content.Library, port int, name, classID string, forceNew bool) tea.Cmd {
	return func() tea.Msg {
		session, err := netplay.StartHostedSession(lib, port, name, classID, forceNew)
		if err != nil {
			return multiplayerConnectedMsg{err: err}
		}
		return multiplayerConnectedMsg{session: session, snapshot: session.CurrentSnapshot()}
	}
}

func startJoinedSessionCmd(addr, name, classID string) tea.Cmd {
	return func() tea.Msg {
		session, err := netplay.StartJoinedSession(addr, name, classID)
		if err != nil {
			return multiplayerConnectedMsg{err: err}
		}
		return multiplayerConnectedMsg{session: session, snapshot: session.CurrentSnapshot()}
	}
}

func waitForMultiplayerUpdate(session *netplay.Session) tea.Cmd {
	return func() tea.Msg {
		select {
		case snap := <-session.Snapshots():
			return multiplayerSnapshotMsg{snapshot: snap}
		case err := <-session.Errors():
			return multiplayerErrorMsg{err: err}
		}
	}
}

func (m *model) closeMultiplayerSession() {
	if m.multiplayerSession != nil {
		_ = m.multiplayerSession.Close()
	}
	m.multiplayerSession = nil
	m.multiplayerSnapshot = nil
	m.multiplayerConnecting = false
	m.multiplayerActionIndex = 0
	m.multiplayerRoomFocus = multiplayerRoomFocusActions
	m.multiplayerCombatMode = multiplayerCombatModeHand
	m.multiplayerCombatIndex = 0
	m.multiplayerPotionIndex = 0
	m.multiplayerCombatTarget = multiplayerTargetState{}
	m.multiplayerStructuredIndex = 0
	m.multiplayerInspectPane = 0
	m.multiplayerCombatTopPage = 0
	m.multiplayerInspectLogPage = 0
	m.multiplayerCommandInput.SetValue("")
	m.multiplayerCommandInput.Focus()
	m.showMapOverlay = false
	m.showStatsOverlay = false
}

func (m model) multiplayerUsesCombatControls() bool {
	return m.multiplayerSnapshot != nil && m.multiplayerSnapshot.Combat != nil
}

func (m model) multiplayerUsesInspectFocus() bool {
	return m.multiplayerUsesCombatControls()
}

func (m model) multiplayerUsesStructuredControls() bool {
	if m.multiplayerSnapshot == nil {
		return false
	}
	return m.multiplayerSnapshot.Map != nil || m.multiplayerSnapshot.Combat != nil || m.multiplayerSnapshot.Reward != nil || m.multiplayerSnapshot.Event != nil || m.multiplayerSnapshot.Shop != nil || m.multiplayerSnapshot.Rest != nil || m.multiplayerSnapshot.Equipment != nil || m.multiplayerSnapshot.Deck != nil || m.multiplayerSnapshot.Summary != nil
}

func (m model) multiplayerStructuredOptionCount() int {
	if m.multiplayerSnapshot == nil {
		return 0
	}
	switch {
	case m.multiplayerSnapshot.Map != nil:
		return len(m.multiplayerSnapshot.Map.Reachable)
	case m.multiplayerSnapshot.Reward != nil:
		return len(m.multiplayerSnapshot.Reward.Cards)
	case m.multiplayerSnapshot.Event != nil:
		return len(m.multiplayerSnapshot.Event.Choices)
	case m.multiplayerSnapshot.Shop != nil:
		return len(m.multiplayerSnapshot.Shop.Offers)
	case m.multiplayerSnapshot.Rest != nil:
		return 2
	case m.multiplayerSnapshot.Equipment != nil:
		return 2
	case m.multiplayerSnapshot.Deck != nil:
		return len(m.multiplayerSnapshot.Deck.Cards)
	case m.multiplayerSnapshot.Summary != nil:
		return 2
	default:
		return 0
	}
}

func (m *model) syncMultiplayerStructuredSelection() {
	if !m.multiplayerUsesStructuredControls() {
		m.multiplayerStructuredIndex = 0
		return
	}
	count := m.multiplayerStructuredOptionCount()
	if count <= 0 {
		m.multiplayerStructuredIndex = 0
		return
	}
	m.multiplayerStructuredIndex = clampMultiplayerActionIndex(m.multiplayerStructuredIndex, count)
}

func (m *model) syncMultiplayerCombatSelection() {
	if !m.multiplayerUsesCombatControls() {
		m.multiplayerCombatMode = multiplayerCombatModeHand
		m.multiplayerCombatIndex = 0
		m.multiplayerPotionIndex = 0
		m.multiplayerCombatTarget = multiplayerTargetState{}
		m.multiplayerCombatTopPage = 0
		return
	}
	combat := m.multiplayerSnapshot.Combat
	if len(combat.Potions) == 0 && m.multiplayerCombatMode == multiplayerCombatModePotion {
		m.multiplayerCombatMode = multiplayerCombatModeHand
	}
	if len(combat.Hand) == 0 && len(combat.Potions) > 0 {
		m.multiplayerCombatMode = multiplayerCombatModePotion
	}
	if len(combat.Hand) == 0 && len(combat.Potions) == 0 {
		m.multiplayerCombatTarget = multiplayerTargetState{}
		return
	}
	if len(combat.Hand) > 0 {
		m.multiplayerCombatIndex = clampMultiplayerActionIndex(m.multiplayerCombatIndex, len(combat.Hand))
	} else {
		m.multiplayerCombatIndex = 0
	}
	if len(combat.Potions) > 0 {
		m.multiplayerPotionIndex = clampMultiplayerActionIndex(m.multiplayerPotionIndex, len(combat.Potions))
	} else {
		m.multiplayerPotionIndex = 0
	}
	m.syncMultiplayerCombatTarget()
}

func (m *model) syncMultiplayerCombatTarget() {
	if !m.multiplayerUsesCombatControls() {
		m.multiplayerCombatTarget = multiplayerTargetState{}
		return
	}
	kind := m.multiplayerCurrentCombatTargetKind()
	if kind == engine.CombatTargetNone {
		m.multiplayerCombatTarget = multiplayerTargetState{}
		return
	}
	if m.multiplayerCombatTarget.Kind == kind && m.multiplayerCombatTargetValid(kind, m.multiplayerCombatTarget.Index) {
		return
	}
	m.multiplayerCombatTarget = m.multiplayerDefaultCombatTarget(kind)
}

func (m model) multiplayerCombatTargetValid(kind engine.CombatTargetKind, index int) bool {
	if !m.multiplayerUsesCombatControls() {
		return false
	}
	combat := m.multiplayerSnapshot.Combat
	switch kind {
	case engine.CombatTargetEnemy:
		for _, enemy := range combat.Enemies {
			if enemy.Index == index && enemy.HP > 0 {
				return true
			}
		}
	case engine.CombatTargetAlly:
		for _, actor := range combat.Party {
			if actor.Index == index && actor.HP > 0 {
				return true
			}
		}
	case engine.CombatTargetEnemies, engine.CombatTargetAllies:
		return true
	}
	return false
}

func (m model) multiplayerCurrentCombatTargetKind() engine.CombatTargetKind {
	if !m.multiplayerUsesCombatControls() {
		return engine.CombatTargetNone
	}
	if m.multiplayerCombatMode == multiplayerCombatModePotion {
		return engine.CombatTargetAlly
	}
	combat := m.multiplayerSnapshot.Combat
	if len(combat.Hand) == 0 {
		return engine.CombatTargetNone
	}
	card := combat.Hand[clampMultiplayerActionIndex(m.multiplayerCombatIndex, len(combat.Hand))]
	return multiplayerTargetKindFromHint(card.TargetHint)
}

func multiplayerTargetKindFromHint(hint string) engine.CombatTargetKind {
	hint = strings.ToLower(strings.TrimSpace(hint))
	switch hint {
	case "single enemy":
		return engine.CombatTargetEnemy
	case "all enemies":
		return engine.CombatTargetEnemies
	case "single ally":
		return engine.CombatTargetAlly
	case "all allies":
		return engine.CombatTargetAllies
	default:
		return engine.CombatTargetNone
	}
}

func (m model) multiplayerDefaultCombatTarget(kind engine.CombatTargetKind) multiplayerTargetState {
	if !m.multiplayerUsesCombatControls() {
		return multiplayerTargetState{}
	}
	combat := m.multiplayerSnapshot.Combat
	switch kind {
	case engine.CombatTargetEnemy:
		for _, enemy := range combat.Enemies {
			if enemy.HP > 0 {
				return multiplayerTargetState{Kind: kind, Index: enemy.Index}
			}
		}
	case engine.CombatTargetAlly:
		if combat != nil && combat.Party != nil {
			for _, actor := range combat.Party {
				if actor.Index == m.multiplayerSnapshot.Seat && actor.HP > 0 {
					return multiplayerTargetState{Kind: kind, Index: actor.Index}
				}
			}
		}
		for _, actor := range combat.Party {
			if actor.HP > 0 {
				return multiplayerTargetState{Kind: kind, Index: actor.Index}
			}
		}
	case engine.CombatTargetEnemies, engine.CombatTargetAllies:
		return multiplayerTargetState{Kind: kind}
	}
	return multiplayerTargetState{}
}

func (m *model) cycleMultiplayerCombatTarget(delta int) {
	kind := m.multiplayerCurrentCombatTargetKind()
	if kind == engine.CombatTargetNone || !m.multiplayerUsesCombatControls() {
		return
	}
	combat := m.multiplayerSnapshot.Combat
	switch kind {
	case engine.CombatTargetEnemy:
		choices := []int{}
		for _, enemy := range combat.Enemies {
			if enemy.HP > 0 {
				choices = append(choices, enemy.Index)
			}
		}
		m.multiplayerCombatTarget = multiplayerTargetState{Kind: kind, Index: cycleMultiplayerTargetIndex(choices, m.multiplayerCombatTarget.Index, delta)}
	case engine.CombatTargetAlly:
		choices := []int{}
		for _, actor := range combat.Party {
			if actor.HP > 0 {
				choices = append(choices, actor.Index)
			}
		}
		m.multiplayerCombatTarget = multiplayerTargetState{Kind: kind, Index: cycleMultiplayerTargetIndex(choices, m.multiplayerCombatTarget.Index, delta)}
	case engine.CombatTargetEnemies, engine.CombatTargetAllies:
		m.multiplayerCombatTarget = multiplayerTargetState{Kind: kind}
	}
}

func cycleMultiplayerTargetIndex(choices []int, current int, delta int) int {
	if len(choices) == 0 {
		return 0
	}
	position := 0
	for i, value := range choices {
		if value == current {
			position = i
			break
		}
	}
	position = wrapIndex(position+delta, len(choices))
	return choices[position]
}

func (m model) multiplayerCombatTargetLabel() string {
	if !m.multiplayerUsesCombatControls() {
		return ""
	}
	state := m.multiplayerCombatTarget
	combat := m.multiplayerSnapshot.Combat
	switch state.Kind {
	case engine.CombatTargetEnemy:
		for _, enemy := range combat.Enemies {
			if enemy.Index == state.Index {
				return fmt.Sprintf("敌人 %d %s", enemy.Index, enemy.Name)
			}
		}
		return fmt.Sprintf("敌人 %d", state.Index)
	case engine.CombatTargetAlly:
		for _, actor := range combat.Party {
			if actor.Index == state.Index {
				if actor.Index == 1 {
					return fmt.Sprintf("自己 %s", actor.Name)
				}
				return fmt.Sprintf("队友 %d %s", actor.Index, actor.Name)
			}
		}
		return fmt.Sprintf("队友 %d", state.Index)
	case engine.CombatTargetEnemies:
		return "全体敌人"
	case engine.CombatTargetAllies:
		return "全体队友"
	default:
		return "无需目标"
	}
}

func (m model) multiplayerCombatRenderState() ui.MultiplayerCombatState {
	state := ui.MultiplayerCombatState{}
	if !m.multiplayerUsesStructuredControls() {
		return state
	}
	state.Enabled = m.multiplayerUsesCombatControls()
	state.OperationsFocused = m.multiplayerRoomFocus == multiplayerRoomFocusActions
	state.InspectFocused = m.multiplayerRoomFocus == multiplayerRoomFocusInspect
	state.ChatFocused = m.multiplayerRoomFocus == multiplayerRoomFocusInput
	state.SelectedIndex = m.multiplayerStructuredIndex
	state.InspectPane = m.multiplayerInspectPane
	state.TopPage = m.multiplayerCombatTopPage
	state.InspectLogPage = m.multiplayerInspectLogPage
	state.Phase = multiplayerSnapshotPhase(m.multiplayerSnapshot)
	state.SelectionLabel = m.multiplayerStructuredSelectionLabel()
	if m.multiplayerCombatMode == multiplayerCombatModePotion {
		state.ModeLabel = "药水"
	} else {
		state.ModeLabel = "手牌"
	}
	state.SelectedCard = m.multiplayerCombatIndex
	state.SelectedPotion = m.multiplayerPotionIndex
	state.TargetKind = string(m.multiplayerCombatTarget.Kind)
	state.TargetIndex = m.multiplayerCombatTarget.Index
	state.TargetLabel = m.multiplayerCombatTargetLabel()
	return state
}

func (m model) multiplayerStructuredSelectionLabel() string {
	if m.multiplayerSnapshot == nil {
		return ""
	}
	index := m.multiplayerStructuredIndex
	switch {
	case m.multiplayerSnapshot.Map != nil && len(m.multiplayerSnapshot.Map.Reachable) > 0:
		node := m.multiplayerSnapshot.Map.Reachable[clampMultiplayerActionIndex(index, len(m.multiplayerSnapshot.Map.Reachable))]
		return fmt.Sprintf("节点 %d: %s", node.Index, node.Label)
	case m.multiplayerSnapshot.Reward != nil && len(m.multiplayerSnapshot.Reward.Cards) > 0:
		card := m.multiplayerSnapshot.Reward.Cards[clampMultiplayerActionIndex(index, len(m.multiplayerSnapshot.Reward.Cards))]
		return fmt.Sprintf("奖励卡 %s", card.Name)
	case m.multiplayerSnapshot.Event != nil && len(m.multiplayerSnapshot.Event.Choices) > 0:
		choice := m.multiplayerSnapshot.Event.Choices[clampMultiplayerActionIndex(index, len(m.multiplayerSnapshot.Event.Choices))]
		return fmt.Sprintf("事件选项 %s", choice.Label)
	case m.multiplayerSnapshot.Shop != nil && len(m.multiplayerSnapshot.Shop.Offers) > 0:
		offer := m.multiplayerSnapshot.Shop.Offers[clampMultiplayerActionIndex(index, len(m.multiplayerSnapshot.Shop.Offers))]
		return fmt.Sprintf("商店商品 %s", offer.Name)
	case m.multiplayerSnapshot.Rest != nil:
		if index == 1 {
			return "营火强化"
		}
		return "营火休息"
	case m.multiplayerSnapshot.Equipment != nil:
		if index == 1 {
			return "跳过装备"
		}
		return "装备候选物品"
	case m.multiplayerSnapshot.Deck != nil:
		if len(m.multiplayerSnapshot.Deck.Cards) == 0 {
			return "返回上一层"
		}
		card := m.multiplayerSnapshot.Deck.Cards[clampMultiplayerActionIndex(index, len(m.multiplayerSnapshot.Deck.Cards))]
		return fmt.Sprintf("卡牌 %s", card.Name)
	case m.multiplayerSnapshot.Summary != nil:
		if index == 1 {
			return "结束房间"
		}
		return "开始下一局"
	default:
		return ""
	}
}

func (m model) buildSelectedMultiplayerCombatCommand() (*netplay.Command, error) {
	if !m.multiplayerUsesCombatControls() {
		return nil, fmt.Errorf("当前不在联机战斗阶段")
	}
	combat := m.multiplayerSnapshot.Combat
	if m.multiplayerCombatMode == multiplayerCombatModePotion {
		if len(combat.Potions) == 0 {
			return nil, fmt.Errorf("当前没有可用药水")
		}
		cmd := &netplay.Command{Action: "potion", ItemIndex: m.multiplayerPotionIndex + 1}
		kind := m.multiplayerCurrentCombatTargetKind()
		if kind == engine.CombatTargetAlly {
			cmd.TargetKind = string(kind)
			cmd.TargetIndex = m.multiplayerCombatTarget.Index
		}
		return cmd, nil
	}
	if len(combat.Hand) == 0 {
		return nil, fmt.Errorf("当前没有可打出的手牌")
	}
	card := combat.Hand[clampMultiplayerActionIndex(m.multiplayerCombatIndex, len(combat.Hand))]
	cmd := &netplay.Command{Action: "play", CardIndex: card.Index}
	kind := m.multiplayerCurrentCombatTargetKind()
	if kind != engine.CombatTargetNone {
		cmd.TargetKind = string(kind)
		if kind == engine.CombatTargetEnemy || kind == engine.CombatTargetAlly {
			cmd.TargetIndex = m.multiplayerCombatTarget.Index
		}
	}
	return cmd, nil
}

func (m model) buildSelectedStructuredMultiplayerCommand() (*netplay.Command, error) {
	if !m.multiplayerUsesStructuredControls() {
		return nil, fmt.Errorf("当前阶段还没有可直接执行的结构化操作")
	}
	if m.multiplayerUsesCombatControls() {
		return m.buildSelectedMultiplayerCombatCommand()
	}
	index := m.multiplayerStructuredIndex
	switch {
	case m.multiplayerSnapshot.Map != nil:
		if len(m.multiplayerSnapshot.Map.Reachable) == 0 {
			return nil, fmt.Errorf("当前没有可前往的节点")
		}
		return &netplay.Command{Action: "node", ItemIndex: index + 1}, nil
	case m.multiplayerSnapshot.Reward != nil:
		if len(m.multiplayerSnapshot.Reward.Cards) == 0 {
			return nil, fmt.Errorf("当前没有可领取的奖励卡")
		}
		return &netplay.Command{Action: "take", ItemIndex: index + 1}, nil
	case m.multiplayerSnapshot.Event != nil:
		if len(m.multiplayerSnapshot.Event.Choices) == 0 {
			return nil, fmt.Errorf("当前没有可选事件")
		}
		return &netplay.Command{Action: "choose", ItemIndex: index + 1}, nil
	case m.multiplayerSnapshot.Shop != nil:
		if len(m.multiplayerSnapshot.Shop.Offers) == 0 {
			return nil, fmt.Errorf("当前没有可购买的商品")
		}
		return &netplay.Command{Action: "buy", ItemIndex: index + 1}, nil
	case m.multiplayerSnapshot.Rest != nil:
		action := "heal"
		if index == 1 {
			action = "upgrade"
		}
		return &netplay.Command{Action: action}, nil
	case m.multiplayerSnapshot.Equipment != nil:
		action := "take"
		if index == 1 {
			action = "skip"
		}
		return &netplay.Command{Action: action}, nil
	case m.multiplayerSnapshot.Deck != nil:
		if len(m.multiplayerSnapshot.Deck.Cards) == 0 {
			return &netplay.Command{Action: "back"}, nil
		}
		return &netplay.Command{Action: "choose", ItemIndex: index + 1}, nil
	case m.multiplayerSnapshot.Summary != nil:
		action := "new"
		if index == 1 {
			action = "abandon"
		}
		return &netplay.Command{Action: action}, nil
	default:
		return nil, fmt.Errorf("当前阶段暂不支持结构化操作")
	}
}

func (m model) sendMultiplayerCommand(cmd *netplay.Command) (tea.Model, tea.Cmd) {
	if cmd == nil {
		return m, nil
	}
	if blocked, clear := m.blockRapidAction("操作过快，请稍候再提交。"); blocked {
		return m, clear
	}
	if m.multiplayerSession == nil {
		m.message = "房间连接尚未就绪，请稍后再试。"
		return m, clearMessage()
	}
	if err := m.multiplayerSession.Send(cmd); err != nil {
		m.message = err.Error()
		return m, clearMessage()
	}
	m.markActionDispatched()
	return m, nil
}

func (m model) updateMultiplayerCombatControls(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.multiplayerCombatMode == multiplayerCombatModePotion {
			if len(m.multiplayerSnapshot.Combat.Potions) > 0 {
				m.multiplayerPotionIndex = wrapIndex(m.multiplayerPotionIndex-1, len(m.multiplayerSnapshot.Combat.Potions))
				m.syncMultiplayerCombatTarget()
			}
			return m, nil
		}
		if len(m.multiplayerSnapshot.Combat.Hand) > 0 {
			m.multiplayerCombatIndex = wrapIndex(m.multiplayerCombatIndex-1, len(m.multiplayerSnapshot.Combat.Hand))
			m.syncMultiplayerCombatTarget()
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.multiplayerCombatMode == multiplayerCombatModePotion {
			if len(m.multiplayerSnapshot.Combat.Potions) > 0 {
				m.multiplayerPotionIndex = wrapIndex(m.multiplayerPotionIndex+1, len(m.multiplayerSnapshot.Combat.Potions))
				m.syncMultiplayerCombatTarget()
			}
			return m, nil
		}
		if len(m.multiplayerSnapshot.Combat.Hand) > 0 {
			m.multiplayerCombatIndex = wrapIndex(m.multiplayerCombatIndex+1, len(m.multiplayerSnapshot.Combat.Hand))
			m.syncMultiplayerCombatTarget()
		}
		return m, nil
	case key.Matches(msg, m.keys.Left):
		m.cycleMultiplayerCombatTarget(-1)
		return m, nil
	case key.Matches(msg, m.keys.Right):
		m.cycleMultiplayerCombatTarget(1)
		return m, nil
	case msg.String() == "[":
		if m.multiplayerCombatTopPage > 0 {
			m.multiplayerCombatTopPage--
		}
		return m, nil
	case msg.String() == "]":
		m.multiplayerCombatTopPage++
		return m, nil
	case key.Matches(msg, m.keys.Potion):
		if len(m.multiplayerSnapshot.Combat.Potions) == 0 {
			m.message = "当前没有药水可用。"
			return m, clearMessage()
		}
		if m.multiplayerCombatMode == multiplayerCombatModePotion {
			m.multiplayerCombatMode = multiplayerCombatModeHand
		} else {
			m.multiplayerCombatMode = multiplayerCombatModePotion
		}
		m.syncMultiplayerCombatTarget()
		return m, nil
	case key.Matches(msg, m.keys.EndTurn):
		return m.sendMultiplayerCommand(&netplay.Command{Action: "end"})
	case key.Matches(msg, m.keys.Select):
		cmd, err := m.buildSelectedMultiplayerCombatCommand()
		if err != nil {
			m.message = err.Error()
			return m, clearMessage()
		}
		return m.sendMultiplayerCommand(cmd)
	default:
		return m, nil
	}
}

func (m model) updateMultiplayerStructuredControls(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	count := m.multiplayerStructuredOptionCount()
	switch {
	case key.Matches(msg, m.keys.Up):
		if count > 0 {
			m.multiplayerStructuredIndex = wrapIndex(m.multiplayerStructuredIndex-1, count)
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if count > 0 {
			m.multiplayerStructuredIndex = wrapIndex(m.multiplayerStructuredIndex+1, count)
		}
		return m, nil
	case key.Matches(msg, m.keys.Left):
		if m.multiplayerSnapshot != nil && (m.multiplayerSnapshot.Shop != nil || m.multiplayerSnapshot.Deck != nil) {
			m.multiplayerStructuredIndex = flipPage(m.multiplayerStructuredIndex, count, -1)
		}
		return m, nil
	case key.Matches(msg, m.keys.Right):
		if m.multiplayerSnapshot != nil && (m.multiplayerSnapshot.Shop != nil || m.multiplayerSnapshot.Deck != nil) {
			m.multiplayerStructuredIndex = flipPage(m.multiplayerStructuredIndex, count, 1)
		}
		return m, nil
	case key.Matches(msg, m.keys.Skip):
		if m.multiplayerSnapshot != nil && m.multiplayerSnapshot.Reward != nil {
			return m.sendMultiplayerCommand(&netplay.Command{Action: "skip"})
		}
		return m, nil
	case key.Matches(msg, m.keys.Leave):
		if m.multiplayerSnapshot != nil && m.multiplayerSnapshot.Shop != nil {
			return m.sendMultiplayerCommand(&netplay.Command{Action: "leave"})
		}
		return m, nil
	case key.Matches(msg, m.keys.Back):
		if m.multiplayerSnapshot != nil && m.multiplayerSnapshot.Shop != nil {
			return m.sendMultiplayerCommand(&netplay.Command{Action: "leave"})
		}
		if m.multiplayerSnapshot != nil && m.multiplayerSnapshot.Deck != nil {
			return m.sendMultiplayerCommand(&netplay.Command{Action: "back"})
		}
		return m, nil
	case key.Matches(msg, m.keys.Select):
		cmd, err := m.buildSelectedStructuredMultiplayerCommand()
		if err != nil {
			m.message = err.Error()
			return m, clearMessage()
		}
		return m.sendMultiplayerCommand(cmd)
	default:
		return m, nil
	}
}

func (m model) updateMultiplayerInspectControls(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "[":
		if m.multiplayerCombatTopPage > 0 {
			m.multiplayerCombatTopPage--
		}
	case msg.String() == "]":
		m.multiplayerCombatTopPage++
	case msg.String() == ",":
		if m.multiplayerInspectPane == 3 {
			m.multiplayerInspectLogPage++
		}
	case msg.String() == ".":
		if m.multiplayerInspectPane == 3 && m.multiplayerInspectLogPage > 0 {
			m.multiplayerInspectLogPage--
		}
	case msg.String() >= "1" && msg.String() <= "5":
		m.multiplayerInspectPane = int(msg.String()[0] - '1')
		m.multiplayerCombatTopPage = 0
		m.multiplayerInspectLogPage = 0
	case key.Matches(msg, m.keys.Left), key.Matches(msg, m.keys.Up):
		m.multiplayerInspectPane = wrapIndex(m.multiplayerInspectPane-1, ui.MultiplayerCombatInspectPaneCount)
		m.multiplayerCombatTopPage = 0
		m.multiplayerInspectLogPage = 0
	case key.Matches(msg, m.keys.Right), key.Matches(msg, m.keys.Down):
		m.multiplayerInspectPane = wrapIndex(m.multiplayerInspectPane+1, ui.MultiplayerCombatInspectPaneCount)
		m.multiplayerCombatTopPage = 0
		m.multiplayerInspectLogPage = 0
	}
	return m, nil
}

func multiplayerQuickActions(snapshot *netplay.Snapshot) []multiplayerQuickAction {
	if snapshot == nil {
		return nil
	}
	actions := make([]multiplayerQuickAction, 0, len(snapshot.Examples)+len(snapshot.Commands))
	seen := map[string]struct{}{}
	appendUnique := func(items []string) {
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			hostOnly := isHostOnlyMultiplayerAction(snapshot, item)
			actions = append(actions, multiplayerQuickAction{
				Label:    formatMultiplayerActionLabel(snapshot, describeMultiplayerAction(snapshot, item), hostOnly),
				Command:  item,
				Template: isTemplateMultiplayerAction(item),
				HostOnly: hostOnly,
			})
		}
	}
	appendUnique(snapshot.Examples)
	appendUnique(snapshot.Commands)
	return actions
}

func multiplayerQuickActionLabels(actions []multiplayerQuickAction) []string {
	labels := make([]string, 0, len(actions))
	for _, action := range actions {
		labels = append(labels, action.Label)
	}
	return labels
}

func isTemplateMultiplayerAction(action string) bool {
	return strings.Contains(action, "<") || strings.Contains(action, "[")
}

func isMultiplayerHost(snapshot *netplay.Snapshot) bool {
	if snapshot == nil {
		return false
	}
	return snapshot.SelfID != "" && snapshot.SelfID == snapshot.HostID
}

func formatMultiplayerActionLabel(snapshot *netplay.Snapshot, label string, hostOnly bool) string {
	if !hostOnly || strings.TrimSpace(label) == "" {
		return label
	}
	if isMultiplayerHost(snapshot) {
		return "Host only: " + label
	}
	return "Host only (locked): " + label
}

func isHostOnlyMultiplayerAction(snapshot *netplay.Snapshot, command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return false
	}
	verb := strings.ToLower(fields[0])
	switch multiplayerSnapshotPhase(snapshot) {
	case "lobby":
		switch verb {
		case "start", "mode", "seed", "drop", "abandon", "host", "cancel-host":
			return true
		}
	case "map":
		return verb == "node"
	case "reward":
		return verb == "take" || verb == "skip"
	case "event":
		return verb == "choose"
	case "shop":
		return verb == "buy" || verb == "leave"
	case "rest":
		return verb == "heal" || verb == "upgrade"
	case "equipment":
		return verb == "take" || verb == "skip"
	case "deck_action":
		return verb == "choose" || verb == "back"
	case "summary":
		return verb == "new" || verb == "abandon"
	}
	return false
}

func describeMultiplayerAction(snapshot *netplay.Snapshot, command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return command
	}
	verb := strings.ToLower(fields[0])
	phase := multiplayerSnapshotPhase(snapshot)
	switch verb {
	case "quit", "exit":
		return "Leave the current room"
	case "ready":
		return "Mark yourself ready"
	case "start":
		return "Start this run"
	case "mode":
		if len(fields) >= 2 && !isTemplateMultiplayerAction(command) {
			mode := fields[1]
			if strings.EqualFold(mode, "story") {
				mode = "Story"
			} else if strings.EqualFold(mode, "endless") {
				mode = "Endless"
			}
			return fmt.Sprintf("Switch room mode to %s", mode)
		}
		return "Switch room mode"
	case "seed":
		if len(fields) >= 2 && !isTemplateMultiplayerAction(command) {
			return fmt.Sprintf("Set room seed to %s", fields[1])
		}
		return "Set room seed"
	case "class":
		if len(fields) >= 2 && !isTemplateMultiplayerAction(command) {
			return fmt.Sprintf("Switch class to %s", fields[1])
		}
		return "Switch class"
	case "drop":
		if len(fields) >= 2 && strings.EqualFold(fields[1], "all") {
			return "Clear all disconnected seats"
		}
		if len(fields) >= 2 && !isTemplateMultiplayerAction(command) {
			return fmt.Sprintf("Clear disconnected seat %s", fields[1])
		}
		return "Clear disconnected seats"
	case "chat", "say":
		text := strings.TrimSpace(strings.TrimPrefix(command, fields[0]))
		if text == "" || strings.Contains(text, "<") {
			return "Send chat message"
		}
		return fmt.Sprintf("Send chat: %s", text)
	case "node":
		if len(fields) >= 2 && !isTemplateMultiplayerAction(command) {
			return describeMapAction(snapshot, fields[1])
		}
		return "Choose the next map node"
	case "play":
		return describeCombatPlayAction(snapshot, fields)
	case "potion":
		return describeCombatPotionAction(snapshot, fields)
	case "end":
		return "End this turn and submit your vote"
	case "take":
		switch phase {
		case "reward":
			if len(fields) >= 2 && !isTemplateMultiplayerAction(command) {
				return describeRewardTakeAction(snapshot, fields[1])
			}
			return "Take reward"
		case "equipment":
			return "Take the candidate equipment"
		default:
			return "Confirm take"
		}
	case "skip":
		switch phase {
		case "reward":
			return "Skip reward"
		case "equipment":
			return "Skip equipment"
		default:
			return "Skip current choice"
		}
	case "choose":
		return describeChooseAction(snapshot, fields)
	case "buy":
		return describeShopBuyAction(snapshot, fields)
	case "leave":
		return "Leave the shop"
	case "heal":
		return "Rest at the campfire"
	case "upgrade":
		return "Upgrade a card at the campfire"
	case "back":
		return "Return to the previous selection"
	case "new":
		return "Start a new run"
	case "abandon":
		if phase == "summary" {
			return "Close the room and end the run"
		}
		return "Abandon the current room"
	case "host":
		if len(fields) >= 2 && !isTemplateMultiplayerAction(command) {
			return fmt.Sprintf("Transfer host authority to seat %s", fields[1])
		}
		return "Transfer host authority"
	case "accept-host":
		return "Accept host authority"
	case "deny-host":
		return "Decline host authority"
	case "cancel-host":
		return "Cancel host transfer"
	default:
		return command
	}
}

func multiplayerSnapshotPhase(snapshot *netplay.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	switch {
	case snapshot.Lobby != nil:
		return "lobby"
	case snapshot.Map != nil:
		return "map"
	case snapshot.Combat != nil:
		return "combat"
	case snapshot.Reward != nil:
		return "reward"
	case snapshot.Event != nil:
		return "event"
	case snapshot.Shop != nil:
		return "shop"
	case snapshot.Rest != nil:
		return "rest"
	case snapshot.Equipment != nil:
		return "equipment"
	case snapshot.Deck != nil:
		return "deck_action"
	case snapshot.Summary != nil:
		return "summary"
	default:
		return strings.ToLower(snapshot.Phase)
	}
}

func describeMapAction(snapshot *netplay.Snapshot, indexText string) string {
	if snapshot != nil && snapshot.Map != nil {
		for _, node := range snapshot.Map.Reachable {
			if fmt.Sprintf("%d", node.Index) == indexText {
				return fmt.Sprintf("Go to node %s: %s", indexText, node.Label)
			}
		}
	}
	return fmt.Sprintf("Go to node %s", indexText)
}

func describeCombatPlayAction(snapshot *netplay.Snapshot, fields []string) string {
	if len(fields) < 2 || isTemplateMultiplayerAction(strings.Join(fields, " ")) {
		return "Play a card"
	}
	cardName := fmt.Sprintf("Card %s", fields[1])
	if snapshot != nil && snapshot.Combat != nil {
		for _, card := range snapshot.Combat.Hand {
			if fmt.Sprintf("%d", card.Index) == fields[1] {
				cardName = card.Name
				break
			}
		}
	}
	if len(fields) >= 4 {
		return fmt.Sprintf("Play %s -> %s", cardName, describeCombatTarget(snapshot, fields[2], fields[3]))
	}
	return fmt.Sprintf("Play %s", cardName)
}

func describeCombatPotionAction(snapshot *netplay.Snapshot, fields []string) string {
	if len(fields) < 2 || isTemplateMultiplayerAction(strings.Join(fields, " ")) {
		return "Use a potion"
	}
	potionLabel := "Potion slot " + fields[1]
	if snapshot != nil && snapshot.Combat != nil {
		if slot, err := strconv.Atoi(fields[1]); err == nil && slot >= 1 && slot <= len(snapshot.Combat.Potions) {
			name := strings.TrimSpace(snapshot.Combat.Potions[slot-1])
			if name != "" {
				potionLabel = "Potion " + name
			}
		}
	}
	target := ""
	if len(fields) >= 4 {
		target = " -> " + describeCombatTarget(snapshot, fields[2], fields[3])
	}
	return fmt.Sprintf("Use %s%s", potionLabel, target)
}

func describeCombatTarget(snapshot *netplay.Snapshot, kind string, indexText string) string {
	if snapshot != nil && snapshot.Combat != nil {
		switch strings.ToLower(kind) {
		case "enemy":
			for _, enemy := range snapshot.Combat.Enemies {
				if fmt.Sprintf("%d", enemy.Index) == indexText {
					return fmt.Sprintf("enemy %s %s", indexText, enemy.Name)
				}
			}
		case "ally":
			for _, actor := range snapshot.Combat.Party {
				if fmt.Sprintf("%d", actor.Index) == indexText {
					return fmt.Sprintf("ally %s %s", indexText, actor.Name)
				}
			}
		}
	}
	if strings.EqualFold(kind, "ally") {
		return fmt.Sprintf("ally %s", indexText)
	}
	return fmt.Sprintf("enemy %s", indexText)
}

func describeRewardTakeAction(snapshot *netplay.Snapshot, indexText string) string {
	if snapshot != nil && snapshot.Reward != nil {
		for _, card := range snapshot.Reward.Cards {
			if fmt.Sprintf("%d", card.Index) == indexText {
				return fmt.Sprintf("Take reward card %s", card.Name)
			}
		}
	}
	return fmt.Sprintf("Take reward %s", indexText)
}

func describeChooseAction(snapshot *netplay.Snapshot, fields []string) string {
	if len(fields) < 2 || isTemplateMultiplayerAction(strings.Join(fields, " ")) {
		switch multiplayerSnapshotPhase(snapshot) {
		case "event":
			return "Choose an event option"
		case "deck_action":
			return "Choose a deck target"
		default:
			return "Choose the current option"
		}
	}
	indexText := fields[1]
	switch multiplayerSnapshotPhase(snapshot) {
	case "event":
		if snapshot != nil && snapshot.Event != nil {
			for _, choice := range snapshot.Event.Choices {
				if fmt.Sprintf("%d", choice.Index) == indexText {
					return fmt.Sprintf("Choose event %s: %s", indexText, choice.Label)
				}
			}
		}
		return fmt.Sprintf("Choose event option %s", indexText)
	case "deck_action":
		if snapshot != nil && snapshot.Deck != nil {
			for _, card := range snapshot.Deck.Cards {
				if fmt.Sprintf("%d", card.Index) == indexText {
					return fmt.Sprintf("Choose card %s: %s", indexText, card.Name)
				}
			}
		}
		return fmt.Sprintf("Choose card %s", indexText)
	default:
		return fmt.Sprintf("Choose option %s", indexText)
	}
}

func describeShopBuyAction(snapshot *netplay.Snapshot, fields []string) string {
	if len(fields) < 2 || isTemplateMultiplayerAction(strings.Join(fields, " ")) {
		return "Buy a shop item"
	}
	indexText := fields[1]
	if snapshot != nil && snapshot.Shop != nil {
		for _, offer := range snapshot.Shop.Offers {
			if fmt.Sprintf("%d", offer.Index) == indexText {
				return fmt.Sprintf("Buy %s (%d gold)", offer.Name, offer.Price)
			}
		}
	}
	return fmt.Sprintf("Buy shop item %s", indexText)
}

func clampMultiplayerActionIndex(selected, length int) int {
	if length <= 0 {
		return 0
	}
	if selected < 0 {
		return 0
	}
	if selected >= length {
		return length - 1
	}
	return selected
}

func (m *model) syncMultiplayerRoomSelection() {
	m.syncMultiplayerCombatSelection()
	m.syncMultiplayerStructuredSelection()
	actions := multiplayerQuickActions(m.multiplayerSnapshot)
	if m.multiplayerUsesCombatControls() {
		m.multiplayerActionIndex = 0
		if m.multiplayerRoomFocus != multiplayerRoomFocusInput && m.multiplayerRoomFocus != multiplayerRoomFocusInspect {
			m.multiplayerRoomFocus = multiplayerRoomFocusActions
			m.multiplayerCommandInput.Blur()
			return
		}
		if m.multiplayerRoomFocus == multiplayerRoomFocusInspect {
			m.multiplayerCommandInput.Blur()
			return
		}
		m.multiplayerCommandInput.Focus()
		return
	}
	if m.multiplayerUsesStructuredControls() {
		m.multiplayerActionIndex = 0
		if m.multiplayerRoomFocus != multiplayerRoomFocusInput {
			m.multiplayerRoomFocus = multiplayerRoomFocusActions
			m.multiplayerCommandInput.Blur()
			return
		}
		m.multiplayerCommandInput.Focus()
		return
	}
	if len(actions) == 0 {
		m.multiplayerActionIndex = 0
		m.multiplayerRoomFocus = multiplayerRoomFocusInput
		m.multiplayerCommandInput.Focus()
		return
	}
	m.multiplayerActionIndex = clampMultiplayerActionIndex(m.multiplayerActionIndex, len(actions))
	if m.multiplayerRoomFocus != multiplayerRoomFocusInput {
		m.multiplayerRoomFocus = multiplayerRoomFocusActions
		m.multiplayerCommandInput.Blur()
		return
	}
	m.multiplayerCommandInput.Focus()
}

func (m *model) toggleMultiplayerRoomFocus() {
	if m.multiplayerUsesCombatControls() {
		if m.multiplayerRoomFocus == multiplayerRoomFocusActions {
			m.multiplayerRoomFocus = multiplayerRoomFocusInspect
			m.multiplayerCommandInput.Blur()
			return
		}
		if m.multiplayerRoomFocus == multiplayerRoomFocusInspect {
			m.multiplayerRoomFocus = multiplayerRoomFocusInput
			m.multiplayerCommandInput.Focus()
			return
		}
		m.multiplayerRoomFocus = multiplayerRoomFocusActions
		m.multiplayerCommandInput.Blur()
		return
	}
	if m.multiplayerUsesStructuredControls() {
		if m.multiplayerRoomFocus == multiplayerRoomFocusActions {
			m.multiplayerRoomFocus = multiplayerRoomFocusInput
			m.multiplayerCommandInput.Focus()
			return
		}
		m.multiplayerRoomFocus = multiplayerRoomFocusActions
		m.multiplayerCommandInput.Blur()
		return
	}
	actions := multiplayerQuickActions(m.multiplayerSnapshot)
	if len(actions) == 0 {
		m.multiplayerRoomFocus = multiplayerRoomFocusInput
		m.multiplayerCommandInput.Focus()
		return
	}
	if m.multiplayerRoomFocus == multiplayerRoomFocusActions {
		m.multiplayerRoomFocus = multiplayerRoomFocusInput
		m.multiplayerCommandInput.Focus()
		return
	}
	m.multiplayerRoomFocus = multiplayerRoomFocusActions
	m.multiplayerCommandInput.Blur()
}

func (m model) submitMultiplayerCommand(line string) (tea.Model, tea.Cmd) {
	line = strings.TrimSpace(line)
	if line == "" {
		return m, nil
	}
	cmd, quit, err := netplay.ParseTextCommand(m.multiplayerSnapshot, line)
	if err != nil {
		m.message = err.Error()
		return m, clearMessage()
	}
	if quit {
		m.closeMultiplayerSession()
		m.screen = screenMultiplayerMenu
		m.index = 0
		m.message = "已离开当前房间。"
		return m, clearMessage()
	}
	next, command := m.sendMultiplayerCommand(cmd)
	m = next.(model)
	if command != nil {
		return m, command
	}
	m.multiplayerCommandInput.SetValue("")
	return m, nil
}

func (m model) applySelectedMultiplayerAction() (tea.Model, tea.Cmd) {
	actions := multiplayerQuickActions(m.multiplayerSnapshot)
	if len(actions) == 0 {
		return m, nil
	}
	action := actions[clampMultiplayerActionIndex(m.multiplayerActionIndex, len(actions))]
	if action.Template {
		m.multiplayerRoomFocus = multiplayerRoomFocusInput
		m.multiplayerCommandInput.SetValue(action.Command)
		m.multiplayerCommandInput.CursorEnd()
		m.multiplayerCommandInput.Focus()
		m.message = "已把命令模板放到输入框，补全参数后回车发送。"
		return m, clearMessage()
	}
	return m.submitMultiplayerCommand(action.Command)
}

func (m model) updateMultiplayerRoom(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back) && !(m.multiplayerUsesStructuredControls() && m.multiplayerRoomFocus == multiplayerRoomFocusActions && m.multiplayerSnapshot != nil && (m.multiplayerSnapshot.Shop != nil || m.multiplayerSnapshot.Deck != nil)):
		m.closeMultiplayerSession()
		m.screen = screenMultiplayerMenu
		m.index = 0
		m.message = "已离开当前房间。"
		return m, clearMessage()
	case key.Matches(msg, m.keys.Cycle):
		m.toggleMultiplayerRoomFocus()
		return m, nil
	case m.multiplayerUsesInspectFocus() && m.multiplayerRoomFocus == multiplayerRoomFocusInspect:
		return m.updateMultiplayerInspectControls(msg)
	case m.multiplayerUsesCombatControls() && m.multiplayerRoomFocus == multiplayerRoomFocusActions:
		return m.updateMultiplayerCombatControls(msg)
	case m.multiplayerUsesStructuredControls() && m.multiplayerRoomFocus == multiplayerRoomFocusActions:
		return m.updateMultiplayerStructuredControls(msg)
	case m.multiplayerRoomFocus == multiplayerRoomFocusActions && key.Matches(msg, m.keys.Up):
		actions := multiplayerQuickActions(m.multiplayerSnapshot)
		m.multiplayerActionIndex = moveClamped(m.multiplayerActionIndex, len(actions), -1)
		return m, nil
	case m.multiplayerRoomFocus == multiplayerRoomFocusActions && key.Matches(msg, m.keys.Down):
		actions := multiplayerQuickActions(m.multiplayerSnapshot)
		m.multiplayerActionIndex = moveClamped(m.multiplayerActionIndex, len(actions), 1)
		return m, nil
	case m.multiplayerRoomFocus == multiplayerRoomFocusActions && key.Matches(msg, m.keys.Select):
		return m.applySelectedMultiplayerAction()
	case key.Matches(msg, m.keys.Select):
		return m.submitMultiplayerCommand(m.multiplayerCommandInput.Value())
	default:
		var cmd tea.Cmd
		if m.multiplayerRoomFocus != multiplayerRoomFocusInput {
			m.multiplayerRoomFocus = multiplayerRoomFocusInput
			m.multiplayerCommandInput.Focus()
		}
		m.multiplayerCommandInput, cmd = m.multiplayerCommandInput.Update(msg)
		return m, cmd
	}
}

func multiplayerAddressSummary(portValue string) string {
	port := strings.TrimSpace(portValue)
	if port == "" {
		port = "7777"
	}
	addresses := []string{fmt.Sprintf("127.0.0.1:%s", port)}
	for _, host := range localIPv4Hosts() {
		addresses = append(addresses, fmt.Sprintf("%s:%s", host, port))
	}
	if len(addresses) > 3 {
		addresses = addresses[:3]
	}
	return strings.Join(addresses, " / ")
}

func localIPv4Hosts() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	hosts := []string{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			value := ip.String()
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			hosts = append(hosts, value)
		}
	}
	return hosts
}
