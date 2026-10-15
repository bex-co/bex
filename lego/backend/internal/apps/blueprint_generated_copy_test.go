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
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/m175: a fromService.envVarKey copy of a sibling's
// generateValue, the pattern Render's own ghost, wordpress and mattermost
// examples use for a database password, used to refuse the whole Blueprint.

// generatedCopyManifest declares the consumer first, so nothing can depend on
// the source having been applied earlier in the file.
const generatedCopyManifest = `
services:
  - name: consumer
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: DB_PASSWORD
        fromService: {type: pserv, name: database, envVarKey: PASSWORD}
  - name: database
    type: pserv
    runtime: image
    image: {url: mysql:8}
    envVars:
      - {key: PASSWORD, generateValue: true}
`

// copyLog records seeds and copies in one sequence, so a test can check that
// a copy runs after its source was minted.
type copyLog struct{ calls []string }

type loggingSeeder struct{ log *copyLog }

func (s loggingSeeder) SeedEnvVars(_ context.Context, service string, _ map[string]string, generates []string) error {
	s.log.calls = append(s.log.calls, "seed "+service+" "+strings.Join(generates, ","))
	return nil
}

type loggingCopier struct {
	log *copyLog
	err error
}

func (c loggingCopier) CopyEnvVars(_ context.Context, to string, copies []core.EnvVarCopy) error {
	for _, cp := range copies {
		c.log.calls = append(c.log.calls, "copy "+cp.From+"."+cp.FromKey+" -> "+to+"."+cp.ToKey)
	}
	return c.err
}

func parsedStackService(t *testing.T, manifest, name string) parsedService {
	t.Helper()
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, svc := range st.services {
		if svc.req.Name == name {
			return svc
		}
	}
	t.Fatalf("%s not parsed", name)
	return parsedService{}
}

func parseError(t *testing.T, manifest string) error {
	t.Helper()
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	_, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	return err
}

func hasEnv(env []appv1alpha1.EnvVar, name string) bool {
	return slices.ContainsFunc(env, func(e appv1alpha1.EnvVar) bool { return e.Name == name })
}

func TestGeneratedCopyIsAnApplyTimeCopyNeverASpecLiteral(t *testing.T) {
	consumer := parsedStackService(t, generatedCopyManifest, "consumer")
	want := []generatedCopy{{key: "DB_PASSWORD", source: "database", sourceKey: "PASSWORD"}}
	if !slices.Equal(consumer.generatedCopies, want) {
		t.Fatalf("generatedCopies = %+v, want %+v", consumer.generatedCopies, want)
	}
	if hasEnv(consumer.req.Env, "DB_PASSWORD") {
		t.Fatalf("the copy reached spec.Env (%+v): a secret must never ride the App spec", consumer.req.Env)
	}
}

// The source's own generated value wins over a linked group's literal for the
// same key, so the copy takes the generated value at apply.
func TestGeneratedCopyFollowsTheSourcesOwnStoreOverItsGroup(t *testing.T) {
	manifest := `
envVarGroups:
  - name: g
    envVars:
      - {key: PASSWORD, value: from-group}
` + strings.Replace(generatedCopyManifest, "      - {key: PASSWORD, generateValue: true}", "      - {fromGroup: g}\n      - {key: PASSWORD, generateValue: true}", 1)
	consumer := parsedStackService(t, manifest, "consumer")
	if len(consumer.generatedCopies) != 1 || hasEnv(consumer.req.Env, "DB_PASSWORD") {
		t.Fatalf("consumer = copies %+v env %+v, want one apply-time copy", consumer.generatedCopies, consumer.req.Env)
	}
}

// A literal for the same key wins over the generated value at runtime, so the
// copy takes that literal at parse, as any known value is copied.
func TestGeneratedCopyTakesALiteralThatOverridesTheGeneratedKey(t *testing.T) {
	manifest := strings.Replace(generatedCopyManifest, "{key: PASSWORD, generateValue: true}", "{key: PASSWORD, generateValue: true}\n      - {key: PASSWORD, value: fixed}", 1)
	consumer := parsedStackService(t, manifest, "consumer")
	if len(consumer.generatedCopies) != 0 || !slices.ContainsFunc(consumer.req.Env, func(e appv1alpha1.EnvVar) bool { return e.Name == "DB_PASSWORD" && e.Value == "fixed" }) {
		t.Fatalf("consumer = copies %+v env %+v, want the literal copied", consumer.generatedCopies, consumer.req.Env)
	}
}

