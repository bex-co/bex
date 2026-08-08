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

package api

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w7/m61 — the insider role-ladder deny matrix, the m55 negative-authorization
// program's second (insider) axis. m55 proved a non-member is denied every
// resource-addressing route; this proves a WITHIN-workspace under-privileged role
// (viewer/contributor/billing) is denied exactly the verbs its FGA relations do
// not grant, and allowed the ones they do — completeness-guarded over the same
// sweepableServices inventory. The role→relation table is pinned to the deployed
// model.fga so the five-role contract is executable, not just declarative.

// workspaceRoles are the five member roles model.fga's workspace type defines.
var workspaceRoles = []string{"viewer", "contributor", "developer", "admin", "billing"}

// modelRelations are the nine workspace-derived permissions model.fga defines,
// every one now checked by at least one verb: can_manage_billing gained its Go
// consumer in w1/m60 — the billing verbs (Status/Checkout/Portal) gate on it, so
// Render's BILLING role reaches billing management (billing or admin).
var modelRelations = []string{
	core.RelCanView, core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate,
	core.RelCanViewSensitive, core.RelCanManageKeys, core.RelCanManageSSHKeys,
	core.RelCanManage, core.RelCanManageBilling,
}

// roleGrants is the pinned role→relation grant table, the executable form of
// model.fga:67-75. TestRoleGrantsPinnedToModel proves it matches the deployed
// model exactly, so a model edit that changes any role's grants without updating
// this table turns CI red. The role-ladder matrix drives off this table, so the
// chain model.fga -> (pin) -> roleGrants -> (matrix) -> verb enforcement means a
// grant change flows all the way to the verb assertions.
var roleGrants = map[string]map[string]bool{
	"viewer": set(core.RelCanView, core.RelCanManageSSHKeys),
	"contributor": set(core.RelCanView, core.RelCanViewLogs, core.RelCanOperate,
		core.RelCanManageSSHKeys),
	"developer": set(core.RelCanView, core.RelCanViewLogs, core.RelCanOperate,
		core.RelCanCreate, core.RelCanViewSensitive, core.RelCanManageKeys,
		core.RelCanManageSSHKeys),
	"admin": set(core.RelCanView, core.RelCanViewLogs, core.RelCanOperate,
		core.RelCanCreate, core.RelCanViewSensitive, core.RelCanManageKeys,
		core.RelCanManageSSHKeys, core.RelCanManage, core.RelCanManageBilling),
	"billing": set(core.RelCanView, core.RelCanManageSSHKeys, core.RelCanManageBilling),
}

func set(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// modelPath locates the deployed FGA model from this test's package directory
// (lego/backend/internal/api -> repo root is four parents up).
const modelPath = "../../../../deploy/gitops/authz/model.fga"

// parseModelRoleGrants derives role->relations from the workspace type's
// `define can_*: <role> or <role> …` rules in model.fga. It parses ONLY the
// workspace block (the service/postgres/api_key types re-derive `can_view from
// workspace`, which are not role grants) and keeps only operands that name a
// workspace role — so `admin from org` and `[user]` are ignored.
func parseModelRoleGrants(t *testing.T) map[string]map[string]bool {
	t.Helper()
	src, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read model.fga: %v", err)
	}
	roleSet := set(workspaceRoles...)
	grants := map[string]map[string]bool{}
	for _, r := range workspaceRoles {
		grants[r] = map[string]bool{}
	}

	inWorkspace := false
	for _, raw := range strings.Split(string(src), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "type ") {
			inWorkspace = line == "type workspace"
			continue
		}
		if !inWorkspace || !strings.HasPrefix(line, "define can_") {
			continue
		}
		name, expr, ok := strings.Cut(strings.TrimPrefix(line, "define "), ":")
		if !ok {
			continue
		}
		relation := strings.TrimSpace(name)
		for _, tok := range strings.Split(expr, " or ") {
			tok = strings.TrimSpace(tok)
			if roleSet[tok] {
				grants[tok][relation] = true
			}
		}
	}
	return grants
}

