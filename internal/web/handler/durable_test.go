package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"gostarter/internal/project"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"net/url"
	"reflect"
	"testing"
)

func TestRequestTransportEquivalence(t *testing.T) {
	body := []byte(`{"module":"github.com/acme/orders","service_name":"orders","description":"Order café","protocol":"grpc","extensions":["auth"],"include_auth":true}`)
	want, err := parseProject(body)
	if err != nil {
		t.Fatal(err)
	}
	variants := []url.Values{{"module": {"github.com/acme/orders"}, "name": {"orders"}, "description": {"Order café"}, "api": {"grpc"}, "extensions": {"auth"}, "auth": {"true"}}}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		variants = append(variants, url.Values{"config": {encoding.EncodeToString(body)}})
	}
	variants = append(variants, url.Values{"config": {string(body)}})
	for _, q := range variants {
		got, err := queryProject(q)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("different input: %+v vs %+v", want, got)
		}
	}
}
func TestRequestRejectsAmbiguity(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `{"protocol":"dual","protocol":"grpc"}`, `{"extensions":null}`, `{"extensions":[],"requested_extensions":[]}`, `{"include_auth":"true"}`, `{"unexpected":true}`, `{"plan_id":"bad"}`, `{} {}`, `{"protocol":"rest"}`} {
		if _, err := parseProject([]byte(body)); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
	for _, q := range []url.Values{{"module": {"a", "b"}}, {"m": {"a"}, "module": {"b"}}, {"config": {"{}"}, "protocol": {"grpc"}}, {"auth": {"maybe"}}, {"config": {"{}"}, "c": {"{}"}}} {
		if _, err := queryProject(q); err == nil {
			t.Errorf("accepted %s", q)
		}
	}
}
func TestLegacyPresence(t *testing.T) {
	a, err := parseProject([]byte(`{"include_auth":true}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := parseProject([]byte(`{"include_auth":true,"include_user":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Flags["user"]; ok {
		t.Fatal("absent flag became false")
	}
	if _, ok := b.Flags["user"]; !ok {
		t.Fatal("false flag lost")
	}
	encoded, _ := json.Marshal(a.Config)
	if len(encoded) == 0 || len(a.Config.RequestedExtensions) != 1 {
		t.Fatal("legacy intent missing")
	}
}

func TestPlanTerminalStates(t *testing.T) {
	h := &APIHandler{}
	for _, item := range []struct {
		status, engine, template string
		want                     int
	}{{"expired", plan.EngineVersion, templates.Version, 410}, {"failed", plan.EngineVersion, templates.Version, 422}, {"ready", "old", templates.Version, 409}, {"pending", plan.EngineVersion, templates.Version, 202}, {"validating", plan.EngineVersion, templates.Version, 202}} {
		p := project.StoredPlan{Status: item.status, Plan: plan.Plan{EngineVersion: item.engine, TemplateVersion: item.template, TemplateSHA256: templates.Digest()}}
		status, _, err := h.state(context.Background(), p)
		if err != nil || status != item.want {
			t.Fatalf("%s: %d %v", item.status, status, err)
		}
	}
}
