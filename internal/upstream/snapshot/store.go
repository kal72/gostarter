// Package snapshot stores upstream archives only. Generated projects never enter this store.
package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"gostarter/internal/project/plan"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
	"gostarter/internal/upstream/source"
)

type Metadata struct {
	ID            string          `json:"id"`
	Repository    string          `json:"repository"`
	Ref           string          `json:"ref"`
	CommitSHA     string          `json:"commit_sha"`
	ArchiveSHA256 string          `json:"archive_sha256"`
	ContentSHA256 string          `json:"content_sha256"`
	ArchiveKey    string          `json:"archive_key"`
	Manifest      json.RawMessage `json:"manifest"`
	Compatibility json.RawMessage `json:"compatibility"`
}

type Store struct{ root string }

var shaPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func digest(data []byte) string { s := sha256.Sum256(data); return hex.EncodeToString(s[:]) }
func NewStore(directory string) (*Store, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("snapshot directory is required")
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("snapshot root must be a directory")
	}
	return &Store{root}, nil
}

// Prepare performs static contract/profile validation, never claiming a successful build.
func Prepare(repository, ref, sha string, archive []byte) (Metadata, profile.Source, error) {
	var m Metadata
	repository = strings.ToLower(repository)
	if !repoPattern.MatchString(repository) || strings.Contains(repository, "..") || ref == "" || len(ref) > 512 || !shaPattern.MatchString(sha) {
		return m, nil, fmt.Errorf("invalid source identity")
	}
	files, err := source.UnpackZip(archive)
	if err != nil {
		return m, nil, err
	}
	catalog, err := manifest.Parse(files[manifest.FileName])
	if err != nil {
		return m, nil, err
	}
	adapter, err := profile.Registry("go-feature-based-clean", "1", catalog)
	if err != nil {
		return m, nil, err
	}
	report, err := profile.ValidateSource(catalog, adapter, files)
	if err != nil {
		return m, nil, err
	}
	manifestData, err := json.Marshal(catalog)
	if err != nil {
		return m, nil, err
	}
	compatibility, err := json.Marshal(report)
	if err != nil {
		return m, nil, err
	}
	archiveHash := digest(archive)
	id := digest([]byte(repository + "\x00" + ref + "\x00" + sha + "\x00" + archiveHash))
	m = Metadata{ID: id, Repository: repository, Ref: ref, CommitSHA: sha, ArchiveSHA256: archiveHash, ContentSHA256: plan.ContentDigest(files), ArchiveKey: id + ".zip", Manifest: manifestData, Compatibility: compatibility}
	return m, files, nil
}
func (m Metadata) Identity() plan.Snapshot {
	return plan.Snapshot{ID: m.ID, Kind: "immutable", CommitSHA: m.CommitSHA, ContentSHA256: m.ContentSHA256}
}
func (m Metadata) Validate() error {
	if !digestPattern.MatchString(m.ID) || !repoPattern.MatchString(m.Repository) || m.Repository != strings.ToLower(m.Repository) || !shaPattern.MatchString(m.CommitSHA) || !digestPattern.MatchString(m.ArchiveSHA256) || !digestPattern.MatchString(m.ContentSHA256) || m.ArchiveKey != m.ID+".zip" || m.Ref == "" || !json.Valid(m.Manifest) || !json.Valid(m.Compatibility) {
		return fmt.Errorf("invalid snapshot metadata")
	}
	if m.ID != digest([]byte(m.Repository+"\x00"+m.Ref+"\x00"+m.CommitSHA+"\x00"+m.ArchiveSHA256)) {
		return fmt.Errorf("snapshot identity mismatch")
	}
	return nil
}

// Put stages and fsyncs bytes then atomically links without replacing an existing archive.
// A crash before metadata insertion may leave an unreferenced archive, never an active pointer.
func (s *Store) Put(ctx context.Context, m Metadata, archive []byte) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if digest(archive) != m.ArchiveSHA256 {
		return fmt.Errorf("archive checksum mismatch")
	}
	checked, _, err := Prepare(m.Repository, m.Ref, m.CommitSHA, archive)
	if err != nil {
		return err
	}
	if checked.ContentSHA256 != m.ContentSHA256 || !sameJSON(checked.Manifest, m.Manifest) {
		return fmt.Errorf("snapshot metadata differs from archive")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.root, ".staging-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(archive); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Chmod(f.Name(), 0400); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	destination := filepath.Join(s.root, m.ArchiveKey)
	if err = os.Link(f.Name(), destination); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		_, err = s.Read(ctx, m)
		return err
	}
	directory, err := os.Open(s.root)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr = directory.Close()
	return errors.Join(err, closeErr)
}
func (s *Store) Read(ctx context.Context, m Metadata) (profile.Source, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(m.ArchiveKey)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > source.MaxArchiveBytes {
		return nil, fmt.Errorf("invalid archive file")
	}
	f, err := root.Open(m.ArchiveKey)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, source.MaxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if digest(data) != m.ArchiveSHA256 {
		return nil, fmt.Errorf("archive checksum mismatch")
	}
	files, err := source.UnpackZip(data)
	if err != nil {
		return nil, err
	}
	if plan.ContentDigest(files) != m.ContentSHA256 {
		return nil, fmt.Errorf("source checksum mismatch")
	}
	return files, ctx.Err()
}

// Remove only accepts a content-addressed key. Callers must hold the retention transaction lock.
func (s *Store) Remove(key string) error {
	if !strings.HasSuffix(key, ".zip") || !digestPattern.MatchString(strings.TrimSuffix(key, ".zip")) {
		return fmt.Errorf("invalid archive key")
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return err
	}
	defer root.Close()
	err = root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
