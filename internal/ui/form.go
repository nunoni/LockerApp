package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lockerapp/internal/vault"
)

type entrySavedMsg struct{}
type formCancelledMsg struct{}

const (
	fieldName = iota
	fieldGroup
	fieldUsername
	fieldSecret
	fieldNotes
	fieldCount
)

type formModel struct {
	vault     *vault.Vault
	entryType vault.EntryType
	editingID string
	inputs    []textinput.Model
	focused   int
	err       error
}

func newFormModel(v *vault.Vault, entryType vault.EntryType, editingID string) formModel {
	m := formModel{
		vault:     v,
		entryType: entryType,
		editingID: editingID,
		focused:   fieldName,
	}

	mk := func(placeholder string, masked bool) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		if masked {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		return ti
	}

	userLabel := "username (optional)"
	secretLabel := "password"
	if entryType == vault.EntryAPIKey {
		userLabel = "key id / owner (optional)"
		secretLabel = "api key"
	}

	m.inputs = []textinput.Model{
		mk("name", false),
		mk("group (default: "+vault.DefaultGroup+")", false),
		mk(userLabel, false),
		mk(secretLabel, true),
		mk("notes (optional)", false),
	}

	if editingID != "" {
		if e, err := v.Get(editingID); err == nil {
			m.inputs[fieldName].SetValue(e.Name)
			m.inputs[fieldGroup].SetValue(e.Group)
			m.inputs[fieldUsername].SetValue(e.Username)
			m.inputs[fieldNotes].SetValue(e.Notes)
		}
	}
	m.inputs[fieldName].Focus()
	m.inputs[fieldName].PromptStyle = FocusedInputStyle
	m.inputs[fieldName].TextStyle = FocusedInputStyle
	return m
}

func (m formModel) Init() tea.Cmd { return textinput.Blink }

func (m formModel) Update(msg tea.Msg) (formModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return formCancelledMsg{} }
		case "ctrl+s":
			return m, m.save()
		case "tab", "down":
			m.focused = (m.focused + 1) % len(m.inputs)
			return m, m.updateFocus()
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			return m, m.updateFocus()
		case "enter":
			if m.focused == len(m.inputs)-1 {
				return m, m.save()
			}
			m.focused++
			return m, m.updateFocus()
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

func (m *formModel) updateFocus() tea.Cmd {
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

func (m *formModel) save() tea.Cmd {
	name := strings.TrimSpace(m.inputs[fieldName].Value())
	secret := m.inputs[fieldSecret].Value()
	if name == "" {
		m.err = errf("name is required")
		return nil
	}
	if secret == "" && m.editingID == "" {
		m.err = errf("secret is required")
		return nil
	}
	e := vault.Entry{
		Type:     m.entryType,
		Name:     name,
		Group:    strings.TrimSpace(m.inputs[fieldGroup].Value()),
		Username: strings.TrimSpace(m.inputs[fieldUsername].Value()),
		Notes:    strings.TrimSpace(m.inputs[fieldNotes].Value()),
	}
	var err error
	if m.editingID != "" {
		err = m.vault.UpdateEntry(m.editingID, e, secret)
	} else {
		err = m.vault.AddEntry(e, secret)
	}
	if err != nil {
		m.err = err
		return nil
	}
	return func() tea.Msg { return entrySavedMsg{} }
}

func (m formModel) View() string {
	var b strings.Builder
	verb := "Add"
	if m.editingID != "" {
		verb = "Edit"
	}
	kind := "Password"
	if m.entryType == vault.EntryAPIKey {
		kind = "API Key"
	}
	b.WriteString(TitleStyle.Render(verb + " " + kind))
	b.WriteString("\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n")
	}
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ErrorStyle.Render(m.err.Error()))
		b.WriteString("\n")
	}
	if m.editingID != "" {
		b.WriteString(EntryMetaStyle.Render("\nleave the secret field empty to keep the current one\n"))
	}
	b.WriteString(HelpStyle.Render("tab: next field • enter/ctrl+s: save • esc: cancel"))
	return BoxStyle.Render(b.String())
}
