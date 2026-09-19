package domain

// User is an account that can authenticate against the API.
type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Roles        []string
}

// AccountPayload is the caller-defined payload embedded in issued JWTs.
type AccountPayload struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
}