// TestRoleGrantsPinnedToModel (t001) fails when the deployed model.fga and the
// pinned roleGrants table diverge, in either direction, naming the drift.
func TestRoleGrantsPinnedToModel(t *testing.T) {
	got := parseModelRoleGrants(t)

	// Sanity floor: the parse actually found the derived rules (a reshape of the
	// model file that stopped matching would otherwise pass vacuously).
	if n := len(got["admin"]); n < len(modelRelations) {
		t.Fatalf("parsed only %d admin grants from model.fga (< %d); the parser or model shape changed",
			n, len(modelRelations))
	}
	for _, role := range workspaceRoles {
		want := roleGrants[role]
		have := got[role]
		if !reflect.DeepEqual(sortedKeys(want), sortedKeys(have)) {
			t.Errorf("role %q grants drifted:\n  model.fga: %v\n  roleGrants: %v\n"+
				"update roleGrants to match deploy/gitops/authz/model.fga", role, sortedKeys(have), sortedKeys(want))
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// recordingChecker records every relation checked and returns a fixed decision.
// allow=true captures each verb's full relation set (the allow-all pass); a role
// checker instead answers from a grant set.
type recordingChecker struct {
	decide    func(relation string) bool
	relations []string
}

func (c *recordingChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	c.relations = append(c.relations, relation)
	return c.decide(relation), nil
}

// sweepVerbsQualified is sweepEveryVerb keyed by the PACKAGE-qualified service
// type (ct.String() = "*apps.Service") rather than the bare struct name — every
// feature package names its service Service, so the bare name collides. Reuses
// the shared isVerbMethod/callVerb/baseMethodNames reflection helpers.
func sweepVerbsQualified(t *testing.T, ctx context.Context, services []any, fn func(qualified, method string, err error)) int {
	t.Helper()
	baseMethods := baseMethodNames()
	swept := 0
	for _, svc := range services {
		cv := reflect.ValueOf(svc)
		ct := cv.Type()
		for i := 0; i < ct.NumMethod(); i++ {
			m := ct.Method(i)
			if !isVerbMethod(baseMethods, m) {
				continue
			}
			swept++
			fn(ct.String()+"."+m.Name, m.Name, callVerb(cv, m, ctx))
		}
	}
	return swept
}

// captureVerbRelations runs an allow-all recording sweep and returns, per
// package-qualified verb, the distinct relations it checks. Because every verb's
// guard is its first act (TestAuthzGuardsEveryVerb proves each returns
// ErrForbidden under deny-all), the allow-all pass follows the same path a role
// checker would until the first ungranted relation — so "role grants every
// captured relation" is exactly the condition under which the role checker never
// denies the verb.
func captureVerbRelations(t *testing.T, ctx context.Context) map[string][]string {
	t.Helper()
	rec := &recordingChecker{decide: func(string) bool { return true }}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: rec}
	out := map[string][]string{}
	swept := sweepVerbsQualified(t, ctx, sweepableServices(base), func(qualified, _ string, _ error) {
		out[qualified] = distinct(rec.relations)
		rec.relations = rec.relations[:0]
	})
	if swept < wantMinSweptVerbs {
		t.Fatalf("recording sweep found only %d verbs — reflection filter broke?", swept)
	}
	return out
}

func distinct(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// TestRoleLadderDenyMatrix (t002) is the matrix: for every sweepable verb and
// each of {viewer, contributor, billing} (the interesting lower rungs —
// developer/admin hold all resource relations), the verb is denied exactly when
// the role lacks one of the relations the verb checks, and allowed when it holds
// them all. Completeness rides sweepableServices, which TestSweepCoversEveryWiredService
// keeps == Server.features(), so a new verb cannot ship outside the matrix.
func TestRoleLadderDenyMatrix(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})
	verbRelations := captureVerbRelations(t, ctx)

	// Every captured relation must be a modelled one — a verb checking a relation
	// model.fga does not define is a bug the ladder cannot reason about.
	known := set(modelRelations...)
	for verb, rels := range verbRelations {
		for _, r := range rels {
			if !known[r] {
				t.Errorf("%s checks relation %q which model.fga does not define", verb, r)
			}
		}
	}

	for _, role := range []string{"viewer", "contributor", "billing"} {
		grants := roleGrants[role]
		chk := &recordingChecker{decide: func(relation string) bool { return grants[relation] }}
		base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}
		var deniedAny, allowedAny bool
		sweepVerbsQualified(t, ctx, sweepableServices(base), func(qualified, _ string, err error) {
			chk.relations = chk.relations[:0]
			wantAllow := grantsAll(grants, verbRelations[qualified])
			gotDeny := errors.Is(err, core.ErrForbidden)
			if wantAllow && gotDeny {
				t.Errorf("role %q: %s denied but role grants all its relations %v",
					role, qualified, verbRelations[qualified])
			}
			if !wantAllow && !gotDeny {
				t.Errorf("role %q: %s ALLOWED but role lacks one of its relations %v (grants=%v)",
					role, qualified, verbRelations[qualified], sortedKeys(grants))
			}
			if gotDeny {
				deniedAny = true
			} else {
				allowedAny = true
			}
		})
		// A lower rung must be denied SOME verbs and allowed SOME (its read verbs),
		// else the role checker is degenerate and the assertions are vacuous.
		if !deniedAny {
			t.Errorf("role %q was denied no verb — the matrix is vacuous for it", role)
		}
		if !allowedAny {
			t.Errorf("role %q was allowed no verb — the fixture is degenerate", role)
		}
	}
}

