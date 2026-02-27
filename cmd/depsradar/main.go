package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"depsradar/internal/advisory"
	"depsradar/internal/cache"
	"depsradar/internal/config"
	"depsradar/internal/model"
	depsreport "depsradar/internal/report"
	"depsradar/internal/registry"
	"depsradar/internal/scanner"
	"depsradar/internal/tui"
	"depsradar/internal/updater"
)

var (
	Version = "1.1.0"
)

var (
	flagJSON       bool
	flagOnlyVulns  bool
	flagCacheTTL   int
	flagNoCache    bool
	flagExportHTML string
	flagParallel   int
	flagVerbose    bool
	flagRecursive  bool
	flagConfig     string
	flagVersion    bool
)

func init() {
	flag.BoolVar(&flagJSON, "json", false, "Output JSON")
	flag.BoolVar(&flagOnlyVulns, "only-vulns", false, "Show only CVEs")
	flag.IntVar(&flagCacheTTL, "cache-ttl", 24, "Cache TTL in hours")
	flag.BoolVar(&flagNoCache, "no-cache", false, "Disable cache")
	flag.StringVar(&flagExportHTML, "out", "", "Export HTML report")
	flag.IntVar(&flagParallel, "parallel", 10, "Max parallel requests")
	flag.BoolVar(&flagVerbose, "v", false, "Verbose output")
	flag.BoolVar(&flagRecursive, "r", false, "Recursive scan")
	flag.StringVar(&flagConfig, "config", ".depsradar.toml", "Path to config file")
	flag.BoolVar(&flagVersion, "version", false, "Show version")
	flag.Usage = func() {
		fmt.Printf("DepsRadar %s\n", Version)
		fmt.Println("Usage: depsradar <command> [options]")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  scan <paths>     Scan project(s) for vulnerabilities")
		fmt.Println("  export <paths>   Export results to HTML")
		fmt.Println("  self-update      Update depsradar to the latest version")
		fmt.Println("  version          Show version information")
		fmt.Println("")
		fmt.Println("Options:")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if flagVersion {
		fmt.Printf("DepsRadar version %s\n", Version)
		os.Exit(0)
	}

	level := slog.LevelInfo
	if flagVerbose {
		level = slog.LevelDebug
	}
	if flagJSON {
		level = slog.LevelError // Silent logs when outputting JSON
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	switch cmd {
	case "scan":
		checkForUpdates()
		runScan(flag.Args()[1:])
	case "export":
		runExport(flag.Args()[1:])
	case "version":
		fmt.Printf("DepsRadar version %s\n", Version)
	case "self-update":
		runSelfUpdate()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func checkForUpdates() {
	if flagJSON {
		return
	}
	
	newVersion, err := updater.CheckForUpdate(Version)
	if err == nil && newVersion != "" {
		fmt.Fprintf(os.Stderr, "\n✨ Une nouvelle version est disponible : %s (actuelle: %s)\n", newVersion, Version)
		fmt.Fprintf(os.Stderr, "👉 Lancez 'depsradar self-update' pour mettre à jour.\n\n")
	}
}

func runSelfUpdate() {
	slog.Info("Vérification des mises à jour...")
	newVersion, err := updater.CheckForUpdate(Version)
	if err != nil {
		slog.Error("Erreur lors de la vérification", "error", err)
		os.Exit(1)
	}

	if newVersion == "" {
		fmt.Println("✅ Vous utilisez déjà la dernière version.")
		return
	}

	fmt.Printf("Mise à jour vers la version %s...\n", newVersion)
	err = updater.Update(newVersion)
	if err != nil {
		slog.Error("Échec de la mise à jour", "error", err)
		fmt.Println("Note: Vous devrez peut-être lancer cette commande avec sudo.")
		os.Exit(1)
	}

	fmt.Println("🎉 Mise à jour terminée avec succès !")
}

func runScan(paths []string) {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var allManifests []string
	for _, p := range paths {
		m, err := scanner.FindManifests(p, flagRecursive)
		if err != nil {
			slog.Warn("Could not search path", "path", p, "error", err)
			continue
		}
		allManifests = append(allManifests, m...)
	}

	if len(allManifests) == 0 {
		slog.Error("No manifests found")
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(flagConfig)
	if err != nil {
		slog.Warn("Could not load config", "error", err)
	}

	if flagCacheTTL != 24 {
		cfg.CacheTTL = flagCacheTTL
	}
	if flagParallel != 10 {
		cfg.Parallel = flagParallel
	}
	if cfg.Parallel == 0 {
		cfg.Parallel = 10
	}

	// JSON mode: no TUI, silent logs
	if flagJSON {
		report := doScan(allManifests, cfg, nil)
		report.PrintJSON(os.Stdout)
		if report.TotalCritical > 0 || report.TotalHigh > 0 {
			os.Exit(1)
		}
		return
	}

	// TUI mode
	logCh := make(chan string, 200)
	logger := tui.NewLogger(logCh, slog.LevelInfo)
	slog.SetDefault(logger)

	scanFn := func() (*model.Report, error) {
		report := doScan(allManifests, cfg, logCh)
		return &report, nil
	}

	m := tui.New(scanFn, logCh, 0)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func doScan(allManifests []string, cfg *config.Config, logCh chan string) model.Report {
	var cacheDB *cache.DB
	if !flagNoCache {
		ttl := cfg.CacheTTL
		if ttl == 0 {
			ttl = 24
		}
		var err error
		cacheDB, err = cache.NewDB(ttl)
		if err != nil {
			slog.Warn("Cache disabled", "error", err)
		}
	}

	registryClient := registry.NewClient(cfg.Parallel)
	advisoryClient := advisory.NewClient(cfg.Parallel)
	s := scanner.NewScanner(registryClient, advisoryClient, cacheDB, cfg, cfg.Parallel)

	results := make(chan model.ScanResult, len(allManifests))
	start := time.Now()

	for _, m := range allManifests {
		go func(path string) {
			results <- s.ScanManifest(path)
		}(m)
	}

	var scanResults []model.ScanResult
	for i := 0; i < len(allManifests); i++ {
		scanResults = append(scanResults, <-results)
	}

	elapsed := time.Since(start).Seconds()

	report := model.Report{
		Projects:      scanResults,
		TotalDuration: elapsed,
	}

	for _, r := range scanResults {
		for _, v := range r.Vulnerabilities {
			switch v.Severity {
			case "CRITICAL":
				report.TotalCritical++
			case "HIGH":
				report.TotalHigh++
			case "MEDIUM":
				report.TotalMedium++
			}
		}
		report.TotalOutdated += len(r.OutdatedDeps)
	}

	if flagExportHTML != "" {
		report.ExportHTML(flagExportHTML)
	}

	if cacheDB != nil {
		cacheDB.Close()
	}

	return report
}

// keep depsreport imported (used by export command)
var _ = depsreport.Render

func runExport(paths []string) {
	if len(paths) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: depsradar export <path> --out <file.html>")
		os.Exit(1)
	}

	var allManifests []string
	for _, p := range paths {
		m, _ := scanner.FindManifests(p, flagRecursive)
		allManifests = append(allManifests, m...)
	}

	cfg, _ := config.LoadConfig(flagConfig)
	registryClient := registry.NewClient(flagParallel)
	advisoryClient := advisory.NewClient(flagParallel)
	cacheDB, _ := cache.NewDB(flagCacheTTL)
	s := scanner.NewScanner(registryClient, advisoryClient, cacheDB, cfg, flagParallel)

	var results []model.ScanResult
	for _, m := range allManifests {
		results = append(results, s.ScanManifest(m))
	}

	rep := model.Report{Projects: results}
	rep.ExportHTML(flagExportHTML)
	fmt.Printf("Exported to: %s\n", flagExportHTML)
}
