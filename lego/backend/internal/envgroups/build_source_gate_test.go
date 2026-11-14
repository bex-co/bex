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

package envgroups

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m254 t004: a group link or group change never rolls a build source out —
// Render gives it the group at build time and builds only when asked — even
// with auto-deploy on; services keep Render's auto-deploy rule.
func TestGroupChangesNeverRollABuildSource(t *testing.T) {
	for name, tc := range map[string]struct {
		spec  appv1alpha1.AppSpec
		gated bool
	}{
		"git build source, auto-deploy on":      {appv1alpha1.AppSpec{Type: appv1alpha1.TypeBuildSource, Repo: "r", AutoDeploy: true}, true},
		"image build source":                    {appv1alpha1.AppSpec{Type: appv1alpha1.TypeBuildSource, Image: "nginx"}, true},
		"git service, auto-deploy on":           {appv1alpha1.AppSpec{Repo: "r", AutoDeploy: true}, false},
		"git service, auto-deploy off":          {appv1alpha1.AppSpec{Repo: "r"}, true},
		"image service (no auto-deploy toggle)": {appv1alpha1.AppSpec{Image: "nginx"}, false},
	} {
		if got := autoDeployGated(&appv1alpha1.App{Spec: tc.spec}); got != tc.gated {
			t.Errorf("%s: gated = %v, want %v", name, got, tc.gated)
		}
	}
}

// A group save resolves a build source its link set names and leaves it alone:
// no deploy, no rebuild — the carrier reads the group at its next build.
func TestGroupSaveSkipsALinkedBuildSource(t *testing.T) {
	carrier := &appv1alpha1.App{}
	carrier.Name, carrier.Namespace = "bds-1", "default"
	carrier.Labels = map[string]string{core.LabelAppID: "bds-1"}
	carrier.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeBuildSource, Repo: "r", EnvFromSecrets: []string{envSecretName("evg-1")}}
	s := newService(nil, carrier)
	rebuilt := false
	s.RebuildService = func(context.Context, string) error { rebuilt = true; return nil }
	for _, mode := range []SaveMode{SaveModeDeploy, SaveModeRebuild} {
		skipped, err := s.rollLink(context.Background(), "evg-1", "bds-1", mode, store.TriggerConfigChange)
		if err != nil || !skipped {
			t.Fatalf("mode %v: rollLink = (%v, %v), want skipped", mode, skipped, err)
		}
	}
	var after appv1alpha1.App
	if err := s.Client.Get(context.Background(), client.ObjectKeyFromObject(carrier), &after); err != nil {
		t.Fatal(err)
	}
	if rebuilt || after.Spec.RestartedAt != "" {
		t.Fatalf("the group save rolled the build source: rebuilt %v, restartedAt %q", rebuilt, after.Spec.RestartedAt)
	}
}
