// Package stack manages the optional local Docker stack (PostgreSQL + pgvector
// and docling-serve) used by the desktop guided setup.
package stack

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"rag_project/internal/settings"
)

const composeTemplate = `services:
  postgres:
    image: pgvector/pgvector:pg16
    restart: unless-stopped
    environment:
      POSTGRES_DB: rag_studio
      POSTGRES_USER: ragstudio
      POSTGRES_PASSWORD: ${RAG_DB_PASSWORD}
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ragstudio -d rag_studio"]
      interval: 5s
      timeout: 3s
      retries: 12

  docling:
    image: quay.io/docling-project/docling-serve:latest
    restart: unless-stopped
    ports:
      - "5001:5001"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:5001/health"]
      interval: 10s
      timeout: 5s
      retries: 12

volumes:
  pgdata:
`

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbName     = "rag_studio"
	dbUser     = "ragstudio"
	doclingURL = "http://localhost:5001"
	ollamaURL  = "http://localhost:11434"
)

// Status describes which local services are reachable.
type Status struct {
	Docker    bool   `json:"docker"`
	Postgres  bool   `json:"postgres"`
	Docling   bool   `json:"docling"`
	Ollama    bool   `json:"ollama"`
	Directory string `json:"directory"`
}

// Result is the outcome of a stack operation.
type Result struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Service is Wails-bindable.
type Service struct {
	store *settings.Store
	dir   string
}

func NewService(store *settings.Store) *Service {
	dir := ""
	if base, err := os.UserConfigDir(); err == nil {
		dir = filepath.Join(base, "rag-studio", "stack")
	}
	return &Service{store: store, dir: dir}
}

func (s *Service) ensureDir() error {
	if s.dir == "" {
		return fmt.Errorf("could not locate config directory")
	}
	return os.MkdirAll(s.dir, 0o700)
}

// StackStatus checks Docker availability and service reachability.
func (s *Service) StackStatus() (Status, error) {
	status := Status{Directory: s.dir}
	if _, err := exec.LookPath("docker"); err == nil {
		status.Docker = exec.Command("docker", "compose", "version").Run() == nil
	}
	status.Postgres = tcpReachable(fmt.Sprintf("%s:%d", dbHost, dbPort))
	status.Docling = httpReachable(doclingURL + "/health")
	status.Ollama = httpReachable(ollamaURL + "/api/tags")
	return status, nil
}

// StartLocalStack writes the compose files and starts the containers.
func (s *Service) StartLocalStack() (Result, error) {
	if err := s.ensureDir(); err != nil {
		return Result{}, err
	}
	if err := s.writeFiles(); err != nil {
		return Result{}, err
	}
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = s.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Message: string(output)}, fmt.Errorf("docker compose up: %w", err)
	}
	return Result{Success: true, Message: "Local stack started. Waiting for services to become healthy."}, nil
}

// StopLocalStack stops the containers without deleting data volumes.
func (s *Service) StopLocalStack() (Result, error) {
	if s.dir == "" {
		return Result{}, fmt.Errorf("stack directory is unavailable")
	}
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = s.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Message: string(output)}, fmt.Errorf("docker compose down: %w", err)
	}
	return Result{Success: true, Message: "Local stack stopped. Data volume is preserved."}, nil
}

func (s *Service) writeFiles() error {
	password, err := s.store.GetSecret(secretDatabasePassword)
	if err != nil || strings.TrimSpace(password) == "" {
		password, err = randomPassword()
		if err != nil {
			return err
		}
		if err := s.store.SetSecret(secretDatabasePassword, password); err != nil {
			return err
		}
	}
	composePath := filepath.Join(s.dir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte(composeTemplate), 0o600); err != nil {
		return err
	}
	envPath := filepath.Join(s.dir, ".env")
	env := "RAG_DB_PASSWORD=" + password + "\n"
	return os.WriteFile(envPath, []byte(env), 0o600)
}

const secretDatabasePassword = "database_password"

func randomPassword() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ApplyLocalPreset fills Settings with local-stack defaults. It also starts
// the stack first so database migration can run during SaveDatabase.
func (s *Service) ApplyLocalPreset() (Result, error) {
	status, err := s.StackStatus()
	if err != nil {
		return Result{}, err
	}
	if !status.Postgres || !status.Docling {
		return Result{Message: "Start the local stack before applying the preset."}, nil
	}
	password, err := s.store.GetSecret(secretDatabasePassword)
	if err != nil || strings.TrimSpace(password) == "" {
		return Result{Message: "Database password is missing from the secret store."}, nil
	}

	database, err, _ := s.store.SaveDatabase(settings.Database{
		Host: dbHost, Port: dbPort, DatabaseName: dbName, Username: dbUser,
		Password: password, SSLMode: "disable", SchemaName: "public", Configured: true,
	})
	if err != nil {
		return Result{}, err
	}
	_ = database

	_, err = s.store.SaveDocling(settings.DoclingConnection{
		Mode: "serve", BaseURL: doclingURL, TimeoutSeconds: 300, Configured: true,
	})
	if err != nil {
		return Result{}, err
	}
	_, err = s.store.SaveEmbeddingModel(settings.EmbeddingModel{
		Provider: "ollama", BaseURL: ollamaURL, ModelName: "bge-m3",
		VectorDimension: 1024, ChunkMaxTokens: 512, Tokenizer: "BAAI/bge-m3", Configured: true,
	})
	if err != nil {
		return Result{}, err
	}
	_, err = s.store.SaveChatModel(settings.ChatModel{
		Provider: "ollama", BaseURL: ollamaURL, ModelName: "llama3.1",
		Temperature: 0.2, MaxOutputTokens: 2048, Configured: true,
	})
	if err != nil {
		return Result{}, err
	}

	note := ""
	if !status.Ollama {
		note = " Ollama was not detected. Install it from ollama.com/download, run 'ollama serve', then choose your models."
	}
	return Result{Success: true, Message: "Local settings applied." + note}, nil
}

func tcpReachable(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func httpReachable(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode < 500
}
