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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/operator/internal/identity"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// kept_builds_test.go is w7/m429 t003: Render keeps 5 builds for rollback on
// Hobby, 15 on Pro and 30 on Scale and Enterprise, and the registry's
// retention follows the count bex-api labels each App with.

// keptApp is an App of workspace ws carrying kept as its kept-builds label
// ("" for none).
func keptApp(ws, name, kept string) *appv1alpha1.App {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: ws,
		Labels: map[string]string{appv1alpha1.LabelWorkspace: ws},
	}}
	if kept != "" {
		app.Labels[appv1alpha1.LabelKeptBuilds] = kept
	}
	return app
}

// keptBuildsCreds is a Creds whose App list is a fake cluster holding apps.
func keptBuildsCreds(t *testing.T, apps ...*appv1alpha1.App) *Creds {
	t.Helper()
	scheme := testScheme(t)
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	objs := make([]client.Object, 0, 1+len(apps))
	objs = append(objs, htpasswdSecret())
	for _, app := range apps {
		objs = append(objs, app)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	c := newTestCreds(t)
	c.Client, c.Apps = cl, cl
	return c
}

// retentionPolicies is a storage block's retention policies as Zot reads them.
func retentionPolicies(t *testing.T, storage map[string]any) []map[string]any {
	t.Helper()
	retention, _ := storage["retention"].(map[string]any)
	raw, _ := retention["policies"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, p := range raw {
		policy, _ := p.(map[string]any)
		out = append(out, policy)
	}
	return out
}

// policyShape is one policy reduced to what decides retention: its
// repositories, then each keepTags entry as "<pattern>=<count>" in order.
func policyShape(policy map[string]any) string {
	rawRepos := policy["repositories"].([]any)
	repos := make([]string, 0, len(rawRepos))
	for _, r := range rawRepos {
		repos = append(repos, r.(string))
	}
	keepTags := policy["keepTags"].([]any)
	rules := make([]string, 0, len(keepTags))
	for _, e := range keepTags {
		entry := e.(map[string]any)
		var patterns []string
		for _, p := range entry["patterns"].([]any) {
			patterns = append(patterns, p.(string))
		}
		rules = append(rules, fmt.Sprintf("%s=%v", strings.Join(patterns, "|"), entry["mostRecentlyPushedCount"]))
	}
	return strings.Join(repos, ",") + " " + strings.Join(rules, " ")
}

func TestEachPlanCountIsAPolicyAheadOfTheCatchAll(t *testing.T) {
	deleting := keptApp("tea-gone", "web", "30")
	deleting.Finalizers = []string{"app.bex.co/finalizer"}
	carrier := keptApp("tea-pro", "builds", "15")
	carrier.Spec.Type = appv1alpha1.TypeBuildSource
	c := keptBuildsCreds(t,
		keptApp("tea-hobby", "web", "5"), keptApp("tea-hobby", "api", "5"),
		keptApp("tea-pro", "web", "15"), carrier,
		keptApp("tea-scale", "web", "30"), keptApp("tea-ent", "web", "30"),
		// No label, or one that is not a positive count: the default.
		keptApp("tea-old", "web", ""), keptApp("tea-zero", "web", "0"),
		keptApp("tea-neg", "web", "-3"), keptApp("tea-word", "web", "ten"),
		deleting,
	)
	ctx := context.Background()
	if err := c.Client.Delete(ctx, deleting); err != nil {
		t.Fatal(err)
	}

	policies := retentionPolicies(t, c.desiredStorage(ctx))
	got := make([]string, 0, len(policies))
	for _, policy := range policies {
		got = append(got, policyShape(policy))
		if policy["deleteUntagged"] != true {
			t.Errorf("policy %s keeps untagged manifests", policyShape(policy))
		}
	}
	// Fewest first, repositories sorted, the live rule before the pushed
	// window in each, and the catch-all last with today's ten live builds.
	want := []string{
		"tea-hobby/api,tea-hobby/web ^live-.*=5 .*=5",
		"tea-pro/web ^live-.*=15 .*=5",
		"tea-ent/web,tea-scale/web ^live-.*=30 .*=5",
		"** ^live-.*=10 .*=5",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("retention policies =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// With no App carrying a count the block is exactly today's, so the first
// release of this operator changes nothing until bex-api labels the Apps.
func TestNoKeptBuildsLabelKeepsTodaysRetention(t *testing.T) {
	c := keptBuildsCreds(t, keptApp("tea-a", "web", ""), keptApp("tea-b", "web", "0"))
	if got := c.desiredStorage(context.Background()); !reflect.DeepEqual(got, c.canonicalStorage()) {
		t.Fatalf("storage = %v, want the canonical block", got)
	}
}

// The same Apps render the same block whatever order the cache lists them in,
// so a converged Secret is not rewritten every pass.
func TestKeptBuildsPoliciesDoNotDependOnListOrder(t *testing.T) {
	apps := []appv1alpha1.App{
		*keptApp("tea-b", "web", "15"), *keptApp("tea-a", "web", "5"),
		*keptApp("tea-a", "api", "5"), *keptApp("tea-c", "web", "30"),
	}
	first := keptBuildsPolicies(apps, 5)
	slices.Reverse(apps)
	if second := keptBuildsPolicies(apps, 5); !reflect.DeepEqual(first, second) {
		t.Fatalf("policies depend on list order:\n%v\n%v", first, second)
	}
}

func TestTheRegistryKeepsWhatEachPlanKeepsAndADeletedWorkspaceLeaves(t *testing.T) {
	ctx := context.Background()
	hobby, scale := keptApp("tea-a", "web", "5"), keptApp("tea-b", "web", "30")
	hobby.Finalizers = []string{"app.bex.co/finalizer"}
	c := keptBuildsCreds(t, hobby, scale)
	shapes := func() []string {
		t.Helper()
		storage, _ := storedConfig(t, c)["storage"].(map[string]any)
		policies := retentionPolicies(t, storage)
		out := make([]string, 0, len(policies))
		for _, policy := range policies {
			out = append(out, policyShape(policy))
		}
		return out
	}

	if err := c.EnsureCredsFor(ctx, identity.ForApp("web", "tea-a"), "tea-a"); err != nil {
		t.Fatal(err)
	}
	want := []string{"tea-a/web ^live-.*=5 .*=5", "tea-b/web ^live-.*=30 .*=5", "** ^live-.*=10 .*=5"}
	if got := shapes(); !slices.Equal(got, want) {
		t.Fatalf("stored policies = %v, want %v", got, want)
	}
	// A converged config is not written again.
	before := readSecret(t, c, testZotNS, "zot-config").ResourceVersion
	if err := c.EnsureCredsFor(ctx, identity.ForApp("web", "tea-a"), "tea-a"); err != nil {
		t.Fatal(err)
	}
	if after := readSecret(t, c, testZotNS, "zot-config").ResourceVersion; after != before {
		t.Fatalf("a converged config was rewritten: resourceVersion %s -> %s", before, after)
	}

	// The Scale workspace is deleted: its policy leaves at the next write.
	if err := c.Client.Delete(ctx, scale); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureCredsFor(ctx, identity.ForApp("web", "tea-a"), "tea-a"); err != nil {
		t.Fatal(err)
	}
	want = []string{"tea-a/web ^live-.*=5 .*=5", "** ^live-.*=10 .*=5"}
	if got := shapes(); !slices.Equal(got, want) {
		t.Fatalf("after the Scale workspace's deletion: stored policies = %v, want %v", got, want)
	}

	// The last App's own revoke, while it is being deleted, takes its policy
	// with its grant: no later write is needed.
	if err := c.Client.Delete(ctx, hobby); err != nil {
		t.Fatal(err)
	}
	if err := c.RevokeCredsFor(ctx, identity.ForApp("web", "tea-a")); err != nil {
		t.Fatal(err)
	}
	if got := shapes(); !slices.Equal(got, []string{"** ^live-.*=10 .*=5"}) {
		t.Fatalf("after the last App's revoke: stored policies = %v, want the catch-all alone", got)
	}
}

// An App list that cannot be read never empties the block: that would cut a
// Scale service to the default at the next retention run. The credential
// write itself still succeeds.
func TestAnUnreadableAppListLeavesTheStoredRetention(t *testing.T) {
	ctx := context.Background()
	c := keptBuildsCreds(t, keptApp("tea-b", "web", "30"))
	if err := c.EnsureCredsFor(ctx, identity.ForApp("web", "tea-b"), "tea-b"); err != nil {
		t.Fatal(err)
	}
	stored := storedConfig(t, c)["storage"]
	c.Apps = interceptor.NewClient(c.Apps.(client.WithWatch), interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New("cache not started")
		},
	})
	if err := c.EnsureCredsFor(ctx, identity.ForApp("other", "tea-c"), "tea-c"); err != nil {
		t.Fatalf("a credential write with the App list unreadable: %v", err)
	}
	cfg := storedConfig(t, c)
	if !reflect.DeepEqual(cfg["storage"], stored) {
		t.Fatalf("storage = %v, want it left as %v", cfg["storage"], stored)
	}
	if !zotRepoGrants(cfg, "tea-c/other", identity.ForApp("other", "tea-c").ZotUsername(), zotReadWriteActions) {
		t.Fatal("the credential's grant was not written")
	}
}

// pushed is one tag push, oldest first.
type pushed struct{ tag, digest string }

// zotRetains is the set of digests Zot v2.1.22 keeps in repo under storage's
// retention, as pkg/retention decides it: the repository's policy is the
// FIRST whose repositories match it (policyManager.getRepoPolicy), each tag
// goes to the first keepTags entry whose pattern matches (getTagPolicy), and
// an entry keeps its mostRecentlyPushedCount newest tags (NewLatestPush). A
// manifest stays while any kept tag names it. The rendered block names
// repositories only exactly or as "**", which is all this models;
// scripts/verify-zot-retention.py --kept-builds runs a block of this shape on
// the real registry.
func zotRetains(t *testing.T, storage map[string]any, repo string, pushes []pushed) map[string]bool {
	t.Helper()
	var policy map[string]any
	for _, p := range retentionPolicies(t, storage) {
		for _, r := range p["repositories"].([]any) {
			pattern := r.(string)
			if pattern != "**" && strings.ContainsAny(pattern, "*?[{\\") {
				t.Fatalf("repository pattern %q is a glob this model does not read", pattern)
			}
			if policy == nil && (pattern == "**" || pattern == repo) {
				policy = p
			}
		}
	}
	entries := policy["keepTags"].([]any)
	groups := make([][]pushed, len(entries))
	for _, push := range pushes {
		for i, e := range entries {
			pattern := e.(map[string]any)["patterns"].([]any)[0].(string)
			if regexp.MustCompile(pattern).MatchString(push.tag) {
				groups[i] = append(groups[i], push)
				break
			}
		}
	}
	kept := map[string]bool{}
	for i, group := range groups {
		n := int(entries[i].(map[string]any)["mostRecentlyPushedCount"].(float64))
		for _, push := range group[max(0, len(group)-n):] {
			kept[push.digest] = true
		}
	}
	return kept
}

// After 35 releases and a 36th build still rolling out, each plan keeps
// exactly its count of builds that went live, plus the rolling one; the
// serving build and the rolling build are kept whatever the count, and so is
// the build a rollback brings back.
func TestEachPlanKeepsItsBuildsAndNeverTheServingOrRollingOne(t *testing.T) {
	ctx := context.Background()
	c := keptBuildsCreds(t,
		keptApp("tea-hobby", "web", "5"), keptApp("tea-pro", "web", "15"),
		keptApp("tea-scale", "web", "30"), keptApp("tea-ent", "web", "30"),
		keptApp("tea-old", "web", ""),
	)
	storage := c.desiredStorage(ctx)
	digest := func(n int) string { return fmt.Sprintf("sha256:%064d", n) }
	const releases = 35
	var pushes []pushed
	for n := 1; n <= releases; n++ {
		// The build is pushed as gen-<n>, and live-<n> once its release runs.
		pushes = append(pushes, pushed{fmt.Sprintf("gen-%d", n), digest(n)}, pushed{fmt.Sprintf("live-%d", n), digest(n)})
	}
	rolling := releases + 1
	pushes = append(pushes, pushed{fmt.Sprintf("gen-%d", rolling), digest(rolling)})

	for repo, kept := range map[string]int{
		"tea-hobby/web": 5, "tea-pro/web": 15, "tea-scale/web": 30, "tea-ent/web": 30,
		"tea-old/web": liveRetentionCount, // no label: today's ten
	} {
		got := zotRetains(t, storage, repo, pushes)
		want := map[string]bool{digest(rolling): true}
		for n := releases - kept + 1; n <= releases; n++ {
			want[digest(n)] = true
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s keeps %d builds, want the %d newest live builds and the rolling one", repo, len(got), kept)
		}
		if !got[digest(releases)] || !got[digest(rolling)] {
			t.Errorf("%s lost the serving or the rolling build", repo)
		}

		// A rollback to the oldest kept build pushes live-<next> on it; it is
		// then the serving build, and stays.
		back := releases - kept + 1
		after := zotRetains(t, storage, repo, append(slices.Clone(pushes), pushed{fmt.Sprintf("live-%d", rolling+1), digest(back)}))
		if !after[digest(back)] {
			t.Errorf("%s removed the build a rollback serves", repo)
		}
	}
}

// The policies decode into the shape the typed block renders, so the stored
// document compares equal to a fresh rendering and setZotStorage is a no-op.
func TestDesiredStorageRoundTripsThroughTheSecret(t *testing.T) {
	c := keptBuildsCreds(t, keptApp("tea-a", "web", "15"))
	storage := c.desiredStorage(context.Background())
	raw, err := json.Marshal(map[string]any{"storage": storage})
	if err != nil {
		t.Fatal(err)
	}
	if setZotStorage(mustDecode(t, raw), c.desiredStorage(context.Background())) {
		t.Fatal("a stored per-plan block was rewritten")
	}
}
