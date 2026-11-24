package auth

// User is an account returned to the frontend. Secrets are never included.
type User struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Role       string   `json:"role"`
	ProjectIDs []string `json:"projectIds"`
}

// SessionStore persists the signed-in user across app restarts.
type SessionStore interface {
	GetSession() (string, error)
	SetSession(userID string) error
	ClearSession() error
}

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
	RoleGuest = "guest"
)

const guestEmail = "guest@rag-studio.local"
