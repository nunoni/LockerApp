package ui

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lockerapp/internal/vault"
)

type screen int

const (
	screenUnlock screen = iota
	screenList
	screenForm
	screenDetail
)

// autoLockAfter is the inactivity period after which the vault locks itself.
const autoLockAfter = 5 * time.Minute

type autoLockTickMsg struct{}

func autoLockTick() tea.Cmd {
	return tea.Every(30*time.Second, func(time.Time) tea.Msg {
		return autoLockTickMsg{}
	})
}

type shutdownMsg struct{}

// watchSignals triggers the same clean shutdown as Ctrl+C/q when the process
// receives SIGINT, SIGTERM or SIGHUP (e.g. `kill`, terminal closed, system
// shutdown). In raw mode Ctrl+C is delivered to Bubble Tea as a key message,
// so these only fire for real signals.
func watchSignals() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		defer signal.Stop(ch)
		<-ch
		return shutdownMsg{}
	}
}

type App struct {
	vault        *vault.Vault
	screen       screen
	unlock       unlockModel
	list         listModel
	form         formModel
	detail       detailModel
	width        int
	height       int
	lastActivity time.Time
	// pendingClear holds the secret currently sitting in the clipboard, so
	// it can be wiped on timeout, lock, or quit even mid-flow.
	pendingClear string
}

func New(v *vault.Vault) App {
	return App{
		vault:        v,
		screen:       screenUnlock,
		unlock:       newUnlockModel(v),
		list:         newListModel(v),
		lastActivity: time.Now(),
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(a.unlock.Init(), autoLockTick(), watchSignals())
}

// shutdown wipes sensitive state before exiting.
func (a *App) shutdown() {
	if a.pendingClear != "" {
		conditionalClear(a.pendingClear)
		a.pendingClear = ""
	}
	a.vault.Lock()
}

// lockNow relocks the vault and returns to the unlock screen.
func (a *App) lockNow() {
	if a.pendingClear != "" {
		conditionalClear(a.pendingClear)
		a.pendingClear = ""
	}
	a.vault.Lock()
	a.unlock = newUnlockModel(a.vault)
	a.screen = screenUnlock
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
	case autoLockTickMsg:
		if a.screen != screenUnlock && time.Since(a.lastActivity) > autoLockAfter {
			a.lockNow()
		}
		return a, autoLockTick()
	case clearClipboardMsg:
		if a.pendingClear != "" {
			conditionalClear(a.pendingClear)
			a.pendingClear = ""
		}
		return a, nil
	case shutdownMsg:
		a.shutdown()
		return a, tea.Quit
	case tea.KeyMsg:
		a.lastActivity = time.Now()
		if msg.String() == "ctrl+c" {
			a.shutdown()
			return a, tea.Quit
		}
		if a.screen == screenList && msg.String() == "q" {
			a.shutdown()
			return a, tea.Quit
		}
	case vaultUnlockedMsg:
		a.screen = screenList
		a.list = newListModel(a.vault)
		return a, nil
	case addEntryMsg:
		a.form = newFormModel(a.vault, msg.entryType, "")
		a.screen = screenForm
		return a, a.form.Init()
	case editEntryMsg:
		e, err := a.vault.Get(msg.id)
		if err != nil {
			return a, nil
		}
		a.form = newFormModel(a.vault, e.Type, msg.id)
		a.screen = screenForm
		return a, a.form.Init()
	case viewEntryMsg:
		a.detail = newDetailModel(a.vault, msg.id)
		a.screen = screenDetail
		return a, nil
	case entrySavedMsg, formCancelledMsg, detailDeletedMsg, backToListMsg:
		a.list.rebuild()
		a.screen = screenList
		if _, ok := msg.(entrySavedMsg); ok {
			a.list.statusMsg = "saved"
		}
		return a, nil
	}

	var cmd tea.Cmd
	switch a.screen {
	case screenUnlock:
		a.unlock, cmd = a.unlock.Update(msg)
	case screenList:
		a.list, cmd = a.list.Update(msg)
	case screenForm:
		a.form, cmd = a.form.Update(msg)
	case screenDetail:
		a.detail, cmd = a.detail.Update(msg)
		if a.detail.armedSecret != "" {
			a.pendingClear = a.detail.armedSecret
			a.detail.armedSecret = ""
		}
	}
	return a, cmd
}

func (a App) View() string {
	switch a.screen {
	case screenUnlock:
		return a.unlock.View()
	case screenList:
		return a.list.View()
	case screenForm:
		return a.form.View()
	case screenDetail:
		return a.detail.View()
	}
	return ""
}
