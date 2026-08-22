package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLock int64 = 860086

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	migrationPath := "migrations/000001_initial.up.sql"
	if len(os.Args) > 1 {
		migrationPath = os.Args[1]
	}
	script, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", migrationPath, err)
	}

	base, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(base, 45*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("reserve migration connection: %w", err)
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLock); err != nil {
		return fmt.Errorf("lock schema: %w", err)
	}
	defer connection.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLock)

	if _, err := connection.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_revisions (
		name text PRIMARY KEY,
		content_sha256 text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	name := filepath.Base(migrationPath)
	digestBytes := sha256.Sum256(script)
	digest := hex.EncodeToString(digestBytes[:])
	var recorded string
	err = connection.QueryRow(ctx, `SELECT content_sha256 FROM schema_revisions WHERE name=$1`, name).Scan(&recorded)
	if err == nil {
		if recorded != digest {
			return fmt.Errorf("migration %s changed after it was applied", name)
		}
		fmt.Println("schema already current:", name)
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read migration ledger: %w", err)
	}

	tx, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, string(script)); err != nil {
		return fmt.Errorf("apply %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_revisions(name,content_sha256) VALUES($1,$2)`, name, digest); err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", name, err)
	}
	fmt.Println("schema advanced:", name)
	return nil
}
