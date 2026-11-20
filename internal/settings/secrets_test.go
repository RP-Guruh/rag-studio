package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSecretsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := &fileSecrets{dir: dir}

	if err := store.Set("api_key", "super-secret"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := store.Get("api_key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "super-secret" {
		t.Errorf("value = %q", got)
	}

	// The plaintext must not appear on disk.
	raw, err := os.ReadFile(filepath.Join(dir, "secrets.json"))
	if err != nil {
		t.Fatalf("read secrets file: %v", err)
	}
	if strings.Contains(string(raw), "super-secret") {
		t.Error("secrets file contains plaintext")
	}

	if err := store.Delete("api_key"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err = store.Get("api_key")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != "" {
		t.Errorf("value after delete = %q", got)
	}
}

func TestFileSecretsWrongKeyFails(t *testing.T) {
	dir := t.TempDir()
	store := &fileSecrets{dir: dir}
	if err := store.Set("api_key", "value"); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Replace the key, simulating a corrupted or foreign key file.
	if err := os.WriteFile(filepath.Join(dir, "secrets.key"), make([]byte, 32), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if _, err := store.Get("api_key"); err == nil {
		t.Fatal("expected decryption error with a different key")
	}
}

func TestNewSecretStoreFallsBack(t *testing.T) {
	dir := t.TempDir()
	store := NewSecretStore(dir)

	if err := store.Set("test_secret", "hello"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := store.Get("test_secret")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "hello" {
		t.Errorf("value = %q", got)
	}
	_ = store.Delete("test_secret")
}
