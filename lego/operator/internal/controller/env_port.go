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
	"slices"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// envPort is the port a configured (non-image-port) service serves on. Render
// lets a service set PORT, from its own variables or a linked group's, and
// routes to it (w5/m149; ADR018 row "PORT env var"). So when the service
// configures a valid PORT, the operator serves, routes and probes on that
// port instead of spec.port, and records it as status.envPort for every
// reader (App.ServingPort). An unusable value is ignored with a log line, and
// spec.port applies, rather than taking the service off the network.
//
// Historical selections read their recorded inline values and selected Secret
// sources even before their new record is materialized. Saved configuration
// stays unchanged. Ordinary deployments retain the existing snapshot projection
// until their new inputs are snapshotted later in the pass.
func (r *AppReconciler) envPort(ctx context.Context, app *appv1alpha1.App, specPort int) (int, error) {
	runtime, err := r.portRuntimeApp(ctx, app)
	if err != nil {
		return 0, err
	}
	value, set, err := r.configuredEnv(ctx, runtime, portEnvName)
	if err != nil {
		return 0, err
	}
	app.Status.EnvPort = 0
	if !set {
		return specPort, nil
	}
	port, perr := strconv.Atoi(value)
	if perr != nil || port < 1 || port > 65535 || appv1alpha1.IsReservedImagePort(int32(port)) {
		logf.FromContext(ctx).Info("ignoring an unusable PORT; serving on the service port", "app", app.Name, "value", value, "port", specPort)
		return specPort, nil
	}
	app.Status.EnvPort = int32(port)
	return port, nil
}

// portRuntimeApp selects only the inputs PORT will read; it never materializes
// configuration or changes the saved App. envPort runs before that commit point.
func (r *AppReconciler) portRuntimeApp(ctx context.Context, app *appv1alpha1.App) (*appv1alpha1.App, error) {
	if heldRelease(app) {
		return r.selectedRuntimeApp(ctx, app)
	}
	ref := app.ActiveReleaseConfig()
	if ref == nil || ref.SourceGeneration == 0 || canceledOverServed(app) {
		return app, nil
	}
	runtime, err := r.selectedRuntimeApp(ctx, app)
	if err == nil {
		runtime.Status.ConfigSnapshotGeneration = ref.Generation
		return runtime, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	// The selected release has not copied its sources yet. Read the same
	// names copySelectedSources will consume, including live rollback groups
	// and historical own/restart values, without waiting for those copies.
	record, err := r.readRuntimeConfigRecord(ctx, app, ref.SourceGeneration)
	if err != nil {
		return nil, fmt.Errorf("read selected release %d: %w", ref.SourceGeneration, err)
	}
	runtime = record.projectRuntimeApp(app, ref.SourceGeneration)
	runtime.Status.ConfigSnapshotGeneration = 0
	runtime.Spec.EnvFromSecrets = slices.Clone(runtime.Spec.EnvFromSecrets)
	for i, source := range runtime.Spec.EnvFromSecrets {
		runtime.Spec.EnvFromSecrets[i] = selectedSourceName(app, ref, record, source)
	}
	if source := runtime.Spec.EnvFromSecret; source != "" {
		runtime.Spec.EnvFromSecret = selectedSourceName(app, ref, record, source)
	}
	return runtime, nil
}
