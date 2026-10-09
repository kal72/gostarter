package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gostarter/internal/project"
	"gostarter/internal/project/protocol"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// request keeps legacy boolean presence separate from false. Canonical selections
// are compared with the dependency closure, never silently OR-ed together.
type projectRequest = project.ProjectRequest

var legacyFlags = map[string]string{"include_auth": "auth", "include_user": "user", "include_idempotency": "idempotency", "include_pagination": "pagination", "include_httpclient": "httpclient", "include_routine": "routine", "include_transaction": "transaction", "include_mongo": "mongo", "include_redis": "redis", "include_docker": "docker", "include_docs": "docs", "include_scaffolder": "scaffolder"}

func parseProject(body []byte) (projectRequest, error) {
	out := projectRequest{Flags: map[string]bool{}}
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return out, fmt.Errorf("expected JSON object")
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return out, err
		}
		k := key.(string)
		if _, ok := fields[k]; ok {
			return out, fmt.Errorf("duplicate field %s", k)
		}
		var raw json.RawMessage
		if err = dec.Decode(&raw); err != nil {
			return out, err
		}
		if string(raw) == "null" {
			return out, fmt.Errorf("null field %s", k)
		}
		fields[k] = raw
	}
	if _, err = dec.Token(); err != nil {
		return out, err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return out, fmt.Errorf("expected one JSON object")
	}
	metadata := map[string]*string{"module": &out.Config.Module, "service_name": &out.Config.ServiceName, "project_version": &out.Config.ProjectVersion, "description": &out.Config.Description, "go_version": &out.Config.GoVersion, "protocol": &out.Config.Protocol, "http_port": &out.Config.HTTPPort, "grpc_port": &out.Config.GRPCPort, "plan_id": &out.ID}
	for k, v := range fields {
		if dst, ok := metadata[k]; ok {
			if err = json.Unmarshal(v, dst); err != nil {
				return out, fmt.Errorf("invalid %s", k)
			}
			continue
		}
		if k == "extensions" || k == "requested_extensions" {
			if out.Explicit {
				return out, fmt.Errorf("provide only one extensions field")
			}
			out.Explicit = true
			if err = json.Unmarshal(v, &out.Config.RequestedExtensions); err != nil {
				return out, fmt.Errorf("invalid extensions")
			}
			continue
		}
		if id, ok := legacyFlags[k]; ok {
			var enabled bool
			if err = json.Unmarshal(v, &enabled); err != nil {
				return out, fmt.Errorf("invalid boolean %s", k)
			}
			out.Flags[id] = enabled
			continue
		}
		return out, fmt.Errorf("unknown field %s", k)
	}
	if _, hasID := fields["plan_id"]; hasID {
		if len(fields) != 1 {
			return out, fmt.Errorf("plan_id cannot be combined with configuration")
		}
		if !validPlanID(out.ID) {
			return out, fmt.Errorf("invalid plan_id")
		}
		return out, nil
	}
	if err = protocol.Validate(out.Config.Protocol); err != nil {
		return out, err
	}
	for id, on := range out.Flags {
		if id == "mongo" || id == "redis" || id == "docker" || id == "docs" || id == "scaffolder" {
			if on {
				return out, fmt.Errorf("unsupported legacy extension %s", id)
			}
			continue
		}
		if !out.Explicit && on && id != "routine" && id != "transaction" {
			out.Config.RequestedExtensions = append(out.Config.RequestedExtensions, id)
		}
	}
	return out, nil
}
func validPlanID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'f' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func readProject(w http.ResponseWriter, r *http.Request) (projectRequest, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return projectRequest{}, fmt.Errorf("request exceeds limit or cannot be read")
	}
	return parseProject(body)
}
func queryProject(q url.Values) (projectRequest, error) {
	fields := map[string]any{}
	encoded := ""
	encodedSeen := false
	for k, values := range q {
		if len(values) != 1 {
			return projectRequest{}, fmt.Errorf("duplicate query field %s", k)
		}
		v := values[0]
		switch k {
		case "wait":
			continue
		case "config", "c", "payload", "data", "b64", "cfg":
			if encodedSeen {
				return projectRequest{}, fmt.Errorf("ambiguous encoded config")
			}
			encodedSeen = true
			encoded = v
			continue
		}
		canonical := k
		switch k {
		case "m":
			canonical = "module"
		case "name", "service":
			canonical = "service_name"
		case "go", "v":
			canonical = "go_version"
		case "api":
			canonical = "protocol"
		case "version":
			canonical = "project_version"
		}
		if _, ok := legacyFlags["include_"+k]; ok {
			canonical = "include_" + k
		}
		if _, ok := fields[canonical]; ok {
			return projectRequest{}, fmt.Errorf("conflicting query aliases")
		}
		if _, ok := legacyFlags[canonical]; ok {
			switch v {
			case "true", "1":
				fields[canonical] = true
			case "false", "0":
				fields[canonical] = false
			default:
				return projectRequest{}, fmt.Errorf("invalid boolean %s", k)
			}
		} else if canonical == "extensions" || canonical == "requested_extensions" {
			items := []string{}
			if v != "" {
				items = strings.Split(v, ",")
			}
			fields[canonical] = items
		} else {
			fields[canonical] = v
		}
	}
	if encodedSeen {
		if len(fields) > 0 || len(encoded) > 2<<20 {
			return projectRequest{}, fmt.Errorf("encoded config cannot be combined with query configuration")
		}
		raw, err := decodeConfigParam(encoded)
		if err != nil || len(raw) > 1<<20 {
			return projectRequest{}, fmt.Errorf("invalid encoded config")
		}
		return parseProject(raw)
	}
	raw, _ := json.Marshal(fields)
	return parseProject(raw)
}
