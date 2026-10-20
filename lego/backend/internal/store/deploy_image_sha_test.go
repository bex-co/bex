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

package store

import (
	"context"
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var (
	shaA = "sha256:" + strings.Repeat("a", 64)
	shaB = "sha256:" + strings.Repeat("b", 64)
)

// imageDigestFixture serves generation 1 of an image-backed service with the
// operator's digest record digest1 (nil: none observed), then opens
// generation 2's deploy and reports it rolling. It returns the store, the app
// id, the live first row and the open second row, plus a status setter that
// reconciles after each write.
func imageDigestFixture(t *testing.T, digest1 *appv1alpha1.ImageDigestStatus) (*memStore, string, Deploy, Deploy, func(func(*appv1alpha1.App))) {
	t.Helper()
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	const image = "docker.io/ealen/echo-server:latest"
	row, _ := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: image, Port: 80, Replicas: 1, Tier: "free"})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("create projection: %v", err)
	}
	setStatus := func(mutate func(*appv1alpha1.App)) {
		t.Helper()
		app := getApp(t, cl)
		mutate(app)
		if err := cl.Status().Update(ctx, app); err != nil {
			t.Fatal(err)
		}
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	history, _ := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if len(history) != 1 {
		t.Fatalf("history = %+v, want the create deploy", history)
	}
	gen1 := history[0].Generation
	setStatus(func(a *appv1alpha1.App) {
		a.Status.Phase, a.Status.ReleaseGeneration, a.Status.ActiveRevision = appv1alpha1.PhaseRunning, gen1, fmt.Sprintf("rev-%d", gen1)
		a.Status.Image, a.Status.Conditions, a.Status.ImageDigest = image, nil, digest1
	})
	first, _ := st.GetDeploy(ctx, row.ID, history[0].ID)
	if first.Status != DeployLive {
		t.Fatalf("setup: first deploy %q, want live", first.Status)
	}

	gen2 := gen1 + 1
	second, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, image, gen2, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	setStatus(func(a *appv1alpha1.App) {
		a.Status.Phase, a.Status.ReleaseGeneration = appv1alpha1.PhaseDeploying, gen2
		a.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: "RolloutProgressing", ObservedGeneration: a.Generation,
		}}
	})
	second, _ = st.GetDeploy(ctx, row.ID, second.ID)
	if IsTerminalDeployStatus(second.Status) || second.ImageSHA != "" {
		t.Fatalf("setup: rolling second deploy = %q sha %q, want open with no digest", second.Status, second.ImageSHA)
	}
	return st, row.ID, first, second, setStatus
}

// TestLiveDeploysRecordTheirOwnImageDigests: each live close stores the digest
// the operator recorded for that row's own release, so two consecutive live
// deploys of a moving tag keep distinct digests (w8/m122).
func TestLiveDeploysRecordTheirOwnImageDigests(t *testing.T) {
	ctx := context.Background()
	st, appID, first, second, setStatus := imageDigestFixture(t, &appv1alpha1.ImageDigestStatus{Generation: 1, Digest: shaA})
	if first.ImageSHA != shaA {
		t.Fatalf("first live deploy sha = %q, want %q", first.ImageSHA, shaA)
	}
	setStatus(func(a *appv1alpha1.App) {
		a.Status.Phase, a.Status.ActiveRevision, a.Status.Conditions = appv1alpha1.PhaseRunning, fmt.Sprintf("rev-%d", second.Generation), nil
		a.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: second.Generation, Digest: shaB}
	})
	first, _ = st.GetDeploy(ctx, appID, first.ID)
	second, _ = st.GetDeploy(ctx, appID, second.ID)
	if second.Status != DeployLive || second.ImageSHA != shaB {
		t.Fatalf("second deploy = %q sha %q, want live with %q", second.Status, second.ImageSHA, shaB)
	}
	if first.Status != DeployDeactivated || first.ImageSHA != shaA {
		t.Fatalf("first deploy = %q sha %q, want deactivated keeping %q", first.Status, first.ImageSHA, shaA)
	}
}

// TestLiveCloseNeverTakesAnotherReleasesDigest: a release that goes live
// while the operator's record is still the previous release's (or there is
// none) stores no digest rather than the previous one.
func TestLiveCloseNeverTakesAnotherReleasesDigest(t *testing.T) {
	ctx := context.Background()
	st, appID, _, second, setStatus := imageDigestFixture(t, &appv1alpha1.ImageDigestStatus{Generation: 1, Digest: shaA})
	setStatus(func(a *appv1alpha1.App) {
		a.Status.Phase, a.Status.ActiveRevision, a.Status.Conditions = appv1alpha1.PhaseRunning, fmt.Sprintf("rev-%d", second.Generation), nil
	})
	second, _ = st.GetDeploy(ctx, appID, second.ID)
	if second.Status != DeployLive || second.ImageSHA != "" {
		t.Fatalf("second deploy = %q sha %q, want live with no digest", second.Status, second.ImageSHA)
	}
}

// TestFailedCloseRecordsNoImageDigest: a deploy that never goes live ran no
// digest, even when a record names its generation.
func TestFailedCloseRecordsNoImageDigest(t *testing.T) {
	ctx := context.Background()
	st, appID, _, second, setStatus := imageDigestFixture(t, nil)
	setStatus(func(a *appv1alpha1.App) {
		a.Status.Phase = appv1alpha1.PhaseFailed
		a.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: "ImagePullBackOff", Message: "image pull is failing", ObservedGeneration: a.Generation,
		}}
		a.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: second.Generation, Digest: shaB}
	})
	second, _ = st.GetDeploy(ctx, appID, second.ID)
	if !IsTerminalDeployStatus(second.Status) || second.Status == DeployLive || second.ImageSHA != "" {
		t.Fatalf("second deploy = %q sha %q, want a failed close with no digest", second.Status, second.ImageSHA)
	}
}
