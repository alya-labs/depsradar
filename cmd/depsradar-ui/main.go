package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#04B575")).
		MarginBottom(1)

	platypusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2A76F")) // Brownish/orange color for the platypus body

	billStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")) // Dark gray for the bill

	radarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")). // Cyan for radar waves
		Bold(true)
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*300, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	frame    int
	quitting bool
}

func initialModel() model {
	return model{frame: 0}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case tickMsg:
		m.frame = (m.frame + 1) % 4
		return m, tickCmd()
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Fermeture de DepsRadar UI...\n"
	}

	frames := []string{
		"      ",
		" .    ",
		" ) )  ",
		" ) ) )",
	}

	// Platypus ASCII Art
	// We'll construct it in parts so we can color it nicely
	top := platypusStyle.Render("       _..._")
	head := platypusStyle.Render("     /       \\")
	eyes := platypusStyle.Render("    |  ") + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render("o   o") + platypusStyle.Render("  |")
	nose := platypusStyle.Render("    |    _    |")
	
	bill := billStyle.Render("     \\  ===  /") + " " + radarStyle.Render(frames[m.frame])
	bottom := platypusStyle.Render("      \\_____/")

	platypus := top + "\n" + head + "\n" + eyes + "\n" + nose + "\n" + bill + "\n" + bottom

	view := titleStyle.Render("📡 DepsRadar — Scan de dépendances en cours...") + "\n" +
		platypus + "\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Appuyez sur 'q' pour quitter.")

	return view
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Erreur: %v", err)
		os.Exit(1)
	}
}