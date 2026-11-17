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

package deploys

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rollback_kept_builds_test.go is w7/m429 t003. The registry keeps a paid
// service's 5, 15 or 30 newest builds by workspace plan, and the operator
// publishes what it holds in status.storedImages. After more than 30
// releases, every surface offers exactly the builds held: the deploy list
// and GraphQL's buildAvailable mark the rest, and a rollback to one of them
// answers 409 while the oldest build held still rolls back.
func TestRollbackOffersExactlyTheBuildsThePlanKeeps(t *testing.T) {
	const releases = 35
	digest := func(n int) string { return fmt.Sprintf("sha256:%064d", n) }
	for _, kept := range []int{5, 15, 30} {
		t.Run(fmt.Sprintf("%d builds kept", kept), func(t *testing.T) {
			ctx := context.Background()
			ds := newFakeStore()
			app := sampleApp("web", "srv-1")
			// A paid service: the Free two-deploy rule is not in play.
			app.Spec.Image, app.Spec.Repo, app.Spec.Tier = "", "https://github.com/a/app", "starter"
			deploys := map[int]store.Deploy{}
			for n := 1; n <= releases; n++ {
				deploys[n] = liveDeploy(t, ds, "srv-1", "", int64(n), fmt.Sprintf("%s:gen-%d@%s", clusterRepo, n, digest(n)))
			}
			app.Status.Image = fmt.Sprintf("%s:gen-%d@%s", clusterRepo, releases, digest(releases))
			app.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: releases, Digest: digest(releases)}
			held := []string{}
			for n := releases - kept + 1; n <= releases; n++ {
				held = append(held, digest(n))
			}
			app.Status.StoredImages = &appv1alpha1.StoredImagesStatus{Generation: releases, Digests: held}
			svc, _ := newService(ds, app)

			rows, err := svc.List(ctx, "web", ListFilter{Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != releases {
				t.Fatalf("listed %d deploys, want %d", len(rows), releases)
			}
			buildAvailable := deployGQLType.Fields()["buildAvailable"].Resolve
			offered := 0
			for _, row := range rows {
				available, err := buildAvailable(graphql.ResolveParams{Source: row})
				if err != nil {
					t.Fatal(err)
				}
				if available != !row.BuildGone {
					t.Errorf("deploy %s: buildAvailable = %v with BuildGone %v", row.ID, available, row.BuildGone)
				}
				if !row.BuildGone {
					offered++
				}
			}
			if offered != kept {
				t.Errorf("%d deploys read as having their build, want %d", offered, kept)
			}

			gone, oldestHeld := deploys[releases-kept], deploys[releases-kept+1]
			if _, err := svc.Rollback(ctx, "web", gone.ID); !errors.Is(err, core.ErrConflict) {
				t.Fatalf("rollback to the build just past the %d kept = %v, want a conflict", kept, err)
			}
			if pre := rollbackPrecondition(t, svc, "web"); pre != "" {
				t.Fatalf("deployActions rollback precondition = %q, want none", pre)
			}
			if _, err := svc.Rollback(ctx, "web", oldestHeld.ID); err != nil {
				t.Fatalf("rollback to the oldest of the %d kept builds: %v", kept, err)
			}
		})
	}
}
