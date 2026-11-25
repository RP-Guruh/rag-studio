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
	DockerInstalled bool   `json:"dockerInstalled"`
	ComposePlugin   bool   `json:"composePlugin"`
	Daemon          bool   `json:"daemon"`
	Permission      bool   `json:"permission"`
	Docker          bool   `json:"docker"`
	Postgres        bool   `json:"postgres"`
	Docling         bool   `json:"docling"`
	Ollama          bool   `json:"ollama"`
	Directory       string `json:"directory"`
	Detail          string `json:"detail"`
}

// Result is the outcome of a stack operation.
type Result struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	CredentialIssue bool   `json:"credentialIssue"`
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
	status := Status{Directory: s.dir, Permission: true}
	problems := []string{}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := exec.LookPath("docker"); err != nil {
		problems = append(problems, "Docker CLI was not found. Install Docker Engine first.")
	} else {
		status.DockerInstalled = true

		if _, err := exec.CommandContext(ctx, "docker", "compose", "version").CombinedOutput(); err != nil {
			problems = append(problems, "Docker Compose plugin is missing. Install docker-compose-plugin.")
		} else {
			status.ComposePlugin = true
		}

		output, err := exec.CommandContext(ctx, "docker", "info").CombinedOutput()
		if err != nil {
			text := strings.ToLower(string(output))
			if strings.Contains(text, "permission denied") || strings.Contains(text, "sock") {
				status.Permission = false
				problems = append(problems, "Docker socket is not accessible. Add the user to the docker group and sign in again.")
			} else {
				problems = append(problems, "Docker daemon is not running. Start it with systemctl enable --now docker.")
			}
		} else {
			status.Daemon = true
		}
	}

	status.Docker = status.DockerInstalled && status.ComposePlugin && status.Daemon && status.Permission
	status.Detail = strings.Join(problems, " ")
	status.Postgres = tcpReachable(fmt.Sprintf("%s:%d", dbHost, dbPort))
	status.Docling = httpReachable(doclingURL + "/health")
	status.Ollama = httpReachable(ollamaURL + "/api/tags")
	return status, nil
}

// StartLocalStack writes the compose files and starts the containers.
// Command output is returned in Result so the UI can show the real Docker error.
func (s *Service) StartLocalStack() (Result, error) {
	if err := s.ensureDir(); err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}
	status, err := s.StackStatus()
	if err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}
	if !status.Docker {
		return Result{Success: false, Message: "Docker is not ready. " + status.Detail}, nil
	}
	if err := s.writeFiles(); err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = s.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return s.composeFailure("up", output, err)
	}
	return Result{Success: true, Message: "Local stack started. Waiting for services to become healthy."}, nil
}

func (s *Service) composeFailure(action string, output []byte, err error) (Result, error) {
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	lower := strings.ToLower(message)
	credentialIssue := strings.Contains(lower, "credential") ||
		strings.Contains(lower, "docker-credential") ||
		strings.Contains(lower, "error getting credentials")
	if credentialIssue {
		return Result{
			Success:         false,
			CredentialIssue: true,
			Message: "Docker config uses a credential helper that is not installed. " +
				"Remove credsStore from ~/.docker/config.json, install Docker Desktop, " +
				"or retry with a clean Docker config. Details: " + message,
		}, nil
	}
	return Result{Success: false, Message: "docker compose " + action + " failed: " + message}, nil
}

// StartLocalStackClean retries the stack with an isolated Docker config. This
// avoids a broken credsStore in the user's Docker Desktop config. The images
// used by the local stack are public, so no saved credentials are needed.
func (s *Service) StartLocalStackClean() (Result, error) {
	if err := s.ensureDir(); err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}
	status, err := s.StackStatus()
	if err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}
	if !status.DockerInstalled || !status.ComposePlugin || !status.Daemon {
		return Result{Success: false, Message: "Docker is not ready. " + status.Detail}, nil
	}
	if err := s.writeFiles(); err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}

	configDir := filepath.Join(s.dir, "docker-config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		return Result{Success: false, Message: err.Error()}, nil
	}

	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(), "DOCKER_CONFIG="+configDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return s.composeFailure("up", output, err)
	}
	return Result{Success: true, Message: "Local stack started with an isolated Docker config."}, nil
}

// StopLocalStack stops the containers without deleting data volumes.
func (s *Service) StopLocalStack() (Result, error) {
	if s.dir == "" {
		return Result{Success: false, Message: "Stack directory is unavailable."}, nil
	}
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = s.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return s.composeFailure("down", output, err)
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
