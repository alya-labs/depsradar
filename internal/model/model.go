package model

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

type Dependency struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	VersionLatest string `json:"latest,omitempty"`
	Ecosystem     string `json:"ecosystem"`
}

type Vulnerability struct {
	ID            string  `json:"id"`
	Package       string  `json:"package"`
	Version       string  `json:"version"`
	Severity      string  `json:"severity"`
	CVSS          float64 `json:"cvss"`
	Title         string  `json:"title,omitempty"`
	FixedVersion  string  `json:"fixed_version,omitempty"`
}

type Project struct {
	Path         string       `json:"path"`
	Name         string       `json:"name"`
	ManifestType string       `json:"manifest_type"`
	Dependencies []Dependency `json:"dependencies"`
	Errors       []string     `json:"errors,omitempty"`
}

type ScanResult struct {
	Project         Project         `json:"project"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	OutdatedDeps    []Dependency    `json:"outdated"`
	Duration        float64         `json:"duration_ms"`
	Errors          []string        `json:"errors,omitempty"`
}

type Report struct {
	Projects      []ScanResult `json:"projects"`
	TotalCritical int           `json:"critical"`
	TotalHigh     int           `json:"high"`
	TotalMedium   int           `json:"medium"`
	TotalOutdated int           `json:"outdated"`
	TotalDuration float64       `json:"total_duration"`
	Timestamp     time.Time     `json:"timestamp"`
}

func (r *Report) PrintJSON(w io.Writer) {
	r.Timestamp = time.Now()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(r)
}

func (r *Report) RenderTerminal(w io.Writer) {
	RenderText(w, r)
}

func RenderText(w io.Writer, r *Report) {
	fmt.Fprintf(w, "DepsRadar — %d projets scannés\n\n", len(r.Projects))
	for _, p := range r.Projects {
		fmt.Fprintf(w, "%s (%s — %d deps)\n", p.Project.Name, p.Project.ManifestType, len(p.Project.Dependencies))
		if len(p.Vulnerabilities) > 0 {
			for _, v := range p.Vulnerabilities {
				fmt.Fprintf(w, "  [%s] %s %s\n", v.Severity, v.Package, v.Version)
			}
		}
		if len(p.OutdatedDeps) > 0 {
			for _, d := range p.OutdatedDeps {
				fmt.Fprintf(w, "  [OUTDATED] %s %s -> %s\n", d.Name, d.Version, d.VersionLatest)
			}
		}
		if len(p.Vulnerabilities) == 0 && len(p.OutdatedDeps) == 0 {
			fmt.Fprintf(w, "  ✅ Aucun problème\n")
		}
	}
	fmt.Fprintf(w, "\n%d critiques · %d hautes · %d en retard · scan en %.1fs\n",
		r.TotalCritical, r.TotalHigh, r.TotalOutdated, r.TotalDuration)
}

func (r *Report) ExportHTML(path string) {
	r.Timestamp = time.Now()
	content := GenerateHTML(r)
	os.WriteFile(path, []byte(content), 0644)
}

func GenerateHTML(r *Report) string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DepsRadar Report</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; background: #1a1a2e; color: #eee; }
        h1 { color: #86efac; }
        .project { background: #16213e; padding: 15px; margin: 10px 0; border-radius: 8px; }
        .project-name { font-size: 1.2em; font-weight: bold; color: #86efac; }
        table { width: 100%; border-collapse: collapse; margin: 10px 0; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #333; }
        th { color: #86efac; }
        .critical { color: #f87171; }
        .high { color: #fb923c; }
        .medium { color: #fbbf24; }
        .behind { color: #facc15; }
        .outdated { color: #a3e635; }
        .ok { color: #4ade80; }
        .summary { background: #0f3460; padding: 15px; border-radius: 8px; margin-top: 20px; }
    </style>
</head>
<body>
    <h1>DepsRadar Report</h1>
`)

	for _, result := range r.Projects {
		b.WriteString(fmt.Sprintf(`<div class="project"><div class="project-name">%s (%s — %d deps)</div>`,
			result.Project.Name, result.Project.ManifestType, len(result.Project.Dependencies)))

		if len(result.Vulnerabilities) > 0 {
			b.WriteString(`<table><tr><th>Package</th><th>Version</th><th>CVE</th><th>Severity</th></tr>`)
			for _, v := range result.Vulnerabilities {
				sevClass := strings.ToLower(v.Severity)
				b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td class="%s">%s</td></tr>`,
					v.Package, v.Version, v.ID, sevClass, v.Severity))
			}
			b.WriteString("</table>")
		}

		if len(result.OutdatedDeps) > 0 {
			b.WriteString(`<table><tr><th>Package</th><th>Current</th><th>Latest</th></tr>`)
			for _, d := range result.OutdatedDeps {
				b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td></tr>",
					d.Name, d.Version, d.VersionLatest))
			}
			b.WriteString("</table>")
		}

		if len(result.Vulnerabilities) == 0 && len(result.OutdatedDeps) == 0 {
			b.WriteString(`<span class="ok">✅ Aucun problème</span>`)
		}

		b.WriteString("</div>")
	}

	b.WriteString(fmt.Sprintf(`<div class="summary">
        <strong>Résumé:</strong> %d critiques · %d hautes · %d en retard · scan en %.1fs
    </div>
</body></html>`, r.TotalCritical, r.TotalHigh, r.TotalOutdated, r.TotalDuration))

	return b.String()
}

func (d *Dependency) Score() int {
	if d.VersionLatest == "" || d.Version == d.VersionLatest {
		return 0
	}
	return CompareVersions(d.Version, d.VersionLatest)
}

func (v *Vulnerability) Score() int {
	switch v.Severity {
	case "CRITICAL":
		return 100
	case "HIGH":
		return 75
	case "MEDIUM":
		return 40
	}
	return 0
}

func CompareVersions(current, latest string) int {
	vCurrent, err1 := semver.NewVersion(current)
	vLatest, err2 := semver.NewVersion(latest)

	if err1 != nil || err2 != nil {
		// Fallback to simple comparison if not semver
		if latest != "" && current != latest {
			return 5 // Mark as outdated
		}
		return 0
	}

	if vLatest.LessThan(vCurrent) || vLatest.Equal(vCurrent) {
		return 0
	}

	diffMajor := vLatest.Major() - vCurrent.Major()
	if diffMajor >= 2 {
		return 30
	}
	if diffMajor == 1 {
		return 15
	}
	if vLatest.Minor() > vCurrent.Minor() {
		return 5
	}
	return 0
}

func GetSeverityLabel(score int) string {
	switch {
	case score >= 100:
		return "CRITICAL"
	case score >= 75:
		return "HIGH"
	case score >= 40:
		return "MEDIUM"
	case score >= 15:
		return "BEHIND"
	case score >= 5:
		return "OUTDATED"
	default:
		return "OK"
	}
}
