package scanner

import (
	"fmt"
	"log/slog"
	"time"

	"depsradar/internal/advisory"
	"depsradar/internal/cache"
	"depsradar/internal/config"
	"depsradar/internal/model"
	"depsradar/internal/registry"
)

type Scanner struct {
	registryClient *registry.Client
	advisoryClient *advisory.Client
	cacheDB        *cache.DB
	config         *config.Config
	parallel       int
}

func NewScanner(reg *registry.Client, adv *advisory.Client, db *cache.DB, cfg *config.Config, parallel int) *Scanner {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &Scanner{
		registryClient: reg,
		advisoryClient: adv,
		cacheDB:        db,
		config:         cfg,
		parallel:       parallel,
	}
}

func (s *Scanner) ScanManifest(manifestPath string) model.ScanResult {
	start := time.Now()
	result := model.ScanResult{}

	slog.Info("Scanning manifest", "path", manifestPath)

	parser, err := DetectParser(manifestPath)
	if err != nil {
		slog.Error("Parser not detected", "manifest", manifestPath, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", manifestPath, err))
		return result
	}

	project, err := parser.Parse(manifestPath)
	if err != nil {
		slog.Error("Failed to parse manifest", "manifest", manifestPath, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", manifestPath, err))
		return result
	}

	result.Project = *project
	
	// Filter ignored packages
	var activeDeps []model.Dependency
	for _, d := range project.Dependencies {
		if s.config.IsIgnoredPackage(d.Name) {
			slog.Debug("Ignoring package", "name", d.Name)
			continue
		}
		activeDeps = append(activeDeps, d)
	}
	deps := activeDeps

	// Step 1: Check latest versions in parallel
	type latestResult struct {
		index  int
		latest string
		err    error
	}

	latestChan := make(chan latestResult, len(deps))
	sem := make(chan struct{}, s.parallel)

	for i, dep := range deps {
		sem <- struct{}{}
		go func(i int, d model.Dependency) {
			defer func() { <-sem }()

			cacheKey := cache.MakeKey(d.Ecosystem, d.Name, "latest")
			if s.cacheDB != nil {
				if val, found := s.cacheDB.Get(cacheKey); found {
					latestChan <- latestResult{i, val, nil}
					return
				}
			}

			latest, err := s.registryClient.GetLatestVersion(d.Name, d.Ecosystem)
			if err == nil && s.cacheDB != nil && latest != "" {
				s.cacheDB.Set(cacheKey, latest)
			}
			latestChan <- latestResult{i, latest, err}
		}(i, dep)
	}

	for i := 0; i < len(deps); i++ {
		res := <-latestChan
		if res.err != nil {
			slog.Warn("Error checking latest version", "name", deps[res.index].Name, "error", res.err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", deps[res.index].Name, res.err))
		}
		result.Project.Dependencies[res.index].VersionLatest = res.latest
	}

	// Step 2: Check vulnerabilities in batch
	vulnsBatch, err := s.advisoryClient.GetVulnerabilitiesBatch(result.Project.Dependencies)
	if err != nil {
		slog.Error("Vulnerability scan failed", "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("OSV scan failed: %v", err))
	} else {
		for i, vulns := range vulnsBatch {
			if i >= len(result.Project.Dependencies) {
				break
			}
			var filteredVulns []model.Vulnerability
			for _, v := range vulns {
				if s.config.IsIgnoredID(v.ID) {
					slog.Debug("Ignoring vulnerability", "id", v.ID)
					continue
				}
				filteredVulns = append(filteredVulns, v)
			}

			if len(filteredVulns) > 0 {
				result.Vulnerabilities = append(result.Vulnerabilities, filteredVulns...)
			} else {
				// Only check if outdated if not vulnerable
				dep := result.Project.Dependencies[i]
				if dep.VersionLatest != "" && dep.Version != dep.VersionLatest {
					result.OutdatedDeps = append(result.OutdatedDeps, dep)
				}
			}
		}
	}

	result.Duration = time.Since(start).Seconds()
	return result
}
