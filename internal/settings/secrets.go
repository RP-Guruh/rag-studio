package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// secretService is the keyring service name. All application secrets are stored
// under this service with one key per setting section.
const secretService = "rag-studio"

// SecretStore keeps API keys and database passwords out of the SQLite file.
type SecretStore interface {
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
}

// NewSecretStore prefers the OS keyring (Secret Service, Keychain, Credential
// Manager) and transparently falls back to an AES-GCM encrypted file when the
// keyring is unavailable, for example on a headless Linux host.
func NewSecretStore(dataDir string) SecretStore {
	return &hybridSecrets{
		primary:  keyringSecrets{service: secretService},
		fallback: &fileSecrets{dir: dataDir},
	}
}

type hybridSecrets struct {
	primary  keyringSecrets
	fallback *fileSecrets
}

func (h *hybridSecrets) Get(name string) (string, error) {
	if value, err := h.primary.Get(name); err == nil && value != "" {
		return value, nil
	}
	return h.fallback.Get(name)
}

func (h *hybridSecrets) Set(name, value string) error {
	if err := h.primary.Set(name, value); err == nil {
		// Drop a stale copy from an earlier fallback write, if any.
		_ = h.fallback.Delete(name)
		return nil
	}
	return h.fallback.Set(name, value)
}

func (h *hybridSecrets) Delete(name string) error {
	primaryErr := h.primary.Delete(name)
	fallbackErr := h.fallback.Delete(name)
	if primaryErr != nil && !errors.Is(primaryErr, keyring.ErrNotFound) {
		return primaryErr
	}
	return fallbackErr
}

type keyringSecrets struct {
	service string
}

func (k keyringSecrets) Get(name string) (string, error) {
	value, err := keyring.Get(k.service, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return value, err
}

func (k keyringSecrets) Set(name, value string) error {
	return keyring.Set(k.service, name, value)
}

func (k keyringSecrets) Delete(name string) error {
	return keyring.Delete(k.service, name)
}

// fileSecrets stores secrets in an AES-GCM encrypted JSON file. The 32-byte key
// lives next to it in a 0600 file, separate from the SQLite database.
type fileSecrets struct {
	dir string
	mu  sync.Mutex
}

func (f *fileSecrets) Get(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	values, err := f.load()
	if err != nil {
		return "", err
	}
	return values[name], nil
}

func (f *fileSecrets) Set(name, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	values, err := f.load()
	if err != nil {
		return err
	}
	values[name] = value
	return f.save(values)
}

func (f *fileSecrets) Delete(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	values, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := values[name]; !ok {
		return nil
	}
	delete(values, name)
	return f.save(values)
}

func (f *fileSecrets) load() (map[string]string, error) {
	values := map[string]string{}

	raw, err := os.ReadFile(filepath.Join(f.dir, "secrets.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read secrets file: %w", err)
	}

	key, err := f.masterKey()
	if err != nil {
		return nil, err
	}
	plain, err := decrypt(key, raw)
	if err != nil {
		return nil, fmt.Errorf("decrypt secrets file: %w", err)
	}
	if err := json.Unmarshal(plain, &values); err != nil {
		return nil, fmt.Errorf("parse secrets file: %w", err)
	}
	return values, nil
}

func (f *fileSecrets) save(values map[string]string) error {
	key, err := f.masterKey()
	if err != nil {
		return err
	}
	plain, err := json.Marshal(values)
	if err != nil {
		return err
	}
	encrypted, err := encrypt(key, plain)
	if err != nil {
		return err
	}
	path := filepath.Join(f.dir, "secrets.json")
	return writeFileAtomic(path, encrypted)
}

func (f *fileSecrets) masterKey() ([]byte, error) {
	path := filepath.Join(f.dir, "secrets.key")
	key, err := os.ReadFile(path)
	if err == nil && len(key) == 32 {
		return key, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read secrets key: %w", err)
	}

	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate secrets key: %w", err)
	}
	if err := writeFileAtomic(path, key); err != nil {
		return nil, err
	}
	return key, nil
}

func encrypt(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)
	out := append(nonce, sealed...)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(out)))
	base64.StdEncoding.Encode(encoded, out)
	return encoded, nil
}

func decrypt(key, encoded []byte) ([]byte, error) {
	raw := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, err := base64.StdEncoding.Decode(raw, encoded)
	if err != nil {
		return nil, err
	}
	raw = raw[:n]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("secrets file is truncated")
	}
	return gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
