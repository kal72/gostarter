// Command render-project is an explicit local-development project.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gostarter/internal/project/plan"
	"gostarter/internal/project/render"
	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/profile"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func execute() error {
	sourceDir := flag.String("source", "", "local upstream source")
	fixture := flag.Bool("fixture", false, "explicit development fixture; no production provenance claim")
	output := flag.String("out", "", "new output directory")
	module := flag.String("module", "github.com/example/core-service", "Go module")
	service := flag.String("name", "", "service name")
	description := flag.String("description", "", "short description")
	transport := flag.String("protocol", "dual", "dual or grpc")
	details := flag.Bool("details", false, "include validation receipt")
	extensions := flag.String("extensions", "", "comma-separated manifest extension IDs")
	flag.Parse()
	if !*fixture || *sourceDir == "" || *output == "" {
		return fmt.Errorf("requires -fixture -source <dir> -out <new-dir>; dual or grpc transport")
	}
	source, err := profile.ReadSource(os.DirFS(*sourceDir))
	if err != nil {
		return err
	}
	p, err := plan.Build(source, plan.FixtureIdentity("local-generation-development", source), plan.Config{Module: *module, ServiceName: *service, Description: *description, Protocol: *transport, RequestedExtensions: splitExtensions(*extensions)})
	if err != nil {
		return err
	}
	if _, statErr := os.Lstat(*output); !os.IsNotExist(statErr) {
		return fmt.Errorf("output must not exist: %s", *output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	artifact, err := render.Materialize(ctx, p, source)
	if err != nil {
		return err
	}
	if err = artifact.VerifyFor(p); err != nil {
		return err
	}
	if err = render.WriteNew(*output, artifact.Files); err != nil {
		return err
	}
	summary := map[string]any{"status": "build_passed", "output": *output, "plan_fingerprint": p.Fingerprint, "template_version": templates.Version, "snapshot_kind": p.Snapshot.Kind, "content_sha256": artifact.Receipt.ContentSHA256, "validation_identity": artifact.Receipt.IdentitySHA256, "files": len(artifact.Receipt.Files), "protocol": p.Config.Protocol}
	if *details {
		summary["receipt"] = artifact.Receipt
	}
	return json.NewEncoder(os.Stdout).Encode(summary)
}

func splitExtensions(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}
