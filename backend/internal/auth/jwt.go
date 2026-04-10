package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

var headerEncoded = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Exp    int64     `json:"exp"`
	Iat    int64     `json:"iat"`
}

type TokenService struct {
	secret []byte
	expiry time.Duration
}

func NewTokenService(secret string, expiry time.Duration) *TokenService {
	return &TokenService{
		secret: []byte(secret),
		expiry: expiry,
	}
}

func (t *TokenService) Generate(userID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()

	claims := Claims{
		UserID: userID,
		Email:  email,
		Exp:    now.Add(t.expiry).Unix(),
		Iat:    now.Unix(),
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := headerEncoded + "." + payloadEncoded
	signature := t.sign([]byte(signingInput))
	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureEncoded, nil
}

func (t *TokenService) Verify(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := t.sign([]byte(signingInput))

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().UTC().Unix() > claims.Exp {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

func (t *TokenService) sign(message []byte) []byte {
	mac := hmac.New(sha256.New, t.secret)
	mac.Write(message)
	return mac.Sum(nil)
}
