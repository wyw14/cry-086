package authapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/platform/clock"
	"golang.org/x/crypto/bcrypt"
)

type Repository interface {
	FindUserByUsername(context.Context, string) (identity.User, error)
	FindUser(context.Context, string) (identity.User, error)
	StoreRefreshToken(context.Context, identity.RefreshToken) error
	FindRefreshToken(context.Context, string) (identity.RefreshToken, error)
	UpdateRefreshToken(context.Context, identity.RefreshToken, int64) error
}

type IDGenerator interface{ NewID() string }

type Signer interface {
	SignAccess(user identity.User, expiresAt time.Time) (string, error)
}

type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	AccessExpiry time.Time `json:"access_expires_at"`
}

type Service struct {
	repository Repository
	signer     Signer
	clock      clock.Clock
	ids        IDGenerator
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type refreshCredential struct {
	raw       string
	id        string
	secret    string
	digest    string
	presented time.Time
}

type refreshState struct {
	credential refreshCredential
	stored     identity.RefreshToken
	owner      identity.User
	version    int64
}

func New(repository Repository, signer Signer, c clock.Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, signer: signer, clock: c, ids: ids, accessTTL: 15 * time.Minute, refreshTTL: 7 * 24 * time.Hour}
}

func HashPassword(password string) ([]byte, error) {
	if len(password) < 12 || len(password) > 128 {
		return nil, errors.New("password must contain 12 to 128 characters")
	}
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func (s *Service) Login(ctx context.Context, username, password string) (Tokens, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	user, err := s.repository.FindUserByUsername(ctx, username)
	if err != nil || !user.Active {
		return Tokens{}, errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)); err != nil {
		return Tokens{}, errors.New("invalid credentials")
	}
	return s.issue(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, rawToken string) (Tokens, error) {
	credential, err := s.parseRefreshCredential(rawToken)
	if err != nil {
		return Tokens{}, err
	}
	state, err := s.loadRefreshState(ctx, credential)
	if err != nil {
		return Tokens{}, err
	}
	if err := s.rotateRefreshState(ctx, &state); err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, state.owner)
}

func (s *Service) parseRefreshCredential(rawToken string) (refreshCredential, error) {
	id, secret, ok := strings.Cut(strings.TrimSpace(rawToken), ".")
	if !ok || id == "" || secret == "" {
		return refreshCredential{}, errors.New("invalid refresh token")
	}
	return refreshCredential{raw: rawToken, id: id, secret: secret, digest: digest(secret), presented: s.clock.Now()}, nil
}

func (s *Service) loadRefreshState(ctx context.Context, credential refreshCredential) (refreshState, error) {
	stored, err := s.repository.FindRefreshToken(ctx, credential.id)
	if err != nil {
		return refreshState{}, errors.New("invalid refresh token")
	}
	if !stored.Valid(credential.presented) {
		return refreshState{}, errors.New("invalid refresh token")
	}
	if stored.Digest == "" {
		return refreshState{}, errors.New("invalid refresh token")
	}
	owner, err := s.repository.FindUser(ctx, stored.UserID)
	if err != nil || !owner.Active {
		return refreshState{}, errors.New("refresh token owner unavailable")
	}
	return refreshState{credential: credential, stored: stored, owner: owner, version: stored.Version}, nil
}

func (s *Service) rotateRefreshState(ctx context.Context, state *refreshState) error {
	if err := state.stored.Revoke(s.clock.Now()); err != nil {
		return err
	}
	return s.repository.UpdateRefreshToken(ctx, state.stored, state.version)
}

func (s *Service) Revoke(ctx context.Context, rawToken string) error {
	id, secret, ok := strings.Cut(rawToken, ".")
	if !ok {
		return errors.New("invalid refresh token")
	}
	stored, err := s.repository.FindRefreshToken(ctx, id)
	if err != nil || stored.Digest != digest(secret) {
		return errors.New("invalid refresh token")
	}
	version := stored.Version
	if err := stored.Revoke(s.clock.Now()); err != nil {
		return err
	}
	return s.repository.UpdateRefreshToken(ctx, stored, version)
}

func (s *Service) issue(ctx context.Context, user identity.User) (Tokens, error) {
	accessExpiry := s.clock.Now().Add(s.accessTTL)
	access, err := s.signer.SignAccess(user, accessExpiry)
	if err != nil {
		return Tokens{}, err
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return Tokens{}, err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	refresh := identity.RefreshToken{ID: s.ids.NewID(), UserID: user.ID, Digest: digest(secret), ExpiresAt: s.clock.Now().Add(s.refreshTTL), Version: 1}
	if err := s.repository.StoreRefreshToken(ctx, refresh); err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, RefreshToken: refresh.ID + "." + secret, AccessExpiry: accessExpiry}, nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
