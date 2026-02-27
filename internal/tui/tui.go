package tui

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
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
	clrDark   = lipgloss.Color("#0D0D0D") // contrast text on bright pill backgrounds
	clrLight  = lipgloss.Color("#E8F4E8") // contrast text on dark pill backgrounds
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
			Background(clrMuted).
			Foreground(clrText).
			Bold(true).
			Padding(0, 1)

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
	state          int
	frame          int
	logs           []string
	report         *model.Report
	err            error
	logCh          <-chan string
	scanFn         ScanFunc
	width          int
	height         int
	totalManifests int
	viewport       viewport.Model
	viewportReady  bool
}

func New(scanFn ScanFunc, logCh <-chan string, width int, totalManifests ...int) Model {
	total := 0
	if len(totalManifests) > 0 {
		total = totalManifests[0]
	}
	return Model{
		state:          stateScanning,
		scanFn:         scanFn,
		logCh:          logCh,
		width:          width,
		totalManifests: total,
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
			filename := fmt.Sprintf("depsradar-report-%s.html", time.Now().Format("2006-01-02-150405"))
			m.report.ExportHTML(filename)
		}
		if m.state == stateResults && m.viewportReady {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateResults {
			// Reserve 2 lines for keyhints footer
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
			m.viewportReady = true
		}

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
		if m.report != nil {
			m.report.Timestamp = time.Now()
		}
		m.state = stateResults

		// Initialize viewport with results content
		content := m.renderResultsContent()
		h := m.height - 2 // reserve space for keyhints
		if h < 10 {
			h = 24
		}
		m.viewport = viewport.New(m.width, h)
		m.viewport.SetContent(content)
		m.viewportReady = true
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
		ds("        .───────."),
		ds("       ( ") + ds(d) + ds(" )"),
		ds("        ╲───┬───╱"),
		pl("            ╨"),
		h("        ╭───────╮"),
		h("       ╱ ") + e("◠") + h("     ") + e("◠") + h(" ╲"),
		h("      (  ") + e("●") + h("     ") + e("●") + h("  )"),
		h("       ╲ ") + h(" ◡◡◡ ") + h(" ╱"),
		b("    ━━━━") + h("╰───────╯") + b("━━━━"),
		bd("       ╭─────────╮"),
		bd("      ╱ ") + wv(w+" "+w+" "+w+" "+w+" "+w) + bd(" ╲"),
		bd("     │ ") + wv(w+" "+w+" "+w+" "+w+" "+w+" "+w) + bd(" │"),
		bd("      ╲ ") + wv(w+" "+w+" "+w+" "+w+" "+w) + bd(" ╱"),
		bd("       ╰─────────╯"),
		ft("      ╱╱╱╱") + tl("  ▓▓▓▓") + ft("  ╲╲╲╲"),
		"",
		stMuted.Render("      Perry · DepsRadar"),
	}

	return strings.Join(lines, "\n")
}

