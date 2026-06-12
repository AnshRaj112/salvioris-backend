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
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 30 * 24 * time.Hour
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

	return IssueTherapistTokens(userID, tenantID)
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
