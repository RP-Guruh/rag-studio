// Package auth provides local accounts, guest trial mode and project-level
// authorization for RAG Studio.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"rag_project/internal/settings"
)

// Service is the Wails-bindable authentication service.
type Service struct {
	settingsRepo settings.Repository
	sessions     SessionStore
}

func NewService(repo settings.Repository, sessions SessionStore) *Service {
	return &Service{settingsRepo: repo, sessions: sessions}
}

func (s *Service) pool(ctx context.Context) (*pgxpool.Pool, error) {
	cfg, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	if !cfg.Database.Configured {
		return nil, errors.New("database is not configured. Open Settings → Database and save it first")
	}
	password, err := s.settingsRepo.DatabasePassword()
	if err != nil {
		return nil, fmt.Errorf("load database password: %w", err)
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Database.Username, password),
		Host:   net.JoinHostPort(cfg.Database.Host, strconv.Itoa(cfg.Database.Port)),
		Path:   "/" + cfg.Database.DatabaseName,
	}
	query := u.Query()
	query.Set("sslmode", cfg.Database.SSLMode)
	u.RawQuery = query.Encode()

	poolConfig, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return pool, nil
}

func (s *Service) ensureGuest(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, guestEmail).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("find guest user: %w", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role) VALUES ($1, '', $2)
		ON CONFLICT (email) DO UPDATE SET email = excluded.email
		RETURNING id::text
	`, guestEmail, RoleGuest).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create guest user: %w", err)
	}
	return id, nil
}

// HasUsers reports whether a real (non-guest) account exists.
func (s *Service) HasUsers() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return false, err
	}
	defer pool.Close()
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role <> $1`, RoleGuest).Scan(&count); err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count > 0, nil
}

// Register creates the first account, which becomes the administrator.
func (s *Service) Register(email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("a valid email is required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role <> $1`, RoleGuest).Scan(&count); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil, errors.New("registration is closed. Ask an administrator to create an account")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	var id string
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id::text
	`, email, string(hash), RoleAdmin).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, errors.New("that email is already registered")
		}
		return nil, fmt.Errorf("create account: %w", err)
	}
	if err := s.sessions.SetSession(id); err != nil {
		return nil, err
	}
	return s.userByID(ctx, pool, id)
}

// Login signs in with an email and password.
func (s *Service) Login(email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()

	var id, hash string
	err = pool.QueryRow(ctx, `
		SELECT id::text, password_hash FROM users WHERE email = $1 AND role <> $2
	`, email, RoleGuest).Scan(&id, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("load account: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, errors.New("invalid email or password")
	}
	if err := s.sessions.SetSession(id); err != nil {
		return nil, err
	}
	return s.userByID(ctx, pool, id)
}

// LoginGuest signs in as the read-only trial account.
func (s *Service) LoginGuest() (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()

	if enabled, err := s.guestAccess(ctx, pool); err != nil || !enabled {
		return nil, errors.New("guest access is disabled")
	}
	id, err := s.ensureGuest(ctx, pool)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.SetSession(id); err != nil {
		return nil, err
	}
	return s.userByID(ctx, pool, id)
}

// Logout clears the persisted session.
func (s *Service) Logout() error {
	return s.sessions.ClearSession()
}

// CurrentUser returns the signed-in account, or nil when signed out.
func (s *Service) CurrentUser() (*User, error) {
	id, err := s.sessions.GetSession()
	if err != nil || strings.TrimSpace(id) == "" {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	return s.userByID(ctx, pool, id)
}

func (s *Service) userByID(ctx context.Context, pool *pgxpool.Pool, id string) (*User, error) {
	var user User
	err := pool.QueryRow(ctx, `
		SELECT id::text, email, role FROM users WHERE id = $1::uuid
	`, id).Scan(&user.ID, &user.Email, &user.Role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load user: %w", err)
	}
	rows, err := pool.Query(ctx, `
		SELECT project_id::text FROM user_projects WHERE user_id = $1::uuid ORDER BY project_id
	`, user.ID)
	if err != nil {
		return nil, fmt.Errorf("load project assignments: %w", err)
	}
	defer rows.Close()
	user.ProjectIDs = []string{}
	for rows.Next() {
		var projectID string
		if err := rows.Scan(&projectID); err != nil {
			return nil, err
		}
		user.ProjectIDs = append(user.ProjectIDs, projectID)
	}
	return &user, nil
}

func (s *Service) requireAdmin(ctx context.Context, pool *pgxpool.Pool) error {
	user, err := s.CurrentUser()
	if err != nil {
		return err
	}
	if user == nil || user.Role != RoleAdmin {
		return errors.New("only an administrator can manage accounts")
	}
	return nil
}

func (s *Service) guestAccess(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var value string
	err := pool.QueryRow(ctx, `SELECT value FROM app_config WHERE key = 'guest_access'`).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read guest access setting: %w", err)
	}
	return value == "true", nil
}

// ListUsers returns all accounts. Administrator only.
func (s *Service) ListUsers() ([]User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	if err := s.requireAdmin(ctx, pool); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT id::text, email, role FROM users ORDER BY role, email
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	result := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.Role); err != nil {
			return nil, err
		}
		user.ProjectIDs = []string{}
		result = append(result, user)
	}
	for i := range result {
		full, err := s.userByID(ctx, pool, result[i].ID)
		if err != nil {
			return nil, err
		}
		if full != nil {
			result[i].ProjectIDs = full.ProjectIDs
		}
	}
	return result, nil
}

// CreateUser adds a sub-account with access to the selected projects.
func (s *Service) CreateUser(email, password string, projectIDs []string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("a valid email is required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	if err := s.requireAdmin(ctx, pool); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var id string
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id::text
	`, email, string(hash), RoleUser).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	if err := s.setProjects(ctx, pool, id, projectIDs); err != nil {
		return nil, err
	}
	return s.userByID(ctx, pool, id)
}

