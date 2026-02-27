package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"depsradar/internal/httputil"
)

const userAgent = "depsradar/1.1.0 (https://github.com/depsradar/depsradar)"

type Client struct {
	http   *http.Client
	sem    chan struct{}
	parallel int
}

type NPMResponse struct {
	Version string `json:"version"`
}

type PyPIResponse struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
}

type CrateResponse struct {
	Version string `json:"num"`
	Crates  []struct {
		Version string `json:"version"`
	} `json:"versions"`
}

func NewClient(parallel int) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		sem:      make(chan struct{}, parallel),
		parallel: parallel,
	}
}

func (c *Client) GetLatestVersion(name, ecosystem string) (string, error) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	switch ecosystem {
	case "npm":
		return c.getNPMVersion(name)
	case "PyPI":
		return c.getPyPIVersion(name)
	case "crates.io":
		return c.getCrateVersion(name)
	case "Go":
		return c.getGoVersion(name)
	case "Packagist":
		return c.getPackagistVersion(name)
	default:
		return "", nil
	}
}

func (c *Client) doGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return httputil.DoWithRetry(c.http, req)
}

func (c *Client) getNPMVersion(name string) (string, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/%s/latest", name)
	resp, err := c.doGet(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("npm: %s not found", name)
	}

	var data NPMResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	return data.Version, nil
}

func (c *Client) getPyPIVersion(name string) (string, error) {
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", name)
	resp, err := c.doGet(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("pypi: %s not found", name)
	}

	var data PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	return data.Info.Version, nil
}

func (c *Client) getCrateVersion(name string) (string, error) {
	url := fmt.Sprintf("https://crates.io/api/v1/crates/%s", name)
	resp, err := c.doGet(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("crates.io: %s not found", name)
	}

	var data CrateResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.Crates) > 0 {
		return data.Crates[0].Version, nil
	}

	return "", nil
}

type PackagistResponse struct {
	Packages map[string][]struct {
		Version string `json:"version"`
	} `json:"packages"`
}

func (c *Client) getPackagistVersion(name string) (string, error) {
	url := fmt.Sprintf("https://repo.packagist.org/p2/%s.json", name)
	resp, err := c.doGet(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("packagist: %s not found", name)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	pkgs, ok := data["packages"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	versions, ok := pkgs[name].([]interface{})
	if !ok || len(versions) == 0 {
		return "", nil
	}

	first, ok := versions[0].(map[string]interface{})
	if !ok {
		return "", nil
	}

	ver, ok := first["version"].(string)
	if !ok {
		return "", nil
	}

	return ver, nil
}

type GoProxyResponse struct {
	Version string `json:"Version"`
}

func (c *Client) getGoVersion(name string) (string, error) {
	url := fmt.Sprintf("https://proxy.golang.org/%s/@latest", name)
	resp, err := c.doGet(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("go proxy: %s not found", name)
	}

	var data GoProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	// Strip the "v" prefix for consistency
	ver := data.Version
	if len(ver) > 0 && ver[0] == 'v' {
		ver = ver[1:]
	}

	return ver, nil
}
