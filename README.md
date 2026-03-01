# DepsRadar

A CLI tool to scan multiple projects for vulnerable or outdated dependencies. Built with Go.

```
        .───────.
       ( ◉  ·  ◉ )
        ╲───┬───╱
            ╨
        ╭───────╮
       ╱ ◠     ◠ ╲
      (  ●     ●  )
       ╲  ◡◡◡  ╱
    ━━━━╰───────╯━━━━
       ╭─────────╮
      ╱ ≋ ≋ ≋ ≋ ≋ ╲
     │ ≋ ≋ ≋ ≋ ≋ ≋ │
      ╲ ≋ ≋ ≋ ≋ ≋ ╱
       ╰─────────╯
      ╱╱╱╱  ▓▓▓▓  ╲╲╲╲

      Perry · DepsRadar
```

## Features

- **Multi-language support**: npm, PyPI, Packagist (PHP/Composer), crates.io (Rust), Go
- **Vulnerability detection**: Uses OSV.dev API (no API key required)
- **Version checking**: Fetches latest versions from package registries
- **Parallel scanning**: Configurable concurrent API requests
- **Multiple output formats**: Terminal TUI, JSON, SARIF v2.1.0, plain text, HTML report
- **SARIF export**: Upload results directly to GitHub Code Scanning
- **Severity filtering**: Show only vulnerabilities at or above a given severity (`--min-severity`)
- **Recursive scanning**: Discover manifests in subdirectories (`-r`)
- **CI fail threshold**: Exit non-zero when findings exceed a configurable severity (`--fail-on`)
- **Config file**: Persist settings and ignore lists in `.depsradar.toml`
- **Self-update**: Built-in `self-update` command to upgrade the binary in place
- **Caching**: SQLite cache with configurable TTL
- **Zero CGO**: Cross-compilable binary using modernc.org/sqlite

## Installation

### Pre-built binaries

