package migration

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreMustNotImportBackendInternalPackages(t *testing.T) {
	root := filepath.Clean("..")
	var hits []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		if strings.HasSuffix(path, "migration/no_backend_internal_imports_test.go") {
			return nil
		}
		b, _ := os.ReadFile(path)
		if strings.Contains(string(b), "leeforge-backend/internal/") {
			hits = append(hits, path)
		}
		return nil
	})
	if len(hits) > 0 {
		t.Fatalf("core still imports backend internal packages: %v", hits)
	}
}