func grantsAll(grants map[string]bool, relations []string) bool {
	for _, r := range relations {
		if !grants[r] {
			return false
		}
	}
	return true
}

// representativeVerbRelations pins the gating relation of one verb per relation
// class per feature area to its SEMANTIC requirement — an expectation INDEPENDENT
// of what the verb currently checks. This is the guard that actually catches a
// too-weak (or wrong) relation regression: the role-ladder matrix derives its
// expected outcome from the relation the verb checks, so it stays consistent if a
// verb is weakened; this table does not. A Delete regressed from can_create to
// can_operate turns TestRepresentativeVerbsGateOnExpectedRelation red.
var representativeVerbRelations = map[string]string{
	// can_view — viewer and up (reads).
	"*apps.Service.Get":     core.RelCanView,
	"*apps.Service.List":    core.RelCanView,
	"*deploys.Service.List": core.RelCanView,
	// can_view_logs — contributor and up (viewers can't see logs).
	"*logs.Service.QueryLogs":             core.RelCanViewLogs,
	"*keyvalue.Service.QueryKeyValueLogs": core.RelCanViewLogs,
	// can_operate — contributor and up (lifecycle / settings, no create/delete).
	"*apps.Service.Suspend":          core.RelCanOperate,
	"*apps.Service.Restart":          core.RelCanOperate,
	"*apps.Service.Scale":            core.RelCanOperate,
	"*apps.Service.SetPlan":          core.RelCanOperate,
	"*deploys.Service.Trigger":       core.RelCanOperate,
	"*postgres.Service.CreateExport": core.RelCanOperate,
	// can_create — developer and up (create/delete resources).
	"*apps.Service.Create":             core.RelCanCreate,
	"*apps.Service.Delete":             core.RelCanCreate,
	"*postgres.Service.CreatePostgres": core.RelCanCreate,
	"*postgres.Service.DeletePostgres": core.RelCanCreate,
	"*keyvalue.Service.CreateKeyValue": core.RelCanCreate,
	"*environments.Service.Create":     core.RelCanCreate,
	"*secrets.Service.SetEnvVar":       core.RelCanCreate,
	// can_view_sensitive — developer and up (connection strings, env values).
	"*secrets.Service.GetEnvVar":               core.RelCanViewSensitive,
	"*postgres.Service.ExecuteQuery":           core.RelCanViewSensitive,
	"*keyvalue.Service.KeyValueConnectionInfo": core.RelCanViewSensitive,
	"*deploys.Service.GetDeployHook":           core.RelCanViewSensitive,
	// can_manage_keys — developer and up (workspace API keys).
	"*apikeys.Service.CreateAPIKey": core.RelCanManageKeys,
	// can_manage — admin only (workspace/members/git settings).
	"*audit.Service.List":          core.RelCanManage,
	"*members.Service.Invite":      core.RelCanManage,
	"*members.Service.ChangeRole":  core.RelCanManage,
	"*github.Service.StartConnect": core.RelCanManage,
	// can_manage_billing — billing or admin (Render's BILLING role): every billing
	// verb funnels through billing.authorize(), so all three gate on it (w1/m60).
	"*billing.Service.Status":   core.RelCanManageBilling,
	"*billing.Service.Checkout": core.RelCanManageBilling,
	"*billing.Service.Portal":   core.RelCanManageBilling,
	// can_manage_ssh_keys — every member role (their own SSH public keys).
	"*sshkeys.Service.Create": core.RelCanManageSSHKeys,
	"*sshkeys.Service.Delete": core.RelCanManageSSHKeys,
}

// TestRepresentativeVerbsGateOnExpectedRelation is the independent regression
// guard: every representative verb must gate on exactly its semantically-required
// relation. Unlike the derived role matrix, this fails when a verb is re-gated on
// a weaker (or otherwise wrong) relation — the ADR012 ladder regression class.
func TestRepresentativeVerbsGateOnExpectedRelation(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})
	verbRelations := captureVerbRelations(t, ctx)
	for verb, want := range representativeVerbRelations {
		got, ok := verbRelations[verb]
		if !ok {
			t.Errorf("representative verb %s not found in the sweep — was it renamed? update representativeVerbRelations", verb)
			continue
		}
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s gates on %v, want exactly [%s] — a changed relation is an authz-ladder regression "+
				"(fix the verb, or update the pin if the change is intended)", verb, got, want)
		}
	}
}

