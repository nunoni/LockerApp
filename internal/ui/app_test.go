package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lockerapp/internal/vault"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// drive feeds keys into the root App model synchronously. Multi-character
// strings are sent as individual runes; special keys by name. Returned
// commands are executed once and their resulting messages fed back, except
// blocking ticker messages (blink), which are dropped.
func drive(t *testing.T, m tea.Model, keys ...string) tea.Model {
	t.Helper()
	for _, k := range keys {
		switch k {
		case "enter", "tab", "esc", "shift+tab":
			m = sendOne(t, m, key(k))
		default:
			for _, r := range k {
				m = sendOne(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
		}
	}
	return m
}

func sendOne(t *testing.T, m tea.Model, msg tea.Msg) tea.Model {
	t.Helper()
	nm, cmd := m.Update(msg)
	m = nm
	if cmd == nil {
		return m
	}
	done := make(chan tea.Msg, 1)
	go func() {
		if res := cmd(); res != nil {
			done <- res
		}
	}()
	select {
	case res := <-done:
		nm, _ = m.Update(res)
		m = nm
	case <-time.After(50 * time.Millisecond):
		// blocking ticker commands (cursor blink): drop
	}
	return m
}

func view(t *testing.T, m tea.Model) string {
	t.Helper()
	return m.(App).View()
}

func assertContains(t *testing.T, m tea.Model, want string) {
	t.Helper()
	if !strings.Contains(view(t, m), want) {
		t.Fatalf("expected view to contain %q, got:\n%s", want, view(t, m))
	}
}

func TestModelDrivenFlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, err := vault.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	var m tea.Model = New(v)

	assertContains(t, m, "Welcome to Locker")

	m = drive(t, m, "testmaster123", "enter", "testmaster123", "enter")
	assertContains(t, m, "No entries yet")

	m = drive(t, m, "a")
	assertContains(t, m, "Add Password")
	m = drive(t, m, "GitHub", "enter", "Work", "enter", "nuno@example.com", "enter", "ghp_secret123", "enter", "enter")
	assertContains(t, m, "GitHub")

	m = drive(t, m, "tab")
	assertContains(t, m, "No entries yet")
	m = drive(t, m, "a")
	assertContains(t, m, "Add API Key")
	m = drive(t, m, "Stripe", "enter", "Payments", "enter", "enter", "sk_live_abc", "enter", "enter")
	assertContains(t, m, "Stripe")

	m = drive(t, m, "enter")
	assertContains(t, m, "Payments")
	if strings.Contains(view(t, m), "sk_live_abc") {
		t.Fatal("secret should be masked before reveal")
	}
	m = drive(t, m, "r")
	assertContains(t, m, "sk_live_abc")

	m = drive(t, m, "esc", "shift+tab")
	assertContains(t, m, "GitHub")

	// delete the GitHub entry via the list
	m = drive(t, m, "d")
	assertContains(t, m, "Delete 'GitHub'?")
	m = drive(t, m, "y")
	if strings.Contains(view(t, m), "GitHub") {
		t.Fatal("GitHub should be deleted")
	}

	reloaded, _ := vault.Load(path)
	if err := reloaded.Unlock("testmaster123"); err != nil {
		t.Fatalf("unlock reloaded vault: %v", err)
	}
	entries := reloaded.Entries()
	if len(entries) != 1 || entries[0].Name != "Stripe" {
		t.Fatalf("expected only Stripe on disk, got %+v", entries)
	}
	s, err := reloaded.RevealSecret(entries[0].ID)
	if err != nil || s != "sk_live_abc" {
		t.Fatalf("reveal: %v %q", err, s)
	}
}

func TestModelDrivenWrongPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	if err := v.Setup("testmaster123"); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	reloaded, _ := vault.Load(path)

	var m tea.Model = New(reloaded)
	assertContains(t, m, "Enter your master password")
	m = drive(t, m, "wrongpassword", "enter")
	assertContains(t, m, "incorrect master password")
	m = drive(t, m, "testmaster123", "enter")
	assertContains(t, m, "Passwords")
}

func TestModelDrivenSetupMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)

	var m tea.Model = New(v)
	assertContains(t, m, "Welcome to Locker")
	m = drive(t, m, "testmaster123", "enter", "different999", "enter")
	assertContains(t, m, "do not match")
}

func TestModelDrivenQuitLocksVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	if err := v.Setup("testmaster123"); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(v)
	m = drive(t, m, "testmaster123", "enter")
	assertContains(t, m, "No entries yet")

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_ = nm
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	res := cmd()
	if _, ok := res.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", res)
	}
	if v.Unlocked() {
		t.Fatal("vault should be locked on quit")
	}
}

