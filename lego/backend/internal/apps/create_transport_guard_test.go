/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/graphql-go/graphql"
	apiequality "k8s.io/apimachinery/pkg/api/equality"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestCreateWireFieldsReachTheRequest (w5/m117) guards the step each create
// transport takes before Create: REST's decodeCreateService, GraphQL's
// gqlCreateRequest and every MCP create tool's createRequest. The fields are
// enumerated from each transport's request type or schema, so a field is
// guarded the day it is added. On every shape a create takes (createShapes:
// web and cron × each build, and a static site), moving one field off a
// baseline value must change exactly the CreateRequest fields its rule names,
// answer 400, or change nothing where the rule names the field inert and says
// why. A field that silently changes nothing is how REST once dropped an
// image's dockerCommand (w4/188), and how MCP did again (w5/073).
func TestCreateWireFieldsReachTheRequest(t *testing.T) {
	svc, _ := newService(nil)
	shapes := createShapes()
	transports := []createTransport{
		restCreateTransport(svc, shapes),
		gqlCreateTransport(t, svc, shapes),
		mcpCreateTransport[createWebServiceArgs](t, "create_web_service", shapes, func(s createShape) bool { return !s.cron && !s.static }),
		mcpCreateTransport[createCronJobArgs](t, "create_cron_job", shapes, func(s createShape) bool { return s.cron }),
		mcpCreateTransport[createStaticSiteArgs](t, "create_static_site", shapes, func(s createShape) bool { return s.static }),
	}
	ruled := map[string]map[string]bool{} // rule table → the fields some transport sends it
	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) { tr.check(t) })
		if ruled[tr.table] == nil {
			ruled[tr.table] = map[string]bool{}
		}
		for _, leaf := range tr.leaves {
			ruled[tr.table][leaf.path] = true
		}
	}
	for table, rules := range createWireRuleTables {
		for path := range rules {
			if !ruled[table][path] {
				t.Errorf("%s has a rule for %s, which no transport accepts: remove it", table, path)
			}
		}
	}
}

// createWireRuleTables names each transport's rule table, so a rule no
// transport's field uses any more fails as stale.
var createWireRuleTables = map[string]map[string]wireRule{
	"restCreateRules": restCreateRules,
	"gqlCreateRules":  gqlCreateRules,
	"mcpCreateRules":  mcpCreateRules,
}

