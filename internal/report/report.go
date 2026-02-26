package report

import (
	"fmt"
	"io"

	"depsradar/internal/model"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	criticalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
	highStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("202"))
	mediumStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
	behindStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))
	outdatedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("228"))
	okStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))
)

func Render(w io.Writer, r *model.Report) {
	title := titleStyle.Render(fmt.Sprintf("DepsRadar — %d projets scannés", len(r.Projects)))

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	content := []string{title, ""}

	for _, result := range r.Projects {
		projectLine := fmt.Sprintf("%s (%s — %d deps)",
			result.Project.Name,
			result.Project.ManifestType,
			len(result.Project.Dependencies))

		if len(result.Vulnerabilities) == 0 && len(result.OutdatedDeps) == 0 {
			content = append(content, projectLine+" ✅ Aucun problème")
			continue
		}

		content = append(content, projectLine)

		if len(result.Vulnerabilities) > 0 {
			content = append(content, renderVulnTable(result.Vulnerabilities)...)
		}

		if len(result.OutdatedDeps) > 0 {
			content = append(content, renderOutdatedTable(result.OutdatedDeps)...)
		}
	}

	summary := fmt.Sprintf("%d critiques · %d hautes · %d en retard · scan en %.1fs",
		r.TotalCritical, r.TotalHigh, r.TotalOutdated, r.TotalDuration)
	content = append(content, "", summary)

	final := border.Render(lipgloss.JoinVertical(lipgloss.Left, content...))
	fmt.Fprintln(w, final)
}

func renderVulnTable(vulns []model.Vulnerability) []string {
	rows := [][]string{
		{"Package", "Version", "CVE", "Severity"},
	}

	for _, v := range vulns {
		severity := getSeverityStyle(v.Severity).Render(v.Severity)
		rows = append(rows, []string{
			v.Package,
			v.Version,
			v.ID,
			severity,
		})
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return headerStyle
			}
			return lipgloss.NewStyle()
		}).
		Rows(rows...)

	return []string{t.Render(), ""}
}

func renderOutdatedTable(deps []model.Dependency) []string {
	rows := [][]string{
		{"Package", "Current", "Latest"},
	}

	for _, d := range deps {
		rows = append(rows, []string{
			d.Name,
			d.Version,
			d.VersionLatest,
		})
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return headerStyle
			}
			return lipgloss.NewStyle()
		}).
		Rows(rows...)

	return []string{t.Render(), ""}
}

func getSeverityStyle(severity string) lipgloss.Style {
	switch severity {
	case "CRITICAL":
		return criticalStyle
	case "HIGH":
		return highStyle
	case "MEDIUM":
		return mediumStyle
	case "BEHIND":
		return behindStyle
	case "OUTDATED":
		return outdatedStyle
	default:
		return okStyle
	}
}
