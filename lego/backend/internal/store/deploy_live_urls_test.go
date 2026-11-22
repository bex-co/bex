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

package store

import (
	"context"
	"fmt"
	"slices"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const platformURL = "https://web.onbex.co"

// served is a web service the operator serves at the platform URL and hosts,
// with status.urls in EffectiveHosts order (spec.host, the platform host,
// spec.hosts) and wildcards dropped, as setStatusURLs writes them.
func served(host string, hosts []string, redirects map[string]string) *appv1alpha1.App {
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeWebService, Expose: true, Host: host, Hosts: hosts, HostRedirects: redirects,
	}}
	for _, h := range append([]string{host, "web.onbex.co"}, hosts...) {
		if h != "" && !appv1alpha1.IsWildcardHost(h) {
			app.Status.URLs = append(app.Status.URLs, "https://"+h)
		}
	}
	if len(app.Status.URLs) > 0 {
		app.Status.URL = app.Status.URLs[0]
	}
	return app
}

// w32/m567: Render's primary URL is a custom domain once the service has one,
// and its block counts every other domain the service answers at, the
// redirecting side of a www<->apex pair and the platform subdomain included.
func TestLiveURLsNameRendersPrimaryFirst(t *testing.T) {
	for _, c := range []struct {
		name string
		app  *appv1alpha1.App
		want []string
	}{
		{"the platform URL when there is no custom domain", served("", nil, nil), []string{platformURL}},
		{"the first custom domain over the platform URL",
			served("", []string{"shop.example", "blog.example"}, nil),
			[]string{"https://shop.example", platformURL, "https://blog.example"}},
		{"a primary domain (spec.host) over the rest",
			served("main.example", []string{"shop.example"}, nil),
			[]string{"https://main.example", platformURL, "https://shop.example"}},
		{"the root over its www redirect",
			served("", []string{"teranga.example", "www.teranga.example"}, map[string]string{"www.teranga.example": "teranga.example"}),
			[]string{"https://teranga.example", platformURL, "https://www.teranga.example"}},
		{"www over the root when the root redirects",
			served("", []string{"teranga.example", "www.teranga.example"}, map[string]string{"teranga.example": "www.teranga.example"}),
			[]string{"https://www.teranga.example", platformURL, "https://teranga.example"}},
		{"no wildcard: the next custom domain is the primary",
			served("", []string{"*.wild.example", "shop.example"}, nil),
			[]string{"https://shop.example", platformURL}},
		{"no wildcard: the platform URL when it is the only custom domain",
			served("", []string{"*.wild.example"}, nil), []string{platformURL}},
		{"a custom domain the operator does not serve yet is not the primary", func() *appv1alpha1.App {
			app := served("", nil, nil)
			app.Spec.Hosts = []string{"pending.example"}
			return app
		}(), []string{platformURL}},
		{"the platform subdomain disabled", func() *appv1alpha1.App {
			app := served("", []string{"shop.example"}, nil)
			app.Status.URLs, app.Status.URL = []string{"https://shop.example"}, "https://shop.example"
			return app
		}(), []string{"https://shop.example"}},
		{"nothing when no public URL is served", func() *appv1alpha1.App {
			app := served("", nil, nil)
			// The operator's internal URL, with no public host (setServingURLs).
			app.Status.URLs, app.Status.URL = nil, "http://web.tenant.svc:10000"
			return app
		}(), nil},
		{"a web service of the empty-type default", func() *appv1alpha1.App {
			app := served("", nil, nil)
			app.Spec.Type = ""
			return app
		}(), []string{platformURL}},
	} {
		if got := liveURLs(c.app); !slices.Equal(got, c.want) {
			t.Errorf("%s: liveURLs = %q, want %q", c.name, got, c.want)
		}
	}
	// A static site's log ends at its live line on Render, and a private
	// service, a worker and a cron job have no public URL.
	for _, typ := range []string{appv1alpha1.TypeStaticSite, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob} {
		app := served("", []string{"shop.example"}, nil)
		app.Spec.Type = typ
		if got := liveURLs(app); got != nil {
			t.Errorf("%s: liveURLs = %q, want none", typ, got)
		}
	}
}

// The live close records where the deploy went live on its own row, so a
// domain the service gets later leaves that deploy's record alone, and the
// next deploy that goes live names the domain as its primary.
func TestLiveDeployRecordsTheURLsItWentLiveAt(t *testing.T) {
	ctx := context.Background()
	reconciler, st, cl := newTestReconciler(t)
	tenant, _ := st.CreateTenant(ctx, "live-urls", "free")
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "nginx:alpine", Type: appv1alpha1.TypeWebService, Port: 10000, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	history, _ := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if len(history) != 1 {
		t.Fatalf("create deploys=%v", history)
	}
	first := history[0]
	if first.LiveURLs != nil {
		t.Fatalf("an open row records URLs: %q", first.LiveURLs)
	}
	goLive := func(deploy Deploy, urls ...string) {
		t.Helper()
		app := getApp(t, cl)
		app.Status.Phase, app.Status.Image, app.Status.ReleaseGeneration = appv1alpha1.PhaseRunning, "nginx@sha256:a", deploy.Generation
		app.Status.ActiveRevision = fmt.Sprintf("rev-%d", deploy.Generation)
		app.Status.URL, app.Status.URLs = urls[0], urls
		if err := cl.Status().Update(ctx, app); err != nil {
			t.Fatal(err)
		}
		if err := reconciler.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	goLive(first, platformURL)
	if first = mustGetDeploy(t, st, row.ID, first.ID); first.Status != DeployLive || !slices.Equal(first.LiveURLs, []string{platformURL}) {
		t.Fatalf("first = %s, %q", first.Status, first.LiveURLs)
	}

	if _, err := st.CreateDomain(ctx, row.ID, "shop.example", false); err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if hosts := getApp(t, cl).Spec.Hosts; !slices.Equal(hosts, []string{"shop.example"}) {
		t.Fatalf("spec.hosts = %q, want the verified domain", hosts)
	}
	// The operator serves the domain before any new deploy goes live.
	goLive(first, platformURL, "https://shop.example")
	if first = mustGetDeploy(t, st, row.ID, first.ID); !slices.Equal(first.LiveURLs, []string{platformURL}) {
		t.Fatalf("a domain added after the deploy went live rewrote its record: %q", first.LiveURLs)
	}

	second, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "nginx:alpine", first.Generation+1, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	goLive(second, platformURL, "https://shop.example")
	first, second = mustGetDeploy(t, st, row.ID, first.ID), mustGetDeploy(t, st, row.ID, second.ID)
	if first.Status != DeployDeactivated || !slices.Equal(first.LiveURLs, []string{platformURL}) {
		t.Fatalf("the replaced deploy = %s, %q, want its own record", first.Status, first.LiveURLs)
	}
	if !slices.Equal(second.LiveURLs, []string{"https://shop.example", platformURL}) {
		t.Fatalf("the next deploy recorded %q, want the domain as its primary", second.LiveURLs)
	}
}
