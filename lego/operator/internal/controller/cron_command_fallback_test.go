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
	"slices"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCronCommandFallbackThroughTheCarryingRelease pins w4/m194's runtime
// half: a Blueprint image cron keeps its command in spec.startCommand, which
// cronPodSpec runs while spec.command is empty. Once the backend clears both
// slots, the release recorded from that spec carries no override, and the job
// template selected from it runs the image's own command — even when the App
// being reconciled still holds the old fallback. A native cron's start command
// is baked into its image and never becomes a runtime shell override.
func TestCronCommandFallbackThroughTheCarryingRelease(t *testing.T) {
	const script = "sh -c 'echo qa-a48-start'"
	cron := func(mutate func(*appv1alpha1.App)) *appv1alpha1.App {
		return projectionApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeCronJob
			a.Spec.Runtime = "image"
			a.Spec.Image = "docker.io/library/busybox:1.37"
			a.Spec.StartCommand = script
			if mutate != nil {
				mutate(a)
			}
		})
	}
	command := func(app *appv1alpha1.App) []string {
		return (&AppReconciler{}).cronPodSpec(app, "busybox", 0, nil).Spec.Containers[0].Command
	}

	if got := command(cron(nil)); !slices.Equal(got, []string{shellBinary, "-c", script}) {
		t.Errorf("unedited image cron command = %v, want its configured fallback", got)
	}

	cleared := cron(func(a *appv1alpha1.App) { a.Spec.StartCommand = "" })
	rec := &runtimeConfigRecord{spec: releaseRecordSpec(cleared)}
	selected := rec.projectRuntimeApp(cron(nil), &appv1alpha1.ReleaseConfigReference{Generation: 2})
	if selected.Spec.Command != "" || selected.Spec.StartCommand != "" {
		t.Errorf("carrying release spec command=%q startCommand=%q, want no override", selected.Spec.Command, selected.Spec.StartCommand)
	}
	if got := command(selected); got != nil {
		t.Errorf("cleared cron template command = %v, want the image default", got)
	}

	native := cron(func(a *appv1alpha1.App) {
		a.Spec.Runtime, a.Spec.Image, a.Spec.Repo = "node", "", "https://github.com/acme/jobs"
	})
	if got := command(native); got != nil {
		t.Errorf("native cron command = %v, want its baked start command left to the image", got)
	}
}
