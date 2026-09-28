package judgeinfra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type FileSourceCodeStorage struct {
	root string
}

func NewFileSourceCodeStorage(root string) *FileSourceCodeStorage {
	return &FileSourceCodeStorage{root: root}
}

func (s *FileSourceCodeStorage) PutSourceCode(_ context.Context, params repository.PutSourceCodeParams) error {
	path, err := s.objectPath(params.Key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(params.SourceCode), 0o600)
}

func (s *FileSourceCodeStorage) PutTestCases(_ context.Context, params repository.PutTestCasesParams) (string, error) {
	body, key, err := marshalTestCases(params)
	if err != nil {
		return "", err
	}
	path, err := s.objectPath(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", err
	}
	return key, nil
}

func (s *FileSourceCodeStorage) objectPath(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("source code key must not be empty")
	}

	keyPath := filepath.FromSlash(key)
	if filepath.IsAbs(keyPath) {
		return "", fmt.Errorf("source code key must be relative")
	}

	root := filepath.Clean(s.root)
	path := filepath.Clean(filepath.Join(root, keyPath))
	rootPrefix := root + string(os.PathSeparator)
	if path != root && !strings.HasPrefix(path, rootPrefix) {
		return "", fmt.Errorf("source code key escapes storage root")
	}
	return path, nil
}
