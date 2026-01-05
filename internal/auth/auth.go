package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/fastcp/fastcp/internal/config"
	"github.com/fastcp/fastcp/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrAPIKeyNotFound     = errors.New("api key not found")
	ErrAPIKeyExpired      = errors.New("api key expired")
	ErrUserNotAllowed     = errors.New("user not allowed to access FastCP")
)

// AllowedGroups defines which Unix groups can access FastCP
var AllowedGroups = []string{"root", "sudo", "wheel", "admin", "fastcp"}

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Authenticate authenticates a user with username and password
// Uses Unix/PAM authentication on Linux, falls back to config-based auth
func Authenticate(username, password string) (*models.User, error) {
	// First try Unix authentication (Linux only)
	if runtime.GOOS == "linux" {
		if user, err := authenticateUnix(username, password); err == nil {
			return user, nil
		}
	}

	// Fallback to config-based authentication (for dev mode or non-Linux)
	return authenticateConfig(username, password)
}

// authenticateUnix authenticates against Unix/PAM
func authenticateUnix(username, password string) (*models.User, error) {
	// Verify user exists in the system
	u, err := user.Lookup(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Check if user is in an allowed group
	if !isUserInAllowedGroup(username) {
		return nil, ErrUserNotAllowed
	}

	// Authenticate using PAM via su command
	// This is a simple approach; for production, consider using the pam library
	cmd := exec.Command("su", "-c", "true", username)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := cmd.Start(); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Write password to stdin
	_, _ = stdin.Write([]byte(password + "\n"))
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Determine role based on groups
	role := "user"
	if isUserInGroup(username, "root") || isUserInGroup(username, "sudo") || isUserInGroup(username, "wheel") {
		role = "admin"
	}

	return &models.User{
		ID:        u.Uid,
		Username:  username,
		Email:     username + "@localhost",
		Role:      role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// authenticateConfig authenticates against config file (fallback)
func authenticateConfig(username, password string) (*models.User, error) {
	cfg := config.Get()

	// Constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.AdminUser)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.AdminPassword)) == 1

	if usernameMatch && passwordMatch {
		return &models.User{
			ID:        "admin",
			Username:  cfg.AdminUser,
			Email:     cfg.AdminEmail,
			Role:      "admin",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	return nil, ErrInvalidCredentials
}

// isUserInAllowedGroup checks if user belongs to any allowed group
func isUserInAllowedGroup(username string) bool {
	for _, group := range AllowedGroups {
		if isUserInGroup(username, group) {
			return true
		}
	}
	return false
}

// isUserInGroup checks if a user belongs to a specific group
func isUserInGroup(username, groupName string) bool {
	// Special case for root
	if username == "root" && groupName == "root" {
		return true
	}

	// Use the 'groups' command to get user's groups
	cmd := exec.Command("groups", username)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	groups := strings.Fields(string(output))
	// Output format: "username : group1 group2 group3" or just "group1 group2 group3"
	for _, g := range groups {
		if g == groupName {
			return true
		}
	}
	return false
}

// GenerateToken generates a JWT token for a user
func GenerateToken(user *models.User) (string, error) {
	cfg := config.Get()

	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "fastcp",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*Claims, error) {
	cfg := config.Get()

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// GenerateAPIKey generates a new API key
func GenerateAPIKey(name string, userID string, permissions []string) (*models.APIKey, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}

	return &models.APIKey{
		ID:          uuid.New().String(),
		Name:        name,
		Key:         "fcp_" + hex.EncodeToString(keyBytes),
		Permissions: permissions,
		UserID:      userID,
		CreatedAt:   time.Now(),
	}, nil
}

// HashAPIKey creates a hash of an API key for storage
func HashAPIKey(key string) string {
	// In production, use bcrypt or argon2
	// For now, we store keys directly (not recommended for production)
	return key
}

