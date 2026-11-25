// Package update checks GitHub Releases for a newer RAG Studio version and
// downloads the matching installer asset.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"rag_project/internal/auth"
)

// Version is overridden at build time with -X rag_project/internal/update.Version=x.y.z.
var Version = "0.1.2"

const (
	repoURL      = "https://github.com/RP-Guruh/rag-studio"
	releasesAPI  = "https://api.github.com/repos/RP-Guruh/rag-studio/releases/latest"
	releasesList = "https://api.github.com/repos/RP-Guruh/rag-studio/releases?per_page=10"
	downloadAPI  = "https://api.github.com/repos/RP-Guruh/rag-studio/releases/assets/"
	requestLimit = 30 * time.Second
)

// Info describes the newest release found online.
type Info struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	Notes           string `json:"notes"`
	AssetName       string `json:"assetName"`
	AssetURL        string `json:"assetUrl"`
}

type release struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Service is Wails-bindable.
type Service struct {
	client *http.Client
	auth   *auth.Service
}

func NewService(authService *auth.Service) *Service {
	return &Service{client: &http.Client{Timeout: requestLimit}, auth: authService}
}

// authorize reserves installer actions for the administrator.
func (s *Service) authorize() error {
	if s.auth == nil {
		return nil
	}
	return s.auth.RequireAdmin()
}

func (s *Service) getRelease(ctx context.Context, url string) (*release, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RAG-Studio/"+normalizeVersion(Version))
	response, err := s.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, response.StatusCode, fmt.Errorf("GitHub returned HTTP %d for %s", response.StatusCode, url)
	}
	var result release
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, response.StatusCode, fmt.Errorf("parse release response: %w", err)
	}
	return &result, response.StatusCode, nil
}

func (s *Service) latest(ctx context.Context) (*release, error) {
	result, status, err := s.getRelease(ctx, releasesAPI)
	if err == nil {
		return result, nil
	}
	if status != http.StatusNotFound {
		return nil, err
	}

	// Some repositories have no /releases/latest result. Fall back to the
	// newest published release from the list endpoint.
	req, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, releasesList, nil)
	if requestErr != nil {
		return nil, requestErr
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RAG-Studio/"+normalizeVersion(Version))
	response, requestErr := s.client.Do(req)
	if requestErr != nil {
		return nil, fmt.Errorf("could not reach GitHub: %w", requestErr)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub returned HTTP %d for releases list", response.StatusCode)
	}
	var list []release
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("parse releases list: %w", err)
	}
	for _, item := range list {
		if !item.Draft && normalizeVersion(item.TagName) != "" {
			found := item
			return &found, nil
		}
	}
	return nil, errors.New("no published release found on GitHub")
}

func normalizeVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}

func debInstalled() bool {
	_, err := os.Stat("/var/lib/dpkg/info/rag-studio.list")
	return err == nil
}

// preferredAsset picks the installer for this platform. On Debian-based Linux
// with the .deb installed, the .deb is preferred so the package manager keeps
// ownership of the files. Other Linux installs use the portable archive.
func preferredAsset(assets []struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}) (int64, string, string, bool) {
	arch := runtime.GOARCH
	if runtime.GOOS == "linux" && debInstalled() {
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if strings.HasSuffix(name, ".deb") && strings.Contains(name, arch) {
				return asset.ID, asset.Name, asset.BrowserDownloadURL, true
			}
		}
	}
	for _, asset := range assets {
		if assetMatches(asset.Name) {
			return asset.ID, asset.Name, asset.BrowserDownloadURL, true
		}
	}
	return 0, "", "", false
}

func assetMatches(name string) bool {
	name = strings.ToLower(name)
	if !strings.Contains(name, "rag-studio") {
		return false
	}
	if strings.Contains(name, runtime.GOOS) || (runtime.GOOS == "darwin" && strings.Contains(name, "macos")) {
		return strings.Contains(name, runtime.GOARCH) ||
			(runtime.GOARCH == "amd64" && strings.Contains(name, "x86_64"))
	}
	return false
}

// CheckForUpdate queries GitHub. Network failures are returned as errors so
// the UI can show "update check unavailable" instead of a false result.
func (s *Service) CheckForUpdate() (*Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestLimit)
	defer cancel()
	latest, err := s.latest(ctx)
	if err != nil {
		return nil, err
	}
	current := normalizeVersion(Version)
	version := normalizeVersion(latest.TagName)
	url := latest.HTMLURL
	if url == "" {
		url = repoURL + "/releases/latest"
	}
	info := &Info{
		CurrentVersion:  current,
		LatestVersion:   version,
		UpdateAvailable: version != "" && version != current,
		ReleaseURL:      url,
		Notes:           latest.Body,
	}
	_, name, assetURL, ok := preferredAsset(latest.Assets)
	if ok {
		info.AssetName = name
		info.AssetURL = assetURL
	}
	return info, nil
}