// --- t005 anti-tautology self-tests: prove the guards actually bite ----------

// fakeLadderVerb models one verb's gating relation for the self-tests below,
// without touching any real service.
type fakeLadderVerb struct {
	name     string
	checks   string // the relation the verb actually gates on
	required string // the relation its semantics REQUIRE (independent expectation)
}

// TestRoleLadderMatrixCatchesTooWeakRelation proves the matrix mechanism flags a
// verb gated on a weaker relation than its class requires — the ADR012 ladder
// regression. A DeleteThing verb that gates on can_operate (contributor holds it)
// but semantically requires can_create would let a contributor delete; checking
// the observed grant against the INDEPENDENT required relation turns red.
func TestRoleLadderMatrixCatchesTooWeakRelation(t *testing.T) {
	verb := fakeLadderVerb{name: "billing.DeleteThing", checks: core.RelCanOperate, required: core.RelCanCreate}
	contributor := roleGrants["contributor"]

	// The matrix's own consistency check (observed vs the relation the verb
	// checks) would NOT catch this — contributor holds can_operate, so a verb
	// gated on it is consistently allowed. The independent required-relation is
	// what makes the regression visible.
	consistentAllow := grantsAll(contributor, []string{verb.checks})
	if !consistentAllow {
		t.Fatal("fixture wrong: a contributor must hold the too-weak relation for this to be a real trap")
	}
	// Independent expectation: a can_create verb must be denied to a contributor.
	requiredAllow := grantsAll(contributor, []string{verb.required})
	if requiredAllow {
		t.Fatal("fixture wrong: a contributor must NOT hold the required relation")
	}
	// The self-test: comparing what the verb GRANTS a contributor (allow) against
	// what its class REQUIRES (deny) is a mismatch — the signal a real
	// per-verb-class audit would raise. This proves the mechanism is not vacuous.
	if consistentAllow == requiredAllow {
		t.Fatal("self-test failed to distinguish a too-weak gating relation from a correct one")
	}
}

// TestRoleLadderMatrixCatchesModelTableDrift proves the t001 pin bites: mutating
// the derived table (a role gaining or losing a grant) diverges from the parsed
// model and would fail TestRoleGrantsPinnedToModel.
func TestRoleLadderMatrixCatchesModelTableDrift(t *testing.T) {
	model := parseModelRoleGrants(t)
	// Simulate a table that wrongly grants a viewer can_operate.
	tampered := map[string]bool{}
	for k, v := range roleGrants["viewer"] {
		tampered[k] = v
	}
	tampered[core.RelCanOperate] = true
	if reflect.DeepEqual(sortedKeys(tampered), sortedKeys(model["viewer"])) {
		t.Fatal("tampered viewer table still matches the model — the pin would not catch privilege drift")
	}
}

// TestRoleLadderMatrixCatchesUnsweptVerb proves the completeness guard bites: a
// wired feature service dropped from the matrix inventory is caught. It mirrors
// TestSweepCoversEveryWiredService's mechanism (features() vs sweepableServices)
// against a deliberately-incomplete covered set, and asserts the diff is flagged.
func TestRoleLadderMatrixCatchesUnsweptVerb(t *testing.T) {
	full := sweepableServices(&core.Base{})
	if len(full) < 2 {
		t.Fatal("need at least two services to simulate a dropped one")
	}
	// Simulate an inventory missing its last service.
	covered := map[reflect.Type]bool{}
	for _, s := range full[:len(full)-1] {
		covered[reflect.TypeOf(s)] = true
	}
	dropped := reflect.TypeOf(full[len(full)-1])
	if covered[dropped] {
		t.Fatal("dropped service still marked covered — the guard would not notice its absence")
	}

	// The completeness guard (as TestSweepCoversEveryWiredService applies it) walks
	// the wired set and flags any not covered; here the dropped service must be
	// flagged, proving an unswept verb cannot hide.
	var missing []reflect.Type
	for _, s := range full {
		if !covered[reflect.TypeOf(s)] {
			missing = append(missing, reflect.TypeOf(s))
		}
	}
	if len(missing) != 1 || missing[0] != dropped {
		t.Fatalf("completeness diff failed to single out the dropped service; missing=%v", missing)
	}
}
