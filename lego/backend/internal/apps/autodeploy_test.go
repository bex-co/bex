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

package apps

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestSetAutoDeployFlipsFlagWithoutRebuild(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/x/app", AutoDeploy: true},
	}
	svc, cl := newService(nil, app)

	// Turn it off — the flag flips, and no restartedAt bump (a toggle isn't a deploy).
	if _, err := svc.SetAutoDeploy(context.Background(), "web", false); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.AutoDeploy {
		t.Error("autoDeploy should be false after SetAutoDeploy(false)")
	}
	if got.Spec.RestartedAt != "" {
		t.Error("flipping autoDeploy must not trigger a redeploy (no restartedAt)")
	}

	// Turn it back on — survives the round-trip.
	if _, err := svc.SetAutoDeploy(context.Background(), "web", true); err != nil {
		t.Fatal(err)
	}
	if !getApp(t, cl, "web").Spec.AutoDeploy {
		t.Error("autoDeploy should be true after SetAutoDeploy(true)")
	}
}
