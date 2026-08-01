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

func (s *FileSourceCodeStorage) objectPath(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("source code key must not be empty")
	}

	root := filepath.Clean(s.root)
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(key)))
	rootPrefix := root + string(os.PathSeparator)
	if path != root && !strings.HasPrefix(path, rootPrefix) {
		return "", fmt.Errorf("source code key escapes storage root")
	}
	return path, nil
}
