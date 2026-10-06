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

package controller

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/074: a pass that writes an intermediate status and then wants its
// pre-pass status back must store it, though the cache it compares against has
// not seen its own write. A suspended cron job's poll wrote Deploying, found
// Hibernated equal to the stale cache, and left Deploying stored.
func TestStatusRestoreLandsDespiteALaggingCache(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "cron", Namespace: "tea-a"},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseHibernated},
	}
	store := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	var pass appv1alpha1.App
	g.Expect(store.Get(ctx, client.ObjectKeyFromObject(app), &pass)).To(Succeed())
	cl := &laggingClient{Client: store, seen: map[client.ObjectKey]*appv1alpha1.App{client.ObjectKeyFromObject(app): pass.DeepCopy()}}

	pass.Status.Phase = appv1alpha1.PhaseDeploying
	g.Expect(cl.Status().Update(ctx, &pass)).To(Succeed())
	pass.Status.Phase = appv1alpha1.PhaseHibernated
	g.Expect(updateStatusIfChanged(ctx, cl, &pass)).To(Succeed())

	var stored appv1alpha1.App
	g.Expect(store.Get(ctx, client.ObjectKeyFromObject(app), &stored)).To(Succeed())
	g.Expect(stored.Status.Phase).To(Equal(appv1alpha1.PhaseHibernated), "the pass's final status must be stored")
}

// An unchanged status is not written, whether the cache is at the pass's
// version or ahead of it: another writer moved the object mid-pass, and the
// cache caught up. A stale write there would answer a conflict, which some
// callers record as a deploy failure.
func TestUnchangedStatusIsNotWritten(t *testing.T) {
	for name, otherWriter := range map[string]bool{"cache at the pass's version": false, "cache ahead of the pass": true} {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
				Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
			}
			store := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
			var pass appv1alpha1.App
			g.Expect(store.Get(ctx, client.ObjectKeyFromObject(app), &pass)).To(Succeed())
			if otherWriter {
				moved := pass.DeepCopy()
				moved.Annotations = map[string]string{"other-writer": "yes"}
				g.Expect(store.Update(ctx, moved)).To(Succeed())
			}

			rec := &writeRecorder{Client: store, recordStatus: true}
			g.Expect(updateStatusIfChanged(ctx, rec, &pass)).To(Succeed())
			g.Expect(rec.writes).To(BeEmpty(), "an unchanged status must not be written")
		})
	}
}
