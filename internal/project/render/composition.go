package render

import (
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/scanner"

	"golang.org/x/mod/modfile"
	"gostarter/internal/project/plan"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

type featureTemplate struct {
	profile.FeatureIntegration
	Alias string
}
type templateData struct {
	plan.Config
	Features         []featureTemplate
	PublicMethods    []string
	Protos           []string
	NeedsTransaction bool
	HasAuth          bool
}

func composition(p *plan.Plan, source profile.Source) (templateData, error) {
	data := templateData{Config: p.Config}
	catalog, err := manifest.Parse(source[manifest.FileName])
	if err != nil {
		return data, err
	}
	prof, err := profile.Registry(p.ArchitectureID, p.ProfileRevision, catalog)
	if err != nil {
		return data, err
	}
	module, err := modfile.Parse("go.mod", source["go.mod"], nil)
	if err != nil {
		return data, err
	}
	byID := map[string]profile.FeatureIntegration{}
	for _, f := range prof.Features {
		byID[f.ID] = f
	}
	for _, e := range p.Resolved.Extensions {
		if e.Kind != "feature" {
			continue
		}
		f, ok := byID[e.ID]
		if !ok {
			return data, fmt.Errorf("missing integration profile: %s", e.ID)
		}
		data.Features = append(data.Features, featureTemplate{f, fmt.Sprintf("feature%d", len(data.Features))})
		// Public routes remain application policy, independent of selecting a package named auth.
		switch f.SourcePath {
		case "internal/user":
			data.PublicMethods = append(data.PublicMethods, "/user.v1.UserService/CreateUser")
		case "internal/auth":
			data.HasAuth = true
			data.PublicMethods = append(data.PublicMethods, "/auth.v1.AuthService/Login", "/auth.v1.AuthService/Refresh", "/auth.v1.AuthService/Logout")
		}
		for name, body := range source {
			if strings.HasPrefix(name, f.SourcePath+"/") && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
				parsed, err := parser.ParseFile(token.NewFileSet(), name, body, parser.ImportsOnly)
				if err != nil {
					return data, err
				}
				for _, imp := range parsed.Imports {
					value, _ := strconv.Unquote(imp.Path.Value)
					if value == module.Module.Mod.Path+"/pkg/transaction" {
						data.NeedsTransaction = true
					}
				}
			}
		}
	}
	for _, f := range p.Files {
		if strings.HasPrefix(f.Path, "api/proto/") && !strings.HasPrefix(f.Path, "api/proto/plugins/") && strings.HasSuffix(f.Path, ".proto") && f.Action != profile.Exclude {
			data.Protos = append(data.Protos, f.Path)
		}
	}
	sort.Strings(data.Protos)
	sort.Strings(data.PublicMethods)
	return data, nil
}

// rewriteProtoPackage scans declaration tokens and edits only the go_package string.
// Comments, unrelated strings, imports, and protobuf package/service names stay intact.
func rewriteProtoPackage(name string, body []byte, module string) ([]byte, error) {
	var scan scanner.Scanner
	scan.Init(strings.NewReader(string(body)))
	scan.Error = func(*scanner.Scanner, string) {}
	scan.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanComments | scanner.SkipComments
	type lexeme struct {
		text   string
		offset int
		kind   rune
	}
	var tokens []lexeme
	for kind := scan.Scan(); kind != scanner.EOF; kind = scan.Scan() {
		tokens = append(tokens, lexeme{scan.TokenText(), scan.Position.Offset, kind})
	}
	depth, count, start, end := 0, 0, 0, 0
	alias := ""
	for i, t := range tokens {
		if t.text == "{" {
			depth++
		}
		if t.text == "}" {
			depth--
		}
		if depth != 0 || t.text != "option" || i+4 >= len(tokens) {
			continue
		}
		next := tokens[i+1 : i+5]
		if next[0].text != "go_package" || next[1].text != "=" || next[2].kind != scanner.String || next[3].text != ";" {
			continue
		}
		value, err := strconv.Unquote(next[2].text)
		if err != nil {
			return nil, err
		}
		alias = path.Base(value)
		if _, suffix, ok := strings.Cut(value, ";"); ok {
			alias = suffix
		}
		start, end = next[2].offset, next[2].offset+len(next[2].text)
		count++
	}
	if scan.ErrorCount != 0 || count != 1 || !token.IsIdentifier(alias) {
		return nil, fmt.Errorf("%s: expected one valid top-level go_package option", name)
	}
	rel := strings.TrimPrefix(path.Dir(name), "api/proto/")
	replacement := strconv.Quote(module + "/gen/pb/" + rel + ";" + alias)
	out := append([]byte{}, body[:start]...)
	out = append(out, []byte(replacement)...)
	out = append(out, body[end:]...)
	return out, nil
}
