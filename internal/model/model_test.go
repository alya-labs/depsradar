package model

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		expected int
	}{
		{"same version", "1.0.0", "1.0.0", 0},
		{"patch behind", "1.0.0", "1.0.1", 0},
		{"minor behind", "1.0.0", "1.1.0", 5},
		{"major behind by 1", "1.0.0", "2.0.0", 15},
		{"major behind by 2+", "1.0.0", "3.0.0", 30},
		{"ahead", "2.0.0", "1.0.0", 0},
		{"non-semver different", "abc", "def", 5},
		{"non-semver same", "abc", "abc", 0},
		{"empty latest", "1.0.0", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.current, tt.latest)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVulnerabilityScore(t *testing.T) {
	tests := []struct {
		severity string
		expected int
	}{
		{"CRITICAL", 100},
		{"HIGH", 75},
		{"MEDIUM", 40},
		{"LOW", 10},
		{"UNKNOWN", 0},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			v := Vulnerability{Severity: tt.severity}
			assert.Equal(t, tt.expected, v.Score())
		})
	}
}

func TestDependencyScore(t *testing.T) {
	tests := []struct {
		name     string
		dep      Dependency
		expected int
	}{
		{"no latest", Dependency{Version: "1.0.0"}, 0},
		{"same version", Dependency{Version: "1.0.0", VersionLatest: "1.0.0"}, 0},
		{"outdated", Dependency{Version: "1.0.0", VersionLatest: "2.0.0"}, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.dep.Score())
		})
	}
}

func TestGetSeverityLabel(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, "CRITICAL"},
		{75, "HIGH"},
		{40, "MEDIUM"},
		{10, "LOW"},
		{5, "OUTDATED"},
		{0, "OK"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetSeverityLabel(tt.score))
		})
	}
}

func TestReportPrintJSON(t *testing.T) {
	report := Report{
		TotalCritical: 1,
		TotalHigh:     2,
		TotalMedium:   3,
		TotalLow:      1,
	}

	var buf bytes.Buffer
	report.PrintJSON(&buf)

	var parsed map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &parsed)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), parsed["critical"])
	assert.Equal(t, float64(2), parsed["high"])
	assert.Equal(t, float64(3), parsed["medium"])
	assert.Equal(t, float64(1), parsed["low"])
}

func TestReportPrintSARIF(t *testing.T) {
	report := Report{
		Projects: []ScanResult{
			{
				Project: Project{
					Path:         ".",
					Name:         "test",
					ManifestType: "package.json",
				},
				Vulnerabilities: []Vulnerability{
					{
						ID:       "CVE-2024-0001",
						Package:  "lodash",
						Version:  "4.17.20",
						Severity: "HIGH",
						CVSS:     7.5,
						Title:    "Prototype Pollution",
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	report.PrintSARIF(&buf)

	var sarif SARIFReport
	err := json.Unmarshal(buf.Bytes(), &sarif)
	assert.NoError(t, err)
	assert.Equal(t, "2.1.0", sarif.Version)
	assert.Len(t, sarif.Runs, 1)
	assert.Len(t, sarif.Runs[0].Results, 1)
	assert.Equal(t, "CVE-2024-0001", sarif.Runs[0].Results[0].RuleID)
	assert.Equal(t, "error", sarif.Runs[0].Results[0].Level)
}
