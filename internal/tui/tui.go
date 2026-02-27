package tui

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"

	"depsradar/internal/model"
)

// ─── Palette ─────────────────────────────────────────────────────────────────

var (
	clrGreen  = lipgloss.Color("#39D353")
	clrAmber  = lipgloss.Color("#F0A500")
	clrRed    = lipgloss.Color("#FF4D4D")
	clrOrange = lipgloss.Color("#FF7B29")
	clrCyan   = lipgloss.Color("#00E5FF")
	clrTeal   = lipgloss.Color("#3DD9A3")
	clrText   = lipgloss.Color("#C8DCC0")
	clrMuted  = lipgloss.Color("#3A5C3A")
	clrMuted2 = lipgloss.Color("#5A7A5A")
	clrBorder = lipgloss.Color("#1B3320")
	clrBill   = lipgloss.Color("#9A7020")
	clrHead   = lipgloss.Color("#C4883A")
	clrBody   = lipgloss.Color("#A06830")
	clrTail   = lipgloss.Color("#7A5020")
	clrFoot   = lipgloss.Color("#5A3A18")
	clrEye    = lipgloss.Color("#FFFFFF")
)

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	stDish   = lipgloss.NewStyle().Foreground(clrCyan)
	stPole   = lipgloss.NewStyle().Foreground(clrMuted2)
	stHead   = lipgloss.NewStyle().Foreground(clrHead)
	stEye    = lipgloss.NewStyle().Foreground(clrEye).Bold(true)
	stBill   = lipgloss.NewStyle().Foreground(clrBill)
	stBody   = lipgloss.NewStyle().Foreground(clrBody)
	stWave   = lipgloss.NewStyle().Foreground(clrCyan)
	stTail   = lipgloss.NewStyle().Foreground(clrTail)
	stFoot   = lipgloss.NewStyle().Foreground(clrFoot)
	stTitle  = lipgloss.NewStyle().Foreground(clrGreen).Bold(true)
	stMuted  = lipgloss.NewStyle().Foreground(clrMuted2)
	stAmber  = lipgloss.NewStyle().Foreground(clrAmber)
	stCyan   = lipgloss.NewStyle().Foreground(clrCyan)
	stText   = lipgloss.NewStyle().Foreground(clrText)
	stTeal   = lipgloss.NewStyle().Foreground(clrTeal)
	stRed    = lipgloss.NewStyle().Foreground(clrRed).Bold(true)
	stOrange = lipgloss.NewStyle().Foreground(clrOrange)

	stLogInfo = lipgloss.NewStyle().Foreground(clrGreen)
	stLogWarn = lipgloss.NewStyle().Foreground(clrAmber)
	stLogErr  = lipgloss.NewStyle().Foreground(clrRed)

	stBorderBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(clrBorder).
			Padding(0, 1)

	stKeyHint = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(clrMuted).
			Padding(0, 1).
			Foreground(clrText)

	stHdr = lipgloss.NewStyle().Foreground(clrMuted2)
)

// ─── Animation frames ────────────────────────────────────────────────────────

var waveFrames = []string{"≋", "≈", "~", "≈"}
var dishFrames = []string{"·  ◉  ·", "◦  ·  ◦", "◉  ◦  ◉", "·  ◦  ·"}

// ─── Messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time
type logLineMsg string
type scanDoneMsg struct {
	report *model.Report
	err    error
}

func doTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ─── Model ───────────────────────────────────────────────────────────────────

const (
	stateScanning = iota
	stateResults
)

type ScanFunc func() (*model.Report, error)

type Model struct {
	state  int
	frame  int
	logs   []string
	report *model.Report
	err    error
	logCh  <-chan string
	scanFn ScanFunc
	width  int
}

func New(scanFn ScanFunc, logCh <-chan string, width int) Model {
	return Model{
		state:  stateScanning,
		scanFn: scanFn,
		logCh:  logCh,
		width:  width,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(doTick(), m.startScan())
}

func (m Model) startScan() tea.Cmd {
	return func() tea.Msg {
		r, err := m.scanFn()
		return scanDoneMsg{report: r, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "e" && m.state == stateResults && m.report != nil {
			m.report.ExportHTML("depsradar-report.html")
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tickMsg:
		m.frame++
		// Drain log channel
		for {
			select {
			case line := <-m.logCh:
				m.logs = append(m.logs, line)
			default:
				return m, doTick()
			}
		}

	case logLineMsg:
		m.logs = append(m.logs, string(msg))
		return m, doTick()

	case scanDoneMsg:
		m.err = msg.err
		m.report = msg.report
		m.state = stateResults
		return m, nil
	}

	return m, nil
}

// ─── View ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.state == stateScanning {
		return m.viewScanning()
	}
	return m.viewResults()
}

// ── Scanning screen ──────────────────────────────────────────────────────────

func (m Model) viewScanning() string {
	mascot := m.renderMascot()
	logs := m.renderLogs()

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		mascot,
		"   ",
		logs,
	)

	return "\n" + content + "\n"
}

