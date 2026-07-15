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

// bex-activator is the shared interstitial responder for two App states the
// Ingress can route here instead of the app's own Service: (1) auto-hibernated
// free-tier apps — it wakes the app on the next request and returns 503
// Retry-After so the client retries once the pod is ready; (2) maintenance
// mode (docs/render-artifacts/maintenance-mode.md) — it serves the
// maintenance page (default or spec.maintenanceMode.uri, fetched and served)
// and returns 503 without touching replicas or last-active, since the pods
// are meant to keep running untouched. A request is resolved to an App by
// Host, then maintenance mode is checked first (it takes priority — the
// operator suppresses auto-hibernate while maintenance is enabled, so the two
// states shouldn't coincide in steady state, but maintenance still wins any
// transient overlap since it's the state a human deliberately requested).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const annotLastActive = "app.bex.co/last-active"

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(zap.New())
	log := ctrl.Log.WithName("activator")

	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "activator: k8s client: %v\n", err)
		os.Exit(1)
	}

	addr := ":8888"
	if v := os.Getenv("BEX_ACTIVATOR_ADDR"); v != "" {
		addr = v
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err2 := net.SplitHostPort(host); err2 == nil {
			host = h
		}

		ctx := r.Context()
		app, err2 := findAppByHost(ctx, c, host)
		if err2 != nil {
			log.Error(err2, "listing apps", "host", host)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		if app == nil {
			log.Info("no hibernated app found for host", "host", host)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		if app.Spec.MaintenanceMode != nil && app.Spec.MaintenanceMode.Enabled {
			log.Info("serving maintenance page", "app", app.Name, "host", host)
			serveMaintenancePage(w, r, app.Spec.MaintenanceMode.URI)
			return
		}

		// Touch last-active so the operator's shouldAutoHibernate returns false
		// on the next reconcile, allowing the Ingress to switch back to the app.
		now := time.Now().UTC().Format(time.RFC3339)
		base := app.DeepCopy()
		if app.Annotations == nil {
			app.Annotations = map[string]string{}
		}
		app.Annotations[annotLastActive] = now
		if err2 := c.Patch(ctx, app, client.MergeFrom(base)); err2 != nil {
			log.Error(err2, "patching last-active annotation", "app", app.Name)
		}

		// Bump Deployment replicas to 1 — this is what triggers the operator
		// reconcile (via Owns watch) that switches the Ingress back to the app.
		dep := &appsv1.Deployment{}
		if err2 := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, dep); err2 == nil {
			one := int32(1)
			depBase := dep.DeepCopy()
			dep.Spec.Replicas = &one
			if err2 := c.Patch(ctx, dep, client.MergeFrom(depBase)); err2 != nil {
				log.Error(err2, "patching deployment replicas", "app", app.Name)
			}
		}

		log.Info("waking app", "app", app.Name, "host", host)
		respondNegotiated(w, r, "5", loadingPage, `{"error":"service hibernated","retryAfter":5}`)
	})

	log.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "activator: %v\n", err)
		os.Exit(1)
	}
}

// findAppByHost returns the first App whose status URL matches the given host,
// or nil if none is found (not an error — the host may not be a bex app).
func findAppByHost(ctx context.Context, c client.Client, host string) (*appv1alpha1.App, error) {
	var list appv1alpha1.AppList
	if err := c.List(ctx, &list); err != nil {
		return nil, err
	}
	for i := range list.Items {
		app := &list.Items[i]
		for _, u := range app.Status.URLs {
			if trimScheme(u) == host {
				return app, nil
			}
		}
		if trimScheme(app.Status.URL) == host {
			return app, nil
		}
	}
	return nil, nil
}

func trimScheme(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return u
}

// respondNegotiated writes a 503 with retryAfter, content-negotiated between
// an HTML interstitial (browsers) and a JSON error (API clients) — the shape
// both the wake-on-request and default-maintenance-page responses share.
func respondNegotiated(w http.ResponseWriter, r *http.Request, retryAfter, html, json string) {
	w.Header().Set("Retry-After", retryAfter)
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, html)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, json)
	}
}

