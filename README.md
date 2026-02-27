# DepsRadar

A CLI tool to scan multiple projects for vulnerable or outdated dependencies. Built with Go.

## Features

- **Multi-language support**: npm, PyPI, Packagist (PHP/Composer), crates.io (Rust), Go
- **Vulnerability detection**: Uses OSV.dev API (no API key required)
- **Version checking**: Fetches latest versions from package registries
- **Parallel scanning**: Configurable concurrent API requests
- **Multiple output formats**: Terminal, JSON, HTML report
- **Caching**: SQLite cache with configurable TTL
- **Zero CGO**: Cross-compilable binary using modernc.org/sqlite

## Installation

### Pre-built binaries

Download the latest release from the [GitHub releases](https://github.com/yourusername/depsradar/releases):

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

### Basic scan

```bash
depsradar scan /path/to/project
depsradar scan .                    # scan current directory
depsradar scan ./api ./frontend    # scan multiple projects
```

### Options

```bash
  -cache-ttl int
        Cache TTL in hours (default 24)
  -json   Output JSON format
  -no-cache
        Disable cache, force fresh scan
  -only-vulns
        Show only CVEs, hide outdated deps
  -out string
        Export HTML report
  -parallel int
        Max parallel API requests (default 10)
```

### Examples

```bash
# Scan current directory
depsradar scan

# JSON output for CI/CD
depsradar --json scan . > report.json

# HTML report
depsradar --out report.html scan .

# No cache (fresh scan)
depsradar --no-cache scan .

# Custom cache TTL (e.g., 1 hour)
depsradar --cache-ttl 1 scan .

# More parallel requests
depsradar --parallel 20 scan .
```

## Supported Manifests

| Ecosystem | Manifests |
|-----------|-----------|
| npm | `package.json` |
| PyPI | `requirements.txt`, `pyproject.toml` |
| Packagist | `composer.json` |
| crates.io | `Cargo.toml` |
| Go | `go.mod` |

## Scoring System

| Condition | Score | Label |
|-----------|-------|-------|
| CVE with CVSS >= 9.0 | 100 | CRITICAL |
| CVE with CVSS 7.0-8.9 | 75 | HIGH |
| CVE with CVSS 4.0-6.9 | 40 | MEDIUM |
| Behind by 2+ major versions | 30 | OUTDATED |
| Behind by 1 major version | 15 | BEHIND |
| Up to date | 0 | OK |

## Cache

Cache is stored at `~/.cache/depsradar/cache.db` (SQLite).

- Default TTL: 24 hours
- Use `--no-cache` to bypass cache
- Use `--cache-ttl=X` to customize TTL

## Exit Codes

- `0`: No critical or high vulnerabilities
- `1`: Critical or high vulnerabilities found

## Example Output

```
DepsRadar — 1 projets scannés

my-app (composer.json — 7 deps)
  [MEDIUM] twig/twig 3.4
  [MEDIUM] guzzlehttp/guzzle 7.4
  [OUTDATED] monolog/monolog 3.2 -> 3.10.0
  [OUTDATED] symfony/framework-bundle 6.0 -> v7.0.0

0 critiques · 0 hautes · 2 en retard · scan en 0.5s
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