func TestGeneratedCopyKeepsRefusingWhatItCannotCopy(t *testing.T) {
	for _, tc := range []struct{ name, manifest, want string }{
		{"an unanswered prompt", strings.Replace(generatedCopyManifest, "{key: PASSWORD, generateValue: true}", "{key: PASSWORD, sync: false}", 1), "no such plainly-defined variable"},
		{"a copy of a copy", generatedCopyManifest + `  - name: third
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: AGAIN
        fromService: {type: web, name: consumer, envVarKey: DB_PASSWORD}
`, "no such plainly-defined variable"},
		{"the consumer also declares the key", strings.Replace(generatedCopyManifest, "        fromService: {type: pserv, name: database, envVarKey: PASSWORD}", "        fromService: {type: pserv, name: database, envVarKey: PASSWORD}\n      - {key: DB_PASSWORD, value: shadow}", 1), "must be the only declaration of DB_PASSWORD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := parseError(t, tc.manifest); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("parse = %v, want a located refusal containing %q", err, tc.want)
			}
		})
	}
}

func TestDeployStackCopiesTheGeneratedValueAfterTheSourceIsMinted(t *testing.T) {
	log := &copyLog{}
	svc, _ := newBlueprintEnvService(nil, loggingSeeder{log})
	svc.EnvCopier = loggingCopier{log: log}

	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: generatedCopyManifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	want := []string{"seed database PASSWORD", "copy database.PASSWORD -> consumer.DB_PASSWORD"}
	if !slices.Equal(log.calls, want) {
		t.Fatalf("calls = %q, want %q (the copy must follow the source's mint, whatever the file order)", log.calls, want)
	}
}

func TestDeployStackSurfacesACopyFailure(t *testing.T) {
	log := &copyLog{}
	svc, _ := newBlueprintEnvService(nil, loggingSeeder{log})
	unavailable := errors.New("store unavailable")
	svc.EnvCopier = loggingCopier{log: log, err: unavailable}

	_, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: generatedCopyManifest})
	if !errors.Is(err, unavailable) || !strings.Contains(err.Error(), `copying generated values into "consumer"`) {
		t.Fatalf("err = %v, want the copier's error, located", err)
	}
}

func TestDeployStackRefusesAGeneratedCopyWithoutTheStoreBeforeAnyWrite(t *testing.T) {
	svc, cl := newBlueprintEnvService(nil, &fakeSeeder{})
	_, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: generatedCopyManifest})
	if err == nil || !strings.Contains(err.Error(), "copies a generated value") {
		t.Fatalf("err = %v, want the unavailable-store refusal", err)
	}
	assertNoApps(t, cl)
}

// Copying reads the source's value, so a caller who may not reveal it is
// refused before any service is written, not after every one is.
func TestDeployStackRefusesACopyWithoutSensitiveAccessBeforeAnyWrite(t *testing.T) {
	log := &copyLog{}
	svc, cl := newBlueprintEnvService(nil, loggingSeeder{log})
	svc.EnvCopier = loggingCopier{log: log}
	writeOnly := core.WithIdentity(context.Background(), core.Identity{
		Subject: "writer", Method: "oauth2", Human: true,
		CanonicalScopes: core.ScopeRead + " " + core.ScopeWrite,
	})
	if _, err := svc.DeployStack(writeOnly, DeployRequest{Manifest: generatedCopyManifest}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("write-only copy: %v, want ErrForbidden", err)
	}
	assertNoApps(t, cl)
	if len(log.calls) != 0 {
		t.Fatalf("calls before the refusal: %q", log.calls)
	}
}

func TestBlueprintPlanNamesTheGeneratedCopyWithoutItsValue(t *testing.T) {
	ctx := context.Background()
	log := &copyLog{}
	svc, _ := newBlueprintEnvService(nil, loggingSeeder{log})
	svc.EnvCopier = loggingCopier{log: log}
	const wantMessage = "copies at apply: DB_PASSWORD from database's generated PASSWORD"

	first, err := svc.ValidateBlueprint(ctx, "", generatedCopyManifest, "")
	if err != nil || !first.Valid || first.Plan == nil {
		t.Fatalf("first plan = %+v, %v", first, err)
	}
	consumer := planAction(t, first.Plan.Actions, "consumer")
	if consumer.Operation != BlueprintPlanCreate || consumer.Message != wantMessage {
		t.Fatalf("create plan = %+v", consumer)
	}

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: generatedCopyManifest}); err != nil {
		t.Fatal(err)
	}
	again, err := svc.ValidateBlueprint(ctx, "", generatedCopyManifest, "")
	if err != nil || !again.Valid || again.Plan == nil {
		t.Fatalf("re-sync plan = %+v, %v", again, err)
	}
	consumer = planAction(t, again.Plan.Actions, "consumer")
	if consumer.Operation != BlueprintPlanUpdate || len(consumer.ChangedFields) != 1 || consumer.ChangedFields[0].Path != "envVars" || consumer.Message != wantMessage {
		t.Fatalf("re-sync plan = %+v, want a possible envVars update naming the copy", consumer)
	}
}

