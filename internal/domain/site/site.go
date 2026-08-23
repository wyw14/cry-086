package site

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidSite  = errors.New("invalid construction site")
	ErrInvalidCrane = errors.New("invalid tower crane")
)

type Position struct {
	XMillimeters int64 `json:"x_mm"`
	YMillimeters int64 `json:"y_mm"`
	BaseHeightMM int64 `json:"base_height_mm"`
}

type Site struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Timezone     string    `json:"timezone"`
	OwnerUnitID  string    `json:"owner_unit_id"`
	RetentionDay int       `json:"retention_days"`
	CreatedAt    time.Time `json:"created_at"`
	Version      int64     `json:"version"`
}

func NewSite(id, name, timezone, ownerUnitID string, retention int, now time.Time) (Site, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" || ownerUnitID == "" {
		return Site{}, ErrInvalidSite
	}
	if timezone == "" || retention < 30 {
		return Site{}, ErrInvalidSite
	}
	return Site{ID: id, Name: name, Timezone: timezone, OwnerUnitID: ownerUnitID, RetentionDay: retention, CreatedAt: now.UTC(), Version: 1}, nil
}

type CraneStatus string

const (
	CraneCommissioning CraneStatus = "commissioning"
	CraneRunning       CraneStatus = "running"
	CraneStopped       CraneStatus = "stopped"
	CraneFaulted       CraneStatus = "faulted"
	CraneRetired       CraneStatus = "retired"
)

type TowerCrane struct {
	ID                 string      `json:"id"`
	SiteID             string      `json:"site_id"`
	SerialNumber       string      `json:"serial_number"`
	ModelID            string      `json:"model_id"`
	SafetyConfigID     string      `json:"safety_config_id"`
	SafetyConfigVer    int64       `json:"safety_config_version"`
	Position           Position    `json:"position"`
	ResponsibleUnitID  string      `json:"responsible_unit_id"`
	CurrentDriverID    string      `json:"current_driver_id,omitempty"`
	Status             CraneStatus `json:"status"`
	ManualTakeover     bool        `json:"manual_takeover"`
	Version            int64       `json:"version"`
	LastStateChangedAt time.Time   `json:"last_state_changed_at"`
}

func NewCrane(id, siteID, serial, modelID, configID, unitID string, configVersion int64, pos Position, now time.Time) (TowerCrane, error) {
	if id == "" || siteID == "" || serial == "" || modelID == "" || configID == "" || unitID == "" || configVersion < 1 {
		return TowerCrane{}, ErrInvalidCrane
	}
	return TowerCrane{ID: id, SiteID: siteID, SerialNumber: serial, ModelID: modelID, SafetyConfigID: configID, SafetyConfigVer: configVersion, Position: pos, ResponsibleUnitID: unitID, Status: CraneCommissioning, Version: 1, LastStateChangedAt: now.UTC()}, nil
}

func (c *TowerCrane) Transition(to CraneStatus, now time.Time) error {
	allowed := map[CraneStatus]map[CraneStatus]bool{
		CraneCommissioning: {CraneRunning: true, CraneStopped: true},
		CraneRunning:       {CraneStopped: true, CraneFaulted: true},
		CraneStopped:       {CraneRunning: true, CraneFaulted: true, CraneRetired: true},
		CraneFaulted:       {CraneStopped: true},
	}
	if !allowed[c.Status][to] {
		return errors.New("illegal crane status transition")
	}
	c.Status = to
	c.Version++
	c.LastStateChangedAt = now.UTC()
	return nil
}

type CraneModel struct {
	ID               string `json:"id"`
	Manufacturer     string `json:"manufacturer"`
	Name             string `json:"name"`
	MaxHeightMM      int64  `json:"max_height_mm"`
	MaxRadiusMM      int64  `json:"max_radius_mm"`
	MaxWindMilliMPS  int64  `json:"max_wind_milli_mps"`
	MaxMomentNewtonM int64  `json:"max_moment_newton_m"`
}

type Organization struct {
	ID      string `json:"id"`
	SiteID  string `json:"site_id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Contact string `json:"contact_masked"`
}

type Driver struct {
	ID             string    `json:"id"`
	SiteID         string    `json:"site_id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	LicenseHash    string    `json:"license_hash"`
	LicenseExpires time.Time `json:"license_expires_at"`
	Active         bool      `json:"active"`
}

func (d Driver) CanOperate(at time.Time) bool {
	return d.Active && d.LicenseExpires.After(at.UTC())
}
