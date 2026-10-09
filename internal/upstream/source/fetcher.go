package source

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultRepoOwner = "kal72"
	DefaultRepoName  = "go-feature-based-boilerplate"
	DefaultBranch    = "main"
)

type CommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Date    string `json:"date"`
}

type GitHubCommitResponse struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type Fetcher struct {
	httpClient *http.Client
	owner      string
	repo       string
	branch     string
}

func NewFetcher(owner, repo, branch string) *Fetcher {
	if owner == "" {
		owner = DefaultRepoOwner
	}
	if repo == "" {
		repo = DefaultRepoName
	}
	if branch == "" {
		branch = DefaultBranch
	}
	return &Fetcher{
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		owner:  owner,
		repo:   repo,
		branch: branch,
	}
}

// FetchCommitInfo fetches the latest commit metadata from GitHub API
func (f *Fetcher) FetchCommitInfo(ctx context.Context) (*CommitInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", url.PathEscape(f.owner), url.PathEscape(f.repo), url.PathEscape(f.branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "GoStarter-Generator/1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("github api returned status %d: %s", resp.StatusCode, string(body))
	}

	var ghResp GitHubCommitResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&ghResp); err != nil {
		return nil, err
	}

	if !commitSHA.MatchString(ghResp.SHA) {
		return nil, fmt.Errorf("github returned invalid full SHA")
	}
	firstLine := strings.Split(ghResp.Commit.Message, "\n")[0]

	return &CommitInfo{
		SHA:     ghResp.SHA,
		Message: firstLine,
		Date:    ghResp.Commit.Author.Date,
	}, nil
}

var commitSHA = regexp.MustCompile(`^[a-f0-9]{40}$`)

const MaxArchiveBytes = 32 << 20

func (f *Fetcher) DownloadCommit(ctx context.Context, sha string) ([]byte, error) {
	if !commitSHA.MatchString(sha) {
		return nil, fmt.Errorf("full commit SHA required")
	}
	endpoint := fmt.Sprintf("https://github.com/%s/%s/archive/%s.zip", url.PathEscape(f.owner), url.PathEscape(f.repo), sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "GoStarter-Generator/1.0")
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("archive status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxArchiveBytes {
		return nil, fmt.Errorf("archive exceeds limit")
	}
	return data, nil
}

// Fetch pins metadata and bytes to one resolved commit.
func (f *Fetcher) Fetch(ctx context.Context) (*CommitInfo, []byte, error) {
	info, err := f.FetchCommitInfo(ctx)
	if err != nil {
		return nil, nil, err
	}
	data, err := f.DownloadCommit(ctx, info.SHA)
	if err != nil {
		return nil, nil, err
	}
	return info, data, nil
}

// UnpackZip parses a zip archive and converts it into a map of relative file path -> content.
// It automatically strips the top-level repository directory (e.g. "go-feature-based-boilerplate-main/").
func UnpackZip(zipData []byte) (map[string][]byte, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip data: %w", err)
	}

	if len(zipData) > MaxArchiveBytes || len(zipReader.File) > 10000 {
		return nil, fmt.Errorf("archive exceeds limits")
	}
	files := make(map[string][]byte)
	seen := map[string]bool{}
	root := ""
	total := 0
	for _, file := range zipReader.File {
		name := file.Name
		if !fs.ValidPath(strings.TrimSuffix(name, "/")) || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.Clean(name) != strings.TrimSuffix(name, "/") {
			return nil, fmt.Errorf("unsafe archive path %q", name)
		}
		parts := strings.SplitN(strings.TrimSuffix(name, "/"), "/", 2)
		if root == "" {
			root = parts[0]
		}
		if root != parts[0] {
			return nil, fmt.Errorf("multiple archive roots")
		}
		if file.Mode()&fs.ModeSymlink != 0 || (!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
			return nil, fmt.Errorf("nonregular archive entry")
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if len(parts) != 2 || !fs.ValidPath(parts[1]) {
			return nil, fmt.Errorf("invalid archive path")
		}
		rel := parts[1]
		key := strings.ToLower(rel)
		if seen[key] {
			return nil, fmt.Errorf("duplicate archive path %s", rel)
		}
		seen[key] = true
		if file.UncompressedSize64 > 8<<20 {
			return nil, fmt.Errorf("archive file exceeds limit")
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, (8<<20)+1))
		closeErr := rc.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		total += len(content)
		if len(content) > 8<<20 || total > 64<<20 {
			return nil, fmt.Errorf("archive contents exceed limit")
		}
		files[rel] = content
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("empty archive")
	}
	for name := range files {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if seen[strings.ToLower(parent)] {
				return nil, fmt.Errorf("archive file/directory collision")
			}
		}
	}
	return files, nil
}
