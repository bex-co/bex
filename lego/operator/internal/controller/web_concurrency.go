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
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/types/renderenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WEB_CONCURRENCY is Render's plan-derived worker count (w5/m147; ADR060):
// Gunicorn, Uvicorn and Rails' generated puma.rb read it, so without it a
// paid-plan app migrated from Render runs one worker on the CPUs it pays for.
// Render sets it at runtime only, on web and private services created after
// its own cutoff, to the plan's CPU count rounded up (1 on 0.5 CPU, 2 on 2).
const webConcurrencyEnvName = "WEB_CONCURRENCY"

// webConcurrencyCutoff mirrors Render's: only services created after this
// shipped get the default, so a running service's worker count (and memory)
// never changes under it on an operator upgrade.
var webConcurrencyCutoff = time.Date(2026, time.October, 21, 0, 0, 0, 0, time.UTC)

// webConcurrencyEligible is the type and cutoff half of the rule.
func webConcurrencyEligible(app *appv1alpha1.App) bool {
	return app.Spec.InternallyAddressable() && !app.CreationTimestamp.Time.Before(webConcurrencyCutoff)
}

// webConcurrency is the value the app container gets, or 0 for none. It must
// yield to every value the service configures, but a container env entry
// beats every envFrom key, so the operator injects it only when no literal
// and no envFrom Secret the pod reads (linked groups, the service's own set,
// or their release snapshots) already names it — an explicit empty value
// included.
func (r *AppReconciler) webConcurrency(ctx context.Context, app *appv1alpha1.App) (int, error) {
	if !webConcurrencyEligible(app) {
		return 0, nil
	}
	for _, e := range app.Spec.Env {
		if e.Name == webConcurrencyEnvName {
			return 0, nil
		}
	}
	for _, src := range envFromSources(app) {
		if src.SecretRef == nil {
			continue
		}
		secret := &corev1.Secret{}
		err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: src.SecretRef.Name}, secret)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("%s: read env Secret %s: %w", webConcurrencyEnvName, src.SecretRef.Name, err)
		}
		if _, ok := secret.Data[webConcurrencyEnvName]; ok {
			return 0, nil
		}
	}
	return renderenv.PlanCPUCount(app.Spec.Tier), nil
}

// withWebConcurrency adds the default just before PORT, which appEnv always
// appends last.
func withWebConcurrency(env []corev1.EnvVar, n int) []corev1.EnvVar {
	if n <= 0 || len(env) == 0 {
		return env
	}
	v := corev1.EnvVar{Name: webConcurrencyEnvName, Value: strconv.Itoa(n)}
	return append(env[:len(env)-1:len(env)-1], v, env[len(env)-1])
}
