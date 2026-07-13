package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	UserID   string
	Username string
}

type jwtClaims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret     []byte
	expiration time.Duration
	clock      func() time.Time
}

func NewJWTService(secret string, expiration time.Duration, clock func() time.Time) (*JWTService, error) {
	if len(secret) < 32 || strings.TrimSpace(secret) == "" {
		return nil, errors.New("JWT secret must contain at least 32 non-blank characters")
	}
	if expiration <= 0 {
		return nil, errors.New("JWT expiration must be positive")
	}
	if clock == nil {
		clock = time.Now
	}
	return &JWTService{
		secret:     []byte(secret),
		expiration: expiration,
		clock:      clock,
	}, nil
}

func (service *JWTService) Sign(principal Principal) (string, error) {
	if strings.TrimSpace(principal.UserID) == "" || strings.TrimSpace(principal.Username) == "" {
		return "", errors.New("JWT principal is incomplete")
	}
	now := service.clock().UTC()
	claims := jwtClaims{
		UserID:   principal.UserID,
		Username: principal.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(service.expiration)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(service.secret)
}

func (service *JWTService) Verify(value string) (Principal, error) {
	claims := &jwtClaims{}
	token, err := jwt.ParseWithClaims(
		value,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected JWT algorithm %q", token.Method.Alg())
			}
			return service.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(service.clock),
	)
	if err != nil || token == nil || !token.Valid {
		return Principal{}, errors.New("invalid or expired token")
	}
	if strings.TrimSpace(claims.UserID) == "" || strings.TrimSpace(claims.Username) == "" {
		return Principal{}, errors.New("token principal is incomplete")
	}
	return Principal{UserID: claims.UserID, Username: claims.Username}, nil
}
