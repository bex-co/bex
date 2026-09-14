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

package rollout

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeDeployStore records CreateDeploy arguments and returns a configured prior
// commit from LatestDeployCommit — enough to unit-test Tracker.open's carry.
type fakeDeployStore struct {
	prior   store.CommitInfo
	created []store.Deploy
}

func (f *fakeDeployStore) LatestDeployCommit(_ context.Context, _ string) (store.CommitInfo, error) {
	return f.prior, nil
}

func (f *fakeDeployStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo) (store.Deploy, error) {
	d := store.Deploy{
		ID: "dep-recorded", AppID: appID, Trigger: trigger, Image: image, Generation: generation,
		Commit: commit.Hash, CommitMessage: commit.Message, CommitAuthorAt: commit.AuthorAt,
	}
	f.created = append(f.created, d)
	return d, nil
}

func openConfigChange(t *testing.T, ds *fakeDeployStore) {
	t.Helper()
	tr := &Tracker{Store: ds}
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 2},
		Spec:       appv1alpha1.AppSpec{Image: "web:v1"},
	}
	tr.open(context.Background(), Snapshot{appID: "srv-web", generation: 1, rolls: true}, a, store.TriggerConfigChange)
	if len(ds.created) != 1 {
		t.Fatalf("CreateDeploy calls = %d, want 1", len(ds.created))
	}
}

// TestOpenCarriesPriorCommit: a config_change re-roll of a repo-backed release
// must record the same commit id+message(+authorAt) the prior deploy carried.
func TestOpenCarriesPriorCommit(t *testing.T) {
	authorAt := time.Date(2026, 9, 13, 9, 35, 0, 0, time.UTC)
	prior := store.CommitInfo{Hash: "5ef5e18799fa7edbc6477ca3128c78686833b06b", Message: "update", AuthorAt: &authorAt}
	ds := &fakeDeployStore{prior: prior}
	openConfigChange(t, ds)
	got := ds.created[0]
	if got.Commit != prior.Hash || got.CommitMessage != prior.Message {
		t.Fatalf("carried commit = %q/%q, want %q/%q", got.Commit, got.CommitMessage, prior.Hash, prior.Message)
	}
	if got.CommitAuthorAt == nil || !got.CommitAuthorAt.Equal(authorAt) {
		t.Fatalf("carried authorAt = %v, want %v", got.CommitAuthorAt, authorAt)
	}
	if got.Trigger != store.TriggerConfigChange {
		t.Errorf("trigger = %q, want %q", got.Trigger, store.TriggerConfigChange)
	}
}

// TestOpenKeepsEmptyCommitWithoutPrior: image-backed services (no prior commit
// row) must still open an empty-commit config_change — never synthesize one.
func TestOpenKeepsEmptyCommitWithoutPrior(t *testing.T) {
	ds := &fakeDeployStore{}
	openConfigChange(t, ds)
	got := ds.created[0]
	if got.Commit != "" || got.CommitMessage != "" || got.CommitAuthorAt != nil {
		t.Fatalf("commit = %+v, want empty CommitInfo for image-backed / no prior", got)
	}
}
