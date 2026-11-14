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

// w21/m394 t011 (R25-01): a parked App's hibernated() wrote only Status.URL, and
// only when a host remained, so a host the owner dropped while the App was
// suspended or asleep stayed in Status.URLs. The activator indexes Status.URLs,
// so a later claimant of that host was woken, held and proxied to the former
// holder's app.

const (
	hostKeep = "keep.example.com"
	hostGone = "gone.example.com"
)

// withStaleURLs marks the App as having last served on web.onbex.co plus both
// custom hosts, as its Running pass left status.
func withStaleURLs(app *appv1alpha1.App) *appv1alpha1.App {
	app.Spec.Hosts = []string{hostKeep, hostGone}
	app.Status.URL = "https://web.onbex.co"
	app.Status.URLs = []string{"https://web.onbex.co", "https://" + hostKeep, "https://" + hostGone}
	return app
}

func TestParkedAppStatusURLsFollowTheSpec(t *testing.T) {
	for _, tc := range []struct {
		name     string
		app      func() *appv1alpha1.App
		drop     func(*appv1alpha1.App)
		wantURL  string
		wantURLs []string
		wantNone bool // no public host remains: the internal URL, no URLs
	}{
		{
			name:     "suspended, one custom host dropped",
			app:      func() *appv1alpha1.App { return withStaleURLs(suspendedWebApp()) },
			drop:     func(a *appv1alpha1.App) { a.Spec.Hosts = []string{hostKeep} },
			wantURL:  "https://web.onbex.co",
			wantURLs: []string{"https://web.onbex.co", "https://" + hostKeep},
		},
		{
			name: "suspended, platform host dropped by the subdomain policy",
			app:  func() *appv1alpha1.App { return withStaleURLs(suspendedWebApp()) },
			drop: func(a *appv1alpha1.App) {
				a.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyDisabled
				a.Spec.Hosts = []string{hostKeep}
			},
			wantURL:  "https://" + hostKeep,
			wantURLs: []string{"https://" + hostKeep},
		},
		{
			name: "suspended, every public host dropped",
			app:  func() *appv1alpha1.App { return withStaleURLs(suspendedWebApp()) },
			drop: func(a *appv1alpha1.App) {
				a.Spec.Expose = false
				a.Spec.Hosts = nil
			},
			wantNone: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.app()
			r, cl, nn := lifecycleFixture(t, app)
			reconcileTwice(t, r, nn)
			updateApp(t, cl, nn, tc.drop)
			reconcileTwice(t, r, nn)

			live := liveApp(t, cl, nn)
			if live.Status.Phase != appv1alpha1.PhaseHibernated {
				t.Fatalf("phase = %q, want Hibernated", live.Status.Phase)
			}
			if tc.wantNone {
				if live.Status.URLs != nil || live.Status.URL != internalURL(&live) {
					t.Fatalf("URL %q URLs %v, want the internal URL %q and no URLs", live.Status.URL, live.Status.URLs, internalURL(&live))
				}
				return
			}
			if live.Status.URL != tc.wantURL || !slices.Equal(live.Status.URLs, tc.wantURLs) {
				t.Fatalf("URL %q URLs %v, want %q %v", live.Status.URL, live.Status.URLs, tc.wantURL, tc.wantURLs)
			}
		})
	}
}

// An idle auto-hibernated App takes the same status write as a suspended one.
func TestAutoHibernatedAppStatusURLsFollowTheSpec(t *testing.T) {
	app := activeApp("tea-abc123")
	app.Spec.Hosts = []string{hostKeep, hostGone}
	r, cl, nn := lifecycleFixture(t, app)
	serveReleaseOne(t, r, cl, nn)
	if got := liveApp(t, cl, nn).Status.URLs; !slices.Contains(got, "https://"+hostGone) {
		t.Fatalf("setup: running URLs = %v, want %s among them", got, hostGone)
	}
	parkIdle(t, r, cl, nn)
	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Spec.Hosts = []string{hostKeep} })
	reconcileTwice(t, r, nn)

	live := liveApp(t, cl, nn)
	if live.Status.Phase != appv1alpha1.PhaseHibernated {
		t.Fatalf("phase = %q, want Hibernated", live.Status.Phase)
	}
	want := []string{"https://web.onbex.co", "https://" + hostKeep}
	if !slices.Equal(live.Status.URLs, want) {
		t.Fatalf("URLs = %v, want %v", live.Status.URLs, want)
	}
}
