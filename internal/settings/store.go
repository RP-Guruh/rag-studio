package settings

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists the single-row settings tables and delegates secrets to a
// SecretStore.
type Store struct {
	db      *sql.DB
	secrets SecretStore
}

// Store must satisfy the Repository contract used by the Wails services.
var _ Repository = (*Store)(nil)

// OpenDefault opens the settings database in the per-user configuration
// directory. Secrets use the OS keyring with an encrypted-file fallback.
func OpenDefault() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locate config directory: %w", err)
	}
	appDir := filepath.Join(dir, "rag-studio")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}
	return Open(filepath.Join(appDir, "settings.db"), NewSecretStore(appDir))
}

// Open opens (and migrates) the SQLite database at path.
func Open(path string, secrets SecretStore) (*Store, error) {
	if secrets == nil {
		return nil, errors.New("settings: secret store is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open settings database: %w", err)
	}
	// SQLite allows a single writer; one connection avoids "database is locked".
	db.SetMaxOpenConns(1)

	s := &Store{db: db, secrets: secrets}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// schemaV1 is applied when PRAGMA user_version is below 1.
// schemaV1 creates the settings tables. Configuration values intentionally have
// no SQL defaults: a row only exists after the user saves that section, and
// every save writes explicit values.
const schemaV1 = `
CREATE TABLE IF NOT EXISTS chat_model_settings (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    provider          TEXT    NOT NULL,
    base_url          TEXT    NOT NULL CHECK (length(trim(base_url)) > 0),
    api_key_set       INTEGER NOT NULL DEFAULT 0 CHECK (api_key_set IN (0, 1)),
    api_key_hint      TEXT    NOT NULL DEFAULT '',
    model_name        TEXT    NOT NULL CHECK (length(trim(model_name)) > 0),
    temperature       REAL    NOT NULL CHECK (temperature BETWEEN 0.0 AND 2.0),
    max_output_tokens INTEGER NOT NULL CHECK (max_output_tokens > 0),
    created_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
) STRICT;

CREATE TABLE IF NOT EXISTS embedding_model_settings (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    provider         TEXT    NOT NULL,
    base_url         TEXT    NOT NULL CHECK (length(trim(base_url)) > 0),
    api_key_set      INTEGER NOT NULL DEFAULT 0 CHECK (api_key_set IN (0, 1)),
    api_key_hint     TEXT    NOT NULL DEFAULT '',
    model_name       TEXT    NOT NULL CHECK (length(trim(model_name)) > 0),
    vector_dimension INTEGER NOT NULL CHECK (vector_dimension > 0),
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
) STRICT;

CREATE TABLE IF NOT EXISTS database_settings (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    host            TEXT    NOT NULL CHECK (length(trim(host)) > 0),
    port            INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    database_name   TEXT    NOT NULL CHECK (length(trim(database_name)) > 0),
    username        TEXT    NOT NULL CHECK (length(trim(username)) > 0),
    password_set    INTEGER NOT NULL DEFAULT 0 CHECK (password_set IN (0, 1)),
    password_hint   TEXT    NOT NULL DEFAULT '',
    ssl_mode        TEXT    NOT NULL
                    CHECK (ssl_mode IN ('disable','allow','prefer','require','verify-ca','verify-full')),
    schema_name     TEXT    NOT NULL CHECK (length(trim(schema_name)) > 0),
    created_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
) STRICT;
`

// schemaV2 adds the Docling document-processing connection. It is applied when
// PRAGMA user_version is below 2, so databases created before this table existed
// are upgraded in place.
const schemaV2 = `
CREATE TABLE IF NOT EXISTS docling_connection (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    mode            TEXT    NOT NULL
                    CHECK (mode IN ('local','serve')),
    base_url        TEXT    NOT NULL
                    CHECK (mode = 'local' OR length(trim(base_url)) > 0),
    api_key_set     INTEGER NOT NULL DEFAULT 0 CHECK (api_key_set IN (0, 1)),
    api_key_hint    TEXT    NOT NULL DEFAULT '',
    timeout_seconds INTEGER NOT NULL
                    CHECK (timeout_seconds BETWEEN 5 AND 3600),
    created_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
) STRICT;
`

// schemaV3 adds chunking options to the embedding model settings. The defaults
// only fill rows that already existed before this migration; the application
// never relies on them, because SaveEmbeddingModel always writes explicit
// values and rejects empty input.
const schemaV3 = `
ALTER TABLE embedding_model_settings
    ADD COLUMN chunk_max_tokens INTEGER NOT NULL DEFAULT 512
    CHECK (chunk_max_tokens BETWEEN 1 AND 8192);

ALTER TABLE embedding_model_settings
    ADD COLUMN tokenizer TEXT NOT NULL DEFAULT 'BAAI/bge-m3';
`

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version < 1 {
		if _, err := s.db.Exec(schemaV1); err != nil {
			return fmt.Errorf("create settings schema: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 1"); err != nil {
			return fmt.Errorf("record schema version: %w", err)
		}
		version = 1
	}
	if version < 2 {
		if _, err := s.db.Exec(schemaV2); err != nil {
			return fmt.Errorf("create docling schema: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("record schema version: %w", err)
		}
		version = 2
	}
	if version < 3 {
		if _, err := s.db.Exec(schemaV3); err != nil {
			return fmt.Errorf("add chunking options: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 3"); err != nil {
			return fmt.Errorf("record schema version: %w", err)
		}
	}
	return nil
}

// Get returns every section. Sections without a saved row are returned with
// Configured=false and zero values; no defaults are substituted.
func (s *Store) Get() (Settings, error) {
	chat, err := s.GetChatModel()
	if err != nil {
		return Settings{}, err
	}
	embedding, err := s.GetEmbeddingModel()
	if err != nil {
		return Settings{}, err
	}
	database, err := s.GetDatabase()
	if err != nil {
		return Settings{}, err
	}
	docling, err := s.GetDocling()
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		ChatModel:      chat,
		EmbeddingModel: embedding,
		Database:       database,
		Docling:        docling,
	}, nil
}

func (s *Store) GetChatModel() (ChatModel, error) {
	row := s.db.QueryRow(`
		SELECT provider, base_url, api_key_set, api_key_hint, model_name,
		       temperature, max_output_tokens, updated_at
		FROM chat_model_settings WHERE id = 1`)

	var out ChatModel
	err := row.Scan(
		&out.Provider, &out.BaseURL, &out.APIKeySet, &out.APIKeyHint,
		&out.ModelName, &out.Temperature, &out.MaxOutputTokens, &out.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatModel{}, nil
	}
	if err != nil {
		return ChatModel{}, fmt.Errorf("read chat model settings: %w", err)
	}
	out.Configured = true
	return out, nil
}

func (s *Store) GetEmbeddingModel() (EmbeddingModel, error) {
	row := s.db.QueryRow(`
		SELECT provider, base_url, api_key_set, api_key_hint, model_name,
		       vector_dimension, chunk_max_tokens, tokenizer, updated_at
		FROM embedding_model_settings WHERE id = 1`)

	var out EmbeddingModel
	err := row.Scan(
		&out.Provider, &out.BaseURL, &out.APIKeySet, &out.APIKeyHint,
		&out.ModelName, &out.VectorDimension, &out.ChunkMaxTokens, &out.Tokenizer,
		&out.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return EmbeddingModel{}, nil
	}
	if err != nil {
		return EmbeddingModel{}, fmt.Errorf("read embedding model settings: %w", err)
	}
	out.Configured = true
	return out, nil
}

func (s *Store) GetDatabase() (Database, error) {
	row := s.db.QueryRow(`
		SELECT host, port, database_name, username, password_set, password_hint,
		       ssl_mode, schema_name, updated_at
		FROM database_settings WHERE id = 1`)

	var out Database
	err := row.Scan(
		&out.Host, &out.Port, &out.DatabaseName, &out.Username,
		&out.PasswordSet, &out.PasswordHint, &out.SSLMode, &out.SchemaName, &out.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Database{}, nil
	}
	if err != nil {
		return Database{}, fmt.Errorf("read database settings: %w", err)
	}
	out.Configured = true
	return out, nil
}

func (s *Store) GetDocling() (DoclingConnection, error) {
	row := s.db.QueryRow(`
		SELECT mode, base_url, api_key_set, api_key_hint, timeout_seconds, updated_at
		FROM docling_connection WHERE id = 1`)

	var out DoclingConnection
	err := row.Scan(
		&out.Mode, &out.BaseURL, &out.APIKeySet, &out.APIKeyHint,
		&out.TimeoutSeconds, &out.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return DoclingConnection{}, nil
	}
	if err != nil {
		return DoclingConnection{}, fmt.Errorf("read docling settings: %w", err)
	}
	out.Configured = true
	return out, nil
}

// SaveChatModel validates and upserts the single chat model row. An empty
// APIKey keeps the stored secret; ClearAPIKey removes it.
func (s *Store) SaveChatModel(in ChatModel) (ChatModel, error) {
	in.Provider = normalizeProvider(in.Provider)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.ModelName = strings.TrimSpace(in.ModelName)
	if err := ValidateChatModel(in); err != nil {
		return ChatModel{}, err
	}

	current, err := s.ChatModelSecretState()
	if err != nil {
		return ChatModel{}, err
	}
	set, hint, err := s.resolveSecret(secretChatModel, in.APIKey, in.ClearAPIKey, current.set, current.hint)
	if err != nil {
		return ChatModel{}, err
	}

	_, err = s.db.Exec(`
		INSERT INTO chat_model_settings
			(id, provider, base_url, api_key_set, api_key_hint, model_name,
			 temperature, max_output_tokens, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider          = excluded.provider,
			base_url          = excluded.base_url,
			api_key_set       = excluded.api_key_set,
			api_key_hint      = excluded.api_key_hint,
			model_name        = excluded.model_name,
			temperature       = excluded.temperature,
			max_output_tokens = excluded.max_output_tokens,
			updated_at        = excluded.updated_at`,
		in.Provider, in.BaseURL, set, hint, in.ModelName,
		in.Temperature, in.MaxOutputTokens, nowUTC(),
	)
	if err != nil {
		return ChatModel{}, fmt.Errorf("save chat model settings: %w", err)
	}
	return s.GetChatModel()
}

// SaveEmbeddingModel validates and upserts the single embedding model row.
func (s *Store) SaveEmbeddingModel(in EmbeddingModel) (EmbeddingModel, error) {
	in.Provider = normalizeProvider(in.Provider)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.ModelName = strings.TrimSpace(in.ModelName)
	in.Tokenizer = strings.TrimSpace(in.Tokenizer)
	if err := ValidateEmbeddingModel(in); err != nil {
		return EmbeddingModel{}, err
	}

	current, err := s.EmbeddingSecretState()
	if err != nil {
		return EmbeddingModel{}, err
	}
	set, hint, err := s.resolveSecret(secretEmbeddingModel, in.APIKey, in.ClearAPIKey, current.set, current.hint)
	if err != nil {
		return EmbeddingModel{}, err
	}

	_, err = s.db.Exec(`
		INSERT INTO embedding_model_settings
			(id, provider, base_url, api_key_set, api_key_hint, model_name,
			 vector_dimension, chunk_max_tokens, tokenizer, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider         = excluded.provider,
			base_url         = excluded.base_url,
			api_key_set      = excluded.api_key_set,
			api_key_hint     = excluded.api_key_hint,
			model_name       = excluded.model_name,
			vector_dimension = excluded.vector_dimension,
			chunk_max_tokens = excluded.chunk_max_tokens,
			tokenizer        = excluded.tokenizer,
			updated_at       = excluded.updated_at`,
		in.Provider, in.BaseURL, set, hint, in.ModelName, in.VectorDimension,
		in.ChunkMaxTokens, in.Tokenizer, nowUTC(),
	)
	if err != nil {
		return EmbeddingModel{}, fmt.Errorf("save embedding model settings: %w", err)
	}
	return s.GetEmbeddingModel()
}

// SaveDatabase validates and upserts the single database row.
// It also performs schema migration if the database has been previously
// configured but not yet migrated. If no database settings exist yet,
// migration is skipped per the logical migration rule.
// Returns (Database, error, bool) where the bool indicates if migration was performed.
func (s *Store) SaveDatabase(in Database) (Database, error, bool) {
	// Check if database settings row already exists
	var rowCount int
	err := s.db.QueryRow("SELECT COUNT(*) FROM database_settings").Scan(&rowCount)
	if err != nil {
		return Database{}, fmt.Errorf("check database settings: %w", err), false
	}

	// Logical migration rule: if no database settings exist yet,
	// skip migration (don't migrate to PostgreSQL if not configured)
	if rowCount == 0 {
		// No existing settings - just save without migration
		in.Host = strings.TrimSpace(in.Host)
		in.DatabaseName = strings.TrimSpace(in.DatabaseName)
		in.Username = strings.TrimSpace(in.Username)
		in.SSLMode = strings.ToLower(strings.TrimSpace(in.SSLMode))
		in.SchemaName = strings.TrimSpace(in.SchemaName)
		if in.SchemaName == "" {
			in.SchemaName = "public"
		}
		if err := ValidateDatabase(in); err != nil {
			return Database{}, err, false
		}

		current, err := s.DatabaseSecretState()
		if err != nil {
			return Database{}, err, false
		}
		set, hint, err := s.resolveSecret(secretDatabase, in.Password, in.ClearPassword, current.set, current.hint)
		if err != nil {
			return Database{}, err, false
		}

		_, err = s.db.Exec(`
			INSERT INTO database_settings
				(id, host, port, database_name, username, password_set, password_hint,
				 ssl_mode, schema_name, updated_at)
			VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				host          = excluded.host,
				port          = excluded.port,
				database_name = excluded.database_name,
				username      = excluded.username,
				password_set  = excluded.password_set,
				password_hint = excluded.password_hint,
				ssl_mode      = excluded.ssl_mode,
				schema_name   = excluded.schema_name,
				updated_at    = excluded.updated_at`,
			in.Host, in.Port, in.DatabaseName, in.Username, set, hint,
			in.SSLMode, in.SchemaName, nowUTC(),
		)
		if err != nil {
			return Database{}, fmt.Errorf("save database settings: %w", err), false
		}
		db, dbErr := s.GetDatabase()
		if dbErr != nil {
			return Database{}, dbErr, false
		}
		return db, dbErr, false // false = migration not performed (no prior settings)
	}

	// Database settings exist - perform migration if needed
	migrated, err := s.migrateIfNeeded()
	if err != nil {
		return Database{}, fmt.Errorf("migrate then save database settings: %w", err), false
	}

	in.Host = strings.TrimSpace(in.Host)
	in.DatabaseName = strings.TrimSpace(in.DatabaseName)
	in.Username = strings.TrimSpace(in.Username)
	in.SSLMode = strings.ToLower(strings.TrimSpace(in.SSLMode))
	in.SchemaName = strings.TrimSpace(in.SchemaName)
	if in.SchemaName == "" {
		in.SchemaName = "public"
	}
	if err := ValidateDatabase(in); err != nil {
		return Database{}, err, migrated
	}

	current, err := s.DatabaseSecretState()
	if err != nil {
		return Database{}, err, migrated
	}
	set, hint, err := s.resolveSecret(secretDatabase, in.Password, in.ClearPassword, current.set, current.hint)
	if err != nil {
		return Database{}, err, migrated
	}

	_, err = s.db.Exec(`
		INSERT INTO database_settings
			(id, host, port, database_name, username, password_set, password_hint,
			 ssl_mode, schema_name, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			host          = excluded.host,
			port          = excluded.port,
			database_name = excluded.database_name,
			username      = excluded.username,
			password_set  = excluded.password_set,
			password_hint = excluded.password_hint,
			ssl_mode      = excluded.ssl_mode,
			schema_name   = excluded.schema_name,
			updated_at    = excluded.updated_at`,
		in.Host, in.Port, in.DatabaseName, in.Username, set, hint,
		in.SSLMode, in.SchemaName, nowUTC(),
	)
	if err != nil {
		return Database{}, fmt.Errorf("save database settings: %w", err), migrated
	}
	db, dbErr := s.GetDatabase()
	if dbErr != nil {
		return Database{}, dbErr, migrated
	}
	return db, dbErr, migrated // true = migration was performed
}

// migrateIfNeeded checks the current schema version and applies pending migrations.
// Returns true if any migration was applied, false if already at latest version.
func (s *Store) migrateIfNeeded() (bool, error) {
	var version int
	err := s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		return false, fmt.Errorf("read schema version: %w", err)
	}

	needsMigration := false

	if version < 1 {
		if _, err := s.db.Exec(schemaV1); err != nil {
			return false, fmt.Errorf("create settings schema: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 1"); err != nil {
			return false, fmt.Errorf("record schema version: %w", err)
		}
		needsMigration = true
		version = 1
	}
	if version < 2 {
		if _, err := s.db.Exec(schemaV2); err != nil {
			return false, fmt.Errorf("create docling schema: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 2"); err != nil {
			return false, fmt.Errorf("record schema version: %w", err)
		}
		needsMigration = true
		version = 2
	}
	if version < 3 {
		if _, err := s.db.Exec(schemaV3); err != nil {
			return false, fmt.Errorf("add chunking options: %w", err)
		}
		if _, err := s.db.Exec("PRAGMA user_version = 3"); err != nil {
			return false, fmt.Errorf("record schema version: %w", err)
		}
		needsMigration = true
	}
	return needsMigration, nil
}

// SaveDocling validates and upserts the single Docling connection row. An empty
// APIKey keeps the stored secret; ClearAPIKey removes it.
func (s *Store) SaveDocling(in DoclingConnection) (DoclingConnection, error) {
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	if err := ValidateDocling(in); err != nil {
		return DoclingConnection{}, err
	}

	current, err := s.DoclingSecretState()
	if err != nil {
		return DoclingConnection{}, err
	}
	set, hint, err := s.resolveSecret(secretDocling, in.APIKey, in.ClearAPIKey, current.set, current.hint)
	if err != nil {
		return DoclingConnection{}, err
	}

	_, err = s.db.Exec(`
		INSERT INTO docling_connection
			(id, mode, base_url, api_key_set, api_key_hint, timeout_seconds, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			mode            = excluded.mode,
			base_url        = excluded.base_url,
			api_key_set     = excluded.api_key_set,
			api_key_hint    = excluded.api_key_hint,
			timeout_seconds = excluded.timeout_seconds,
			updated_at      = excluded.updated_at`,
		in.Mode, in.BaseURL, set, hint, in.TimeoutSeconds, nowUTC(),
	)
	if err != nil {
		return DoclingConnection{}, fmt.Errorf("save docling settings: %w", err)
	}
	return s.GetDocling()
}

// normalizeProvider normalizes a provider name to lowercase.
func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// nowUTC returns the current UTC time formatted as an ISO 8601 string.
func nowUTC() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// ChatModelSecretState reports the stored chat API key state without reading the secret itself.
func (s *Store) ChatModelSecretState() (secretState, error) {
	current, err := s.GetChatModel()
	if err != nil {
		return secretState{}, err
	}
	return secretState{set: current.APIKeySet, hint: current.APIKeyHint}, nil
}

// EmbeddingSecretState reports the stored embedding API key state without reading the secret itself.
func (s *Store) EmbeddingSecretState() (secretState, error) {
	current, err := s.GetEmbeddingModel()
	if err != nil {
		return secretState{}, err
	}
	return secretState{set: current.APIKeySet, hint: current.APIKeyHint}, nil
}

// DatabaseSecretState reports the stored database password state without reading the secret itself.
func (s *Store) DatabaseSecretState() (secretState, error) {
	current, err := s.GetDatabase()
	if err != nil {
		return secretState{}, err
	}
	return secretState{set: current.PasswordSet, hint: current.PasswordHint}, nil
}

// DoclingSecretState reports the stored Docling API key state without reading the secret itself.
func (s *Store) DoclingSecretState() (secretState, error) {
	current, err := s.GetDocling()
	if err != nil {
		return secretState{}, err
	}
	return secretState{set: current.APIKeySet, hint: current.APIKeyHint}, nil
}

// secretState holds the set/clear state of a secret for display/hint purposes.
type secretState struct {
	set  bool
	hint string
}

// SecretHint returns a non-reversible hint such as "…9f2a" for display.
func SecretHint(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return "••••"
	}
	return "…" + secret[len(secret)-4:]
}

// resolveSecret validates and manages the stored secret.
// If clear is true, the secret is removed. If incoming is not empty, it's stored.
// Otherwise, the current stored state is returned.
func (s *Store) resolveSecret(name, incoming string, clear bool, currentSet bool, currentHint string) (bool, string, error) {
	if clear {
		if err := s.secrets.Delete(name); err != nil {
			return false, "", fmt.Errorf("remove stored secret: %w", err)
		}
		return false, "", nil
	}
	if incoming != "" {
		if err := s.secrets.Set(name, incoming); err != nil {
			return false, "", fmt.Errorf("store secret: %w", err)
		}
		return true, SecretHint(incoming), nil
	}
	return currentSet, currentHint, nil
}

// Secret getters implementing Repository interface
func (s *Store) ChatModelAPIKey() (string, error) {
	return s.secrets.Get(secretChatModel)
}

func (s *Store) EmbeddingAPIKey() (string, error) {
	return s.secrets.Get(secretEmbeddingModel)
}

func (s *Store) DatabasePassword() (string, error) {
	return s.secrets.Get(secretDatabase)
}

func (s *Store) DoclingAPIKey() (string, error) {
	return s.secrets.Get(secretDocling)
}
