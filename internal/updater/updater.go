package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
)

const Repo = "alya-labs/depsradar"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func CheckForUpdate(currentVersion string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", nil // no releases published yet
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if release.TagName != "v"+currentVersion && release.TagName != currentVersion {
		return release.TagName, nil
	}

	return "", nil
}

func Update(version string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var release GitHubRelease
	json.NewDecoder(resp.Body).Decode(&release)

	targetAsset := fmt.Sprintf("depsradar-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == targetAsset {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download new binary to a temp file
	tmpFile, err := os.CreateTemp("", "depsradar-update")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	resp, err = http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return err
	}
	tmpFile.Close()

	// Get current executable path
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// On Linux/macOS we can just overwrite
	// But it's safer to move
	oldExe := exe + ".old"
	os.Rename(exe, oldExe)
	
	err = moveFile(tmpFile.Name(), exe)
	if err != nil {
		os.Rename(oldExe, exe) // rollback
		return err
	}

	os.Chmod(exe, 0755)
	os.Remove(oldExe)

	return nil
}

func moveFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0755)
}
