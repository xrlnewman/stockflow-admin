package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func Issue(secret, userID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: userID, Role: role, RegisteredClaims: jwt.RegisteredClaims{Subject: userID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl))}}).SignedString([]byte(secret))
}

func Parse(secret, tokenString string) (Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		if err == nil {
			err = errors.New("invalid token")
		}
		return Claims{}, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return Claims{}, errors.New("invalid claims")
	}
	return *claims, nil
}