// maintenanceFetchTimeout bounds how long serveMaintenancePage waits on a
// custom uri before treating it as a fetch failure.
const maintenanceFetchTimeout = 5 * time.Second

// errBlockedAddr is returned by maintenanceFetchClient's dial Control hook to
// abort a connection to a disallowed address.
var errBlockedAddr = errors.New("blocked address")

// maintenanceFetchClient fetches a tenant-supplied maintenance-mode uri
// (docs/render-artifacts/maintenance-mode.md). spec.maintenanceMode.uri is
// tenant input and the activator runs with a k8s ServiceAccount token — an
// unrestricted fetch would be SSRF into the cluster (the API server, OpenBao,
// other tenants' pods) or the cloud metadata endpoint. The dialer's Control
// hook runs after DNS resolution, on the actual IP being connected to (so a
// hostname that resolves to a public IP on lookup and a private one at
// connect time — DNS rebinding — is still blocked); redirects are not
// followed, so a 3xx response can't be used to reach a blocked address after
// the initial one passed validation.
var maintenanceFetchClient = &http.Client{
	Timeout:       maintenanceFetchTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: maintenanceFetchTimeout,
			Control: func(_, address string, c syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				ip := net.ParseIP(host)
				if ip == nil || isBlockedMaintenanceFetchIP(ip) {
					return errBlockedAddr
				}
				return nil
			},
		}).DialContext,
	},
}

// isBlockedMaintenanceFetchIP reports whether ip is loopback, private,
// link-local (covers the 169.254.169.254 cloud-metadata address), or
// unspecified — every non-public range a tenant-supplied uri must not reach.
func isBlockedMaintenanceFetchIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// serveMaintenancePage answers a request while spec.maintenanceMode.enabled:
// always 503 (matching Render). Empty uri serves the default page,
// content-negotiated (HTML for browsers, JSON for API clients) like the
// wake-interstitial above. A non-empty uri is fetched and its body/content-type
// streamed through as-is — not a redirect, per
// docs/render-artifacts/maintenance-mode.md — so the visitor sees the tenant's
// own page while the address bar stays on the service's own host. A fetch
// failure (network error or non-2xx) is surfaced to the visitor as 502 rather
// than silently falling back to the default page, matching Render's
// documented behavior.
func serveMaintenancePage(w http.ResponseWriter, r *http.Request, uri string) {
	if uri == "" {
		respondNegotiated(w, r, "60", defaultMaintenancePage, `{"error":"service in maintenance mode"}`)
		return
	}

	resp, err := maintenanceFetchClient.Get(uri)
	if err != nil || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		http.Error(w, "maintenance page unavailable", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Retry-After", "60")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.Copy(w, resp.Body)
}

const loadingPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="5">
  <title>Waking up&hellip;</title>
  <style>
    body{font-family:system-ui,sans-serif;max-width:480px;margin:120px auto;text-align:center;color:#333}
    h1{font-size:1.5rem;margin-bottom:.5rem}
    p{color:#666}
    .s{display:inline-block;width:24px;height:24px;border:3px solid #ddd;border-top-color:#555;
       border-radius:50%;animation:spin 1s linear infinite}
    @keyframes spin{to{transform:rotate(360deg)}}
  </style>
</head>
<body>
  <div class="s"></div>
  <h1>Waking up your app&hellip;</h1>
  <p>This free-tier app was sleeping. It will be ready in a few seconds.</p>
  <p><small>Auto-refreshing in 5&nbsp;seconds&hellip;</small></p>
</body>
</html>`

const defaultMaintenancePage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Down for maintenance</title>
  <style>
    body{font-family:system-ui,sans-serif;max-width:480px;margin:120px auto;text-align:center;color:#333}
    h1{font-size:1.5rem;margin-bottom:.5rem}
    p{color:#666}
  </style>
</head>
<body>
  <h1>Down for maintenance</h1>
  <p>This service is temporarily offline for maintenance. Please check back soon.</p>
</body>
</html>`