// UpdateUserProjects replaces the projects a sub-account may manage.
func (s *Service) UpdateUserProjects(userID string, projectIDs []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := s.requireAdmin(ctx, pool); err != nil {
		return err
	}
	return s.setProjects(ctx, pool, userID, projectIDs)
}

func (s *Service) setProjects(ctx context.Context, pool *pgxpool.Pool, userID string, projectIDs []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM user_projects WHERE user_id = $1::uuid`, userID); err != nil {
		return fmt.Errorf("clear assignments: %w", err)
	}
	for _, projectID := range projectIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_projects (user_id, project_id) VALUES ($1::uuid, $2::uuid)
		`, userID, projectID); err != nil {
			return fmt.Errorf("assign project: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// DeleteUser removes a sub-account. Administrator and guest cannot be removed.
func (s *Service) DeleteUser(userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := s.requireAdmin(ctx, pool); err != nil {
		return err
	}
	var role string
	if err := pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1::uuid`, userID).Scan(&role); err != nil {
		return fmt.Errorf("load user: %w", err)
	}
	if role != RoleUser {
		return errors.New("administrator and guest accounts cannot be deleted")
	}
	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, userID)
	return err
}

// SetGuestAccess enables or disables the trial account. Administrator only.
func (s *Service) SetGuestAccess(enabled bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := s.requireAdmin(ctx, pool); err != nil {
		return err
	}
	value := "false"
	if enabled {
		value = "true"
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO app_config (key, value) VALUES ('guest_access', $1)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value
	`, value)
	return err
}

// GuestAccess returns whether trial logins are enabled.
func (s *Service) GuestAccess() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := s.pool(ctx)
	if err != nil {
		return false, err
	}
	defer pool.Close()
	return s.guestAccess(ctx, pool)
}

// RequireManager rejects guests and signed-out users.
func (s *Service) RequireManager() error {
	user, err := s.CurrentUser()
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("please sign in first")
	}
	if user.Role == RoleGuest {
		return errors.New("guest mode is read-only. Register or sign in to manage projects")
	}
	return nil
}

// VisibleProjectIDs returns the project ids the current user may see. Admins
// see every project; sub-users only their assignments; guests see none.
func (s *Service) VisibleProjectIDs() ([]string, error) {
	user, err := s.CurrentUser()
	if err != nil {
		return nil, err
	}
	if user == nil || user.Role == RoleGuest {
		return []string{}, nil
	}
	if user.Role == RoleAdmin {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool, err := s.pool(ctx)
		if err != nil {
			return nil, err
		}
		defer pool.Close()
		rows, err := pool.Query(ctx, `SELECT id::text FROM projects`)
		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, nil
	}
	return user.ProjectIDs, nil
}

// Authorize rejects operations on projects the current user may not manage.
// Guest accounts are read-only and have no real projects.
func (s *Service) Authorize(projectID string) error {
	user, err := s.CurrentUser()
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("please sign in first")
	}
	if user.Role == RoleGuest {
		return errors.New("guest mode is read-only. Register or sign in to manage projects")
	}
	if user.Role == RoleAdmin {
		return nil
	}
	for _, id := range user.ProjectIDs {
		if id == projectID {
			return nil
		}
	}
	return errors.New("this project is not assigned to your account")
}
