package ui

import "github.com/charmbracelet/lipgloss"

var (
	pink       = lipgloss.Color("#FF75B7")
	hotPink    = lipgloss.Color("#FF5FAF")
	purple     = lipgloss.Color("#7D56F4")
	cyan       = lipgloss.Color("#6DEFEF")
	white      = lipgloss.Color("#FAFAFA")
	subtle     = lipgloss.Color("#626262")
	errorRed   = lipgloss.Color("#FF5555")
	successGrn = lipgloss.Color("#50FA7B")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(pink).
			Padding(0, 2).
			MarginBottom(1)

	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(purple).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(pink).
				Background(lipgloss.Color("#3C3C3C")).
				Padding(0, 2)

	GroupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(cyan).
				MarginTop(1)

	SelectedEntryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(hotPink).
				PaddingLeft(2)

	EntryStyle = lipgloss.NewStyle().
			Foreground(white).
			PaddingLeft(2)

	EntryMetaStyle = lipgloss.NewStyle().
			Foreground(subtle)

	LabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(pink)

	ValueStyle = lipgloss.NewStyle().
			Foreground(white)

	SecretStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Background(lipgloss.Color("#2D2D2D")).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(subtle).
			MarginTop(1)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(errorRed)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(successGrn)

	FocusedInputStyle = lipgloss.NewStyle().
				Foreground(hotPink)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1, 2)
)
