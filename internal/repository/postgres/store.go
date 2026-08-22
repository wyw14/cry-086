package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("record not found")

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 20
	config.MinConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ready(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) withTx(ctx context.Context, operation func(pgx.Tx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := operation(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func putObject(ctx context.Context, tx pgx.Tx, kind, id, siteID, secondary string, version int64, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_objects(kind, id, site_id, secondary_key, version, payload)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6)
	`, kind, id, siteID, secondary, version, payload)
	return err
}

func getObject[T any](ctx context.Context, pool *pgxpool.Pool, kind, id string) (T, error) {
	var zero T
	var payload []byte
	err := pool.QueryRow(ctx, `SELECT payload FROM domain_objects WHERE kind=$1 AND id=$2`, kind, id).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, ErrNotFound
	}
	if err != nil {
		return zero, err
	}
	if err := json.Unmarshal(payload, &zero); err != nil {
		return zero, fmt.Errorf("decode %s %s: %w", kind, id, err)
	}
	return zero, nil
}

func updateObject(ctx context.Context, tx pgx.Tx, kind, id string, expectedVersion, nextVersion int64, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE domain_objects
		SET payload=$4, version=$3, updated_at=now()
		WHERE kind=$1 AND id=$2 AND version=$5
	`, kind, id, nextVersion, payload, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("optimistic version conflict")
	}
	return nil
}
