package services

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessTokenDuration  = 7 * 24 * time.Hour  // 7 days — matches session duration
	RefreshTokenDuration = 30 * 24 * time.Hour // 30 days
)

type TokenClaims struct {
	UserID   string `json:"uid"`
	TenantID string `json:"tid"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

var jwtSecret []byte

func InitJWT(secret string) {
	jwtSecret = []byte(secret)
}

func issueAccessToken(therapistID, tenantID uuid.UUID) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		UserID:   therapistID.String(),
		TenantID: tenantID.String(),
		Role:     "therapist",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   therapistID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenDuration)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func IssueTherapistTokens(therapistID, tenantID uuid.UUID) (*TokenPair, error) {
	if len(jwtSecret) == 0 {
		return nil, errors.New("jwt not configured")
	}

	accessToken, err := issueAccessToken(therapistID, tenantID)
	if err != nil {
		return nil, err
	}

	refreshRaw, err := newRefreshToken()
	if err != nil {
		return nil, err
	}

	_, err = database.PostgresDB.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, tenant_id, expires_at)
		VALUES ($1, $2, $3, $4)
	`, therapistID, hashToken(refreshRaw), tenantID, time.Now().Add(RefreshTokenDuration))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshRaw,
		ExpiresIn:    int(AccessTokenDuration.Seconds()),
	}, nil
}

func ValidateAccessToken(tokenStr string) (*TokenClaims, bool) {
	if len(jwtSecret) == 0 || tokenStr == "" {
		return nil, false
	}
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, false
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || claims.Role != "therapist" {
		return nil, false
	}
	return claims, true
}

// issueReceptionistAccessToken mints a short-lived JWT for a receptionist.
func issueReceptionistAccessToken(receptionistID, tenantID uuid.UUID) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		UserID:   receptionistID.String(),
		TenantID: tenantID.String(),
		Role:     "receptionist",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   receptionistID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenDuration)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// IssueReceptionistTokens creates an access+refresh token pair for a receptionist.
func IssueReceptionistTokens(receptionistID, tenantID uuid.UUID) (*TokenPair, error) {
	if len(jwtSecret) == 0 {
		return nil, errors.New("jwt not configured")
	}

	accessToken, err := issueReceptionistAccessToken(receptionistID, tenantID)
	if err != nil {
		return nil, err
	}

	refreshRaw, err := newRefreshToken()
	if err != nil {
		return nil, err
	}

	// Reuse the refresh_tokens table — role is embedded in the access token claims
	_, err = database.PostgresDB.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, tenant_id, expires_at)
		VALUES ($1, $2, $3, $4)
	`, receptionistID, hashToken(refreshRaw), tenantID, time.Now().Add(RefreshTokenDuration))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshRaw,
		ExpiresIn:    int(AccessTokenDuration.Seconds()),
	}, nil
}

// ValidateReceptionistAccessToken validates a JWT and ensures role == "receptionist".
func ValidateReceptionistAccessToken(tokenStr string) (*TokenClaims, bool) {
	if len(jwtSecret) == 0 || tokenStr == "" {
		return nil, false
	}
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, false
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || claims.Role != "receptionist" {
		return nil, false
	}
	return claims, true
}

func RefreshAccessToken(refreshRaw string) (*TokenPair, error) {
	if len(jwtSecret) == 0 {
		return nil, errors.New("jwt not configured")
	}

	var userID, tenantID uuid.UUID
	var expiresAt time.Time
	err := database.PostgresDB.QueryRow(`
		SELECT user_id, tenant_id, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, hashToken(refreshRaw)).Scan(&userID, &tenantID, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("invalid refresh token")
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(expiresAt) {
		return nil, errors.New("refresh token expired")
	}

	// Check if the user is a therapist
	var isTherapist bool
	err = database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM therapists WHERE id = $1)
	`, userID).Scan(&isTherapist)
	if err == nil && isTherapist {
		return IssueTherapistTokens(userID, tenantID)
	}

	// Check if the user is a receptionist
	var isReceptionist bool
	err = database.PostgresDB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM receptionists WHERE id = $1)
	`, userID).Scan(&isReceptionist)
	if err == nil && isReceptionist {
		return IssueReceptionistTokens(userID, tenantID)
	}

	return nil, errors.New("user not found or inactive")
}

func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
