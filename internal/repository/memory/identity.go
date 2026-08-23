package memory

import (
	"context"
	"errors"
	"strings"

	"github.com/wyw14/cry-086/internal/domain/identity"
)

func (s *Store) SeedUser(value identity.User) {
	s.mu.Lock()
	s.users[value.ID] = value
	s.usernames[strings.ToLower(value.Username)] = value.ID
	s.mu.Unlock()
}

func (s *Store) FindUser(ctx context.Context, id string) (identity.User, error) {
	if err := ctx.Err(); err != nil {
		return identity.User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.users[id]
	if !ok {
		return identity.User{}, ErrNotFound
	}
	value.SiteIDs = append([]string(nil), value.SiteIDs...)
	value.Roles = append([]identity.Role(nil), value.Roles...)
	value.PasswordHash = append([]byte(nil), value.PasswordHash...)
	return value, nil
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (identity.User, error) {
	s.mu.RLock()
	id, ok := s.usernames[strings.ToLower(username)]
	s.mu.RUnlock()
	if !ok {
		return identity.User{}, ErrNotFound
	}
	return s.FindUser(ctx, id)
}

func (s *Store) StoreRefreshToken(ctx context.Context, value identity.RefreshToken) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.refreshTokens[value.ID]; exists {
		return errors.New("refresh token already exists")
	}
	s.refreshTokens[value.ID] = value
	return nil
}

func (s *Store) FindRefreshToken(ctx context.Context, id string) (identity.RefreshToken, error) {
	if err := ctx.Err(); err != nil {
		return identity.RefreshToken{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.refreshTokens[id]
	if !ok {
		return identity.RefreshToken{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) UpdateRefreshToken(ctx context.Context, value identity.RefreshToken, expectedVersion int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.refreshTokens[value.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Version != expectedVersion || value.Version != expectedVersion+1 {
		return errors.New("refresh token optimistic version conflict")
	}
	s.refreshTokens[value.ID] = value
	return nil
}
