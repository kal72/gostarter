package project

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestExplorePreservesMaterializedFiles(t *testing.T) {
	files := map[string][]byte{"go.mod": []byte("module example.com/app\n"), "internal/user/handler.go": []byte("package user\n"), "assets/icon.bin": {0, 1, 2}}
	view := BuildExploreData(files, "app")
	if view.Root != "app" || view.TotalFiles != 3 || len(view.Files) != 3 {
		t.Fatalf("incomplete preview: %+v", view)
	}
	if view.Files["go.mod"] != string(files["go.mod"]) || view.Files["internal/user/handler.go"] != string(files["internal/user/handler.go"]) || view.Files["assets/icon.bin"] != "[Binary file]" {
		t.Fatal("preview lost materialized content or binary placeholder")
	}
	if len(view.Tree.Children) != 3 || view.Tree.Children[0].Name != "assets" || view.Tree.Children[1].Name != "internal" || view.Tree.Children[2].Name != "go.mod" {
		t.Fatal("preview ordering changed")
	}
	user := view.Tree.Children[1].Children[0]
	if user.Path != "internal/user" || len(user.Children) != 1 || user.Children[0].Path != "internal/user/handler.go" || user.Children[0].Size != len(files["internal/user/handler.go"]) {
		t.Fatal("nested preview path or size lost")
	}
	encoded, err := json.Marshal(view)
	if err != nil || !bytes.Contains(encoded, []byte(`"path":"internal/user/handler.go"`)) {
		t.Fatalf("preview JSON path contract changed: %s %v", encoded, err)
	}

}
