package assetstore

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const qrCodeAssetPrefix = "qr-codes"

type Store struct {
	root string
}

func New(dataPath string) *Store {
	root := strings.TrimSpace(dataPath)
	if root == "" {
		root = "data"
	}
	return &Store{root: filepath.Join(root, "assets")}
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) PutQRCodeAsset(siteID, qrID uuid.UUID, checksum, filename, contentType string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("asset data is empty")
	}
	ext := assetExtension(filename, contentType)
	if ext == "" {
		return "", fmt.Errorf("unsupported asset content type %q", contentType)
	}
	key := filepath.ToSlash(filepath.Join(
		qrCodeAssetPrefix,
		siteID.String(),
		qrID.String(),
		safeChecksum(checksum)+ext,
	))
	fullPath, err := s.pathForKey(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("create asset directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(fullPath), ".qr-asset-*")
	if err != nil {
		return "", fmt.Errorf("create temporary asset: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temporary asset: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary asset: %w", err)
	}
	if err := os.Rename(tmpName, fullPath); err != nil {
		return "", fmt.Errorf("store asset: %w", err)
	}
	cleanup = false
	return key, nil
}

func (s *Store) Open(key string) (*os.File, error) {
	fullPath, err := s.pathForKey(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open asset: %w", err)
	}
	return file, nil
}

func (s *Store) Delete(key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	fullPath, err := s.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete asset: %w", err)
	}
	_ = pruneEmptyParents(filepath.Dir(fullPath), s.root)
	return nil
}

func (s *Store) DeleteQRCodeAssetDir(siteID, qrID uuid.UUID) error {
	return s.removeAll(filepath.Join(qrCodeAssetPrefix, siteID.String(), qrID.String()))
}

func (s *Store) DeleteQRCodeAssetsForSite(siteID uuid.UUID) error {
	return s.removeAll(filepath.Join(qrCodeAssetPrefix, siteID.String()))
}

func (s *Store) removeAll(relative string) error {
	fullPath, err := s.pathForKey(filepath.ToSlash(relative))
	if err != nil {
		return err
	}
	if err := os.RemoveAll(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete asset directory: %w", err)
	}
	_ = pruneEmptyParents(filepath.Dir(fullPath), s.root)
	return nil
}

func (s *Store) pathForKey(key string) (string, error) {
	key = filepath.Clean(filepath.FromSlash(strings.TrimSpace(key)))
	if key == "." || key == "" || filepath.IsAbs(key) || strings.HasPrefix(key, ".."+string(filepath.Separator)) || key == ".." {
		return "", fmt.Errorf("invalid asset key")
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve asset root: %w", err)
	}
	fullPath := filepath.Join(root, key)
	if !strings.HasPrefix(fullPath, root+string(filepath.Separator)) && fullPath != root {
		return "", fmt.Errorf("asset key escapes root")
	}
	return fullPath, nil
}

func assetExtension(filename, contentType string) string {
	contentType, _, _ = mime.ParseMediaType(strings.TrimSpace(contentType))
	switch contentType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return ext
	default:
		return ""
	}
}

func safeChecksum(checksum string) string {
	checksum = strings.TrimSpace(strings.TrimPrefix(checksum, "sha256:"))
	var b strings.Builder
	for _, r := range checksum {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "asset"
	}
	return b.String()
}

func pruneEmptyParents(dir, root string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for {
		dirAbs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		if dirAbs == rootAbs || !strings.HasPrefix(dirAbs, rootAbs+string(filepath.Separator)) {
			return nil
		}
		if err := os.Remove(dirAbs); err != nil {
			if os.IsNotExist(err) || strings.Contains(err.Error(), "directory not empty") {
				return nil
			}
			return err
		}
		dir = filepath.Dir(dirAbs)
	}
}
