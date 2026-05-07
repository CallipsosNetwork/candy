// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// Claims is the JWT payload: sub=user id, jti=ksuid, role, iat, exp.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// IssueJWT mints a new HS256 JWT. jti is a pre-generated ksuid.
func IssueJWT(secret []byte, userID shared.Id, role shared.Role, jti string, now time.Time) (string, error) {
	exp := now.Add(7 * 24 * time.Hour)
	claims := Claims{
		Role: string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   string(userID),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

// ParseJWT validates the signature and expiry of a JWT string.
// Returns (claims, jti, err). Does NOT check the revocation table.
func ParseJWT(secret []byte, tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, shared.ErrSessionInvalid
		}
		return nil, shared.ErrSessionInvalid
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, shared.ErrSessionInvalid
	}
	return claims, nil
}
