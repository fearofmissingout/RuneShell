package app

import (
	"time"

	"cmdcards/internal/content"
	"cmdcards/internal/engine"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	menuCodexLabel       = "图鉴"
	menuProgressionLabel = "局外养成"
)

const (
	codexTabCards = iota
	codexTabRelics
	codexTabEquipments
	codexTabCount
)

const (
	profileTabPerks = iota
	profileTabEquipment
	profileTabLoadout
	profileTabCount
)

type loadoutRow struct {
	ClassID string
	Slot    string
}

func (m model) updateCodex(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "1":
		m.codexTab = codexTabCards
		m.index = 0
	case msg.String() == "2":
		m.codexTab = codexTabRelics
		m.index = 0
	case msg.String() == "3":
		m.codexTab = codexTabEquipments
		m.index = 0
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, m.codexItemCount(), -1)
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, m.codexItemCount(), 1)
	case key.Matches(msg, m.keys.Left), msg.String() == "[":
		m.index = flipPage(m.index, m.codexItemCount(), -1)
	case key.Matches(msg, m.keys.Right), msg.String() == "]":
		m.index = flipPage(m.index, m.codexItemCount(), 1)
	case key.Matches(msg, m.keys.Cycle):
		m.codexTab = (m.codexTab + 1) % codexTabCount
		m.index = 0
	case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Select):
		m.screen = screenMenu
		m.index = 0
	}
	return m, nil
}

func (m model) updateProfile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.index = moveClamped(m.index, m.profileItemCount(), -1)
	case key.Matches(msg, m.keys.Down):
		m.index = moveClamped(m.index, m.profileItemCount(), 1)
	case key.Matches(msg, m.keys.Left):
		m.index = flipPage(m.index, m.profileItemCount(), -1)
	case key.Matches(msg, m.keys.Right):
		m.index = flipPage(m.index, m.profileItemCount(), 1)
	case key.Matches(msg, m.keys.Cycle):
		m.profileTab = (m.profileTab + 1) % profileTabCount
		m.index = 0
	case key.Matches(msg, m.keys.Back):
		m.screen = screenMenu
		m.index = 0
	case key.Matches(msg, m.keys.Select):
		switch m.profileTab {
		case profileTabPerks:
			perks := engine.ProgressionPerks()
			if len(perks) == 0 {
				return m, nil
			}
			if err := engine.UpgradePerk(&m.profile, perks[m.index].ID); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			if err := m.persistProfile(); err != nil {
				m.err = err
				return m, nil
			}
			m.message = "局外成长已提升"
			return m, clearMessage()
		case profileTabEquipment:
			items := progressionEquipmentItems(m.lib)
			if len(items) == 0 {
				return m, nil
			}
			if err := engine.UnlockEquipment(m.lib, &m.profile, items[m.index].ID); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			if err := m.persistProfile(); err != nil {
				m.err = err
				return m, nil
			}
			m.message = "装备已解锁"
			return m, clearMessage()
		case profileTabLoadout:
			rows := progressionLoadoutRows(m.lib)
			if len(rows) == 0 {
				return m, nil
			}
			row := rows[m.index]
			if _, err := engine.CycleLoadoutEquipment(m.lib, &m.profile, row.ClassID, row.Slot); err != nil {
				m.message = err.Error()
				return m, clearMessage()
			}
			if err := m.persistProfile(); err != nil {
				m.err = err
				return m, nil
			}
			m.message = "初始装备已切换"
			return m, clearMessage()
		}
	}
	return m, nil
}

func (m model) persistProfile() error {
	m.profile.LastUpdated = time.Now()
	return m.store.SaveProfile(m.profile)
}

func (m model) codexItemCount() int {
	switch m.codexTab {
	case codexTabCards:
		return len(m.lib.CardList())
	case codexTabRelics:
		return len(m.lib.RelicList())
	default:
		return len(m.lib.EquipmentList())
	}
}

func (m model) profileItemCount() int {
	switch m.profileTab {
	case profileTabPerks:
		return len(engine.ProgressionPerks())
	case profileTabEquipment:
		return len(progressionEquipmentItems(m.lib))
	default:
		return len(progressionLoadoutRows(m.lib))
	}
}

func progressionEquipmentItems(lib *content.Library) []content.EquipmentDef {
	items := []content.EquipmentDef{}
	for _, equipment := range lib.EquipmentList() {
		if equipment.Rarity == "starter" {
			continue
		}
		items = append(items, equipment)
	}
	return items
}

func progressionLoadoutRows(lib *content.Library) []loadoutRow {
	rows := []loadoutRow{}
	for _, class := range lib.ClassList() {
		for _, slot := range []string{"weapon", "armor", "accessory"} {
			rows = append(rows, loadoutRow{ClassID: class.ID, Slot: slot})
		}
	}
	return rows
}
