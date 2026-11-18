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

// Package managedoidc keeps every opted-in service's Managed OIDC tokens
// fresh (docs/ADR116-managed-oidc.md D9, w2/m477). The operator decides, per
// release, which providers a service's pods mount (status.managedOIDC); this
// worker mints those providers' tokens through the workspace's issuer and
// writes them to the App's <app>-oidc Secret, replacing each before a third
// of its life has passed. One bex-api replica writes at a time.
package managedoidc

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/oidcissuer"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	// PassInterval is how often a pass checks every opted-in App. A release
	// that starts mounting a token kicks a pass at once (Kick), so this is
	// the backstop and the refresh clock, not the first token's latency.
	PassInterval = time.Minute
	// RefreshAfter is the token age at which a pass replaces it: a third of
	// its 60 minutes, so a refresh that fails for half an hour still leaves
	// the running file valid, and the kubelet's Secret propagation (about a
	// minute) never hands a container a token close to expiry.
	RefreshAfter = 20 * time.Minute
	// MaxMintsPerPass bounds one pass's writes. A pass that hits it kicks the
	// next at once, so a backlog drains without waiting a minute per batch.
	MaxMintsPerPass = 100

	// exclusiveName is the advisory lock that keeps passes single-writer
	// across replicas.
	exclusiveName = "managed-oidc-minter"

	// annotationIssuedAt is the Unix second the Secret's tokens were signed.
	annotationIssuedAt = "app.bex.co/oidc-issued-at"
	// annotationSubject is the sub claim the Secret's tokens carry, so a
	// service that moves to another environment gets a new token at once.
	annotationSubject = "app.bex.co/oidc-subject"

	managedBy = "bex-api"
)

// Minter is the token refresh worker.
type Minter struct {
	Client client.Client
	Issuer *oidcissuer.Issuer
	// Exclusive runs fn only where no other replica runs it
	// (store.PGStore.RunExclusive). nil runs every pass: a single replica.
	Exclusive func(ctx context.Context, name string, fn func(context.Context) error) (bool, error)
	Metrics   *Metrics
	Now       func() time.Time // nil means time.Now

	wake chan struct{}
}

// NewMinter returns a Minter whose Kick wakes Run.
func NewMinter(cl client.Client, issuer *oidcissuer.Issuer) *Minter {
	return &Minter{Client: cl, Issuer: issuer, wake: make(chan struct{}, 1)}
}

// Kick asks for a pass ahead of the next tick: the operator recorded a
// decision (the App status watch), so a pod may be waiting for its token.
// It never blocks; kicks during a pass coalesce into one more pass.
func (m *Minter) Kick() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Run runs a pass every PassInterval and on every Kick until ctx ends.
func (m *Minter) Run(ctx context.Context) {
	core.PollWake(ctx, "managed oidc: minter", PassInterval, m.wake, m.Pass)
}

func (m *Minter) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Pass refreshes every due token, on one replica at a time.
func (m *Minter) Pass(ctx context.Context) error {
	if m.Exclusive == nil {
		return m.pass(ctx)
	}
	_, err := m.Exclusive(ctx, exclusiveName, m.pass)
	return err
}

func (m *Minter) pass(ctx context.Context) error {
	var apps appv1alpha1.AppList
	if err := m.Client.List(ctx, &apps, client.MatchingLabels{appv1alpha1.LabelManagedOIDC: appv1alpha1.ManagedOIDCEnabled}); err != nil {
		return fmt.Errorf("list opted-in Apps: %w", err)
	}
	now := m.now()
	var errs []error
	minted, oldest := 0, time.Duration(0)
	for i := range apps.Items {
		app := &apps.Items[i]
		providers := appv1alpha1.ManagedOIDCProvidersNamed(app.Status.ManagedOIDC.Mounted())
		if len(providers) == 0 || app.DeletionTimestamp != nil {
			continue
		}
		if minted >= MaxMintsPerPass {
			m.Kick()
			break
		}
		age, wrote, err := m.ensure(ctx, app, providers, now)
		if err != nil {
			m.Metrics.failed()
			errs = append(errs, fmt.Errorf("%s/%s: %w", app.Namespace, app.Name, err))
		}
		if wrote {
			minted++
		}
		oldest = max(oldest, age)
	}
	m.Metrics.observe(oldest)
	return errors.Join(errs...)
}

