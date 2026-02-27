package scanner

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"depsradar/internal/model"

	"github.com/BurntSushi/toml"
)

var manifestFiles = []string{
	"package.json",
	"composer.json",
	"requirements.txt",
	"pyproject.toml",
	"Cargo.toml",
	"go.mod",
}

func FindManifests(root string, recursive bool) ([]string, error) {
	var manifests []string

	if !recursive {
		m, err := FindManifest(root)
		if err != nil {
			return nil, err
		}
		return []string{m}, nil
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip node_modules, vendor, etc.
			if info.Name() == "node_modules" || info.Name() == "vendor" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		filename := filepath.Base(path)
		for _, mf := range manifestFiles {
			if filename == mf {
				manifests = append(manifests, path)
				break
			}
		}
		return nil
	})

	return manifests, err
}

func FindManifest(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		for _, mf := range manifestFiles {
			manifestPath := filepath.Join(path, mf)
			if _, err := os.Stat(manifestPath); err == nil {
				return manifestPath, nil
			}
		}
		return "", errors.New("no manifest found")
	}

	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

type ManifestParser interface {
	Parse(path string) (*model.Project, error)
	SupportedExtensions() []string
}

func DetectParser(path string) (ManifestParser, error) {
	filename := filepath.Base(path)

	switch filename {
	case "package.json":
		return &PackageJSONParser{}, nil
	case "composer.json":
		return &ComposerParser{}, nil
	case "requirements.txt":
		return &RequirementsParser{}, nil
	case "pyproject.toml":
		return &PyProjectParser{}, nil
	case "Cargo.toml":
		return &CargoTomlParser{}, nil
	case "go.mod":
		return &GoModParser{}, nil
	default:
		return nil, errors.New("unsupported manifest type")
	}
}

type PackageJSONParser struct{}

type PackageJSON struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

func (p *PackageJSONParser) Parse(path string) (*model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	name := pkg.Name
	if name == "" {
		name = filepath.Base(dir)
	}

	deps := make([]model.Dependency, 0, len(pkg.Dependencies))
	for name, version := range pkg.Dependencies {
		deps = append(deps, model.Dependency{
			Name:      name,
			Version:   cleanVersion(version),
			Ecosystem: "npm",
		})
	}

	return &model.Project{
		Path:         dir,
		Name:         name,
		ManifestType: "package.json",
		Dependencies: deps,
	}, nil
}

func (p *PackageJSONParser) SupportedExtensions() []string {
	return []string{"package.json"}
}

func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	// Remove common prefixes
	prefixes := []string{"^", "~", ">=", "<=", ">", "<", "v"}
	for _, p := range prefixes {
		if strings.HasPrefix(v, p) {
			v = strings.TrimPrefix(v, p)
			// Don't break, some versions might have multiple prefixes like ">= v1.2.3"
			return cleanVersion(v) 
		}
	}
	return v
}

type RequirementsParser struct{}

func (p *RequirementsParser) Parse(path string) (*model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	name := filepath.Base(dir)

	deps := make([]model.Dependency, 0)

	re := regexp.MustCompile(`^([a-zA-Z0-9_-]+)([<>=!~]+)(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) == 4 {
			deps = append(deps, model.Dependency{
				Name:      matches[1],
				Version:   cleanVersion(matches[3]),
				Ecosystem: "PyPI",
			})
		}
	}

	return &model.Project{
		Path:         dir,
		Name:         name,
		ManifestType: "requirements.txt",
		Dependencies: deps,
	}, nil
}

func (p *RequirementsParser) SupportedExtensions() []string {
	return []string{"requirements.txt"}
}

type PyProjectParser struct{}

type PyProject struct {
	Project struct {
		Name         string `toml:"name"`
		Dependencies []string `toml:"dependencies"`
	} `toml:"project"`
}

func (p *PyProjectParser) Parse(path string) (*model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var proj PyProject
	if err := toml.Unmarshal(data, &proj); err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	name := proj.Project.Name
	if name == "" {
		name = filepath.Base(dir)
	}

	deps := make([]model.Dependency, 0)
	for _, dep := range proj.Project.Dependencies {
		nameVer := strings.Split(dep, " ")
		depName := nameVer[0]
		ver := ""
		if len(nameVer) > 1 {
			ver = cleanVersion(strings.Trim(nameVer[1], "\""))
		}
		deps = append(deps, model.Dependency{
			Name:      depName,
			Version:   ver,
			Ecosystem: "PyPI",
		})
	}

	return &model.Project{
		Path:         dir,
		Name:         name,
		ManifestType: "pyproject.toml",
		Dependencies: deps,
	}, nil
}

func (p *PyProjectParser) SupportedExtensions() []string {
	return []string{"pyproject.toml"}
}

type CargoTomlParser struct{}

type Cargo struct {
	Dependencies map[string]tomlInfo `toml:"dependencies"`
}

type tomlInfo struct {
	Version string
}

func (p *CargoTomlParser) Parse(path string) (*model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cargo Cargo
	if err := toml.Unmarshal(data, &cargo); err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	name := filepath.Base(dir)

	deps := make([]model.Dependency, 0, len(cargo.Dependencies))
	for depName, info := range cargo.Dependencies {
		deps = append(deps, model.Dependency{
			Name:      depName,
			Version:   cleanVersion(info.Version),
			Ecosystem: "crates.io",
		})
	}

	return &model.Project{
		Path:         dir,
		Name:         name,
		ManifestType: "Cargo.toml",
		Dependencies: deps,
	}, nil
}

func (p *CargoTomlParser) SupportedExtensions() []string {
	return []string{"Cargo.toml"}
}

type GoModParser struct{}

func (p *GoModParser) Parse(path string) (*model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	name := filepath.Base(dir)

	deps := make([]model.Dependency, 0)

	re := regexp.MustCompile(`^\s*([a-zA-Z0-9./_-]+)\s+v?(\S+)`)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name = parts[1]
			}
			continue
		}
		if strings.HasPrefix(line, "require (") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			deps = append(deps, model.Dependency{
				Name:      matches[1],
				Version:   matches[2],
				Ecosystem: "Go",
			})
		}
	}

	return &model.Project{
		Path:         dir,
		Name:         name,
		ManifestType: "go.mod",
		Dependencies: deps,
	}, nil
}

func (p *GoModParser) SupportedExtensions() []string {
	return []string{"go.mod"}
}

type ComposerParser struct{}

type Composer struct {
	Name       string          `json:"name"`
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

func (p *ComposerParser) Parse(path string) (*model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var comp Composer
	if err := json.Unmarshal(data, &comp); err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	name := comp.Name
	if name == "" {
		name = filepath.Base(dir)
	}

	deps := make([]model.Dependency, 0)

	for pkg, version := range comp.Require {
		if pkg == "php" || strings.HasPrefix(pkg, "ext-") {
			continue
		}
		deps = append(deps, model.Dependency{
			Name:      pkg,
			Version:   cleanVersion(version),
			Ecosystem: "Packagist",
		})
	}

	for pkg, version := range comp.RequireDev {
		deps = append(deps, model.Dependency{
			Name:      pkg,
			Version:   cleanVersion(version),
			Ecosystem: "Packagist",
		})
	}

	return &model.Project{
		Path:         dir,
		Name:         name,
		ManifestType: "composer.json",
		Dependencies: deps,
	}, nil
}

func (p *ComposerParser) SupportedExtensions() []string {
	return []string{"composer.json"}
}
