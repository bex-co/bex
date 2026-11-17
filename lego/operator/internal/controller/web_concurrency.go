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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/build"
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
// yield to every value the service configures — a literal, a linked group, the
// service's own set or their release snapshots, an explicit empty value
// included — because a container env entry beats every envFrom key.
func (r *AppReconciler) webConcurrency(ctx context.Context, app *appv1alpha1.App) (int, error) {
	if !webConcurrencyEligible(app) {
		return 0, nil
	}
	if _, set, err := r.configuredEnv(ctx, app, webConcurrencyEnvName); err != nil || set {
		return 0, err
	}
	return renderenv.PlanCPUCount(app.Spec.Tier), nil
}

// configuredEnv is the value the service itself gives name, in Kubernetes'
// own precedence: a spec.env literal beats every envFrom key, and among the
// envFrom Secrets (linked groups first, the service's own set last) the last
// wins. set is false when nothing the pod reads names it. A literal that
// references a Secret key resolves through that key.
func (r *AppReconciler) configuredEnv(ctx context.Context, app *appv1alpha1.App, name string) (value string, set bool, err error) {
	for _, e := range app.Spec.Env {
		if e.Name != name {
			continue
		}
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			return e.Value, true, nil
		}
		ref := e.ValueFrom.SecretKeyRef
		secret := &corev1.Secret{}
		if err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: ref.Name}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				return "", true, nil
			}
			return "", false, fmt.Errorf("%s: read Secret %s: %w", name, ref.Name, err)
		}
		return string(secret.Data[ref.Key]), true, nil
	}
	for _, src := range slices.Backward(envFromSources(app)) {
		ref := src.SecretRef
		if ref == nil {
			continue
		}
		secret := &corev1.Secret{}
		err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: ref.Name}, secret)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("%s: read env Secret %s: %w", name, ref.Name, err)
		}
		if v, ok := secret.Data[name]; ok {
			return string(v), true, nil
		}
	}
	return "", false, nil
}

// withWebConcurrency adds the default just before PORT.
func withWebConcurrency(env []corev1.EnvVar, n int) []corev1.EnvVar {
	if n <= 0 {
		return env
	}
	return beforePort(env, corev1.EnvVar{Name: webConcurrencyEnvName, Value: strconv.Itoa(n)})
}

// beforePort inserts v just before PORT, which appEnv always appends last.
func beforePort(env []corev1.EnvVar, v corev1.EnvVar) []corev1.EnvVar {
	if len(env) == 0 {
		return env
	}
	return append(env[:len(env)-1:len(env)-1], v, env[len(env)-1])
}

// servingPortDefault reports whether the app container gets its runtime's
// port variable (build.NativeServingPortEnv): Gunicorn's flags with their
// bind for Python, PASSENGER_PORT for Ruby, ROCKET_PORT for Rust. Render pins
// each to its default port 10000, so a Gunicorn config-file bind, a Passenger
// app or a Rocket app listens where the platform routes with no
// configuration (w8/273, w32/m424). On bex the routed port can be a
// configured or a detected one, and a fixed 10000 would move the app off it,
// so the value follows the serving port: Render's wherever the port is the
// default. Only a web or private service is routed; a worker or cron job
// keeps the image's default, as on Render. Like WEB_CONCURRENCY it must yield
// to every value the service configures, an explicit empty one included,
// because a container env entry beats every envFrom key.
func (r *AppReconciler) servingPortDefault(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	name, _ := build.NativeServingPortEnv(app.Spec.Runtime, 0)
	if name == "" || !app.Spec.InternallyAddressable() {
		return false, nil
	}
	_, set, err := r.configuredEnv(ctx, app, name)
	return !set && err == nil, err
}

// withServingPortDefault adds runtime's port variable, valued for the
// serving port, just before PORT.
func withServingPortDefault(env []corev1.EnvVar, runtime string, port int, add bool) []corev1.EnvVar {
	if !add {
		return env
	}
	name, value := build.NativeServingPortEnv(runtime, port)
	return beforePort(env, corev1.EnvVar{Name: name, Value: value})
}
