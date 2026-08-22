package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wyw14/cry-086/internal/domain/identity"
)

type Claims struct {
	Subject   string          `json:"sub"`
	SiteIDs   []string        `json:"site_ids"`
	Roles     []identity.Role `json:"roles"`
	ExpiresAt int64           `json:"exp"`
	IssuedAt  int64           `json:"iat"`
}

type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer { return &Signer{secret: []byte(secret)} }

func (s *Signer) SignAccess(user identity.User, expiresAt time.Time) (string, error) {
	claims := Claims{Subject: user.ID, SiteIDs: append([]string(nil), user.SiteIDs...), Roles: append([]identity.Role(nil), user.Roles...), ExpiresAt: expiresAt.Unix(), IssuedAt: time.Now().UTC().Unix()}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := s.signature(encoded)
	return encoded + "." + signature, nil
}

func (s *Signer) Verify(raw string, now time.Time) (Claims, error) {
	payload, signature, ok := strings.Cut(raw, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(s.signature(payload))) {
		return Claims{}, errors.New("access token signature is invalid")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Claims{}, errors.New("access token payload is invalid")
	}
	var claims Claims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return Claims{}, errors.New("access token claims are invalid")
	}
	if claims.Subject == "" || now.Unix() >= claims.ExpiresAt {
		return Claims{}, errors.New("access token expired")
	}
	return claims, nil
}

func (s *Signer) signature(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
