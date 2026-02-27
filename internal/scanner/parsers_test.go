package scanner

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── GoModParser ──────────────────────────────────────────────────────────────

func TestGoModParser_Parse(t *testing.T) {
	parser := &GoModParser{}
	project, err := parser.Parse(filepath.Join("testdata", "go.mod"))

	require.NoError(t, err)
	assert.Equal(t, "example.com/testproject", project.Name)
	assert.Equal(t, "go.mod", project.ManifestType)
	assert.GreaterOrEqual(t, len(project.Dependencies), 3)
}

func TestGoModParser_Dependencies(t *testing.T) {
	parser := &GoModParser{}
	project, err := parser.Parse(filepath.Join("testdata", "go.mod"))
	require.NoError(t, err)

	depMap := make(map[string]string)
	for _, d := range project.Dependencies {
		depMap[d.Name] = d.Version
		assert.Equal(t, "Go", d.Ecosystem)
	}

	assert.Equal(t, "1.8.0", depMap["github.com/gorilla/mux"])
	assert.Equal(t, "1.9.3", depMap["github.com/sirupsen/logrus"])
	assert.Equal(t, "0.14.0", depMap["golang.org/x/text"])
}

func TestGoModParser_SupportedExtensions(t *testing.T) {
	parser := &GoModParser{}
	assert.Contains(t, parser.SupportedExtensions(), "go.mod")
}

// ── RequirementsParser ───────────────────────────────────────────────────────

func TestRequirementsParser_Parse(t *testing.T) {
	parser := &RequirementsParser{}
	project, err := parser.Parse(filepath.Join("testdata", "requirements.txt"))

	require.NoError(t, err)
	assert.Equal(t, "requirements.txt", project.ManifestType)
	assert.Equal(t, 4, len(project.Dependencies))
}

func TestRequirementsParser_Dependencies(t *testing.T) {
	parser := &RequirementsParser{}
	project, err := parser.Parse(filepath.Join("testdata", "requirements.txt"))
	require.NoError(t, err)

	depMap := make(map[string]string)
	for _, d := range project.Dependencies {
		depMap[d.Name] = d.Version
		assert.Equal(t, "PyPI", d.Ecosystem)
	}

	assert.Equal(t, "2.28.0", depMap["requests"])
	assert.Equal(t, "2.0.0", depMap["flask"])
	assert.Equal(t, "1.24.0", depMap["numpy"])
}

func TestRequirementsParser_SkipsComments(t *testing.T) {
	parser := &RequirementsParser{}
	project, err := parser.Parse(filepath.Join("testdata", "requirements.txt"))
	require.NoError(t, err)

	for _, d := range project.Dependencies {
		assert.NotEqual(t, "#", string(d.Name[0]))
	}
}

// ── PyProjectParser ──────────────────────────────────────────────────────────

func TestPyProjectParser_Parse(t *testing.T) {
	parser := &PyProjectParser{}
	project, err := parser.Parse(filepath.Join("testdata", "pyproject.toml"))

	require.NoError(t, err)
	assert.Equal(t, "test-pyproject", project.Name)
	assert.Equal(t, "pyproject.toml", project.ManifestType)
	assert.Equal(t, 4, len(project.Dependencies))
}

func TestPyProjectParser_PEP508NoSpace(t *testing.T) {
	parser := &PyProjectParser{}
	project, err := parser.Parse(filepath.Join("testdata", "pyproject.toml"))
	require.NoError(t, err)

	depMap := make(map[string]string)
	for _, d := range project.Dependencies {
		depMap[d.Name] = d.Version
		assert.Equal(t, "PyPI", d.Ecosystem)
	}

	// "requests>=2.25.0" should parse correctly without space
	assert.Equal(t, "2.25.0", depMap["requests"])
	// "flask" with no version
	assert.Equal(t, "", depMap["flask"])
	// "click>=8.0.0"
	assert.Equal(t, "8.0.0", depMap["click"])
}

// ── CargoTomlParser ──────────────────────────────────────────────────────────

func TestCargoTomlParser_Parse(t *testing.T) {
	parser := &CargoTomlParser{}
	project, err := parser.Parse(filepath.Join("testdata", "Cargo.toml"))

	require.NoError(t, err)
	assert.Equal(t, "Cargo.toml", project.ManifestType)
	assert.Equal(t, 3, len(project.Dependencies))
}

func TestCargoTomlParser_Dependencies(t *testing.T) {
	parser := &CargoTomlParser{}
	project, err := parser.Parse(filepath.Join("testdata", "Cargo.toml"))
	require.NoError(t, err)

	depMap := make(map[string]string)
	for _, d := range project.Dependencies {
		depMap[d.Name] = d.Version
		assert.Equal(t, "crates.io", d.Ecosystem)
	}

	assert.Equal(t, "1.0", depMap["serde"])
	assert.Equal(t, "1.35.0", depMap["tokio"])
	assert.Equal(t, "4.4.0", depMap["clap"])
}

// ── ComposerParser ───────────────────────────────────────────────────────────

func TestComposerParser_Parse(t *testing.T) {
	parser := &ComposerParser{}
	project, err := parser.Parse(filepath.Join("testdata", "composer.json"))

	require.NoError(t, err)
	assert.Equal(t, "test/composer-project", project.Name)
	assert.Equal(t, "composer.json", project.ManifestType)
}

func TestComposerParser_SkipsPHPAndExtensions(t *testing.T) {
	parser := &ComposerParser{}
	project, err := parser.Parse(filepath.Join("testdata", "composer.json"))
	require.NoError(t, err)

	for _, d := range project.Dependencies {
		assert.NotEqual(t, "php", d.Name)
		assert.False(t, len(d.Name) > 4 && d.Name[:4] == "ext-", "should skip ext- packages")
	}
}

func TestComposerParser_IncludesDevDeps(t *testing.T) {
	parser := &ComposerParser{}
	project, err := parser.Parse(filepath.Join("testdata", "composer.json"))
	require.NoError(t, err)

	hasPhpunit := false
	for _, d := range project.Dependencies {
		if d.Name == "phpunit/phpunit" {
			hasPhpunit = true
			assert.Equal(t, "Packagist", d.Ecosystem)
		}
	}
	assert.True(t, hasPhpunit, "should include dev dependencies")
}

// ── DetectParser ─────────────────────────────────────────────────────────────

func TestDetectParser_AllTypes(t *testing.T) {
	tests := []struct {
		filename string
		expected interface{}
	}{
		{"package.json", &PackageJSONParser{}},
		{"composer.json", &ComposerParser{}},
		{"requirements.txt", &RequirementsParser{}},
		{"pyproject.toml", &PyProjectParser{}},
		{"Cargo.toml", &CargoTomlParser{}},
		{"go.mod", &GoModParser{}},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			parser, err := DetectParser(tt.filename)
			require.NoError(t, err)
			assert.IsType(t, tt.expected, parser)
		})
	}
}

// ── FindManifests ────────────────────────────────────────────────────────────

func TestFindManifests_NonRecursive(t *testing.T) {
	manifests, err := FindManifests("testdata", false)
	require.NoError(t, err)
	assert.Greater(t, len(manifests), 0)
}
