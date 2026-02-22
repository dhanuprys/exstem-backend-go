package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stemsi/exstem-backend/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// Common auth errors.
var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrSessionAlreadyActive = errors.New("another session is already active, please contact admin to reset")
)

// TokenType distinguishes student vs admin tokens.
type TokenType string

const (
	TokenTypeStudent TokenType = "student"
	TokenTypeAdmin   TokenType = "admin"
)

// Claims extends JWT standard claims with app-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	TokenType   TokenType `json:"token_type"`
	UserID      int       `json:"user_id"`
	ClassID     int       `json:"class_id,omitempty"`    // Student only
	RoleID      int       `json:"role_id,omitempty"`     // Admin only
	Permissions []string  `json:"permissions,omitempty"` // Admin only
}

// AuthService handles authentication, JWT, and session management.
type AuthService struct {
	cfg *config.Config
	rdb *redis.Client
}

// NewAuthService creates a new AuthService.
func NewAuthService(cfg *config.Config, rdb *redis.Client) *AuthService {
	return &AuthService{cfg: cfg, rdb: rdb}
}

// HashPassword hashes a password with the configured bcrypt cost.
// Default cost is 6 for high-concurrency performance. Adjustable via BCRYPT_COST env.
func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.BcryptCost)
	return string(hash), err
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func (s *AuthService) CheckPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// GenerateStudentToken creates a JWT for a student and registers the session in Redis.
// Returns an error if a session already exists (new logins are rejected).
func (s *AuthService) GenerateStudentToken(ctx context.Context, studentID, classID int) (string, error) {
	sessionKey := config.CacheKey.StudentSessionKey(studentID)

	// Check if an active session exists — reject new login if so.
	existing, err := s.rdb.Get(ctx, sessionKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("check session: %w", err)
	}
	if existing != "" {
		return "", ErrSessionAlreadyActive
	}

	jti := uuid.New().String()
	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   strconv.Itoa(studentID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTExpiry)),
		},
		TokenType: TokenTypeStudent,
		UserID:    studentID,
		ClassID:   classID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	// Store session in Redis with same expiry as JWT.
	if err := s.rdb.Set(ctx, sessionKey, jti, s.cfg.JWTExpiry).Err(); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}

	return signed, nil
}

// GenerateAdminToken creates a JWT for an admin with permissions embedded.
func (s *AuthService) GenerateAdminToken(adminID, roleID int, permissions []string) (string, error) {
	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   strconv.Itoa(adminID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTExpiry)),
		},
		TokenType:   TokenTypeAdmin,
		UserID:      adminID,
		RoleID:      roleID,
		Permissions: permissions,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// ValidateToken parses and validates a JWT, returning the claims.
func (s *AuthService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// ValidateStudentSession checks that the token's JTI matches the active session in Redis.
func (s *AuthService) ValidateStudentSession(ctx context.Context, studentID int, jti string) error {
	sessionKey := config.CacheKey.StudentSessionKey(studentID)
	stored, err := s.rdb.Get(ctx, sessionKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errors.New("no active session")
		}
		return fmt.Errorf("check session: %w", err)
	}
	if stored != jti {
		return errors.New("session invalidated")
	}
	return nil
}

// ResetStudentSession removes a student's session from Redis, allowing a new login.
func (s *AuthService) ResetStudentSession(ctx context.Context, studentID int) error {
	sessionKey := config.CacheKey.StudentSessionKey(studentID)
	return s.rdb.Del(ctx, sessionKey).Err()
}
