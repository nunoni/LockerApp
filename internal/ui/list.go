package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lockerapp/internal/vault"
)

type viewEntryMsg struct{ id string }
type addEntryMsg struct{ entryType vault.EntryType }
type editEntryMsg struct{ id string }

type listRow struct {
	header bool
	group  string
	entry  vault.Entry
}

type listModel struct {
	vault      *vault.Vault
	activeTab  int
	rows       []listRow
	cursor     int
	statusMsg  string
	confirmDel string
	width      int
	height     int
}

var listTabs = []struct {
	label     string
	entryType vault.EntryType
}{
	{"Passwords", vault.EntryPassword},
	{"API Keys", vault.EntryAPIKey},
}

func newListModel(v *vault.Vault) listModel {
	m := listModel{vault: v}
	m.rebuild()
	return m
}

func (m *listModel) rebuild() {
	m.rows = nil
	entries := m.vault.Entries()
	t := listTabs[m.activeTab].entryType
	groups := map[string][]vault.Entry{}
	order := []string{}
	for _, e := range entries {
		if e.Type != t {
			continue
		}
		if _, ok := groups[e.Group]; !ok {
			order = append(order, e.Group)
		}
		groups[e.Group] = append(groups[e.Group], e)
	}
	for _, g := range order {
		m.rows = append(m.rows, listRow{header: true, group: g})
		for _, e := range groups[g] {
			m.rows = append(m.rows, listRow{entry: e})
		}
	}
	m.clampCursor()
}

func (m *listModel) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.rows[m.cursor].header {
		m.moveCursor(1)
		if m.rows[m.cursor].header {
			m.moveCursor(-1)
		}
	}
}

func (m *listModel) moveCursor(dir int) {
	if len(m.rows) == 0 {
		return
	}
	next := m.cursor
	for {
		next += dir
		if next < 0 || next >= len(m.rows) {
			return
		}
		if !m.rows[next].header {
			m.cursor = next
			return
		}
	}
}

func (m listModel) selected() (vault.Entry, bool) {
	if len(m.rows) == 0 || m.cursor >= len(m.rows) || m.rows[m.cursor].header {
		return vault.Entry{}, false
	}
	return m.rows[m.cursor].entry, true
}

func (m listModel) Init() tea.Cmd { return nil }

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if m.confirmDel != "" {
			switch key {
			case "y", "Y":
				m.vault.DeleteEntry(m.confirmDel)
				m.confirmDel = ""
				m.statusMsg = "entry deleted"
				m.rebuild()
			case "n", "N", "esc":
				m.confirmDel = ""
			}
			return m, nil
		}
		switch key {
		case "tab", "right", "l", "shift+tab", "left", "h":
			if key == "tab" || key == "right" || key == "l" {
				m.activeTab = (m.activeTab + 1) % len(listTabs)
			} else {
				m.activeTab = (m.activeTab - 1 + len(listTabs)) % len(listTabs)
			}
			m.cursor = 0
			m.rebuild()
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			if e, ok := m.selected(); ok {
				return m, func() tea.Msg { return viewEntryMsg{id: e.ID} }
			}
		case "a":
			return m, func() tea.Msg { return addEntryMsg{entryType: listTabs[m.activeTab].entryType} }
		case "e":
			if e, ok := m.selected(); ok {
				return m, func() tea.Msg { return editEntryMsg{id: e.ID} }
			}
		case "d":
			if e, ok := m.selected(); ok {
				m.confirmDel = e.ID
				m.statusMsg = ""
			}
		}
	}
	return m, nil
}

func (m listModel) View() string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render("Locker"))
	b.WriteString("\n")
	tabs := make([]string, len(listTabs))
	for i, t := range listTabs {
		if i == m.activeTab {
			tabs[i] = ActiveTabStyle.Render(t.label)
		} else {
			tabs[i] = InactiveTabStyle.Render(t.label)
		}
	}
	b.WriteString(strings.Join(tabs, " "))
	b.WriteString("\n")

	if len(m.rows) == 0 {
		b.WriteString(GroupHeaderStyle.Render(""))
		b.WriteString(EntryMetaStyle.Render("  No entries yet. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		for i, row := range m.rows {
			if row.header {
				b.WriteString(GroupHeaderStyle.Render(row.group))
				b.WriteString("\n")
				continue
			}
			cursor := "  "
			style := EntryStyle
			if i == m.cursor {
				cursor = "> "
				style = SelectedEntryStyle
			}
			line := cursor + row.entry.Name
			if row.entry.Username != "" {
				line += EntryMetaStyle.Render("  (" + row.entry.Username + ")")
			}
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	if m.confirmDel != "" {
		e, _ := m.vault.Get(m.confirmDel)
		b.WriteString("\n")
		b.WriteString(ErrorStyle.Render("Delete '" + e.Name + "'? (y/n)"))
		b.WriteString("\n")
	} else if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(SuccessStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}
	b.WriteString(HelpStyle.Render("tab: switch • j/k: move • enter: view • a: add • e: edit • d: delete • q: quit"))
	return b.String()
}
