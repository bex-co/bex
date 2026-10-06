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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestAMetadataPatchKeepsThePassUnsavedStatus (w5/096): a pass records status
// in memory and writes it once, at its end. A metadata patch in between,
// stamping last-active or an autoscale annotation, used to decode the
// server's whole App into the pass's, so the status it had not written yet was
// lost: a release identity, a built artifact.
func TestAMetadataPatchKeepsThePassUnsavedStatus(t *testing.T) {
	ctx := context.Background()
	r, cl, nn := lifecycleFixture(t, activeApp("tea-metadata-patch"))
	reconcileOnce(t, r, nn)

	app := liveApp(t, cl, nn)
	app.Status.ArtifactImage = "zot.test:5000/web@sha256:built"
	app.Status.ReleaseGeneration = 7
	stamp := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	if err := r.stampLastActive(ctx, &app, stamp); err != nil {
		t.Fatalf("stampLastActive: %v", err)
	}
	if app.Status.ArtifactImage != "zot.test:5000/web@sha256:built" || app.Status.ReleaseGeneration != 7 {
		t.Fatalf("the patch discarded the pass's unsaved status: %+v", app.Status)
	}
	if err := updateStatusIfChanged(ctx, cl, &app); err != nil {
		t.Fatalf("the pass's status write after the patch: %v", err)
	}
	live := liveApp(t, cl, nn)
	if live.Status.ArtifactImage != "zot.test:5000/web@sha256:built" || live.Annotations[annotLastActive] != "2026-10-06T12:00:00Z" {
		t.Fatalf("stored App = status %+v, last-active %q; want both the status and the stamp", live.Status, live.Annotations[annotLastActive])
	}
}

// TestAMetadataPatchConflictsWithAnAppChangedSinceTheRead (w5/096): the patch
// holds the resourceVersion the pass read, so it lands only on that App. A
// change since, to its spec or to status another controller wrote, makes it
// conflict, and the pass retries from current state instead of writing its
// status over the change.
func TestAMetadataPatchConflictsWithAnAppChangedSinceTheRead(t *testing.T) {
	ctx := context.Background()
	r, cl, nn := lifecycleFixture(t, activeApp("tea-metadata-changed"))
	reconcileOnce(t, r, nn)

	app := liveApp(t, cl, nn)
	read := app.DeepCopy()
	updateApp(t, cl, nn, func(concurrent *appv1alpha1.App) {
		concurrent.Generation, concurrent.Spec.Image = read.Generation+1, "nginx:next"
	})

	err := r.patchAppMeta(ctx, &app, func(meta *metav1.ObjectMeta) {
		metav1.SetMetaDataAnnotation(meta, annotAutoscaleReplicas, "2")
	})
	if !apierrors.IsConflict(err) {
		t.Fatalf("patchAppMeta over a changed App = %v, want a conflict", err)
	}
	if !equality.Semantic.DeepEqual(&app, read) {
		t.Fatal("a refused patch changed the pass's App")
	}
	if live := liveApp(t, cl, nn); live.Annotations[annotAutoscaleReplicas] != "" || live.Spec.Image != "nginx:next" {
		t.Fatalf("stored App = annotations %v, image %q; want the concurrent change alone", live.Annotations, live.Spec.Image)
	}
}

// TestPassesPatchTheirAppOnlyThroughPatchAppMeta (w5/096): patching the App
// a pass holds decodes the server's App over it. A reconcile pass patches its
// App's metadata through patchAppMeta, which patches a copy. Only a pass that
// holds no unsaved status may patch app itself.
func TestPassesPatchTheirAppOnlyThroughPatchAppMeta(t *testing.T) {
	allowed := map[string]string{
		"reclaimAppExternalArtifacts": "the deletion pass holds no unsaved status, and its finalizer Update needs the server's spec with the server's resourceVersion",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Patch" {
					return true
				}
				if recv, ok := sel.X.(*ast.Ident); !ok || recv.Name != "r" {
					return true // a status patch, r.Status().Patch, or another client
				}
				if target, ok := call.Args[1].(*ast.Ident); ok && target.Name == "app" {
					if _, ok := allowed[fn.Name.Name]; !ok {
						t.Errorf("%s: %s patches the pass's App directly; use patchAppMeta", fset.Position(call.Pos()), fn.Name.Name)
					}
				}
				return true
			})
		}
	}
}
