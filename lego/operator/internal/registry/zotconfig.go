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

package registry

import (
	"encoding/json"
	"reflect"
	"slices"
)

// The Zot config document. The operator owns only parts of it — the storage
// policy, the builder grants, and one ACL entry per tenant repository — so
// every read-modify-write below edits a decoded map[string]any rather than
// round-tripping through the typed base config: a typed round-trip would
// silently erase every field this package does not model (extensions, TLS,
// alternate auth backends, storage drivers) on the next reconcile.
//
// The typed shape at the bottom of this file therefore has exactly one job:
// producing the canonical document the operator installs and migrates toward.
const (
	// zotConfigKey is the data key of the Zot config Secret.
	zotConfigKey = "config.json"

	zotActionRead   = "read"
	zotActionCreate = "create"
	zotActionUpdate = "update"
	zotActionDelete = "delete"

	// zotBuilderUser is the platform build identity: buildkitd, kpack, skopeo
	// and cosign all authenticate as it.
	zotBuilderUser = "bex-builder"

	// defaultRetentionCount is the image history kept per repository when
	// BEX_ZOT_RETENTION_COUNT is unset.
	defaultRetentionCount = 5

	// zotGCDelay is how long an untagged/dangling blob survives before garbage
	// collection may remove it. It MUST stay above the worst-case push duration
	// or GC can delete blobs belonging to a push that is still in flight
	// (docs/ADR060 D4). The bound that matters is the build Job's own deadline,
	// since a push cannot outlive the build that produces it — the invariant
	// zotGCDelay >= build.BuildTimeout is asserted by gc_invariant_test.go, so
	// neither value can drift away from the other unnoticed.
	// Pinned explicitly rather than left to Zot, whose GC defaults change.
	zotGCDelay = "1h"
	// gcInterval is how often GC runs; equal to the delay by design, so a blob
	// is examined at most one interval after it becomes eligible.
	zotGCInterval = zotGCDelay
)

var (
	// zotReadWriteActions is a tenant's grant on its own repository: its build
	// pushes there and its kubelet pulls from there.
	zotReadWriteActions = []string{zotActionRead, zotActionCreate, zotActionUpdate, zotActionDelete}

	// zotReadOnlyActions is a snapshot consumer's grant. Sandbox rootfs
	// snapshots are pushed by the commit Job as zotBuilderUser through the
	// global adminPolicy, so no tenant identity ever needs write.
	zotReadOnlyActions = []string{zotActionRead}
)

// -- accessors ----------------------------------------------------------------

// zotAccessControl returns data's http.accessControl block, or nil when absent.
// It never mutates data, so predicates cannot disturb the document they read.
func zotAccessControl(data map[string]any) map[string]any {
	httpBlock, _ := data["http"].(map[string]any)
	ac, _ := httpBlock["accessControl"].(map[string]any)
	return ac
}

// zotAccessControlFor returns data's http.accessControl block, creating the
// intermediate maps as needed. Mutations to the result reach data.
func zotAccessControlFor(data map[string]any) map[string]any {
	httpBlock, ok := data["http"].(map[string]any)
	if !ok {
		httpBlock = map[string]any{}
		data["http"] = httpBlock
	}
	ac, ok := httpBlock["accessControl"].(map[string]any)
	if !ok {
		ac = map[string]any{}
		httpBlock["accessControl"] = ac
	}
	return ac
}

// zotRepos returns data's repository ACL map, or nil when absent. Read-only.
func zotRepos(data map[string]any) map[string]any {
	repos, _ := zotAccessControl(data)["repositories"].(map[string]any)
	return repos
}

// zotReposFor returns data's repository ACL map, creating it as needed.
// Mutations to the result reach data.
func zotReposFor(data map[string]any) map[string]any {
	ac := zotAccessControlFor(data)
	repos, ok := ac["repositories"].(map[string]any)
	if !ok {
		repos = map[string]any{}
		ac["repositories"] = repos
	}
	return repos
}

// sameStrings reports whether a JSON-decoded list holds exactly want, in any
// order. want must be duplicate-free.
func sameStrings(got []any, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		if !slices.Contains(got, any(w)) {
			return false
		}
	}
	return true
}

func anySlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// -- repository ACLs ----------------------------------------------------------

// zotRepoGrants reports whether repo's ACL grants user exactly actions and
// nothing else. Exactness is the tenant-isolation property: an extra user on a
// repository is a cross-tenant grant and an extra action is an unintended
// privilege, so either must be repaired rather than accepted as converged.
func zotRepoGrants(data map[string]any, repo, user string, actions []string) bool {
	entry, _ := zotRepos(data)[repo].(map[string]any)
	policies, _ := entry["policies"].([]any)
	if len(policies) != 1 {
		return false
	}
	policy, _ := policies[0].(map[string]any)
	users, _ := policy["users"].([]any)
	granted, _ := policy["actions"].([]any)
	return sameStrings(users, []string{user}) && sameStrings(granted, actions)
}