func (m Model) renderMascot() string {
	w := waveFrames[m.frame%4]
	d := dishFrames[m.frame%4]

	h := stHead.Render
	e := stEye.Render
	b := stBill.Render
	bd := stBody.Render
	wv := stWave.Render
	tl := stTail.Render
	ft := stFoot.Render
	ds := stDish.Render
	pl := stPole.Render

	lines := []string{
		ds("         .──────."),
		ds("        ╱ ") + ds(d) + ds(" ╲"),
		ds("       ╱──────────╲"),
		pl("            │"),
		pl("       ─────┴─────"),
		h("       .─────────."),
		h("      ╱ ") + e("●") + h("       ") + e("●") + h(" ╲"),
		h("     │      ~~~      │"),
		h("     │  ") + b("╔═══════╗") + h("  │"),
		h("      ╲─") + b("╚═══════╝") + h("─╱"),
		bd("      ╭────────────╮"),
		bd("     ╱ ") + wv(w+" "+w+" "+w+" "+w+" "+w+" "+w) + bd(" ╲"),
		bd("    │ ") + wv(w+" "+w+" "+w+" "+w+" "+w+" "+w+" "+w) + bd(" │"),
		bd("     ╲ ") + wv(w+" "+w+" "+w+" "+w+" "+w+" "+w) + bd(" ╱"),
		bd("      ╰────────────╯"),
		ft("       ╱╱         ╲╲"),
		tl("      ╱─────────────╲"),
		tl("     ╱───────────────╲"),
		"",
		stMuted.Render("       Perry · DepsRadar"),
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderLogs() string {
	title := stTitle.Render("📡 Scanning dependencies")
	divider := stMuted.Render(strings.Repeat("─", 44))

	var logLines []string
	logLines = append(logLines, title)
	logLines = append(logLines, divider)

	const maxLogs = 12
	start := 0
	if len(m.logs) > maxLogs {
		start = len(m.logs) - maxLogs
	}

	for _, line := range m.logs[start:] {
		logLines = append(logLines, formatLogLine(line))
	}

	// Spinning indicator
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spin := stCyan.Render(spinners[m.frame%len(spinners)])
	logLines = append(logLines, "")
	logLines = append(logLines, spin+" "+stMuted.Render("scanning..."))

	return strings.Join(logLines, "\n")
}

func formatLogLine(line string) string {
	// Parse "time=... level=... msg=..."
	var ts, level, msg string
	parts := strings.Fields(line)
	for _, p := range parts {
		if strings.HasPrefix(p, "time=") {
			t := strings.TrimPrefix(p, "time=")
			if len(t) >= 23 {
				ts = t[11:23] // HH:MM:SS.mmm
			}
		} else if strings.HasPrefix(p, "level=") {
			level = strings.TrimPrefix(p, "level=")
		} else if strings.HasPrefix(p, "msg=") {
			msg = strings.Trim(strings.TrimPrefix(p, "msg="), "\"")
		}
	}

	if ts == "" {
		return stMuted.Render(line)
	}

	var lvlStyle lipgloss.Style
	switch strings.ToUpper(level) {
	case "WARN":
		lvlStyle = stLogWarn
	case "ERROR":
		lvlStyle = stLogErr
	default:
		lvlStyle = stLogInfo
	}

	// Collect key=val attrs
	var attrs []string
	for _, p := range parts {
		if strings.Contains(p, "=") &&
			!strings.HasPrefix(p, "time=") &&
			!strings.HasPrefix(p, "level=") &&
			!strings.HasPrefix(p, "msg=") {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) == 2 {
				attrs = append(attrs, stCyan.Render(kv[0])+"="+stAmber.Render(strings.Trim(kv[1], "\"")))
			}
		}
	}

	result := stMuted.Render(ts) + " " +
		lipgloss.NewStyle().Width(5).Render(lvlStyle.Render(level)) +
		" " + stText.Render(msg)

	if len(attrs) > 0 {
		result += " " + strings.Join(attrs, " ")
	}
	return result
}

// ── Results screen ───────────────────────────────────────────────────────────

func (m Model) viewResults() string {
	if m.err != nil {
		return stRed.Render("\n  Scan failed: "+m.err.Error()) + "\n"
	}
	if m.report == nil {
		return ""
	}

	r := m.report
	var sb strings.Builder

	// Header
	sb.WriteString("\n")
	title := stTitle.Render("📡 DepsRadar") + "  " +
		lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(clrBorder).
			Padding(0, 1).
			Foreground(clrMuted2).
			Render("v1.1.0")
	sb.WriteString("  " + title + "\n")
	sb.WriteString("  " + stMuted.Render(strings.Repeat("─", 60)) + "\n\n")

	// Projects
	for _, res := range r.Projects {
		sb.WriteString(renderProject(res))
		sb.WriteString("\n")
	}

	// Stats bar
	sb.WriteString(renderStats(r))
	sb.WriteString("\n")

	// Keyhints
	sb.WriteString(renderKeyhints())
	sb.WriteString("\n")

	return sb.String()
}

func renderProject(res model.ScanResult) string {
	var sb strings.Builder

	icon := stTeal.Render("◈")
	name := stAmber.Render(res.Project.Name)
	meta := stMuted.Render(fmt.Sprintf("(%s — %d deps)", res.Project.ManifestType, len(res.Project.Dependencies)))
	sb.WriteString(fmt.Sprintf("  %s  %s  %s\n\n", icon, name, meta))

	if len(res.Vulnerabilities) == 0 && len(res.OutdatedDeps) == 0 {
		sb.WriteString("  " + stTeal.Render("✓") + "  " + stText.Render("No issues found") + "\n")
		return sb.String()
	}

	// Build combined table rows
	var rows [][]string
	for _, v := range res.Vulnerabilities {
		rows = append(rows, []string{
			v.Package,
			v.Version,
			v.ID,
			severityCell(v.Severity),
			"",
		})
	}
	for _, d := range res.OutdatedDeps {
		rows = append(rows, []string{
			d.Name,
			d.Version,
			stMuted.Render("—"),
			severityCell("OUTDATED"),
			stTeal.Render(d.VersionLatest),
		})
	}

	t := lgtable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(clrBorder)).
		Headers("PACKAGE", "VERSION", "CVE", "SEVERITY", "LATEST").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == lgtable.HeaderRow {
				return stHdr
			}
			return lipgloss.NewStyle().Foreground(clrText)
		}).
		Rows(rows...)

	for _, line := range strings.Split(t.Render(), "\n") {
		sb.WriteString("  " + line + "\n")
	}

	return sb.String()
}

