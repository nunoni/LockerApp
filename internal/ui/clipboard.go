package ui

import (
	"errors"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// writeClipboard and readClipboard are variables so tests can stub them out.
var (
	writeClipboard = systemClipboardWrite
	readClipboard  = systemClipboardRead
)

type clearClipboardMsg struct{}

func systemClipboardWrite(s string) error {
	// Only stdin-based tools are used so the secret never appears in argv
	// (and therefore never in the process list or shell history). The qdbus
	// setClipboardContents call is deliberately avoided for this reason.
	candidates := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}
	for _, c := range candidates {
		path, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c[1:]...)
		cmd.Stdin = strings.NewReader(s)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return errors.New("no clipboard tool found (install wl-clipboard, xclip or xsel)")
}

func systemClipboardRead() (string, error) {
	candidates := [][]string{
		{"wl-paste", "--no-newline"},
		{"xclip", "-selection", "clipboard", "-o"},
		{"xsel", "--clipboard", "--output"},
		{"qdbus6", "org.kde.klipper", "/klipper", "getClipboardContents"},
		{"qdbus", "org.kde.klipper", "/klipper", "getClipboardContents"},
	}
	for _, c := range candidates {
		path, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		out, err := exec.Command(path, c[1:]...).Output()
		if err == nil {
			// xclip/xsel (and some qdbus responses) append a trailing newline;
			// trim it so comparisons match the copied secret.
			return strings.TrimRight(string(out), "\r\n"), nil
		}
	}
	return "", errors.New("no clipboard tool found")
}

// conditionalClear wipes the clipboard only if it still holds the secret we
// copied, so a later unrelated copy is not destroyed. KDE Klipper keeps a
// clipboard history; it is purged only when we actually wipe our secret.
func conditionalClear(expected string) {
	current, err := readClipboard()
	if err != nil || current != expected {
		return
	}
	writeClipboard("")
	clearKlipperHistory()
}

func clearKlipperHistory() {
	for _, bin := range []string{"qdbus6", "qdbus"} {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		if err := exec.Command(path, "org.kde.klipper", "/klipper", "clearClipboardHistory").Run(); err == nil {
			return
		}
	}
}

func scheduleClipboardClear(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearClipboardMsg{}
	})
}
