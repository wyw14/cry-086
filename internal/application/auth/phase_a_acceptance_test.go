package authapp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/cry-086/internal/domain/identity"
	platformclock "github.com/wyw14/cry-086/internal/platform/clock"
	"github.com/wyw14/cry-086/internal/repository/memory"
)

type authIDs struct{ next int }

func (i *authIDs) NewID() string { i.next++; return fmt.Sprintf("refresh-%d", i.next) }

type fixedSigner struct{}

func (fixedSigner) SignAccess(user identity.User, expiresAt time.Time) (string, error) {
	return user.ID + ":" + expiresAt.UTC().Format(time.RFC3339Nano), nil
}

func TestRefreshRejectsTokenWithWrongSecret(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	repository := memory.New()
	passwordHash, err := HashPassword("StrongPassword!2026")
	if err != nil {
		t.Fatal(err)
	}
	user := identity.User{ID: "user-refresh", Username: "operator", PasswordHash: passwordHash, SiteIDs: []string{"site-r"}, Roles: []identity.Role{identity.RoleSafetyOfficer}, Active: true}
	repository.SeedUser(user)
	service := New(repository, fixedSigner{}, platformclock.NewFixed(now), &authIDs{})
	issued, err := service.Login(context.Background(), user.Username, "StrongPassword!2026")
	if err != nil {
		t.Fatal(err)
	}
	id, _, ok := strings.Cut(issued.RefreshToken, ".")
	if !ok {
		t.Fatal("issued refresh token has no secret")
	}
	if _, err := service.Refresh(context.Background(), id+".attacker-controlled-secret"); err == nil {
		t.Fatal("refresh accepted a token with the wrong secret")
	}
	stored, err := repository.FindRefreshToken(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Valid(now) {
		t.Fatal("failed forgery revoked the legitimate refresh token")
	}
}