// setZotRepoPolicy grants user exactly actions on repo, replacing whatever
// policy that repository carried.
func setZotRepoPolicy(data map[string]any, repo, user string, actions []string) {
	zotReposFor(data)[repo] = map[string]any{
		"policies": []any{map[string]any{
			"users":   []any{user},
			"actions": anySlice(actions),
		}},
	}
}

// zotHasRepo reports whether repo carries any ACL entry.
func zotHasRepo(data map[string]any, repo string) bool {
	_, ok := zotRepos(data)[repo]
	return ok
}

// -- platform grants ----------------------------------------------------------

// zotHasBuilderAdminPolicy reports whether zotBuilderUser holds the global
// actions buildkitd and cosign need. A ** repository policy is not sufficient:
// Zot applies only the longest repository match, so an exact per-App rule
// shadows the builder's ** rule.
func zotHasBuilderAdminPolicy(data map[string]any) bool {
	admin, _ := zotAccessControl(data)["adminPolicy"].(map[string]any)
	users, _ := admin["users"].([]any)
	actions, _ := admin["actions"].([]any)
	if !slices.Contains(users, any(zotBuilderUser)) {
		return false
	}
	for _, action := range zotReadWriteActions {
		if !slices.Contains(actions, any(action)) {
			return false
		}
	}
	return true
}

// setZotBuilderAdminPolicy installs the canonical global builder grant.
// zot-config is operator-managed, so this policy is authoritative.
func setZotBuilderAdminPolicy(data map[string]any) {
	zotAccessControlFor(data)["adminPolicy"] = map[string]any{
		"users":   []any{zotBuilderUser},
		"actions": anySlice(zotReadWriteActions),
	}
}

// zotHasPlatformBuilderPolicy reports whether every authenticated identity can
// pull the shared kpack builder without gaining any write action. App builds
// authenticate as app-<name>, so their repository-scoped credential must cover
// this one platform input as well as their own output repository.
func zotHasPlatformBuilderPolicy(data map[string]any) bool {
	entry, _ := zotRepos(data)[platformBuilderRepository].(map[string]any)
	actions, _ := entry["defaultPolicy"].([]any)
	return sameStrings(actions, zotReadOnlyActions)
}

// setZotPlatformBuilderPolicy installs the exact, read-only rule for the shared
// kpack builder repository. The global builder adminPolicy still overrides this
// exact match for platform publishing, so no tenant identity can create,
// update, or delete the builder image.
func setZotPlatformBuilderPolicy(data map[string]any) {
	zotReposFor(data)[platformBuilderRepository] = map[string]any{
		"defaultPolicy": anySlice(zotReadOnlyActions),
	}
}

// setZotStorage replaces the operator-owned storage block, reporting whether
// anything moved. Returning false keeps the caller from rewriting a converged
// Secret, which Go's randomized map ordering would otherwise do on every pass.
func setZotStorage(data, desired map[string]any) bool {
	if current, _ := data["storage"].(map[string]any); reflect.DeepEqual(current, desired) {
		return false
	}
	data["storage"] = desired
	return true
}

// -- canonical base config ----------------------------------------------------