// restCreateRules is what each POST /v1/services field does.
var restCreateRules = map[string]wireRule{
	"ownerId":         to("OwnerID"),
	"type":            to("Type"),
	"schedule":        to("Schedule"),
	"command":         to("Command"),
	"name":            to("Name"),
	"repo":            to("Repo"),
	"image.imagePath": to("Image"),
	// Validation only: a value other than the effective owner is refused,
	// and it never selects a workspace of its own.
	"image.ownerId":                          refused("tea-a", "tea-b"),
	"image.registryCredentialId":             to("RegistryCredentialID").given("rc-a", "rc-b"),
	"branch":                                 to("Branch"),
	"environmentId":                          to("EnvironmentID"),
	"autoDeploy":                             to("AutoDeploy").given("yes", "no"),
	"autoDeployTrigger":                      to("AutoDeploy").given("commit", "off"),
	"notifyOnFail":                           to("NotifyOnFail"),
	"envVars[].key":                          to("Env"),
	"envVars[].value":                        to("Env"),
	"secretFiles[].name":                     to("SecretFiles"),
	"secretFiles[].content":                  to("SecretFiles"),
	"serviceDetails.plan":                    to("Plan"),
	"serviceDetails.region":                  inert("bex is single-region: the pinned Render schema validates region, and the service lands in the configured placement"),
	"serviceDetails.numInstances":            to("Replicas"),
	"serviceDetails.healthCheckPath":         to("HealthCheckPath"),
	"serviceDetails.maxShutdownDelaySeconds": to("MaxShutdownDelaySeconds").given(30, 45),
	"serviceDetails.runtime":                 to("Runtime"),
	"serviceDetails.env": {want: func(s createShape) wireOutcome {
		if s.namesRuntime() {
			return wireOutcome{inert: "Render's deprecated spelling of runtime, which wins when both are sent"}
		}
		return wireOutcome{fields: []string{"Runtime"}}
	}},
	"serviceDetails.envSpecificDetails.buildCommand": to("BuildCommand"),
	"serviceDetails.envSpecificDetails.startCommand": {want: func(s createShape) wireOutcome {
		return wireOutcome{fields: restCronBridge(s, "StartCommand")}
	}},
	"serviceDetails.envSpecificDetails.dockerCommand": {want: func(s createShape) wireOutcome {
		if !s.readsDockerCommand() {
			return wireOutcome{inert: notReadDockerCommand}
		}
		return wireOutcome{fields: restCronBridge(s, "StartCommand")}
	}},
	"serviceDetails.envSpecificDetails.dockerContext":        dockerfileBuildOnly("DockerContext"),
	"serviceDetails.envSpecificDetails.dockerfilePath":       dockerfileBuildOnly("DockerfilePath"),
	"serviceDetails.envSpecificDetails.registryCredentialId": to("RegistryCredentialID").given("rc-a", "rc-b"),
	"serviceDetails.schedule":                                to("Schedule"),
	"serviceDetails.command":                                 to("Command"),
	"serviceDetails.buildCommand":                            to("BuildCommand"),
	"serviceDetails.publishPath":                             to("PublishPath"),
	"serviceDetails.preDeployCommand":                        to("PreDeployCommand"),
	"serviceDetails.renderSubdomainPolicy":                   to("SubdomainPolicy"),
	"serviceDetails.previews":                                refused(map[string]any{"generation": "off"}, map[string]any{"generation": "automatic"}),
	"serviceDetails.maintenanceMode": to("MaintenanceMode").given(
		map[string]any{"enabled": true, "uri": "https://a.example.com"},
		map[string]any{"enabled": false, "uri": "https://a.example.com"},
		map[string]any{"enabled": true, "uri": "https://b.example.com"},
	),
	"serviceDetails.ipAllowList": to("IPAllowList").given(
		[]any{map[string]any{"cidrBlock": "10.0.0.0/8", "description": "a"}},
		[]any{map[string]any{"cidrBlock": "10.1.0.0/16", "description": "a"}},
		[]any{map[string]any{"cidrBlock": "10.0.0.0/8", "description": "b"}},
	),
	"serviceDetails.disk.name":      to("Disk"),
	"serviceDetails.disk.mountPath": to("Disk"),
	"serviceDetails.disk.sizeGB":    to("Disk"),
	"builder":                       to("Builder"),
	"rootDir":                       to("RootDir"),
	"buildFilter.paths":             to("BuildFilter"),
	"buildFilter.ignoredPaths":      to("BuildFilter"),
	"port":                          to("Port"),
	"plan":                          to("Plan"),
	"domains":                       to("Hosts"),
	"publishPath":                   to("PublishPath"),
	"preDeployCommand":              to("PreDeployCommand"),
	"routes[].type":                 to("Routes"),
	"routes[].source":               to("Routes"),
	"routes[].destination":          to("Routes"),
	"headers[].path":                to("Headers"),
	"headers[].name":                to("Headers"),
	"headers[].value":               to("Headers"),
	"renderSubdomainPolicy":         to("SubdomainPolicy"),
	"dryRun":                        to("DryRun"),
}

// restCronBridge is fields plus Command on a cron: REST stores a cron's start
// command as its run command when no command is sent, because the pinned CLI
// sends a cron's command as envSpecificDetails.startCommand on every runtime.
func restCronBridge(s createShape, fields ...string) []string {
	if s.cron {
		return append(fields, "Command")
	}
	return fields
}

// dockerfileBuildOnly is a Dockerfile-build setting: it applies when the
// create builds the repo's Dockerfile and changes nothing elsewhere.
func dockerfileBuildOnly(fields ...string) wireRule {
	return wireRule{want: func(s createShape) wireOutcome {
		if !s.buildsDockerfile() {
			return wireOutcome{inert: "a Dockerfile-build setting: a native runtime, buildpacks and a prebuilt image build no Dockerfile"}
		}
		return wireOutcome{fields: fields}
	}}
}

