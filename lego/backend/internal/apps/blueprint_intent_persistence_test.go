package apps

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func blueprintIntentApp(managed bool) *appv1alpha1.App {
	labels := map[string]string{core.LabelTenant: "ws-own"}
	if managed {
		labels[store.LabelManagedBy] = store.ManagedByValue
		labels[core.LabelAppID] = ids.New(ids.Service)
	}
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "own-source", Namespace: "default", Generation: 7, Labels: labels},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Repo: "https://github.com/bex-co/bex", Branch: "main",
			Runtime: "node", Tier: "free", Port: 3000, Replicas: 1, IdleTTLSeconds: 300,
		},
	}
}

// A Blueprint update writes each projector-owned field it changes onto the
// managed App row, so the projector's next resync keeps the declared state
// instead of restoring the old row and superseding the deploy (w8/m66).
func TestBlueprintUpdatePersistsProjectorOwnedIntent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*appv1alpha1.AppSpec)
		check  func(t *testing.T, rec *recordingStore, appID string)
	}{
		{"source", func(s *appv1alpha1.AppSpec) {
			s.Repo, s.Branch = "https://github.com/expressjs/express", "master"
		}, func(t *testing.T, rec *recordingStore, appID string) {
			want := []struct{ id, repo, image, branch string }{{appID, "https://github.com/expressjs/express", "", "master"}}
			if len(rec.sourceWrites) != 1 || rec.sourceWrites[0] != want[0] {
				t.Errorf("source writes = %+v, want %+v", rec.sourceWrites, want)
			}
		}},
		{"plan", func(s *appv1alpha1.AppSpec) { s.Tier = "starter" }, func(t *testing.T, rec *recordingStore, appID string) {
			if len(rec.tierCalls) != 1 || rec.tierCalls[0].id != appID || rec.tierCalls[0].tier != "starter" {
				t.Errorf("tier writes = %+v, want starter", rec.tierCalls)
			}
		}},
		{"instances", func(s *appv1alpha1.AppSpec) { s.Replicas = 2 }, func(t *testing.T, rec *recordingStore, appID string) {
			if len(rec.replicasCalls) != 1 || rec.replicasCalls[0].id != appID || rec.replicasCalls[0].replicas != 2 {
				t.Errorf("replica writes = %+v, want 2", rec.replicasCalls)
			}
		}},
		{"disk", func(s *appv1alpha1.AppSpec) {
			s.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
		}, func(t *testing.T, rec *recordingStore, appID string) {
			rows, _ := rec.ListDisks(context.Background(), "ws-own", appID)
			if len(rows) != 1 || rows[0].Name != "data" || rows[0].MountPath != "/var/data" || rows[0].SizeGB != 10 {
				t.Errorf("disk rows = %+v, want the declared disk", rows)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := blueprintIntentApp(true)
			appID := existing.Labels[core.LabelAppID]
			cl := fakeClient(existing)
			rec := &recordingStore{}
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: rec}
			final := existing.DeepCopy()
			tc.mutate(&final.Spec)

			if _, err := svc.patchChangedStackService(context.Background(), CreateRequest{Name: existing.Name}, existing, final, stackChanges{specChanged: true}); err != nil {
				t.Fatal(err)
			}
			tc.check(t, rec, appID)
			if len(rec.deployCalls) != 1 {
				t.Errorf("deploy rows = %d, want the one Blueprint deploy", len(rec.deployCalls))
			}
			if tc.name != "source" && (rec.sourceCalls != 0) {
				t.Errorf("an unchanged source was rewritten %d times", rec.sourceCalls)
			}
		})
	}
}

// Growing an existing disk updates its row rather than adding a second one.
func TestBlueprintUpdateGrowsTheDiskRow(t *testing.T) {
	existing := blueprintIntentApp(true)
	existing.Spec.Tier = "starter"
	existing.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	appID := existing.Labels[core.LabelAppID]
	rec := &recordingStore{}
	if _, err := rec.CreateDisk(context.Background(), "ws-own", appID, "data", "/var/data", 10); err != nil {
		t.Fatal(err)
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(existing), Namespace: "default"}, Store: rec}
	final := existing.DeepCopy()
	final.Spec.Disk.SizeGB = 25
	if _, err := svc.patchChangedStackService(context.Background(), CreateRequest{Name: existing.Name}, existing, final, stackChanges{specChanged: true}); err != nil {
		t.Fatal(err)
	}
	rows, _ := rec.ListDisks(context.Background(), "ws-own", appID)
	if len(rows) != 1 || rows[0].SizeGB != 25 {
		t.Fatalf("disk rows = %+v, want the one row grown to 25GB", rows)
	}
}

// A failed row write leaves the CR and the deploy history untouched, and a
// source change stays marked pending so a resync cannot roll it unrecorded.
func TestBlueprintUpdateRowFailureOpensNoDeploy(t *testing.T) {
	existing := blueprintIntentApp(true)
	cl := fakeClient(existing)
	rec := &recordingStore{err: errors.New("row write failed")}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: rec}
	final := existing.DeepCopy()
	final.Spec.Repo, final.Spec.Branch = "https://github.com/expressjs/express", "master"
	if _, err := svc.patchChangedStackService(context.Background(), CreateRequest{Name: existing.Name}, existing, final, stackChanges{specChanged: true}); err == nil {
		t.Fatal("want the row failure returned")
	}
	if len(rec.deployCalls) != 0 {
		t.Errorf("deploy rows = %d, want none after a failed row write", len(rec.deployCalls))
	}
	stored := &appv1alpha1.App{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(existing), stored); err != nil {
		t.Fatal(err)
	}
	if stored.Spec.Repo != "https://github.com/bex-co/bex" || stored.Spec.Branch != "main" {
		t.Errorf("CR source = %s@%s, want the old source kept", stored.Spec.Repo, stored.Spec.Branch)
	}
	if stored.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "8" {
		t.Errorf("pending source generation = %q, want 8", stored.Annotations[appv1alpha1.AnnotationPendingSourceGeneration])
	}
}

// An unmanaged (bare-CR) App has no row: the update patches the CR only.
func TestBlueprintUpdateOfUnmanagedAppWritesNoRow(t *testing.T) {
	existing := blueprintIntentApp(false)
	rec := &recordingStore{}
	svc := &Service{Base: &core.Base{Client: fakeClient(existing), Namespace: "default"}, Store: rec}
	final := existing.DeepCopy()
	final.Spec.Repo, final.Spec.Tier, final.Spec.Replicas = "https://github.com/expressjs/express", "starter", 2
	if _, err := svc.patchChangedStackService(context.Background(), CreateRequest{Name: existing.Name}, existing, final, stackChanges{specChanged: true}); err != nil {
		t.Fatal(err)
	}
	if rec.sourceCalls+len(rec.tierCalls)+len(rec.replicasCalls) != 0 {
		t.Errorf("unmanaged App wrote rows: source %d tier %d replicas %d", rec.sourceCalls, len(rec.tierCalls), len(rec.replicasCalls))
	}
}
