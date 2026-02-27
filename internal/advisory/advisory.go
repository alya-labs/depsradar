package advisory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"depsradar/internal/httputil"
	"depsradar/internal/model"
)

type Client struct {
	http     *http.Client
	sem      chan struct{}
	parallel int
}

type OSVQuery struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Version string `json:"version"`
}

type OSVResponse struct {
	Vulns []OSVVuln `json:"vulns"`
}

type OSVVuln struct {
	ID        string        `json:"id"`
	Summary   string        `json:"summary"`
	Details   string        `json:"details"`
	Severity  []OSVSeverity `json:"severity"`
	Affected  []OSVAffected `json:"affected"`
	Published string        `json:"published"`
	Modified  string        `json:"modified"`
}

type OSVSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type OSVAffected struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Ranges []struct {
		Type   string `json:"type"`
		Events []struct {
			Introduced  string `json:"introduced"`
			Fixed       string `json:"fixed"`
			LastAffects string `json:"last_affected"`
		} `json:"events"`
	} `json:"ranges"`
	Versions []string `json:"versions"`
}

type OSVBatchQuery struct {
	Queries []OSVQuery `json:"queries"`
}

type OSVBatchResponse struct {
	Results []OSVResponse `json:"results"`
}

var ecosystemMap = map[string]string{
	"npm":       "npm",
	"PyPI":      "PyPI",
	"crates.io": "crates.io",
	"Go":        "Go",
	"Packagist": "Packagist",
}

func NewClient(parallel int) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		sem:      make(chan struct{}, parallel),
		parallel: parallel,
	}
}

func (c *Client) GetVulnerabilitiesBatch(deps []model.Dependency) ([][]model.Vulnerability, error) {
	if len(deps) == 0 {
		return nil, nil
	}

	batchQuery := OSVBatchQuery{
		Queries: make([]OSVQuery, 0, len(deps)),
	}

	for _, d := range deps {
		osvEco, ok := ecosystemMap[d.Ecosystem]
		if !ok || d.Version == "" {
			batchQuery.Queries = append(batchQuery.Queries, OSVQuery{})
			continue
		}
		q := OSVQuery{}
		q.Package.Name = d.Name
		q.Package.Ecosystem = osvEco
		q.Version = d.Version
		batchQuery.Queries = append(batchQuery.Queries, q)
	}

	body, err := json.Marshal(batchQuery)
	if err != nil {
		return nil, err
	}

	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	req, err := http.NewRequest("POST", "https://api.osv.dev/v1/querybatch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "depsradar/1.1.0")

	resp, err := httputil.DoWithRetry(c.http, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("osv batch: status %d", resp.StatusCode)
	}

	var batchResp OSVBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, err
	}

	results := make([][]model.Vulnerability, len(deps))
	for i, osvResp := range batchResp.Results {
		if i >= len(deps) {
			break
		}
		if len(osvResp.Vulns) == 0 {
			continue
		}

		vulns := make([]model.Vulnerability, 0, len(osvResp.Vulns))
		for _, v := range osvResp.Vulns {
			severity := c.parseSeverity(v.Severity)
			fixed := c.parseFixedVersion(v.Affected)

			vulns = append(vulns, model.Vulnerability{
				ID:           v.ID,
				Package:      deps[i].Name,
				Version:      deps[i].Version,
				Severity:     severity.label,
				CVSS:         severity.cvss,
				Title:        v.Summary,
				FixedVersion: fixed,
			})
		}
		results[i] = vulns
	}

	return results, nil
}

type severityResult struct {
	label string
	cvss  float64
}

func (c *Client) parseSeverity(sev []OSVSeverity) severityResult {
	if len(sev) == 0 {
		return severityResult{label: "MEDIUM", cvss: 5.0}
	}

	var cvss float64
	fmt.Sscanf(sev[0].Score, "%f", &cvss)

	label := "LOW"
	if cvss >= 9.0 {
		label = "CRITICAL"
	} else if cvss >= 7.0 {
		label = "HIGH"
	} else if cvss >= 4.0 {
		label = "MEDIUM"
	}

	return severityResult{label: label, cvss: cvss}
}

func (c *Client) parseFixedVersion(affected []OSVAffected) string {
	for _, a := range affected {
		for _, r := range a.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" {
					return e.Fixed
				}
			}
		}
		for _, v := range a.Versions {
			return v
		}
	}
	return ""
}