func TestModelDrivenCopyToClipboard(t *testing.T) {
	var copied string
	old := writeClipboard
	writeClipboard = func(s string) error { copied = s; return nil }
	defer func() { writeClipboard = old }()

	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	if err := v.Setup("testmaster123"); err != nil {
		t.Fatal(err)
	}
	if err := v.AddEntry(vault.Entry{Type: vault.EntryPassword, Name: "GitHub", Group: "Work"}, "ghp_secret123"); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(v)
	m = drive(t, m, "testmaster123", "enter")
	assertContains(t, m, "GitHub")

	// 'c' on the detail view copies without needing to reveal first
	m = drive(t, m, "enter")
	if strings.Contains(view(t, m), "ghp_secret123") {
		t.Fatal("secret should be masked before reveal")
	}
	m = drive(t, m, "c")
	if copied != "ghp_secret123" {
		t.Fatalf("expected detail 'c' to copy secret, got %q", copied)
	}
	assertContains(t, m, "copied to clipboard")
	if strings.Contains(view(t, m), "ghp_secret123") {
		t.Fatal("copying should not reveal the secret on screen")
	}
}

func TestAutoLockOnInactivity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	if err := v.Setup("testmaster123"); err != nil {
		t.Fatal(err)
	}

	a := New(v)
	var m tea.Model = a
	m = drive(t, m, "testmaster123", "enter")
	assertContains(t, m, "No entries yet")

	a = m.(App)
	a.lastActivity = time.Now().Add(-10 * time.Minute)
	nm, _ := a.Update(autoLockTickMsg{})
	a = nm.(App)
	if v.Unlocked() {
		t.Fatal("vault should be locked after inactivity")
	}
	if a.screen != screenUnlock {
		t.Fatal("should return to unlock screen after auto-lock")
	}
}

func TestClearClipboardMsgHandledOnAnyScreen(t *testing.T) {
	var cleared []string
	oldWrite, oldRead := writeClipboard, readClipboard
	writeClipboard = func(s string) error { cleared = append(cleared, s); return nil }
	readClipboard = func() (string, error) { return "the-secret", nil }
	defer func() { writeClipboard, readClipboard = oldWrite, oldRead }()

	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	v.Setup("testmaster123")

	// simulate: user copied from detail, then moved to the form screen
	a := New(v)
	a.screen = screenForm
	a.pendingClear = "the-secret"
	nm, _ := a.Update(clearClipboardMsg{})
	a = nm.(App)
	if a.pendingClear != "" {
		t.Fatal("pendingClear should be consumed")
	}
	if len(cleared) == 0 || cleared[len(cleared)-1] != "" {
		t.Fatal("clipboard should be cleared with empty string")
	}
}

func TestClearClipboardSkippedWhenReplacedByUser(t *testing.T) {
	var writes []string
	oldWrite, oldRead := writeClipboard, readClipboard
	writeClipboard = func(s string) error { writes = append(writes, s); return nil }
	readClipboard = func() (string, error) { return "something-else", nil }
	defer func() { writeClipboard, readClipboard = oldWrite, oldRead }()

	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	v.Setup("testmaster123")

	a := New(v)
	a.pendingClear = "the-secret"
	nm, _ := a.Update(clearClipboardMsg{})
	a = nm.(App)
	for _, w := range writes {
		if w == "" {
			t.Fatal("must not wipe clipboard content the user copied afterwards")
		}
	}
	if a.pendingClear != "" {
		t.Fatal("pendingClear should still be consumed")
	}
}

func TestQuitClearsPendingClipboard(t *testing.T) {
	var cleared []string
	oldWrite, oldRead := writeClipboard, readClipboard
	writeClipboard = func(s string) error { cleared = append(cleared, s); return nil }
	readClipboard = func() (string, error) { return "the-secret", nil }
	defer func() { writeClipboard, readClipboard = oldWrite, oldRead }()

	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	v.Setup("testmaster123")

	a := New(v)
	a.pendingClear = "the-secret"
	nm, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	a = nm.(App)
	if cmd == nil {
		t.Fatal("expected quit")
	}
	if len(cleared) == 0 || cleared[len(cleared)-1] != "" {
		t.Fatal("quit should wipe the pending clipboard secret")
	}
	if v.Unlocked() {
		t.Fatal("quit should lock the vault")
	}
}
