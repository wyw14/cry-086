package evidence

import (
	"context"
	"errors"
	"io"

	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/platform/files"
)

type Catalog interface {
	RegisterEvidence(context.Context, files.Metadata) error
	FindSiteEvidence(context.Context, string, string) (files.Metadata, error)
}

type IdentityProvider interface {
	FindUser(context.Context, string) (identity.User, error)
}

type Storage interface {
	Save(context.Context, string, string, string, string, io.Reader) (files.Metadata, error)
	Open(context.Context, files.Metadata, string) (io.ReadCloser, error)
}

type IDGenerator interface{ NewID() string }

type Service struct {
	catalog    Catalog
	identities IdentityProvider
	storage    Storage
	ids        IDGenerator
}

func New(catalog Catalog, identities IdentityProvider, storage Storage, ids IDGenerator) *Service {
	return &Service{catalog: catalog, identities: identities, storage: storage, ids: ids}
}

func (s *Service) Upload(ctx context.Context, actorID, siteID, name, mime string, source io.Reader) (files.Metadata, error) {
	if err := s.authorize(ctx, actorID, siteID, identity.RoleMaintainer, identity.RoleSafetyOfficer); err != nil {
		return files.Metadata{}, err
	}
	metadata, err := s.storage.Save(ctx, s.ids.NewID(), siteID, name, mime, source)
	if err != nil {
		return files.Metadata{}, err
	}
	if err := s.catalog.RegisterEvidence(ctx, metadata); err != nil {
		return files.Metadata{}, err
	}
	metadata.Path = ""
	return metadata, nil
}

func (s *Service) Download(ctx context.Context, actorID, siteID, fileID string) (files.Metadata, io.ReadCloser, error) {
	if err := s.authorize(ctx, actorID, siteID, identity.RoleMaintainer, identity.RoleSafetyOfficer, identity.RoleRegulator); err != nil {
		return files.Metadata{}, nil, err
	}
	metadata, err := s.catalog.FindSiteEvidence(ctx, siteID, fileID)
	if err != nil {
		return files.Metadata{}, nil, err
	}
	reader, err := s.storage.Open(ctx, metadata, siteID)
	if err != nil {
		return files.Metadata{}, nil, err
	}
	return metadata, reader, nil
}

func (s *Service) authorize(ctx context.Context, actorID, siteID string, roles ...identity.Role) error {
	user, err := s.identities.FindUser(ctx, actorID)
	if err != nil || !user.Active || !user.OwnsSite(siteID) || !user.HasRole(roles...) {
		return errors.New("file operation is not authorized")
	}
	return nil
}
