package app

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Keep infrastructure and presentation dependencies from creeping back inward.
func TestLayerDependencies(t *testing.T) {
	rules := map[string][]string{
		"web/handler": {"gostarter/internal/database", "gostarter/internal/storage"},
		"auth":        {"gostarter/internal/database", "gostarter/internal/storage", "gostarter/internal/web", "net/http"},
		"project":     {"gostarter/internal/database", "gostarter/internal/storage", "gostarter/internal/web", "net/http"},
		"upstream":    {"gostarter/internal/database", "gostarter/internal/storage", "gostarter/internal/web"},
		"web/ui":      {"gostarter/internal/database", "gostarter/internal/storage", "gostarter/internal/auth", "gostarter/internal/project", "gostarter/internal/upstream", "gostarter/internal/web/handler"},
	}
	for layer, forbidden := range rules {
		err := filepath.WalkDir(filepath.Join("..", layer), func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(name, ".go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				for _, prefix := range forbidden {
					if path == prefix || strings.HasPrefix(path, prefix+"/") {
						t.Errorf("%s imports forbidden dependency %s", name, path)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
