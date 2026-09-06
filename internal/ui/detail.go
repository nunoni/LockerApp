package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lockerapp/internal/vault"
)

type backToListMsg struct{}
type detailDeletedMsg struct{}

type detailModel struct {
	vault     *vault.Vault
	entry     vault.Entry
	revealed  bool
	secret    string
	statusMsg string
	confirm   bool
	// armedSecret holds the last copied secret until App picks it up to
	// track a pending clipboard clear.
	armedSecret string
}

func newDetailModel(v *vault.Vault, id string) detailModel {
	e, _ := v.Get(id)
	return detailModel{vault: v, entry: e}
}

func (m detailModel) Init() tea.Cmd { return nil }

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if m.confirm {
			switch key {
			case "y", "Y":
				m.vault.DeleteEntry(m.entry.ID)
				m.confirm = false
				return m, func() tea.Msg { return detailDeletedMsg{} }
			case "n", "N", "esc":
				m.confirm = false
			}
			return m, nil
		}
		switch key {
		case "esc", "backspace":
			return m, func() tea.Msg { return backToListMsg{} }
		case "r":
			if m.revealed {
				m.revealed = false
				m.secret = ""
			} else {
				s, err := m.vault.RevealSecret(m.entry.ID)
				if err != nil {
					m.statusMsg = "failed to reveal: " + err.Error()
					return m, nil
				}
				m.secret = s
				m.revealed = true
			}
		case "c":
			s, err := m.vault.RevealSecret(m.entry.ID)
			if err != nil {
				m.statusMsg = "failed to copy: " + err.Error()
				return m, nil
			}
			if err := writeClipboard(s); err != nil {
				m.statusMsg = "clipboard unavailable: " + err.Error()
				return m, nil
			}
			m.statusMsg = "copied to clipboard (clears in 15s)"
			m.armedSecret = s
			return m, scheduleClipboardClear(15 * time.Second)
		case "e":
			return m, func() tea.Msg { return editEntryMsg{id: m.entry.ID} }
		case "d":
			m.confirm = true
			m.statusMsg = ""
		}
	}
	return m, nil
}

func (m detailModel) View() string {
	var b strings.Builder
	kind := "Password"
	if m.entry.Type == vault.EntryAPIKey {
		kind = "API Key"
	}
	b.WriteString(TitleStyle.Render(m.entry.Name))
	b.WriteString("\n\n")

	field := func(label, value string) {
		if value == "" {
			return
		}
		b.WriteString(LabelStyle.Render(label + ": "))
		b.WriteString(ValueStyle.Render(value))
		b.WriteString("\n")
	}

	field("Type", kind)
	field("Group", m.entry.Group)
	field("Username", m.entry.Username)

	secret := strings.Repeat("•", 16)
	if m.revealed {
		secret = m.secret
	}
	b.WriteString(LabelStyle.Render("Secret: "))
	b.WriteString(SecretStyle.Render(secret))
	b.WriteString("\n")

	field("Notes", m.entry.Notes)
	field("Updated", m.entry.UpdatedAt.Format("2006-01-02 15:04"))

	b.WriteString("\n")
	if m.confirm {
		b.WriteString(ErrorStyle.Render("Delete '" + m.entry.Name + "'? (y/n)"))
		b.WriteString("\n")
	} else if m.statusMsg != "" {
		b.WriteString(SuccessStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}
	b.WriteString(HelpStyle.Render("r: reveal • c: copy • e: edit • d: delete • esc: back"))
	return BoxStyle.Render(b.String())
}
