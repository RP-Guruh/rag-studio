// Package update checks GitHub Releases for a newer RAG Studio version and
// downloads the matching installer asset.
package update

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Version is overridden at build time with -X rag_project/internal/update.Version=x.y.z.
var Version = "0.1.0"

const (
	repoURL      = "https://github.com/RP-Guruh/rag-studio"
	releasesAPI  = "https://api.github.com/repos/RP-Guruh/rag-studio/releases/latest"
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
}

func NewService() *Service {
	return &Service{client: &http.Client{Timeout: requestLimit}}
}

func (s *Service) latest(ctx context.Context) (*release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	response, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
	}
	var result release
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse release response: %w", err)
	}
	return &result, nil
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
	version := strings.TrimPrefix(strings.TrimSpace(latest.TagName), "v")
	url := latest.HTMLURL
	if url == "" {
		url = repoURL + "/releases/latest"
	}
	info := &Info{
		CurrentVersion:  Version,
		LatestVersion:   version,
		UpdateAvailable: version != "" && version != Version,
		ReleaseURL:      url,
		Notes:           latest.Body,
	}
	for _, asset := range latest.Assets {
		if assetMatches(asset.Name) {
			info.AssetName = asset.Name
			info.AssetURL = asset.BrowserDownloadURL
			break
		}
	}
	return info, nil
}

// DownloadLatestInstaller downloads the asset matching this OS/architecture
// into the user's Downloads directory and returns the saved path.
func (s *Service) DownloadLatestInstaller() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	latest, err := s.latest(ctx)
	if err != nil {
		return "", err
	}
	var selected *struct {
		ID   int64
		Name string
	}
	for _, asset := range latest.Assets {
		if assetMatches(asset.Name) {
			selected = &struct {
				ID   int64
				Name string
			}{ID: asset.ID, Name: asset.Name}
			break
		}
	}
	if selected == nil {
		return "", fmt.Errorf("no installer found for %s/%s in this release", runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("%s%d", downloadAPI, selected.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")
	response, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}

	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	downloads := filepath.Join(dir, "Downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		return "", err
	}
	suffix, err := randomSuffix()
	if err != nil {
		return "", err
	}
	path := filepath.Join(downloads, selected.Name+"."+suffix+".part")
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
			emitProgress(selected.Name, written, response.ContentLength)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("read download: %w", readErr)
		}
	}
	final := strings.TrimSuffix(path, ".part")
	if err := os.Rename(path, final); err != nil {
		return "", err
	}
	return final, nil
}

func randomSuffix() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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

// OpenReleasePage opens the release page in the default browser.
func (s *Service) OpenReleasePage() error {
	app := application.Get()
	if app == nil {
		return fmt.Errorf("application is not ready")
	}
	return app.Browser.OpenURL(repoURL + "/releases/latest")
}