// tickingClock advances one second per read, so any restart a write records
// shows up as a new restartedAt.
func tickingClock() func() time.Time {
	var n atomic.Int64
	return func() time.Time { return time.Unix(1_000_000+n.Add(1), 0).UTC() }
}

func newCopyEnvService() (*Service, *secrets.Service) {
	base := &core.Base{Client: fakeClient(), Namespace: "default", Clock: tickingClock()}
	sec := &secrets.Service{Base: base, Store: newMemKV()}
	return &Service{Base: base, EnvSeeder: sec, EnvCopier: secrets.NewEnvCopier(sec)}, sec
}

// The same through the real env-vars service and store the apply writes to.
func TestGeneratedCopyEndToEndThroughTheEnvStore(t *testing.T) {
	svc, sec := newCopyEnvService()
	ctx := context.Background()

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: generatedCopyManifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	minted, err := sec.GetEnvVar(ctx, "database", "PASSWORD")
	if err != nil || len(minted.Value) != 44 {
		t.Fatalf("database PASSWORD = %+v err=%v, want a 44-char generated value", minted, err)
	}
	copied, err := sec.GetEnvVar(ctx, "consumer", "DB_PASSWORD")
	if err != nil || copied.Value != minted.Value {
		t.Fatalf("consumer DB_PASSWORD = %+v err=%v, want the database's value", copied, err)
	}
	if hasEnv(getApp(t, svc.Client, "consumer").Spec.Env, "DB_PASSWORD") {
		t.Fatal("the copied secret is on the consumer's spec.Env")
	}

	// An unchanged re-sync re-mints nothing and restarts nothing.
	restarted := getApp(t, svc.Client, "consumer").Spec.RestartedAt
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: generatedCopyManifest}); err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	if again, _ := sec.GetEnvVar(ctx, "database", "PASSWORD"); again.Value != minted.Value {
		t.Fatalf("re-sync re-minted the source: %q -> %q", minted.Value, again.Value)
	}
	if got := getApp(t, svc.Client, "consumer").Spec.RestartedAt; got != restarted {
		t.Fatalf("an unchanged re-sync restarted the consumer: %q -> %q", restarted, got)
	}

	// A source edited outside the Blueprint is re-copied on the next sync.
	if _, err := sec.SetEnvVar(ctx, "database", "PASSWORD", secrets.EnvVarWrite{Value: "rotated"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: generatedCopyManifest}); err != nil {
		t.Fatalf("sync after rotation: %v", err)
	}
	if got, _ := sec.GetEnvVar(ctx, "consumer", "DB_PASSWORD"); got.Value != "rotated" {
		t.Fatalf("consumer DB_PASSWORD after the source changed = %q, want rotated", got.Value)
	}
}

// The consumer used to declare the key itself as a literal and now copies it.
// That literal would stay on spec.Env (an undeclared literal is kept), where
// the container env would shadow the copy and the copy's write would be
// refused as manifest-owned, so the apply drops it.
func TestGeneratedCopyReplacesTheConsumersEarlierLiteral(t *testing.T) {
	svc, sec := newCopyEnvService()
	ctx := context.Background()

	literal := strings.Replace(generatedCopyManifest, "      - key: DB_PASSWORD\n        fromService: {type: pserv, name: database, envVarKey: PASSWORD}", "      - {key: DB_PASSWORD, value: old-literal}", 1)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: literal}); err != nil {
		t.Fatalf("literal apply: %v", err)
	}
	if !hasEnv(getApp(t, svc.Client, "consumer").Spec.Env, "DB_PASSWORD") {
		t.Fatal("setup: the consumer's literal should be on spec.Env")
	}

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: generatedCopyManifest}); err != nil {
		t.Fatalf("copy apply over the literal: %v", err)
	}
	if hasEnv(getApp(t, svc.Client, "consumer").Spec.Env, "DB_PASSWORD") {
		t.Fatal("the earlier literal is still on spec.Env, shadowing the copy")
	}
	minted, _ := sec.GetEnvVar(ctx, "database", "PASSWORD")
	if got, err := sec.GetEnvVar(ctx, "consumer", "DB_PASSWORD"); err != nil || len(minted.Value) != 44 || got.Value != minted.Value {
		t.Fatalf("consumer DB_PASSWORD = %+v err=%v, want the generated value %q", got, err, minted.Value)
	}
}

func planAction(t *testing.T, actions []BlueprintPlanAction, name string) BlueprintPlanAction {
	t.Helper()
	for _, a := range actions {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no plan action for %q in %+v", name, actions)
	return BlueprintPlanAction{}
}
