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
	"maps"
	"reflect"
	"slices"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/identity"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Builds kept for rollback by workspace plan (w7/m429, docs/ADR060 D4, "The
// builds kept follow the workspace plan"). Render keeps 5 builds on Hobby, 15
// on Pro and 30 on Scale and Enterprise; bex-api writes the workspace's count
// on each of its Apps as appv1alpha1.LabelKeptBuilds, and the registry keeps
// that many of the App's newest live- builds.
//
// Zot picks ONE retention policy per repository, the first whose globs match
// it (pkg/retention policyManager.getRepoPolicy, v2.1.22), and inside it gives
// each tag to the first keepTags entry whose pattern matches
// (getTagPolicy). So each count is a policy of its own, listing the exact
// repositories of the Apps that carry it, ahead of the "**" catch-all every
// other repository keeps falling to: an App with no label, a build cache
// (<repo>_cache), sandbox snapshots, the platform builder, and a Build Source
// carrier, whose builds are what linked services copy rather than rollback
// targets.
//
// Whatever the count, the serving build is the newest live- tag (the operator
// pushes it on promotion, protectServingImage) and a rolling release's build
// is among the newest pushes, which the second entry keeps.

// keptBuildsOf is the count app's LabelKeptBuilds carries. ok is false when
// the label is absent or not a positive integer: Zot reads a zero count as no
// count rule and keeps every tag, so such a value never reaches the config.
func keptBuildsOf(app *appv1alpha1.App) (int, bool) {
	raw, ok := app.Labels[appv1alpha1.LabelKeptBuilds]
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// keptBuildsPolicies is one retention policy per kept-builds count among apps,
// fewest first, each listing its repositories sorted, so the same Apps always
// render the same block and a converged Secret is never rewritten. An App
// being deleted has no policy: its repository is going, and a deleted
// workspace's policies leave with its Apps. Two Apps naming one repository
// (never expected) keep the larger count.
func keptBuildsPolicies(apps []appv1alpha1.App, retentionCount int) []zotRetentionPolicy {
	byRepo := map[string]int{}
	for i := range apps {
		app := &apps[i]
		if app.DeletionTimestamp != nil || app.Spec.Type == appv1alpha1.TypeBuildSource {
			continue
		}
		n, ok := keptBuildsOf(app)
		if !ok {
			continue
		}
		repo := identity.ForApp(app.Name, app.Labels[appv1alpha1.LabelWorkspace]).Repo()
		byRepo[repo] = max(byRepo[repo], n)
	}
	byCount := map[int][]string{}
	for _, repo := range slices.Sorted(maps.Keys(byRepo)) {
		byCount[byRepo[repo]] = append(byCount[byRepo[repo]], repo)
	}
	policies := make([]zotRetentionPolicy, 0, len(byCount))
	for _, n := range slices.Sorted(maps.Keys(byCount)) {
		policies = append(policies, zotRetentionPolicy{
			Repositories:   byCount[n],
			DeleteUntagged: true,
			KeepTags:       zotKeepTags(n, retentionCount),
		})
	}
	return policies
}

// desiredStorage is the storage block every config write converges to: the
// canonical one, with the per-plan policies of the Apps that Apps lists ahead
// of its catch-all. It is computed from the whole App list on every write, so
// an App's count, a plan change and a deleted App all land on the next write,
// and a block that has not moved is the one shared value (setZotStorage then
// writes nothing).
//
// nil means the App list could not be read, and the caller leaves the stored
// block as it is. An empty list in its place would drop every policy, and the
// next retention run would cut a Scale service's 30 builds to the default;
// failing the write instead would fail the App's registry credential, which
// does not depend on retention at all.
func (c *Creds) desiredStorage(ctx context.Context) map[string]any {
	if c.Apps == nil {
		return c.canonicalStorage()
	}
	var apps appv1alpha1.AppList
	if err := c.Apps.List(ctx, &apps, client.UnsafeDisableDeepCopy); err != nil {
		logf.FromContext(ctx).Error(err, "zot retention left as stored: the App list is unreadable")
		return nil
	}
	policies := keptBuildsPolicies(apps.Items, c.retentionCount())
	if len(policies) == 0 {
		return c.canonicalStorage()
	}
	c.keptMu.Lock()
	defer c.keptMu.Unlock()
	if c.keptStorage == nil || !reflect.DeepEqual(c.keptPolicies, policies) {
		// The typed block always marshals; decoded, it compares equal to the
		// stored one read back from the Secret.
		raw, _ := json.Marshal(zotStorageFor(c.retentionCount(), policies))
		var storage map[string]any
		_ = json.Unmarshal(raw, &storage)
		c.keptPolicies, c.keptStorage = policies, storage
	}
	return c.keptStorage
}
