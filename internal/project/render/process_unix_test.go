package render

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeadlineStopsChildProcesses(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "leaked")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := run(ctx, root, os.Environ(), "/bin/sh", "-c", `(sleep 1; printf leak > "$1") & wait`, "test", marker); err == nil {
		t.Fatal("deadline ignored")
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("command child survived cancellation", err)
	}
}