func severityCell(sev string) string {
	switch sev {
	case "CRITICAL":
		return lipgloss.NewStyle().
			Foreground(clrRed).Bold(true).
			Border(lipgloss.NormalBorder()).BorderForeground(clrRed).
			Padding(0, 1).Render("CRITICAL")
	case "HIGH":
		return lipgloss.NewStyle().
			Foreground(clrOrange).Bold(true).
			Border(lipgloss.NormalBorder()).BorderForeground(clrOrange).
			Padding(0, 1).Render("HIGH")
	case "MEDIUM":
		return lipgloss.NewStyle().
			Foreground(clrAmber).
			Border(lipgloss.NormalBorder()).BorderForeground(clrAmber).
			Padding(0, 1).Render("MEDIUM")
	case "OUTDATED":
		return lipgloss.NewStyle().
			Foreground(clrTeal).
			Border(lipgloss.NormalBorder()).BorderForeground(clrTeal).
			Padding(0, 1).Render("OUTDATED")
	default:
		return lipgloss.NewStyle().
			Foreground(clrMuted2).
			Border(lipgloss.NormalBorder()).BorderForeground(clrMuted2).
			Padding(0, 1).Render(sev)
	}
}

func renderStats(r *model.Report) string {
	cell := func(label, value string, vs lipgloss.Style) string {
		return lipgloss.NewStyle().
			Width(14).
			Align(lipgloss.Center).
			Render(
				vs.Render(value) + "\n" +
					stMuted.Render(label),
			)
	}

	cells := []string{
		cell("CRITICAL", fmt.Sprintf("%d", r.TotalCritical), stRed),
		cell("HIGH", fmt.Sprintf("%d", r.TotalHigh), stOrange),
		cell("MEDIUM", fmt.Sprintf("%d", r.TotalMedium), lipgloss.NewStyle().Foreground(clrAmber)),
		cell("OUTDATED", fmt.Sprintf("%d", r.TotalOutdated), stTeal),
		cell("DURATION", fmt.Sprintf("%.1fs", r.TotalDuration), stMuted),
	}

	bar := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(clrBorder).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, cells...))

	return "  " + bar
}

func renderKeyhints() string {
	key := func(k string) string {
		return stKeyHint.Render(k)
	}
	hint := func(k, desc string) string {
		return key(k) + " " + stMuted.Render(desc)
	}

	hints := lipgloss.JoinHorizontal(lipgloss.Left,
		hint("q", "quit"),
		"   ",
		hint("e", "export HTML"),
		"   ",
		hint("?", "help"),
	)

	return "  " + hints
}

// ─── Log interceptor ─────────────────────────────────────────────────────────

// ChanWriter is an io.Writer that forwards lines to a channel.
type ChanWriter struct {
	ch chan<- string
}

func NewChanWriter(ch chan<- string) *ChanWriter {
	return &ChanWriter{ch: ch}
}

func (w *ChanWriter) Write(p []byte) (int, error) {
	s := strings.TrimSpace(string(p))
	if s != "" {
		select {
		case w.ch <- s:
		default:
		}
	}
	return len(p), nil
}

// NewLogger creates a slog.Logger that forwards output to the given channel.
func NewLogger(ch chan<- string, level slog.Level) *slog.Logger {
	handler := slog.NewTextHandler(
		io.Discard, // primary output suppressed; we write via ChanWriter
		&slog.HandlerOptions{Level: level},
	)
	_ = handler
	// Use a simple text handler writing to ChanWriter
	return slog.New(slog.NewTextHandler(NewChanWriter(ch), &slog.HandlerOptions{Level: level}))
}
