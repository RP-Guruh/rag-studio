package projects

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// MaxDocumentSize is the largest document accepted for import (50 MB).
const MaxDocumentSize = 50 << 20

// documentMimeTypes maps the accepted file extensions to their MIME type.
var documentMimeTypes = map[string]string{
	".pdf":  "application/pdf",
	".md":   "text/markdown",
	".txt":  "text/plain",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

// appDataDir returns the per-user application data directory shared with the
// settings store.
func appDataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config directory: %w", err)
	}
	return filepath.Join(dir, "rag-studio"), nil
}

// DocumentsDir returns the directory that holds a project's document files.
func DocumentsDir(projectID string) (string, error) {
	if _, err := uuid.Parse(projectID); err != nil {
		return "", fmt.Errorf("invalid project id: %w", err)
	}
	base, err := appDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "projects", projectID, "documents"), nil
}

// docExtension validates the file name extension and returns it lowercased.
func docExtension(filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if _, ok := documentMimeTypes[ext]; !ok {
		return "", fmt.Errorf("unsupported file type %q: use PDF, Markdown, TXT, or Word", ext)
	}
	return ext, nil
}

// Checksum returns the hex-encoded SHA-256 of data.
func Checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SaveDocumentFile writes data to the project's document directory and returns
// the full path. Files are named after their checksum so identical uploads map
// to the same path.
func SaveDocumentFile(projectID, checksum, extension string, data []byte) (string, error) {
	dir, err := DocumentsDir(projectID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create documents directory: %w", err)
	}
	path := filepath.Join(dir, checksum+extension)
	if err := writeFileAtomic(path, data); err != nil {
		return "", err
	}
	return path, nil
}

// writeFileAtomic writes data to a temporary file in the same directory and
// renames it into place so a crash cannot leave a partial document.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary document file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("set document file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write document file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("flush document file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close document file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("finalize document file: %w", err)
	}
	return nil
}

// RemoveDocumentFile deletes a stored document, refusing paths outside the
// application data directory.
func RemoveDocumentFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	base, err := appDataDir()
	if err != nil {
		return err
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve data directory: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve document path: %w", err)
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("refusing to remove a file outside the application data directory")
	}
	if err := os.Remove(absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove document file: %w", err)
	}
	return nil
}

// RemoveProjectFiles deletes every stored file belonging to a project.
func RemoveProjectFiles(projectID string) error {
	if _, err := uuid.Parse(projectID); err != nil {
		return fmt.Errorf("invalid project id: %w", err)
	}
	base, err := appDataDir()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(base, "projects", projectID)); err != nil {
		return fmt.Errorf("remove project files: %w", err)
	}
	return nil
}
