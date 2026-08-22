package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/site"
	"github.com/wyw14/cry-086/internal/repository/postgres"
)

func TestPostgresRegistryRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := "integration-site-" + time.Now().UTC().Format("150405.000000000")
	created, err := site.NewSite(id, "Integration Site", "UTC", "integration-owner", 30, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSite(ctx, created); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.FindSite(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != created.Name || loaded.Version != created.Version {
		t.Fatalf("loaded = %#v, want %#v", loaded, created)
	}
}
