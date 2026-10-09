// Command validate-upstream performs static contract checks on an explicit local upstream tree.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

func main() {
	root := flag.String("source", "", "local upstream source directory (required)")
	detailed := flag.Bool("details", false, "include file ownership and module requirements")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "-source is required")
		os.Exit(2)
	}
	report, err := validate(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	var output any = map[string]any{"status": "static_contract_valid", "architecture_id": report.ArchitectureID, "profile_revision": report.ProfileRevision, "template_version": report.TemplateVersion, "source_module": report.SourceModule, "go_version": report.GoVersion, "classified_files": len(report.Files), "features": len(report.Features)}
	if *detailed {
		output = report
	}
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func validate(root string) (*profile.Report, error) {
	source, err := profile.ReadSource(os.DirFS(root))
	if err != nil {
		return nil, err
	}
	catalog, err := manifest.Parse(source[manifest.FileName])
	if err != nil {
		return nil, err
	}
	adapter, err := profile.Default(catalog)
	if err != nil {
		return nil, err
	}
	return profile.ValidateSource(catalog, adapter, source)
}
