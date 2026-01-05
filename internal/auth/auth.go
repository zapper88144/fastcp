package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
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
)

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Authenticate authenticates a user with username and password
// For now, uses hardcoded credentials. Later will use Unix users.
func Authenticate(username, password string) (*models.User, error) {
	cfg := config.Get()

	// Constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.AdminUser)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.AdminPassword)) == 1

	if usernameMatch && passwordMatch {
		return &models.User{
			ID:        "admin",
			Username:  cfg.AdminUser,
			Email:     "admin@localhost",
			Role:      "admin",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	return nil, ErrInvalidCredentials
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

