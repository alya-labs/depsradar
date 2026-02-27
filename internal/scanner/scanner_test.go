package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageJSONParser_Parse(t *testing.T) {
	parser := &PackageJSONParser{}
	path := filepath.Join("testdata", "package.json")

	project, err := parser.Parse(path)

	require.NoError(t, err)
	assert.Equal(t, "test-project", project.Name)
	assert.Equal(t, "package.json", project.ManifestType)
	assert.Equal(t, 3, len(project.Dependencies))
}

func TestPackageJSONParser_Dependencies(t *testing.T) {
	parser := &PackageJSONParser{}
	path := filepath.Join("testdata", "package.json")

	project, err := parser.Parse(path)

	require.NoError(t, err)
	deps := project.Dependencies

	depMap := make(map[string]string)
	for _, d := range deps {
		depMap[d.Name] = d.Version
		assert.Equal(t, "npm", d.Ecosystem)
	}

	assert.Equal(t, "0.21.1", depMap["axios"])
	assert.Equal(t, "4.17.20", depMap["lodash"])
	assert.Equal(t, "4.18.2", depMap["express"])
}

func TestPackageJSONParser_ParseError(t *testing.T) {
	parser := &PackageJSONParser{}

	_, err := parser.Parse("nonexistent.json")
	assert.Error(t, err)
}

func TestPackageJSONParser_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "package.json")
	err := os.WriteFile(tmpFile, []byte("invalid json"), 0644)
	require.NoError(t, err)

	parser := &PackageJSONParser{}
	_, err = parser.Parse(tmpFile)
	assert.Error(t, err)
}

func TestDetectParser(t *testing.T) {
	parser, err := DetectParser("package.json")
	require.NoError(t, err)
	assert.IsType(t, &PackageJSONParser{}, parser)
}

func TestDetectParser_Unsupported(t *testing.T) {
	_, err := DetectParser("unknown.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"^1.0.0", "1.0.0"},
		{"~1.0.0", "1.0.0"},
		{">1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"", ""},
	}

	for _, tt := range tests {
		result := cleanVersion(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestPackageJSONParser_NoName(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "package.json")
	content := `{"version":"1.0.0","dependencies":{"express":"4.18.2"}}`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := &PackageJSONParser{}
	project, err := parser.Parse(tmpFile)

	require.NoError(t, err)
	assert.Equal(t, filepath.Base(tmpDir), project.Name)
}

func TestPackageJSONParser_SupportedExtensions(t *testing.T) {
	parser := &PackageJSONParser{}
	exts := parser.SupportedExtensions()
	assert.Contains(t, exts, "package.json")
}
