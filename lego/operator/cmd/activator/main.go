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

// bex-activator is the wake-on-request service for auto-hibernated free-tier
// apps. Traefik routes sleeping-app traffic here; the activator bumps the
// Deployment back to 1 replica (triggering the operator reconcile that switches
// the Ingress back) and returns 503 Retry-After so the client retries once the
// pod is ready.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
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

		// Respond: HTML loading page for browsers, JSON 503 for API clients.
		w.Header().Set("Retry-After", "5")
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, loadingPage)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"error":"service hibernated","retryAfter":5}`)
		}
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
