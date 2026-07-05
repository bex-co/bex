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

// Package api is the bex control-plane seed: the product front door that turns
// user intent (restart / suspend / resume) into App CR spec patches. It is a
// thin policy layer — the operator does all the mechanism.
//
// Structure: ONE domain layer (Core) with TWO adapters over it (REST and
// GraphQL). Both adapters call the same Core methods, so the two APIs can never
// drift in behavior — they share the single implementation, exactly the pattern
// Render uses (public REST + dashboard GraphQL over one internal service layer).
package api

import (
	"context"
	"errors"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/blockeden/bex/operator/api/v1alpha1"
)

// ErrNotFound is returned when an App does not exist (adapters map it to 404 /
// a GraphQL null+error).
var ErrNotFound = errors.New("app not found")

// AppView is the neutral, bex-native projection of an App — spec intent +
// observed status. Core returns this; each adapter maps it to its own wire
// format (the REST/GraphQL adapters render it in Render's Service shape). Keeping
// Core wire-agnostic is what lets one domain layer feed many API dialects.
type AppView struct {
	Name      string   `json:"name"`
	Phase     string   `json:"phase"`
	URL       string   `json:"url"`
	URLs      []string `json:"urls"`
	Image     string   `json:"image"`
	Replicas  int32    `json:"replicas"`
	Suspended bool     `json:"suspended"`
	Revision  string   `json:"revision"`
	CreatedAt string   `json:"createdAt"`
}

// Core is the shared domain layer. Every product action lives here exactly
// once; REST and GraphQL are presentation only.
type Core struct {
	Client    client.Client
	Namespace string
	// Now supplies the restart timestamp; injectable for tests. Defaults to
	// time.Now if nil.
	Now func() time.Time
}

func (c *Core) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func view(a *appv1alpha1.App) AppView {
	created := ""
	if !a.CreationTimestamp.IsZero() {
		created = a.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	return AppView{
		Name:      a.Name,
		Phase:     string(a.Status.Phase),
		URL:       a.Status.URL,
		URLs:      a.Status.URLs,
		Image:     a.Status.Image,
		Replicas:  a.Spec.Replicas,
		Suspended: a.Spec.Suspended,
		Revision:  a.Status.ActiveRevision,
		CreatedAt: created,
	}
}

// List returns every App in the namespace.
func (c *Core) List(ctx context.Context) ([]AppView, error) {
	var list appv1alpha1.AppList
	if err := c.Client.List(ctx, &list, client.InNamespace(c.Namespace)); err != nil {
		return nil, err
	}
	out := make([]AppView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, view(&list.Items[i]))
	}
	return out, nil
}

// Get returns one App, or ErrNotFound.
func (c *Core) Get(ctx context.Context, name string) (AppView, error) {
	a, err := c.fetch(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	return view(a), nil
}

// Restart requests a rolling restart (spec.restartedAt = now). The operator
// stamps the pod template and Kubernetes rolls the pods with no downtime.
func (c *Core) Restart(ctx context.Context, name string) (AppView, error) {
	return c.patch(ctx, name, func(a *appv1alpha1.App) {
		a.Spec.RestartedAt = c.now().UTC().Format(time.RFC3339)
	})
}

// Suspend parks the App (spec.suspended = true): scaled to 0, host/certs kept.
func (c *Core) Suspend(ctx context.Context, name string) (AppView, error) {
	return c.patch(ctx, name, func(a *appv1alpha1.App) { a.Spec.Suspended = true })
}

// Resume brings a suspended App back (spec.suspended = false); the operator
// restores spec.replicas.
func (c *Core) Resume(ctx context.Context, name string) (AppView, error) {
	return c.patch(ctx, name, func(a *appv1alpha1.App) { a.Spec.Suspended = false })
}

func (c *Core) fetch(ctx context.Context, name string) (*appv1alpha1.App, error) {
	var a appv1alpha1.App
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: name}, &a)
	if apierrors.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// patch fetches the App, applies mutate to its spec, and merge-patches — only
// spec fields change; the operator reconciles the rest. This is the single
// write path both adapters share.
func (c *Core) patch(ctx context.Context, name string, mutate func(*appv1alpha1.App)) (AppView, error) {
	a, err := c.fetch(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	base := client.MergeFrom(a.DeepCopy())
	mutate(a)
	if err := c.Client.Patch(ctx, a, base); err != nil {
		return AppView{}, err
	}
	return view(a), nil
}
