// Command plan-project inspects static plans from explicit local fixture source.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gostarter/internal/project/plan"
	"gostarter/internal/upstream/profile"
	"os"
	"strings"
)

func main() {
	root := flag.String("source", "", "local upstream directory (required)")
	fixture := flag.Bool("fixture", false, "explicitly acknowledge local analysis source, not a verified immutable snapshot")
	extensions := flag.String("extensions", "", "comma-separated explicit optional IDs")
	module := flag.String("module", "", "target module")
	transport := flag.String("protocol", "dual", "dual or grpc")
	goVersion := flag.String("go", "source", "target Go version")
	details := flag.Bool("details", false, "include all file decisions")
	flag.Parse()
	if *root == "" || !*fixture {
		fmt.Fprintln(os.Stderr, "-source and -fixture are required; this tool does not activate production snapshots")
		os.Exit(2)
	}
	source, err := profile.ReadSource(os.DirFS(*root))
	if err != nil {
		die(err)
	}
	requested := []string{}
	if strings.TrimSpace(*extensions) != "" {
		requested = strings.Split(*extensions, ",")
	}
	p, err := plan.Build(source, plan.FixtureIdentity("local-analysis", source), plan.Config{Module: *module, Protocol: *transport, GoVersion: *goVersion, RequestedExtensions: requested})
	if err != nil {
		die(err)
	}
	counts := map[profile.Action]int{}
	for _, file := range p.Files {
		counts[file.Action]++
	}
	var output any = map[string]any{"fingerprint": p.Fingerprint, "snapshot": p.Snapshot, "validation_level": p.ValidationLevel, "config": p.Config, "resolved": p.Resolved, "file_actions": counts}
	if *details {
		output = p
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		die(err)
	}
}
func die(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
