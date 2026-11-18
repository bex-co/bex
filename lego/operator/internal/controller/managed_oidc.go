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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Managed OIDC delivery (docs/ADR116-managed-oidc.md D9, w2/m477). A release
// whose environment sets a provider's trigger variable (AWS_ROLE_ARN), in a
// workspace whose plan includes Managed OIDC (LabelManagedOIDC), mounts the
// App's token Secret read-only at /var/lib/render/oidc and gets the
// provider's file variable (AWS_WEB_IDENTITY_TOKEN_FILE) in every container
// bex runs for it: instances, cron runs, the pre-deploy step and the initial
// deploy hook (which copies the instance's template). SSH and shell sessions
// exec into an instance and inherit both. bex-api mints the Secret and keeps
// it fresh; the operator never sees a token. A release that sets no trigger
// gets a pod template without a trace of any of it.

// managedOIDCVolumeName names the token directory's volume.
const managedOIDCVolumeName = "bex-oidc"

// managedOIDC is the Managed OIDC mounts of the release app is projecting.
// A release decides once, when the operator first projects it, from view
// (the release's own environment as the caller projects it: the in-flight
// release, the selected runtime, the cron's effective App; a selected
// release always reads its own) and from the plan label. The decision is
// written to status before any pod of the release exists, so bex-api can
// mint the token before a container waits for it, and it is kept for the
// life of the release: a variable or plan change reaches the service at its
// next deploy, and a pass reads nothing to keep the decision. A release
// already serving when this operator first sees it was rolled out before
// Managed OIDC and mounts nothing until the next deploy, so an upgrade rolls
// no pod.
func (r *AppReconciler) managedOIDC(ctx context.Context, app, view *appv1alpha1.App) ([]appv1alpha1.ManagedOIDCProvider, error) {
	gen := releaseGeneration(app)
	if st := app.Status.ManagedOIDC; st != nil && st.Generation == gen {
		return appv1alpha1.ManagedOIDCProvidersNamed(st.Mounted()), nil
	}
	decision := &appv1alpha1.ManagedOIDCStatus{Generation: gen}
	if app.Status.ActiveRevision != releaseRevision(app) { // not yet serving: a release to decide
		if ref := app.ActiveReleaseConfig(); ref != nil && ref.SourceGeneration > 0 && !canceledOverServed(app) {
			selected, err := r.selectedRuntimeApp(ctx, app)
			if err != nil {
				return nil, err
			}
			view = selected
		}
		for _, p := range appv1alpha1.ManagedOIDCProviders {
			value, set, err := r.configuredEnv(ctx, view, p.Trigger)
			if err != nil {
				return nil, err
			}
			if set && strings.TrimSpace(value) != "" {
				decision.Providers = append(decision.Providers, p.Name)
			}
		}
		decision.PlanAllowed = app.Labels[appv1alpha1.LabelManagedOIDC] == appv1alpha1.ManagedOIDCEnabled
	}
	app.Status.ManagedOIDC = decision
	if err := updateStatusIfChanged(ctx, r.Client, app); err != nil {
		return nil, err
	}
	return appv1alpha1.ManagedOIDCProvidersNamed(decision.Mounted()), nil
}

// recordManagedOIDCOf records the decision a restored template carries: a
// cancel over a served release re-applies that release's recorded template
// verbatim, so its mounts, not a fresh decision, are what the pods read and
// what bex-api must keep minting for.
func recordManagedOIDCOf(app *appv1alpha1.App, tmpl corev1.PodTemplateSpec) {
	names := templateManagedOIDC(tmpl)
	app.Status.ManagedOIDC = &appv1alpha1.ManagedOIDCStatus{Generation: releaseGeneration(app), Providers: names, PlanAllowed: len(names) > 0}
}

// templateManagedOIDC is the providers tmpl's app container reads a token
// file for.
func templateManagedOIDC(tmpl corev1.PodTemplateSpec) []string {
	var names []string
	for _, c := range tmpl.Spec.Containers {
		if c.Name != appContainerName {
			continue
		}
		for _, p := range appv1alpha1.ManagedOIDCProviders {
			if slices.ContainsFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == p.FileEnv && e.Value == p.Path() }) {
				names = append(names, p.Name)
			}
		}
	}
	return names
}

// managedOIDCVolume is the read-only token directory for providers, or
// (nil, nil) for none. The Secret is required and each provider's key is
// listed, so a container does not start before its tokens exist: Render
// writes them "before the service ever starts". The mode is the API server's
// default, spelled out so a template built after its server defaults
// (cronPodSpec) compares equal to the stored one; it leaves every file
// readable by the container's user.
func managedOIDCVolume(app *appv1alpha1.App, providers []appv1alpha1.ManagedOIDCProvider) (*corev1.Volume, *corev1.VolumeMount) {
	if len(providers) == 0 {
		return nil, nil
	}
	items := make([]corev1.KeyToPath, 0, len(providers))
	for _, p := range providers {
		items = append(items, corev1.KeyToPath{Key: p.File, Path: p.File})
	}
	vol := corev1.Volume{
		Name: managedOIDCVolumeName,
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
			SecretName:  appv1alpha1.ManagedOIDCSecretName(app.Name),
			Items:       items,
			DefaultMode: ptr.To(corev1.SecretVolumeSourceDefaultMode),
		}},
	}
	mount := corev1.VolumeMount{Name: managedOIDCVolumeName, MountPath: appv1alpha1.ManagedOIDCMountPath, ReadOnly: true}
	return &vol, &mount
}

// withManagedOIDC sets each provider's file variable just before PORT, which
// appEnv always appends last. A value the service set itself gives way, as on
// Render, which sets the variable and tells users not to: the container entry
// replaces a literal of the same name and beats every envFrom key.
func withManagedOIDC(env []corev1.EnvVar, providers []appv1alpha1.ManagedOIDCProvider) []corev1.EnvVar {
	if len(providers) == 0 || len(env) == 0 {
		return env
	}
	owned := func(e corev1.EnvVar) bool {
		return slices.ContainsFunc(providers, func(p appv1alpha1.ManagedOIDCProvider) bool { return p.FileEnv == e.Name })
	}
	out := slices.DeleteFunc(slices.Clone(env[:len(env)-1]), owned)
	for _, p := range providers {
		out = append(out, corev1.EnvVar{Name: p.FileEnv, Value: p.Path()})
	}
	return append(out, env[len(env)-1])
}

// withManagedOIDCPod adds providers' token directory and file variables to
// spec's app container: an instance's or a cron run's pod template.
func withManagedOIDCPod(spec *corev1.PodSpec, app *appv1alpha1.App, providers []appv1alpha1.ManagedOIDCProvider) {
	vol, mount := managedOIDCVolume(app, providers)
	if vol == nil {
		return
	}
	i := slices.IndexFunc(spec.Containers, func(c corev1.Container) bool { return c.Name == appContainerName })
	if i < 0 {
		return
	}
	c := &spec.Containers[i]
	c.Env = withManagedOIDC(c.Env, providers)
	c.VolumeMounts = append(c.VolumeMounts, *mount)
	spec.Volumes = append(spec.Volumes, *vol)
}
