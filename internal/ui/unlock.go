package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lockerapp/internal/vault"
)

type vaultUnlockedMsg struct{}

type unlockModel struct {
	vault    *vault.Vault
	setup    bool
	inputs   []textinput.Model
	focused  int
	err      error
	unlocked bool
}

func newUnlockModel(v *vault.Vault) unlockModel {
	m := unlockModel{vault: v, setup: !v.Initialized()}
	pw := textinput.New()
	pw.Placeholder = "master password"
	pw.EchoMode = textinput.EchoPassword
	pw.EchoCharacter = '•'
	pw.Focus()
	pw.PromptStyle = FocusedInputStyle
	pw.TextStyle = FocusedInputStyle
	m.inputs = []textinput.Model{pw}
	if m.setup {
		confirm := textinput.New()
		confirm.Placeholder = "confirm master password"
		confirm.EchoMode = textinput.EchoPassword
		confirm.EchoCharacter = '•'
		m.inputs = append(m.inputs, confirm)
	}
	return m
}

func (m unlockModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m unlockModel) Update(msg tea.Msg) (unlockModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % len(m.inputs)
			return m, m.updateFocus()
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			return m, m.updateFocus()
		case "enter":
			if m.setup && m.focused < len(m.inputs)-1 {
				m.focused++
				return m, m.updateFocus()
			}
			return m, m.submit()
		}
	}
	var cmds []tea.Cmd
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *unlockModel) updateFocus() tea.Cmd {
	for i := range m.inputs {
		if i == m.focused {
			m.inputs[i].Focus()
			m.inputs[i].PromptStyle = FocusedInputStyle
			m.inputs[i].TextStyle = FocusedInputStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = EntryMetaStyle
			m.inputs[i].TextStyle = ValueStyle
		}
	}
	return textinput.Blink
}

func (m *unlockModel) submit() tea.Cmd {
	password := m.inputs[0].Value()
	if password == "" {
		m.err = errf("password cannot be empty")
		return nil
	}
	if m.setup {
		if len(password) < 8 {
			m.err = errf("master password must be at least 8 characters")
			return nil
		}
		if password != m.inputs[1].Value() {
			m.err = errf("passwords do not match")
			return nil
		}
		if err := m.vault.Setup(password); err != nil {
			m.err = err
			return nil
		}
	} else {
		if err := m.vault.Unlock(password); err != nil {
			m.inputs[0].SetValue("")
			m.err = errf("incorrect master password")
			return nil
		}
	}
	// Clear the password inputs so the master password does not linger in
	// the UI model any longer than necessary.
	m.inputs[0].SetValue("")
	if m.setup {
		m.inputs[1].SetValue("")
	}
	m.err = nil
	return func() tea.Msg { return vaultUnlockedMsg{} }
}

func (m unlockModel) View() string {
	var b strings.Builder
	if m.setup {
		b.WriteString(TitleStyle.Render("Welcome to Locker"))
		b.WriteString("\n\n")
		b.WriteString(EntryMetaStyle.Render("Create a master password to secure your vault."))
		b.WriteString("\n\n")
	} else {
		b.WriteString(TitleStyle.Render("Locker"))
		b.WriteString("\n\n")
		b.WriteString(EntryMetaStyle.Render("Enter your master password to unlock."))
		b.WriteString("\n\n")
	}
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n")
	}
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ErrorStyle.Render(m.err.Error()))
		b.WriteString("\n")
	}
	b.WriteString(HelpStyle.Render("enter: submit • tab: next field • ctrl+c: quit"))
	return BoxStyle.Render(b.String())
}

type appError struct{ msg string }

func (e appError) Error() string { return e.msg }

func errf(msg string) error { return appError{msg: msg} }