// notReadDockerCommand is why a dockerCommand is inert where the build does
// not read one.
const notReadDockerCommand = "a native runtime or buildpacks take the command elsewhere, so Render's docker command is inert there, and a call Render accepts keeps its native commands"

// gqlCreateRules is what each createService argument does: GraphQL maps every
// argument as given and leaves the build rules to the core.
var gqlCreateRules = map[string]wireRule{
	"name":                             to("Name"),
	"ownerId":                          to("OwnerID"),
	"environmentId":                    to("EnvironmentID"),
	"type":                             to("Type"),
	"schedule":                         to("Schedule"),
	"command":                          to("Command"),
	"repo":                             to("Repo"),
	"image":                            to("Image"),
	"registryCredentialId":             to("RegistryCredentialID"),
	"branch":                           to("Branch"),
	"rootDir":                          to("RootDir"),
	"buildFilter.paths":                to("BuildFilter"),
	"buildFilter.ignoredPaths":         to("BuildFilter"),
	"runtime":                          to("Runtime"),
	"buildCommand":                     to("BuildCommand"),
	"startCommand":                     to("StartCommand"),
	"dockerfilePath":                   to("DockerfilePath"),
	"builder":                          to("Builder"),
	"plan":                             to("Plan"),
	"autoDeploy":                       to("AutoDeploy"),
	"notifyOnFail":                     to("NotifyOnFail"),
	"port":                             to("Port"),
	"replicas":                         to("Replicas"),
	"envVars[].key":                    to("Env"),
	"envVars[].value":                  to("Env"),
	"envVars[].generateValue":          to("Env"),
	"secretFiles[].name":               to("SecretFiles"),
	"secretFiles[].content":            to("SecretFiles"),
	"publishPath":                      to("PublishPath"),
	"routes[].type":                    to("Routes"),
	"routes[].source":                  to("Routes"),
	"routes[].destination":             to("Routes"),
	"headers[].path":                   to("Headers"),
	"headers[].name":                   to("Headers"),
	"headers[].value":                  to("Headers"),
	"healthCheckPath":                  to("HealthCheckPath"),
	"maxShutdownDelaySeconds":          to("MaxShutdownDelaySeconds"),
	"preDeployCommand":                 to("PreDeployCommand"),
	"maintenanceMode.enabled":          to("MaintenanceMode"),
	"maintenanceMode.uri":              to("MaintenanceMode"),
	"ipAllowList":                      to("IPAllowList"),
	"ipAllowListEntries[].cidrBlock":   to("IPAllowList"),
	"ipAllowListEntries[].description": to("IPAllowList"),
	"dryRun":                           to("DryRun"),
}