// subjectOf is the sub claim of app's tokens, from the labels bex-api
// stamps on every App it manages: its workspace, its environment (none:
// Render's "default") and its service id.
func subjectOf(app *appv1alpha1.App) oidcissuer.Subject {
	return oidcissuer.Subject{
		Workspace:   app.Labels[core.LabelTenant],
		Environment: app.Labels[core.LabelEnvironment],
		Service:     app.Labels[core.LabelAppID],
	}
}

// ensure writes app's tokens when its Secret is missing, stale, for another
// subject or for other providers; CreateOrUpdate writes nothing for tokens
// that are still fresh. age is the age of the tokens the service reads after
// the call; wrote reports a write.
func (m *Minter) ensure(ctx context.Context, app *appv1alpha1.App, providers []appv1alpha1.ManagedOIDCProvider, now time.Time) (age time.Duration, wrote bool, err error) {
	sub := subjectOf(app)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: app.Namespace, Name: appv1alpha1.ManagedOIDCSecretName(app.Name)}}
	var held, issued time.Time // the tokens the service had, and the new ones
	op, err := controllerutil.CreateOrUpdate(ctx, m.Client, secret, func() error {
		var fresh bool
		if held, fresh = freshAt(secret, sub, providers, now); fresh {
			issued = held
			return nil
		}
		data := make(map[string][]byte, len(providers))
		for _, p := range providers {
			tok, err := m.Issuer.Mint(sub, p.Audience)
			if err != nil {
				return fmt.Errorf("mint %s token: %w", p.Name, err)
			}
			data[p.File] = []byte(tok.Raw)
			issued = tok.IssuedAt
		}
		fill(secret, app, sub, issued, data)
		return nil
	})
	if err != nil {
		if held.IsZero() {
			return 0, false, err
		}
		return now.Sub(held), false, err
	}
	return now.Sub(issued), op != controllerutil.OperationResultNone, nil
}

// freshAt reports secret's signing time when its tokens are the ones app
// needs now: the same subject, exactly providers' keys, and younger than
// RefreshAfter.
func freshAt(secret *corev1.Secret, sub oidcissuer.Subject, providers []appv1alpha1.ManagedOIDCProvider, now time.Time) (time.Time, bool) {
	issued, ok := issuedAt(secret)
	if !ok || secret.Annotations[annotationSubject] != sub.String() || now.Sub(issued) >= RefreshAfter {
		return issued, false
	}
	want := make([]string, 0, len(providers))
	for _, p := range providers {
		if len(secret.Data[p.File]) == 0 {
			return issued, false
		}
		want = append(want, p.File)
	}
	return issued, slices.Equal(slices.Sorted(maps.Keys(secret.Data)), slices.Sorted(slices.Values(want)))
}

func issuedAt(secret *corev1.Secret) (time.Time, bool) {
	sec, err := strconv.ParseInt(secret.Annotations[annotationIssuedAt], 10, 64)
	if err != nil || sec <= 0 {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}

// fill sets the Secret's tokens and metadata. The App owns it, so deleting
// the service deletes its tokens, but not as controller: the operator's App
// watch must not wake on every refresh. It is protected from tenant mounts:
// another service naming it in its own env would act as this service.
func fill(secret *corev1.Secret, app *appv1alpha1.App, sub oidcissuer.Subject, issued time.Time, data map[string][]byte) {
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels["app.kubernetes.io/managed-by"] = managedBy
	secret.Labels[appv1alpha1.LabelApp] = app.Name
	secret.Labels[appv1alpha1.LabelProtectedFromTenantMount] = appv1alpha1.ProtectedFromTenantMount
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[annotationIssuedAt] = strconv.FormatInt(issued.Unix(), 10)
	secret.Annotations[annotationSubject] = sub.String()
	secret.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: appv1alpha1.SchemeGroupVersion.String(),
		Kind:       "App",
		Name:       app.Name,
		UID:        app.UID,
	}}
	secret.Type = corev1.SecretTypeOpaque
	secret.Data = data
}
