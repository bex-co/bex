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
	"strconv"

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
// It reads the configuration the projection currently serves. A release that
// changes PORT is snapshotted later in the same pass, so it first rolls out
// still serving on the old port (the injected PORT keeps the process there),
// and the next pass moves it to the new one: two rollouts, never a mismatch
// between the port the process binds and the port the platform routes to.
func (r *AppReconciler) envPort(ctx context.Context, app *appv1alpha1.App, specPort int) (int, error) {
	value, set, err := r.configuredEnv(ctx, app, portEnvName)
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
