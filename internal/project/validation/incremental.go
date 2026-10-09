package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"gostarter/internal/project/plan"
	"gostarter/internal/project/render"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

// CaseCache stores only successful build-and-smoke evidence, never generated files.
type CaseCache interface {
	LookupValidationCase(context.Context, string) (Result, bool, error)
	SaveValidationCase(context.Context, string, Result) error
}

type caseEvidence struct {
	Input  string
	Result Result
}

// globalInputs deliberately invalidates the matrix when architecture/shared inputs change.
func globalInputs(files profile.Source, p *plan.Plan) (string, error) {
	catalog, err := manifest.Parse(files[manifest.FileName])
	if err != nil {
		return "", err
	}
	adapter, err := profile.Registry(p.ArchitectureID, p.ProfileRevision, catalog)
	if err != nil {
		return "", err
	}
	shared := profile.Source{manifest.FileName: files[manifest.FileName], "go.mod": files["go.mod"], "go.sum": files["go.sum"]}
	for name, body := range files {
		rule, err := adapter.Classify(name)
		if err != nil {
			return "", err
		}
		if _, optional := catalog.Lookup(rule.Owner); !optional && rule.Action != profile.Exclude {
			shared[name] = body
		}
	}
	return digest([]any{Key(), p.ProfileSHA256, p.Environment, plan.ContentDigest(shared)})
}

func caseInput(global string, p *plan.Plan, files profile.Source) (string, error) {
	assembled, err := render.Assemble(p, files)
	if err != nil {
		return "", err
	}
	// The snapshot-wide fingerprint must not be used: it changes for unrelated extensions.
	return caseContentKey(global, p, assembled)
}

func caseContentKey(global string, p *plan.Plan, assembled profile.Source) (string, error) {
	return digest([]any{global, p.Config, p.Resolved, plan.ContentDigest(assembled)})
}

func digest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func validResult(r Result) bool {
	for _, value := range []string{r.ContentSHA256, r.IdentitySHA256} {
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size {
			return false
		}
	}
	return true
}

func validateCases(ctx context.Context, id plan.Snapshot, files profile.Source, cases []*plan.Plan, execution string, cache CaseCache, run func(context.Context, *plan.Plan, profile.Source) (Result, error)) (Result, error) {
	if len(cases) == 0 {
		return Result{}, fmt.Errorf("empty validation matrix")
	}
	global, err := globalInputs(files, cases[0])
	if err != nil {
		return Result{}, err
	}
	global, err = digest([]string{global, execution})
	if err != nil {
		return Result{}, err
	}
	inputs := make([]validationCase, 0, len(cases))
	for _, p := range cases {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		input, err := caseInput(global, p, files)
		if err != nil {
			return Result{}, err
		}
		inputs = append(inputs, validationCase{p, input})
	}
	return runCases(ctx, id, files, inputs, cache, run)
}

type validationCase struct {
	plan  *plan.Plan
	input string
}

func runCases(ctx context.Context, id plan.Snapshot, files profile.Source, cases []validationCase, cache CaseCache, run func(context.Context, *plan.Plan, profile.Source) (Result, error)) (Result, error) {
	evidence := make([]caseEvidence, 0, len(cases))
	executed, reused := 0, 0
	for _, item := range cases {
		p, input := item.plan, item.input
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var result Result
		hit := false
		if cache != nil {
			cached, found, lookupErr := cache.LookupValidationCase(ctx, input)
			if lookupErr == nil && found && validResult(cached) {
				result, hit = cached, true
			}
		}
		if hit {
			reused++
		} else {
			var err error
			result, err = run(ctx, p, files)
			if err != nil {
				return Result{}, fmt.Errorf("%s %v: %w", p.Config.Protocol, p.Config.RequestedExtensions, err)
			}
			if !validResult(result) {
				return Result{}, fmt.Errorf("validation case returned invalid evidence")
			}
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			executed++
			if cache != nil {
				_ = cache.SaveValidationCase(ctx, input, result)
			}
		}
		evidence = append(evidence, caseEvidence{input, result})
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	identity, err := digest([]any{Key(), id, evidence})
	if err != nil {
		return Result{}, err
	}
	log.Printf("[Validation] Snapshot matrix: %d executed, %d reused, %d total", executed, reused, len(cases))
	return Result{plan.ContentDigest(files), identity}, nil
}