// mcpCreateRules is what each create tool argument does; the three tools
// share their argument names, so they share the table.
var mcpCreateRules = map[string]wireRule{
	"name":                     to("Name"),
	"environmentId":            to("EnvironmentID"),
	"type":                     to("Type"),
	"schedule":                 to("Schedule"),
	"command":                  to("Command"),
	"repo":                     to("Repo"),
	"image":                    to("Image"),
	"registryCredentialId":     to("RegistryCredentialID"),
	"branch":                   to("Branch"),
	"rootDir":                  to("RootDir"),
	"buildFilter.paths":        to("BuildFilter"),
	"buildFilter.ignoredPaths": to("BuildFilter"),
	"runtime":                  to("Runtime"),
	"buildCommand":             to("BuildCommand"),
	// A prebuilt image's cron runs its start command when no command is sent,
	// REST's cron bridge for the image runtime.
	"startCommand": {want: func(s createShape) wireOutcome {
		if s.cron && s.runsImage() {
			return wireOutcome{fields: []string{"StartCommand", "Command"}}
		}
		return wireOutcome{fields: []string{"StartCommand"}}
	}},
	"dockerfilePath": dockerfileBuildOnly("DockerfilePath"),
	"dockerContext":  dockerfileBuildOnly("DockerContext"),
	"dockerCommand": {want: func(s createShape) wireOutcome {
		if !s.readsDockerCommand() {
			return wireOutcome{inert: notReadDockerCommand}
		}
		if s.cron {
			return wireOutcome{fields: []string{"StartCommand", "Command"}}
		}
		return wireOutcome{fields: []string{"StartCommand"}}
	}},
	"builder":                          to("Builder"),
	"plan":                             to("Plan"),
	"envVars[].key":                    to("Env"),
	"envVars[].value":                  to("Env"),
	"secretFiles[].name":               to("SecretFiles"),
	"secretFiles[].content":            to("SecretFiles"),
	"autoDeploy":                       to("AutoDeploy").given("yes", "no"),
	"notifyOnFail":                     to("NotifyOnFail"),
	"healthCheckPath":                  to("HealthCheckPath"),
	"maxShutdownDelaySeconds":          to("MaxShutdownDelaySeconds"),
	"preDeployCommand":                 to("PreDeployCommand"),
	"maintenanceMode.enabled":          to("MaintenanceMode"),
	"maintenanceMode.uri":              to("MaintenanceMode"),
	"port":                             to("Port"),
	"replicas":                         to("Replicas"),
	"dryRun":                           to("DryRun"),
	"ipAllowList":                      to("IPAllowList"),
	"ipAllowListEntries[].cidrBlock":   to("IPAllowList"),
	"ipAllowListEntries[].description": to("IPAllowList"),
	"publishPath":                      to("PublishPath"),
	"domains":                          to("Hosts"),
	"routes[].type":                    to("Routes"),
	"routes[].source":                  to("Routes"),
	"routes[].destination":             to("Routes"),
	"headers[].path":                   to("Headers"),
	"headers[].name":                   to("Headers"),
	"headers[].value":                  to("Headers"),
}

// createTransport is one create transport's wire-to-request step.
type createTransport struct {
	name   string
	table  string // its key in createWireRuleTables
	leaves []wireLeaf
	shapes []createShape
	decode func(in map[string]any) (CreateRequest, error)
}

// createShape is one kind of create a transport maps: a create that works,
// in the transport's own spelling.
type createShape struct {
	name         string // e.g. "cron/docker"
	build        string // a createBuildInputs build
	cron, static bool
	in           map[string]any
}

func (s createShape) namesRuntime() bool {
	return s.build == "docker" || s.build == "image" || s.build == "native"
}

func (s createShape) buildsDockerfile() bool {
	return s.build == "docker" || s.build == "dockerfile" || s.build == "auto"
}

func (s createShape) runsImage() bool {
	return s.build == "image" || s.build == "bare-image"
}

func (s createShape) readsDockerCommand() bool {
	return s.buildsDockerfile() || s.runsImage()
}

// wireLeaf is one field a transport accepts. A field inside a list element is
// spelled "list[].field".
type wireLeaf struct {
	path string
	kind wireKind
	// required are the sibling fields the leaf's object cannot be sent
	// without (GraphQL's non-null input fields); the guard fills them.
	required []wireLeaf
}

type wireKind int

const (
	kindString wireKind = iota
	kindInt
	kindBool
	kindStrings
	// kindRaw is a field the transport decodes by hand (json.RawMessage, a
	// json.Unmarshaler); its rule names the values to send.
	kindRaw
)

// sentinels is the kind's baseline and variant values.
func (k wireKind) sentinels() []any {
	switch k {
	case kindString:
		return []any{"sentinel-a", "sentinel-b"}
	case kindInt:
		return []any{1, 2}
	case kindBool:
		return []any{false, true}
	case kindStrings:
		return []any{[]any{"sentinel-a"}, []any{"sentinel-b"}}
	}
	return nil
}

// filler is the value a required sibling holds, the same in the baseline and
// every variant.
func (k wireKind) filler() any {
	switch k {
	case kindInt:
		return 7
	case kindBool:
		return true
	case kindStrings:
		return []any{"filler"}
	}
	return "filler"
}

// wireRule is what one field must do to the CreateRequest on a shape.
type wireRule struct {
	// values are the field's baseline (first) and variants (every later one);
	// nil means its kind's sentinels.
	values []any
	want   func(createShape) wireOutcome
}

