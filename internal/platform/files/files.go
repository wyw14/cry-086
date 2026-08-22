package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

type Metadata struct {
	ID       string `json:"id"`
	SiteID   string `json:"site_id"`
	Name     string `json:"name"`
	MIME     string `json:"mime"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Path     string `json:"-"`
}

type LocalStore struct {
	root       string
	maxSize    int64
	extensions map[string]bool
}

func NewLocalStore(root string, maxSize int64, extensions []string) (*LocalStore, error) {
	absolute, err := filepath.Abs(root)
	if err != nil || maxSize <= 0 {
		return nil, errors.New("invalid file store configuration")
	}
	allowed := make(map[string]bool, len(extensions))
	for _, extension := range extensions {
		allowed[strings.ToLower(extension)] = true
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, err
	}
	return &LocalStore{root: absolute, maxSize: maxSize, extensions: allowed}, nil
}

func (s *LocalStore) Save(ctx context.Context, id, siteID, name, declaredMIME string, source io.Reader) (Metadata, error) {
	extension := strings.ToLower(filepath.Ext(name))
	if id == "" || siteID == "" || !s.extensions[extension] {
		return Metadata{}, errors.New("file extension is not allowed")
	}
	expectedMIME := mime.TypeByExtension(extension)
	if expectedMIME != "" && !strings.HasPrefix(declaredMIME, strings.Split(expectedMIME, ";")[0]) {
		return Metadata{}, errors.New("file MIME does not match extension")
	}
	directory := filepath.Join(s.root, siteID)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return Metadata{}, err
	}
	target := filepath.Join(directory, id+extension)
	temporary, err := os.CreateTemp(directory, ".upload-*")
	if err != nil {
		return Metadata{}, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	hash := sha256.New()
	reader := io.LimitReader(source, s.maxSize+1)
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), reader)
	closeErr := temporary.Close()
	if copyErr != nil || closeErr != nil {
		return Metadata{}, errors.Join(copyErr, closeErr)
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	if written > s.maxSize {
		return Metadata{}, errors.New("file exceeds size limit")
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return Metadata{}, err
	}
	return Metadata{ID: id, SiteID: siteID, Name: filepath.Base(name), MIME: declaredMIME, Size: written, Checksum: hex.EncodeToString(hash.Sum(nil)), Path: target}, nil
}

func (s *LocalStore) Open(ctx context.Context, metadata Metadata, siteID string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if metadata.SiteID != siteID || !strings.HasPrefix(filepath.Clean(metadata.Path), filepath.Join(s.root, siteID)+string(os.PathSeparator)) {
		return nil, errors.New("file access denied")
	}
	return os.Open(metadata.Path)
}