Download the latest release from the [GitHub releases](https://github.com/alya-labs/depsradar/releases):

```bash
# Linux
curl -L -o depsradar https://github.com/alya-labs/depsradar/releases/latest/download/depsradar-linux-amd64
chmod +x depsradar
sudo mv depsradar /usr/local/bin/

# macOS
curl -L -o depsradar https://github.com/alya-labs/depsradar/releases/latest/download/depsradar-darwin-amd64
chmod +x depsradar
sudo mv depsradar /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/alya-labs/depsradar.git
cd depsradar
go build -o depsradar ./cmd/depsradar
sudo mv depsradar /usr/local/bin/
```

## Usage

### Commands

```
depsradar scan <paths>     Scan project(s) for vulnerabilities
depsradar export <paths>   Export results to HTML
depsradar self-update      Update depsradar to the latest version
depsradar version          Show version information
```

### Basic scan

```bash
depsradar scan /path/to/project
depsradar scan .                      # scan current directory
depsradar scan ./api ./frontend       # scan multiple projects
depsradar scan -r .                   # scan recursively
```

### Options

```
  -cache-ttl int
        Cache TTL in hours (default 24)
  -config string
        Path to config file (default ".depsradar.toml")
  -fail-on string
        Exit 1 on severity >= threshold: critical, high, medium, low (default "high")
  -format string
        Output format: json, sarif, text
  -json
        Output JSON format
  -min-severity string
        Only show vulnerabilities at or above this severity: critical, high, medium, low
  -no-cache
        Disable cache, force fresh scan
  -no-color
        Disable colored output
  -only-vulns
        Show only CVEs, hide outdated deps
  -out string
        Export HTML report to file
  -parallel int
        Max parallel API requests (default 10)
  -r
        Recursive manifest scanning
  -v
        Verbose output
  -version
        Show version and exit
```

### Examples

```bash
# Scan current directory (interactive TUI)
depsradar scan .

# Recursive scan (finds manifests in all subdirectories)
depsradar scan -r .

# JSON output for CI/CD
depsradar scan --format json . > report.json

# SARIF output (for GitHub Code Scanning)
depsradar scan --format sarif . > results.sarif

# Plain text output (no TUI)
depsradar scan --format text .

# HTML report
depsradar scan --out report.html .

# Fail CI if any HIGH or above finding is detected (default)
depsradar scan --fail-on high .

# Fail CI only on CRITICAL findings
depsradar scan --fail-on critical .

# Show only HIGH and above vulnerabilities
depsradar scan --min-severity high .

# Disable colored output (useful for logs)
depsradar scan --no-color .

# No cache (fresh scan)
depsradar scan --no-cache .

# Custom cache TTL (1 hour)
depsradar scan --cache-ttl 1 .

# Increase parallel requests
depsradar scan --parallel 20 .

# Update depsradar to the latest version
depsradar self-update
```

## Supported Manifests

| Ecosystem | Manifests |
|-----------|-----------|
| npm | `package.json` |
| PyPI | `requirements.txt`, `pyproject.toml` |
| Packagist | `composer.json` |
| crates.io | `Cargo.toml` |
| Go | `go.mod` |

## Output Formats

| Format | Flag | Use case |
|--------|------|----------|
| TUI (default) | _(none)_ | Interactive terminal, human review |
| JSON | `--format json` or `--json` | Machine-readable, CI pipelines |
| SARIF | `--format sarif` | GitHub Code Scanning, security dashboards |
| Text | `--format text` | Plain-text logs, non-interactive terminals |
| HTML | `--out file.html` | Shareable reports |

## Scoring System

| Condition | Score | Label |
|-----------|-------|-------|
| CVE with CVSS >= 9.0 | 100 | CRITICAL |
| CVE with CVSS 7.0–8.9 | 75 | HIGH |
| CVE with CVSS 4.0–6.9 | 40 | MEDIUM |
| CVE with CVSS < 4.0 | 10 | LOW |
| Behind by 2+ major versions | 30 | OUTDATED |
| Behind by 1 major version | 15 | BEHIND |
| Up to date | 0 | OK |

## Exit Codes

- `0`: No findings at or above the `--fail-on` threshold
- `1`: One or more findings at or above the `--fail-on` threshold

The threshold defaults to `high` (exits 1 on HIGH or CRITICAL). Use `--fail-on critical` to only fail on critical findings, or `--fail-on medium` to fail on medium and above.

## Configuration File

Place a `.depsradar.toml` file in your project root to persist settings:

```toml
# .depsradar.toml

# CVE IDs to suppress
ignore_ids = ["CVE-2023-12345", "GHSA-xxxx-yyyy-zzzz"]

# Package names to suppress entirely
ignore_packages = ["example/legacy-pkg"]

# Cache TTL in hours (default: 24)
cache_ttl = 12

# Max parallel API requests (default: 10)
parallel = 20
```

The config file is loaded automatically from the current directory. Use `--config path/to/file.toml` to specify a custom location.

## Cache

Cache is stored at `~/.cache/depsradar/cache.db` (SQLite).

- Default TTL: 24 hours
- Use `--no-cache` to bypass cache
- Use `--cache-ttl=X` to customize TTL in hours

## GitHub Actions

```yaml
name: Dependency Scan

on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    permissions:
      security-events: write

    steps:
      - uses: actions/checkout@v4

      - name: Install depsradar
        run: |
          curl -L -o depsradar https://github.com/alya-labs/depsradar/releases/latest/download/depsradar-linux-amd64
          chmod +x depsradar
          sudo mv depsradar /usr/local/bin/

      - name: Scan dependencies
        run: depsradar scan --format sarif -r . > results.sarif

      - name: Upload SARIF to GitHub Code Scanning
        uses: actions/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

## Example Output

Results screen (TUI mode):

```
  ◠‿●>━━━  DepsRadar  v1.1.0  scanned 2026-03-01 14:32:07
  (≋≋≋≋)
  ╱╱  ╲╲▓▓
  ────────────────────────────────────────────────────────────

  ◈  my-app  (composer.json — 7 deps)  ·  2 vuln  ·  2 outdated

  ╭──────────────────────┬─────────┬────────────────┬──────────┬─────────╮
  │ PACKAGE              │ VERSION │ CVE            │ SEVERITY │ LATEST  │
  ├──────────────────────┼─────────┼────────────────┼──────────┼─────────┤
  │ twig/twig            │ 3.4     │ CVE-2024-1234  │ MEDIUM   │         │
  │ guzzlehttp/guzzle    │ 7.4     │ CVE-2024-5678  │ MEDIUM   │         │
  │ monolog/monolog       │ 3.2     │ —              │ OUTDATED │ 3.10.0  │
  │ symfony/framework... │ 6.0     │ —              │ OUTDATED │ 7.0.0   │
  ╰──────────────────────┴─────────┴────────────────┴──────────┴─────────╯

  ╭────────────────┬────────────────┬────────────────┬────────────────┬────────────────┬────────────────╮
  │       0        │       0        │       2        │       0        │       2        │     0.5s       │
  │  ✖ CRITICAL   │  ▲ HIGH       │  ● MEDIUM     │  ○ LOW        │  ↑ OUTDATED   │  ◷ DURATION   │
  ╰────────────────┴────────────────┴────────────────┴────────────────┴────────────────┴────────────────╯

  ────────────────────────────────────────────────────────────
   q  quit    e  export HTML    j/k  scroll    ?  help
```

## Development

```bash
# Run tests
go test ./...

# Build
go build -o depsradar ./cmd/depsradar
```

## License

MIT