type wireOutcome struct {
	fields  []string // the CreateRequest fields a variant changes
	refused bool     // a variant answers 400
	inert   string   // why the field changes nothing on this shape
}

// to is a field that reaches the named CreateRequest fields on every shape.
func to(fields ...string) wireRule {
	return wireRule{want: func(createShape) wireOutcome { return wireOutcome{fields: fields} }}
}

// refused is a field whose variants answer 400 on every shape.
func refused(values ...any) wireRule {
	return wireRule{values: values, want: func(createShape) wireOutcome { return wireOutcome{refused: true} }}
}

// inert is a field that changes nothing on any shape, and why.
func inert(why string) wireRule {
	return wireRule{want: func(createShape) wireOutcome { return wireOutcome{inert: why} }}
}

// given replaces the kind's sentinels with the rule's own values.
func (r wireRule) given(values ...any) wireRule {
	r.values = values
	return r
}

// check runs every field of the transport on every shape.
func (tr createTransport) check(t *testing.T) {
	t.Helper()
	rules := createWireRuleTables[tr.table]
	for _, leaf := range tr.leaves {
		if _, ok := rules[leaf.path]; !ok {
			t.Errorf("%s accepts %s, but %s has no rule for it: map it onto CreateRequest (or refuse it with a 400), then add its rule", tr.name, leaf.path, tr.table)
		}
	}
	for _, shape := range tr.shapes {
		req, err := tr.decode(cloneWire(shape.in))
		if err == nil {
			_, err = specFromCreate(req)
		}
		if err != nil {
			t.Errorf("%s %s: the shape itself is refused: %v", tr.name, shape.name, err)
			continue
		}
		for _, leaf := range tr.leaves {
			if rule, ok := rules[leaf.path]; ok {
				tr.checkLeaf(t, shape, leaf, rule)
			}
		}
	}
}

// checkLeaf moves one field off its baseline on one shape and compares the
// request against the field's rule.
func (tr createTransport) checkLeaf(t *testing.T, shape createShape, leaf wireLeaf, rule wireRule) {
	t.Helper()
	values := rule.values
	if values == nil {
		values = leaf.kind.sentinels()
	}
	if len(values) < 2 {
		t.Errorf("%s: the rule for %s must give a baseline and at least one variant value", tr.name, leaf.path)
		return
	}
	with := func(v any) (CreateRequest, error) {
		in := cloneWire(shape.in)
		for _, sibling := range leaf.required {
			if sibling.path != leaf.path {
				setWire(in, sibling.path, sibling.kind.filler())
			}
		}
		setWire(in, leaf.path, v)
		return tr.decode(in)
	}
	want := rule.want(shape)
	base, baseErr := with(values[0])
	for _, v := range values[1:] {
		got, err := with(v)
		where := fmt.Sprintf("%s %s: %s %s → %s", tr.name, shape.name, leaf.path, wireJSON(values[0]), wireJSON(v))
		switch {
		case want.refused:
			if !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("%s answered %v, want a 400", where, err)
			}
		case baseErr != nil || err != nil:
			t.Errorf("%s is refused (baseline: %v, variant: %v)", where, baseErr, err)
		case want.inert != "":
			if changed := changedFields(base, got); len(changed) > 0 {
				t.Errorf("%s changes %v, but its rule calls it inert here (%s)", where, changed, want.inert)
			}
		default:
			changed := changedFields(base, got)
			wantFields := slices.Sorted(slices.Values(want.fields))
			switch {
			case len(changed) == 0:
				t.Errorf("%s changes nothing: the transport drops it (want %v changed, or a 400)", where, wantFields)
			case !slices.Equal(changed, wantFields):
				t.Errorf("%s changes %v, want %v", where, changed, wantFields)
			}
		}
	}
}

// changedFields names the exported fields of two structs that disagree,
// sorted. A nil and an empty list agree, as they do once Kubernetes stores
// them.
func changedFields[T any](a, b T) []string {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	var out []string
	for i := range va.NumField() {
		if f := va.Type().Field(i); f.IsExported() && !apiequality.Semantic.DeepEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
			out = append(out, f.Name)
		}
	}
	slices.Sort(out)
	return out
}

