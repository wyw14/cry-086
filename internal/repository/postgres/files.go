package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wyw14/cry-086/internal/platform/files"
)

func (s *Store) RegisterEvidence(ctx context.Context, metadata files.Metadata) error {
	if metadata.ID == "" || metadata.SiteID == "" || metadata.Checksum == "" {
		return errors.New("evidence metadata is incomplete")
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode evidence metadata: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO file_metadata(id,site_id,name,mime,size_bytes,checksum,storage_path,payload)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
	`, metadata.ID, metadata.SiteID, metadata.Name, metadata.MIME, metadata.Size, metadata.Checksum, metadata.Path, payload)
	if err != nil {
		return fmt.Errorf("register site evidence: %w", err)
	}
	return nil
}

func (s *Store) FindSiteEvidence(ctx context.Context, siteID, evidenceID string) (files.Metadata, error) {
	var metadata files.Metadata
	err := s.pool.QueryRow(ctx, `
		SELECT id,site_id,name,mime,size_bytes,checksum,storage_path
		FROM file_metadata
		WHERE site_id=$1 AND id=$2
	`, siteID, evidenceID).Scan(
		&metadata.ID, &metadata.SiteID, &metadata.Name, &metadata.MIME,
		&metadata.Size, &metadata.Checksum, &metadata.Path,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return files.Metadata{}, ErrNotFound
	}
	if err != nil {
		return files.Metadata{}, fmt.Errorf("find site evidence: %w", err)
	}
	return metadata, nil
}