func renderMiniPerry() string {
	h := stHead.Render
	e := stEye.Render
	b := stBill.Render
	bd := stBody.Render
	wv := stWave.Render
	ft := stFoot.Render
	tl := stTail.Render

	lines := []string{
		h("◠‿") + e("●") + b(">━━━"),
		bd("(") + wv("≋≋≋≋") + bd(")"),
		ft("╱╱") + ft("  ") + ft("╲╲") + tl("▓▓"),
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

	// Count scanned manifests from log lines
	scanned := 0
	for _, l := range m.logs {
		if strings.Contains(l, "Scanning manifest") || strings.Contains(l, "msg=Scanning") {
			scanned++
		}
	}
	progressText := "scanning..."
	if m.totalManifests > 0 {
		progressText = fmt.Sprintf("scanning %d/%d manifests...", scanned, m.totalManifests)
	}
	logLines = append(logLines, spin+" "+stMuted.Render(progressText))

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

	if m.viewportReady {
		footer := renderKeyhints()
		scrollPct := fmt.Sprintf(" %3.f%%", m.viewport.ScrollPercent()*100)
		footer += stMuted.Render(scrollPct)
		return m.viewport.View() + "\n" + footer + "\n"
	}

	return m.renderResultsContent() + "\n" + renderKeyhints() + "\n"
}

func (m Model) renderResultsContent() string {
	if m.report == nil {
		return ""
	}

	r := m.report
	var sb strings.Builder

	// Header
	sb.WriteString("\n")
	versionPill := lipgloss.NewStyle().
		Background(clrMuted).Foreground(clrText).Padding(0, 1).Render("v1.1.0")

	var tsStr string
	if !r.Timestamp.IsZero() {
		tsStr = "  " + stMuted.Render("scanned "+r.Timestamp.Format("2006-01-02 15:04:05"))
	}

	miniPerry := renderMiniPerry()
	titleText := stTitle.Render("DepsRadar") + "  " + versionPill + tsStr
	titleBlock := lipgloss.NewStyle().PaddingTop(0).Render(titleText)
	header := lipgloss.JoinHorizontal(lipgloss.Top, "  "+miniPerry, "  ", titleBlock)
	sb.WriteString(header + "\n")
	sb.WriteString("  " + stMuted.Render(strings.Repeat("─", 60)) + "\n\n")

	// Projects
	for _, res := range r.Projects {
		sb.WriteString(renderProject(res))
		sb.WriteString("\n")
	}

	// Errors section
	var totalErrors int
	for _, res := range r.Projects {
		totalErrors += len(res.Errors)
	}
	if totalErrors > 0 {
		sb.WriteString("  " + stRed.Render(fmt.Sprintf("⚠ %d error(s) during scan:", totalErrors)) + "\n")
		for _, res := range r.Projects {
			for _, e := range res.Errors {
				sb.WriteString("    " + stMuted.Render("• "+e) + "\n")
			}
		}
		sb.WriteString("\n")
	}

	// Stats bar
	sb.WriteString(renderStats(r))

	return sb.String()
}

func renderProject(res model.ScanResult) string {
	var sb strings.Builder

	icon := stTeal.Render("◈")
	name := stAmber.Render(res.Project.Name)
	meta := stMuted.Render(fmt.Sprintf("(%s — %d deps)", res.Project.ManifestType, len(res.Project.Dependencies)))

	var issueParts []string
	if len(res.Vulnerabilities) > 0 {
		issueParts = append(issueParts, stRed.Render(fmt.Sprintf("%d vuln", len(res.Vulnerabilities))))
	}
	if len(res.OutdatedDeps) > 0 {
		issueParts = append(issueParts, stTeal.Render(fmt.Sprintf("%d outdated", len(res.OutdatedDeps))))
	}
	headerLine := fmt.Sprintf("  %s  %s  %s", icon, name, meta)
	if len(issueParts) > 0 {
		headerLine += stMuted.Render("  ·  ") + strings.Join(issueParts, stMuted.Render("  ·  "))
	}
	sb.WriteString(headerLine + "\n\n")

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
		BorderStyle(lipgloss.NewStyle().Foreground(clrMuted2)).
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
		return lipgloss.NewStyle().Background(clrRed).Foreground(clrDark).Bold(true).Padding(0, 1).Render("CRITICAL")
	case "HIGH":
		return lipgloss.NewStyle().Background(clrOrange).Foreground(clrDark).Bold(true).Padding(0, 1).Render("HIGH")
	case "MEDIUM":
		return lipgloss.NewStyle().Background(clrAmber).Foreground(clrDark).Bold(true).Padding(0, 1).Render("MEDIUM")
	case "LOW":
		return lipgloss.NewStyle().Background(clrMuted2).Foreground(clrLight).Bold(true).Padding(0, 1).Render("LOW")
	case "OUTDATED":
		return lipgloss.NewStyle().Background(clrTeal).Foreground(clrDark).Bold(true).Padding(0, 1).Render("OUTDATED")
	default:
		return lipgloss.NewStyle().Background(clrMuted2).Foreground(clrLight).Padding(0, 1).Render(sev)
	}
}

func renderStats(r *model.Report) string {
	divider := lipgloss.NewStyle().Foreground(clrMuted2).Render("│\n│")

	cell := func(icon, label, value string, vs lipgloss.Style) string {
		return lipgloss.NewStyle().Width(16).Align(lipgloss.Center).Render(
			vs.Bold(true).Render(value) + "\n" + stMuted.Render(icon+" "+label),
		)
	}

	cells := []string{
		cell("✖", "CRITICAL", fmt.Sprintf("%d", r.TotalCritical), stRed),
		divider,
		cell("▲", "HIGH", fmt.Sprintf("%d", r.TotalHigh), stOrange),
		divider,
		cell("●", "MEDIUM", fmt.Sprintf("%d", r.TotalMedium), lipgloss.NewStyle().Foreground(clrAmber)),
		divider,
		cell("○", "LOW", fmt.Sprintf("%d", r.TotalLow), stMuted),
		divider,
		cell("↑", "OUTDATED", fmt.Sprintf("%d", r.TotalOutdated), stTeal),
		divider,
		cell("◷", "DURATION", fmt.Sprintf("%.1fs", r.TotalDuration), stMuted),
	}

	bar := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(clrMuted2).
		Padding(0, 1).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, cells...))

	var lines []string
	for _, l := range strings.Split(bar, "\n") {
		lines = append(lines, "  "+l)
	}
	return strings.Join(lines, "\n")
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
		hint("j/k", "scroll"),
		"   ",
		hint("?", "help"),
	)

	separator := "  " + stMuted.Render(strings.Repeat("─", 60)) + "\n"
	return separator + "  " + hints
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
