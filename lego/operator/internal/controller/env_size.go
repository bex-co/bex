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
	"fmt"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// maxMergedEnvBytes keeps a release's merged environment under the kernel's
// ARG_MAX for the default 8 MiB stack: 2 MiB of argv and envp strings plus
// their pointers. The headroom covers the command line and the platform's own
// variables. bex-api caps each variable set at 512 KiB, but a service and its
// linked groups are capped separately, so together they can pass this, and
// then no instance of the release can exec (w4/423).
const maxMergedEnvBytes = 2<<20 - 128<<10

// mergedEnvBytes is what execve counts for env: each KEY=value string, its
// NUL terminator, and its pointer.
func mergedEnvBytes(env map[string][]byte) int {
	n := 0
	for k, v := range env {
		n += len(k) + len("=") + len(v) + len("\x00") + 8
	}
	return n
}

// checkMergedEnvSize refuses a release whose merged environment no process
// could start with, before its configuration is snapshotted or its template
// rolls, so the serving release keeps running and the deploy names why. It
// reads the mutable sources in envFrom precedence (later keys win); one that
// is absent contributes nothing, as the snapshot treats it.
func (r *AppReconciler) checkMergedEnvSize(ctx context.Context, app *appv1alpha1.App) error {
	var sources []buildSecretSource
	for _, name := range app.Spec.EnvFromSecrets {
		if name != "" && !appv1alpha1.IsReleaseSnapshotName(name) {
			sources = append(sources, buildSecretSource{name: name, optional: true})
		}
	}
	if name := runtimeEnvSecret(app); name != "" && !appv1alpha1.IsReleaseSnapshotName(name) {
		sources = append(sources, buildSecretSource{name: name, optional: true})
	}
	if len(sources) == 0 {
		return nil
	}
	env, err := r.readBuildSecretSources(ctx, app, "env", sources)
	if err != nil {
		return err
	}
	if size := mergedEnvBytes(env); size > maxMergedEnvBytes {
		return fmt.Errorf("the service's environment (its own variables and every linked environment group) totals %d bytes, over the %d bytes a process can start with; move large values into secret files or unlink a group", size, maxMergedEnvBytes)
	}
	return nil
}