// restCreateTransport drives POST /v1/services' body through
// decodeCreateService, strict decoding included.
func restCreateTransport(svc *Service, shapes []createShape) createTransport {
	leaves := jsonWireLeaves(reflect.TypeFor[createServiceRequest](), "", false)
	return createTransport{
		name:   "REST POST /v1/services",
		table:  "restCreateRules",
		leaves: leaves,
		shapes: spellShapes(shapes, leaves, restSpelling),
		decode: func(in map[string]any) (CreateRequest, error) {
			body, err := json.Marshal(in)
			if err != nil {
				return CreateRequest{}, err
			}
			r := httptest.NewRequestWithContext(core.WithStrictJSONDecoding(context.Background()), http.MethodPost, "/v1/services", bytes.NewReader(body))
			return svc.decodeCreateService(r)
		},
	}
}

// gqlCreateTransport drives createService's arguments through graphql-go's own
// coercion: a schema with the real arguments whose resolver keeps what
// gqlCreateRequest maps.
func gqlCreateTransport(t *testing.T, svc *Service, shapes []createShape) createTransport {
	t.Helper()
	field := svc.GraphQLMutation()["createService"]
	var mapped struct {
		req CreateRequest
		err error
		ran bool
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: graphql.Fields{
			"createService": &graphql.Field{Type: graphql.Boolean, Args: field.Args, Resolve: func(p graphql.ResolveParams) (any, error) {
				mapped.req, mapped.err = gqlCreateRequest(p.Args)
				mapped.ran = true
				return true, nil
			}},
		}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	leaves := gqlWireLeaves(field.Args)
	return createTransport{
		name:   "GraphQL createService",
		table:  "gqlCreateRules",
		leaves: leaves,
		shapes: spellShapes(shapes, leaves, nil),
		decode: func(in map[string]any) (CreateRequest, error) {
			mapped.ran = false
			res := graphql.Do(graphql.Params{Schema: schema, RequestString: "mutation { createService(" + gqlArgs(in) + ") }"})
			if !mapped.ran {
				return CreateRequest{}, fmt.Errorf("graphql-go refused the arguments: %v", res.Errors)
			}
			return mapped.req, mapped.err
		},
	}
}

// mcpCreateTransport drives one create tool's JSON arguments through the
// input schema the SDK checks them against, then the tool's createRequest.
func mcpCreateTransport[A interface {
	createRequest(context.Context) (CreateRequest, error)
}](t *testing.T, tool string, shapes []createShape, takes func(createShape) bool) createTransport {
	t.Helper()
	schema, err := jsonschema.For[A](&jsonschema.ForOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	leaves := jsonWireLeaves(reflect.TypeFor[A](), "", true)
	var mine []createShape
	for _, shape := range shapes {
		if takes(shape) {
			mine = append(mine, shape)
		}
	}
	return createTransport{
		name:   "MCP " + tool,
		table:  "mcpCreateRules",
		leaves: leaves,
		shapes: spellShapes(mine, leaves, nil),
		decode: func(in map[string]any) (CreateRequest, error) {
			raw, err := json.Marshal(in)
			if err != nil {
				return CreateRequest{}, err
			}
			var instance map[string]any
			if err := json.Unmarshal(raw, &instance); err != nil {
				return CreateRequest{}, err
			}
			if err := input.Validate(instance); err != nil {
				return CreateRequest{}, fmt.Errorf("%w: the tool's input schema refuses the arguments: %v", core.ErrBadRequest, err)
			}
			var args A
			if err := json.Unmarshal(raw, &args); err != nil {
				return CreateRequest{}, err
			}
			return args.createRequest(context.Background())
		},
	}
}

const (
	shapeRepo  = "https://github.com/acme/app"
	shapeImage = "nginx:1.27"
)

// createBuildInputs are the builds a create selects, in the flat spelling
// GraphQL and MCP use: Render's runtimes, a prebuilt image with and without
// its runtime, and bex's builders with no runtime, which MCP's required
// runtime argument then sends empty.
var createBuildInputs = []struct {
	build string
	in    map[string]any
}{
	{"docker", map[string]any{"repo": shapeRepo, "runtime": "docker"}},
	{"image", map[string]any{"image": shapeImage, "runtime": "image"}},
	{"bare-image", map[string]any{"image": shapeImage, "runtime": ""}},
	{"native", map[string]any{"repo": shapeRepo, "runtime": "node", "buildCommand": "npm ci", "startCommand": "npm start"}},
	{"dockerfile", map[string]any{"repo": shapeRepo, "runtime": "", "builder": "dockerfile"}},
	{"buildpack", map[string]any{"repo": shapeRepo, "runtime": "", "builder": "buildpack"}},
	{"auto", map[string]any{"repo": shapeRepo, "runtime": ""}},
}

// createShapes is every create shape, flat: web and cron × each build, and a
// static site.
func createShapes() []createShape {
	var shapes []createShape
	for _, cron := range []bool{false, true} {
		for _, b := range createBuildInputs {
			in := cloneWire(b.in)
			in["name"] = "probe"
			name := "web/" + b.build
			if cron {
				in["type"], in["schedule"], name = "cron_job", "*/5 * * * *", "cron/"+b.build
			}
			shapes = append(shapes, createShape{name: name, build: b.build, cron: cron, in: in})
		}
	}
	return append(shapes, createShape{name: "static", build: "auto", static: true, in: map[string]any{
		"name": "probe", "type": "static_site", "repo": shapeRepo, "publishPath": "dist",
	}})
}

// spellShapes spells shapes for one transport: through spell when it nests
// fields, keeping only the fields the transport accepts.
func spellShapes(shapes []createShape, leaves []wireLeaf, spell func(map[string]any) map[string]any) []createShape {
	accepted := map[string]bool{}
	for _, leaf := range leaves {
		top, _, _ := strings.Cut(leaf.path, ".")
		accepted[strings.TrimSuffix(top, "[]")] = true
	}
	out := make([]createShape, len(shapes))
	for i, shape := range shapes {
		in := cloneWire(shape.in)
		if spell != nil {
			in = spell(in)
		}
		for key := range in {
			if !accepted[key] {
				delete(in, key)
			}
		}
		shape.in = in
		out[i] = shape
	}
	return out
}

// restSpelling nests a flat create the way a Render client sends it: the
// runtime, a cron's schedule and a static site's publish path under
// serviceDetails, native commands in its envSpecificDetails, and the image as
// an object.
func restSpelling(flat map[string]any) map[string]any {
	in, details, native := map[string]any{}, map[string]any{}, map[string]any{}
	for key, value := range flat {
		switch key {
		case "runtime", "schedule", "publishPath":
			details[key] = value
		case "buildCommand", "startCommand":
			native[key] = value
		case "image":
			in[key] = map[string]any{"imagePath": value}
		default:
			in[key] = value
		}
	}
	if len(native) > 0 {
		details["envSpecificDetails"] = native
	}
	if len(details) > 0 {
		in["serviceDetails"] = details
	}
	return in
}

var jsonUnmarshalerType = reflect.TypeFor[json.Unmarshaler]()

// jsonWireLeaves enumerates the JSON fields a request type accepts. With
// schema, a nested object's fields with no omitempty are its required fields,
// as the input schema the MCP SDK infers has them.
func jsonWireLeaves(typ reflect.Type, prefix string, schema bool) []wireLeaf {
	var out, required []wireLeaf
	var own []int // this object's own fields among out
	for i := range typ.NumField() {
		f := typ.Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if !f.IsExported() || name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		path, ft := prefix+name, f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		leaf := wireLeaf{path: path}
		switch {
		case reflect.PointerTo(ft).Implements(jsonUnmarshalerType):
			leaf.kind = kindRaw
		case ft.Kind() == reflect.Struct:
			out = append(out, jsonWireLeaves(ft, path+".", schema)...)
			continue
		case ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct:
			out = append(out, jsonWireLeaves(ft.Elem(), path+"[].", schema)...)
			continue
		case ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.String:
			leaf.kind = kindStrings
		case ft.Kind() == reflect.String:
			leaf.kind = kindString
		case ft.Kind() == reflect.Bool:
			leaf.kind = kindBool
		case ft.Kind() >= reflect.Int && ft.Kind() <= reflect.Int64:
			leaf.kind = kindInt
		default:
			panic(fmt.Sprintf("jsonWireLeaves: %s has unsupported type %s", path, f.Type))
		}
		own = append(own, len(out))
		out = append(out, leaf)
		if schema && prefix != "" && !strings.Contains(opts, "omitempty") {
			required = append(required, leaf)
		}
	}
	for _, i := range own {
		out[i].required = required
	}
	return out
}

// gqlWireLeaves enumerates the arguments a GraphQL field accepts.
func gqlWireLeaves(args graphql.FieldConfigArgument) []wireLeaf {
	var out []wireLeaf
	for name, arg := range args {
		out = append(out, gqlTypeLeaves(name, arg.Type)...)
	}
	return out
}

func gqlTypeLeaves(path string, typ graphql.Type) []wireLeaf {
	if nonNull, ok := typ.(*graphql.NonNull); ok {
		typ = nonNull.OfType
	}
	switch tt := typ.(type) {
	case *graphql.List:
		elem := tt.OfType
		if nonNull, ok := elem.(*graphql.NonNull); ok {
			elem = nonNull.OfType
		}
		if obj, ok := elem.(*graphql.InputObject); ok {
			return gqlObjectLeaves(path+"[].", obj)
		}
		return []wireLeaf{{path: path, kind: kindStrings}}
	case *graphql.InputObject:
		return gqlObjectLeaves(path+".", tt)
	case *graphql.Scalar:
		switch tt.Name() {
		case "Int":
			return []wireLeaf{{path: path, kind: kindInt}}
		case "Boolean":
			return []wireLeaf{{path: path, kind: kindBool}}
		case "String", "ID":
			return []wireLeaf{{path: path, kind: kindString}}
		}
	}
	panic(fmt.Sprintf("gqlTypeLeaves: %s has unsupported type %s", path, typ))
}

func gqlObjectLeaves(prefix string, obj *graphql.InputObject) []wireLeaf {
	var leaves, required []wireLeaf
	for name, field := range obj.Fields() {
		fieldLeaves := gqlTypeLeaves(prefix+name, field.Type)
		if _, ok := field.Type.(*graphql.NonNull); ok {
			required = append(required, fieldLeaves...)
		}
		leaves = append(leaves, fieldLeaves...)
	}
	for i := range leaves {
		leaves[i].required = required
	}
	return leaves
}

// gqlArgs renders a wire object as GraphQL argument literals.
func gqlArgs(in map[string]any) string {
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(in)) {
		parts = append(parts, k+": "+gqlLiteral(in[k]))
	}
	return strings.Join(parts, ", ")
}

func gqlLiteral(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = gqlLiteral(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return "{" + gqlArgs(x) + "}"
	}
	panic(fmt.Sprintf("gqlLiteral: unsupported %T", v))
}

// setWire sets path in a wire object, creating the objects and the
// one-element lists on its way.
func setWire(m map[string]any, path string, v any) {
	head, rest, nested := strings.Cut(path, ".")
	if !nested {
		m[head] = v
		return
	}
	if list, ok := strings.CutSuffix(head, "[]"); ok {
		elems, _ := m[list].([]any)
		if len(elems) == 0 {
			elems = []any{map[string]any{}}
			m[list] = elems
		}
		setWire(elems[0].(map[string]any), rest, v)
		return
	}
	child, _ := m[head].(map[string]any)
	if child == nil {
		child = map[string]any{}
		m[head] = child
	}
	setWire(child, rest, v)
}

// cloneWire deep-copies a wire value.
func cloneWire[T any](v T) T {
	var out any
	switch x := any(v).(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = cloneWire(e)
		}
		out = m
	case []any:
		s := make([]any, len(x))
		for i, e := range x {
			s[i] = cloneWire(e)
		}
		out = s
	default:
		return v
	}
	return out.(T)
}

func wireJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
