package project

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

// CreateZipArchive packs validated materialized files into a zip archive.
// It places all files under a root directory named after serviceName,
// following the start.spring.io convention.
func CreateZipArchive(files map[string][]byte, serviceName string, w io.Writer) (err error) {
	root := serviceName
	if root != "" && (path.Base(root) != root || root == "." || root == ".." || strings.ContainsAny(root, "\\\r\n")) {
		return fmt.Errorf("unsafe zip root")
	}
	for name := range files {
		if name == "" || name == "." || path.Clean(name) != name || strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") {
			return fmt.Errorf("unsafe zip path")
		}
	}
	zipWriter := zip.NewWriter(w)
	defer func() {
		if err != nil {
			_ = zipWriter.Close()
		}
	}()

	// Sort file paths for deterministic zip output
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	rootPrefix := serviceName
	if rootPrefix == "" {
		rootPrefix = "project"
	}

	fixedModTime := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, relPath := range paths {
		content := files[relPath]
		zipPath := path.Join(rootPrefix, relPath)

		header := &zip.FileHeader{
			Name:     zipPath,
			Method:   zip.Deflate,
			Modified: fixedModTime,
		}

		// Preserve executable permissions for scripts
		if strings.HasSuffix(relPath, ".sh") {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed creating zip entry %s: %w", zipPath, err)
		}

		if _, err := writer.Write(content); err != nil {
			return fmt.Errorf("failed writing zip entry %s: %w", zipPath, err)
		}
	}

	return zipWriter.Close()
}
