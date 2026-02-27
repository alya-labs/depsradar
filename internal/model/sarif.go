package model

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SARIF types for GitHub Code Scanning integration.
// Follows SARIF v2.1.0 specification.

type SARIFReport struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

type SARIFDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []SARIFRule `json:"rules"`
}

type SARIFRule struct {
	ID               string              `json:"id"`
	ShortDescription SARIFMessage        `json:"shortDescription"`
	HelpURI          string              `json:"helpUri,omitempty"`
	Properties       SARIFRuleProperties `json:"properties"`
}

type SARIFRuleProperties struct {
	SecuritySeverity string   `json:"security-severity"`
	Tags             []string `json:"tags"`
}

type SARIFResult struct {
	RuleID    string            `json:"ruleId"`
	Level     string            `json:"level"`
	Message   SARIFMessage      `json:"message"`
	Locations []SARIFLocation   `json:"locations"`
}

type SARIFMessage struct {
	Text string `json:"text"`
}

type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
}

type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

func sarifLevel(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL", "HIGH":
		return "error"
	case "MEDIUM":
		return "warning"
	default:
		return "note"
	}
}

func sarifSecuritySeverity(cvss float64) string {
	return fmt.Sprintf("%.1f", cvss)
}

func (r *Report) PrintSARIF(w io.Writer) {
	ruleMap := make(map[string]SARIFRule)
	var results []SARIFResult

	for _, project := range r.Projects {
		for _, v := range project.Vulnerabilities {
			if _, exists := ruleMap[v.ID]; !exists {
				title := v.Title
				if title == "" {
					title = fmt.Sprintf("%s vulnerability in %s", v.Severity, v.Package)
				}
				ruleMap[v.ID] = SARIFRule{
					ID:               v.ID,
					ShortDescription: SARIFMessage{Text: title},
					HelpURI:          fmt.Sprintf("https://osv.dev/vulnerability/%s", v.ID),
					Properties: SARIFRuleProperties{
						SecuritySeverity: sarifSecuritySeverity(v.CVSS),
						Tags:             []string{"security", "dependency"},
					},
				}
			}

			msg := fmt.Sprintf("Vulnerable dependency %s@%s (%s: %s)",
				v.Package, v.Version, v.Severity, v.ID)
			if v.FixedVersion != "" {
				msg += fmt.Sprintf(". Fix available: upgrade to %s", v.FixedVersion)
			}

			results = append(results, SARIFResult{
				RuleID:  v.ID,
				Level:   sarifLevel(v.Severity),
				Message: SARIFMessage{Text: msg},
				Locations: []SARIFLocation{
					{
						PhysicalLocation: SARIFPhysicalLocation{
							ArtifactLocation: SARIFArtifactLocation{
								URI: project.Project.Path + "/" + project.Project.ManifestType,
							},
						},
					},
				},
			})
		}
	}

	var rules []SARIFRule
	for _, rule := range ruleMap {
		rules = append(rules, rule)
	}

	sarif := SARIFReport{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:           "DepsRadar",
						Version:        "1.1.0",
						InformationURI: "https://github.com/depsradar/depsradar",
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(sarif)
}