// openDownload tries the GitHub API asset endpoint first and falls back to the
// browser download URL when the API rejects the request.
func (s *Service) openDownload(ctx context.Context, apiURL, browserURL string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "RAG-Studio/"+normalizeVersion(Version))
	response, err := s.client.Do(request)
	if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}
	if response != nil {
		_ = response.Body.Close()
	}
	if browserURL == "" {
		if err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}
		return nil, fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, browserURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "RAG-Studio/"+normalizeVersion(Version))
	response, err = s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

// DownloadLatestInstaller downloads the asset matching this OS/architecture
// into the user's Downloads directory and returns the saved path.
func (s *Service) DownloadLatestInstaller() (string, error) {
	if err := s.authorize(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	latest, err := s.latest(ctx)
	if err != nil {
		return "", err
	}
	assetID, name, browserURL, ok := preferredAsset(latest.Assets)
	if !ok {
		return "", fmt.Errorf("no installer found for %s/%s in this release", runtime.GOOS, runtime.GOARCH)
	}

	response, err := s.openDownload(ctx, fmt.Sprintf("%s%d", downloadAPI, assetID), browserURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	downloads := filepath.Join(dir, "Downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		return "", err
	}
	path, final := installerPaths(downloads, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 64*1024)
	var written int64
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err := file.Write(buffer[:n]); err != nil {
				_ = os.Remove(path)
				return "", err
			}
			written += int64(n)
			emitProgress(name, written, response.ContentLength)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("read download: %w", readErr)
		}
	}
	// Replace an older download before renaming: os.Rename cannot overwrite an
	// existing target on Windows, and apt must see the real file extension.
	if _, statErr := os.Stat(final); statErr == nil {
		if err := os.Remove(final); err != nil {
			return "", err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	if err := os.Rename(path, final); err != nil {
		return "", err
	}
	return final, nil
}

// installerPaths keeps the original asset name as the final filename. A random
// suffix must never be appended after the extension, otherwise a .deb turns
// into an un-installable "*.deb.xxxxxxxx" file.
func installerPaths(dir, name string) (string, string) {
	return filepath.Join(dir, name+".part"), filepath.Join(dir, name)
}

func emitProgress(name string, written int64, total int64) {
	app := application.Get()
	if app == nil {
		return
	}
	percent := 0
	if total > 0 {
		percent = int(written * 100 / total)
	}
	app.Event.Emit("update:progress", map[string]any{
		"name": name, "bytes": written, "total": total, "percent": percent,
	})
}

// InstallResult is the outcome of launching the downloaded installer.
type InstallResult struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	RestartRequired bool   `json:"restartRequired"`
}

// InstallUpdate starts the downloaded installer. Package managers need elevated
// rights, so the user still confirms the system prompt. The application itself
// cannot silently replace a running executable on Windows or macOS.
func (s *Service) InstallUpdate(path string) (*InstallResult, error) {
	if err := s.authorize(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("no downloaded installer is available")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(absolute); err != nil {
		return nil, fmt.Errorf("installer file not found: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		if !strings.HasSuffix(strings.ToLower(absolute), ".deb") {
			return &InstallResult{
				Message: "This archive cannot be installed automatically. Extract it and replace rag-studio manually: " + absolute,
			}, nil
		}
		var cmd *exec.Cmd
		if _, err := exec.LookPath("pkexec"); err == nil {
			cmd = exec.Command("pkexec", "apt", "install", "-y", absolute)
		} else if _, err := exec.LookPath("sudo"); err == nil {
			cmd = exec.Command("sudo", "-n", "apt", "install", "-y", absolute)
		} else {
			return &InstallResult{
				Message: "No privileged installer helper found. Run manually: sudo apt install " + absolute,
			}, nil
		}
		output, err := cmd.CombinedOutput()
		message := strings.TrimSpace(string(output))
		if err != nil {
			if message == "" {
				message = err.Error()
			}
			return &InstallResult{Message: "Install failed: " + message}, nil
		}
		return &InstallResult{Success: true, Message: "Update installed. Restart RAG Studio.", RestartRequired: true}, nil

	case "windows":
		cmd := exec.Command("cmd", "/c", "start", "", "\""+absolute+"\"")
		if err := cmd.Start(); err != nil {
			return &InstallResult{Message: "Could not start installer: " + err.Error()}, nil
		}
		return &InstallResult{Success: true, Message: "Installer opened. Approve the system prompt and follow the installer.", RestartRequired: true}, nil

	default:
		return &InstallResult{
			Message: "Open the release page and replace the application manually: " + absolute,
		}, nil
	}
}

// OpenReleasePage opens the release page in the default browser.
func (s *Service) OpenReleasePage() error {
	app := application.Get()
	if app == nil {
		return fmt.Errorf("application is not ready")
	}
	return app.Browser.OpenURL(repoURL + "/releases/latest")
}
