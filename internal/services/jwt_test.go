package services

import (
	"testing"

	"github.com/google/uuid"
)

func TestJWTIssueAndValidate(t *testing.T) {
	InitJWT("test-secret-key-for-jwt-unit-tests-only")
	tid := uuid.New()
	uid := uuid.New()

	accessToken, err := issueAccessToken(uid, tid)
	if err != nil {
		t.Fatal(err)
	}

	claims, ok := ValidateAccessToken(accessToken)
	if !ok {
		t.Fatal("token should validate")
	}
	if claims.UserID != uid.String() || claims.TenantID != tid.String() {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	if claims.Role != "therapist" {
		t.Fatalf("role: %s", claims.Role)
	}

	_, ok = ValidateAccessToken("invalid.token.here")
	if ok {
		t.Fatal("invalid token should fail")
	}
}
