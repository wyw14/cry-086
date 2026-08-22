package identity

import (
	"errors"
	"time"
)

type Role string

const (
	RoleAdministrator Role = "administrator"
	RoleSafetyOfficer Role = "safety_officer"
	RoleDispatcher    Role = "dispatcher"
	RoleMaintainer    Role = "maintainer"
	RoleRegulator     Role = "regulator"
	RoleViewer        Role = "viewer"
)

type User struct {
	ID           string    `json:"id"`
	SiteIDs      []string  `json:"site_ids"`
	Name         string    `json:"name"`
	Username     string    `json:"username"`
	PasswordHash []byte    `json:"-"`
	Roles        []Role    `json:"roles"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	Version      int64     `json:"version"`
}

func (u User) HasRole(roles ...Role) bool {
	for _, owned := range u.Roles {
		for _, required := range roles {
			if owned == required {
				return true
			}
		}
	}
	return false
}

func (u User) OwnsSite(siteID string) bool {
	for _, id := range u.SiteIDs {
		if id == siteID {
			return true
		}
	}
	return false
}

type RefreshToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Digest    string     `json:"digest"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	Version   int64      `json:"version"`
}

func (t *RefreshToken) Revoke(at time.Time) error {
	if t.RevokedAt != nil {
		return errors.New("refresh token already revoked")
	}
	v := at.UTC()
	t.RevokedAt = &v
	t.Version++
	return nil
}

func (t RefreshToken) Valid(at time.Time) bool {
	return t.RevokedAt == nil && at.Before(t.ExpiresAt)
}
