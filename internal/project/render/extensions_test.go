package render

import (
	"bytes"

	"testing"
)

func TestProtoPackageDeclarationOnly(t *testing.T) {
	body := []byte(`syntax = "proto3";
// option go_package = "do-not-edit";
option go_package = "./examplepb";
message A { string go_package = 1; }
`)
	got, err := rewriteProtoPackage("api/proto/example/example.proto", body, "example.com/new")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`option go_package = "example.com/new/gen/pb/example;examplepb";`)) || !bytes.Contains(got, []byte(`// option go_package = "do-not-edit";`)) {
		t.Fatal(string(got))
	}
	for _, bad := range []string{`syntax="proto3";`, `option go_package="./pb"; option go_package="./pb";`} {
		if _, err := rewriteProtoPackage("api/proto/a/a.proto", []byte(bad), "example.com/new"); err == nil {
			t.Fatal("accepted invalid declaration")
		}
	}
}