// The typed canonical document. Used only to PRODUCE config bytes — never to
// parse an existing config, which would drop the fields it does not model.
type (
	zotKeepTag struct {
		Patterns                []string `json:"patterns"`
		MostRecentlyPushedCount int      `json:"mostRecentlyPushedCount"`
	}
	zotRetentionPolicy struct {
		Repositories   []string     `json:"repositories"`
		DeleteUntagged bool         `json:"deleteUntagged"`
		KeepTags       []zotKeepTag `json:"keepTags"`
	}
	zotRetention struct {
		DryRun   bool                 `json:"dryRun"`
		Policies []zotRetentionPolicy `json:"policies"`
	}
	zotStorage struct {
		RootDirectory string       `json:"rootDirectory"`
		Dedupe        bool         `json:"dedupe"`
		GC            bool         `json:"gc"`
		GCDelay       string       `json:"gcDelay"`
		GCInterval    string       `json:"gcInterval"`
		Retention     zotRetention `json:"retention"`
	}
	zotScrub struct {
		Enable   bool   `json:"enable"`
		Interval string `json:"interval"`
	}
	zotExtensions struct {
		Scrub zotScrub `json:"scrub"`
	}
	zotHTPasswdAuth struct {
		Path string `json:"path"`
	}
	zotAuth struct {
		HTPasswd zotHTPasswdAuth `json:"htpasswd"`
	}
	zotPolicy struct {
		Users   []string `json:"users"`
		Actions []string `json:"actions"`
	}
	zotRepoACL struct {
		Policies      []zotPolicy `json:"policies"`
		DefaultPolicy []string    `json:"defaultPolicy,omitempty"`
	}
	zotAccessControlBlock struct {
		Repositories map[string]zotRepoACL `json:"repositories"`
		AdminPolicy  zotPolicy             `json:"adminPolicy"`
	}
	zotHTTPBlock struct {
		Address       string                `json:"address"`
		Port          string                `json:"port"`
		Compat        []string              `json:"compat"`
		ReadTimeout   string                `json:"readTimeout"`
		WriteTimeout  string                `json:"writeTimeout"`
		Auth          zotAuth               `json:"auth"`
		AccessControl zotAccessControlBlock `json:"accessControl"`
	}
	zotLogBlock struct {
		Level string `json:"level"`
	}
	zotDocument struct {
		DistSpecVersion string        `json:"distSpecVersion"`
		Storage         zotStorage    `json:"storage"`
		HTTP            zotHTTPBlock  `json:"http"`
		Log             zotLogBlock   `json:"log"`
		Extensions      zotExtensions `json:"extensions"`
	}
)

// baseZotConfig returns the operator-canonical Zot config JSON. It replaces the
// Helm-embedded configFiles stanza (removed in w7/m36). The bex-puller shared
// user is NOT present: per-App users ("app-<name>") are added dynamically.
func (c *Creds) baseZotConfig() []byte {
	c.baseOnce.Do(c.buildBaseZotConfig)
	return c.baseJSON
}

// canonicalStorage returns the decoded storage block of the canonical config —
// the one subtree the operator force-migrates onto existing configs. Callers
// treat it as immutable; it is shared across every reconcile.
func (c *Creds) canonicalStorage() map[string]any {
	c.baseOnce.Do(c.buildBaseZotConfig)
	return c.baseStorage
}

// buildBaseZotConfig renders the canonical document once per process. Its only
// input is RetentionCount, fixed at construction, and it would otherwise be
// rebuilt and re-marshalled on every reconcile of every App.
func (c *Creds) buildBaseZotConfig() {
	retentionCount := c.RetentionCount
	if retentionCount == 0 {
		retentionCount = defaultRetentionCount
	}
	builderGrant := zotPolicy{Users: []string{zotBuilderUser}, Actions: zotReadWriteActions}

	c.baseJSON, _ = json.Marshal(zotDocument{
		DistSpecVersion: "1.1.0",
		Storage: zotStorage{
			RootDirectory: "/var/lib/registry",
			Dedupe:        true,
			GC:            true,
			GCDelay:       zotGCDelay,
			GCInterval:    zotGCInterval,
			Retention: zotRetention{
				Policies: []zotRetentionPolicy{{
					Repositories:   []string{"**"},
					DeleteUntagged: true,
					KeepTags: []zotKeepTag{{
						Patterns:                []string{".*"},
						MostRecentlyPushedCount: retentionCount,
					}},
				}},
			},
		},
		// Scrub re-verifies blob digests against their content, so silent
		// corruption surfaces as a log line rather than as a deploy pulling a
		// corrupt layer. Single-replica Zot's integrity tripwire (ADR060 D4).
		Extensions: zotExtensions{Scrub: zotScrub{Enable: true, Interval: "24h"}},
		HTTP: zotHTTPBlock{
			Address:      "0.0.0.0",
			Port:         zotHTTPPort,
			Compat:       []string{"docker2s2"},
			ReadTimeout:  "60s",
			WriteTimeout: "60s",
			Auth:         zotAuth{HTPasswd: zotHTPasswdAuth{Path: zotHTPasswdPath}},
			AccessControl: zotAccessControlBlock{
				Repositories: map[string]zotRepoACL{
					// Anonymous access is denied by the empty defaultPolicy;
					// only the builder may reach an unclaimed repository.
					"**":                      {Policies: []zotPolicy{builderGrant}, DefaultPolicy: []string{}},
					platformBuilderRepository: {DefaultPolicy: zotReadOnlyActions},
				},
				AdminPolicy: builderGrant,
			},
		},
		Log: zotLogBlock{Level: "info"},
	})

	var decoded map[string]any
	_ = json.Unmarshal(c.baseJSON, &decoded)
	c.baseStorage, _ = decoded["storage"].(map[string]any)
}
