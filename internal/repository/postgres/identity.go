package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/domain/identity"
)

func (s *Store) FindUser(ctx context.Context, id string) (identity.User, error) {
	return getObject[identity.User](ctx, s.pool, "user", id)
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (identity.User, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM domain_objects WHERE kind='user' AND lower(secondary_key)=lower($1)`, strings.TrimSpace(username)).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, ErrNotFound
	}
	if err != nil {
		return identity.User{}, err
	}
	var value identity.User
	if err := json.Unmarshal(payload, &value); err != nil {
		return identity.User{}, err
	}
	return value, nil
}

func (s *Store) StoreRefreshToken(ctx context.Context, value identity.RefreshToken) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens(id,user_id,digest,expires_at,revoked_at,version,payload)
		VALUES($1,$2,$3,$4,$5,$6,$7)
	`, value.ID, value.UserID, value.Digest, value.ExpiresAt, value.RevokedAt, value.Version, payload)
	return err
}

func (s *Store) FindRefreshToken(ctx context.Context, id string) (identity.RefreshToken, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT payload FROM refresh_tokens WHERE id=$1`, id).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.RefreshToken{}, ErrNotFound
	}
	if err != nil {
		return identity.RefreshToken{}, err
	}
	var value identity.RefreshToken
	if err := json.Unmarshal(payload, &value); err != nil {
		return identity.RefreshToken{}, err
	}
	return value, nil
}

func (s *Store) UpdateRefreshToken(ctx context.Context, value identity.RefreshToken, expectedVersion int64) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at=$3,version=$4,payload=$5
		WHERE id=$1 AND version=$2
	`, value.ID, expectedVersion, value.RevokedAt, value.Version, payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("refresh token version conflict")
	}
	return nil
}
