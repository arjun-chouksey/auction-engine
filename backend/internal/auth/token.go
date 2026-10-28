package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const issuer = "auction-engine"

var ErrInvalidToken = errors.New("invalid token")

type Identity struct {
	UserID string
	Role   domain.Role
}

type Claims struct {
	Role domain.Role `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret must contain at least 32 characters")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("token TTL must be positive")
	}
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

func (manager *TokenManager) Issue(user domain.User) (string, error) {
	now := manager.now().UTC()
	claims := Claims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(manager.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(manager.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (manager *TokenManager) Parse(raw string) (Identity, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(_ *jwt.Token) (any, error) { return manager.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(manager.now),
	)
	if err != nil || !token.Valid || claims.Subject == "" {
		return Identity{}, ErrInvalidToken
	}
	if claims.Role != domain.RoleBuyer && claims.Role != domain.RoleSupplier {
		return Identity{}, ErrInvalidToken
	}
	return Identity{UserID: claims.Subject, Role: claims.Role}, nil
}

func (manager *TokenManager) ExpiresInSeconds() int64 {
	return int64(manager.ttl / time.Second)
}
