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

// bex-activator is the shared public interstitial responder. For an
// auto-hibernated free-tier App it wakes the Deployment and returns 503 with a
// retry hint. For an App in maintenance mode it serves the default or bounded
// custom maintenance page without touching the App or workload.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/bex-co/bex/lego/types/netutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const annotLastActive = "app.bex.co/last-active"

const (
	customPageTimeout = 5 * time.Second
	customPageMaxSize = 1 << 20 // 1 MiB
	maxRedirects      = 5
)

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

	http.Handle("/", newHandler(c, log))

	log.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "activator: %v\n", err)
		os.Exit(1)
	}
}

func newHandler(c client.Client, log logr.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}

		host := requestHost(r.Host)
		ctx := r.Context()
		app, err := findAppByHost(ctx, c, host)
		if err != nil {
			log.Error(err, "listing apps", "host", host)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		if app == nil {
			log.Info("no routed app found for host", "host", host)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		// Maintenance routing is a public interstitial only. Do not touch the App,
		// Deployment, or wake annotations: its workload remains exactly as it was.
		if app.Spec.MaintenanceMode != nil && app.Spec.MaintenanceMode.Enabled {
			serveMaintenance(w, r, app)
			return
		}

		wakeApp(ctx, c, log, app, host)
		writeWakeResponse(w, r)
	})
}

func wakeApp(ctx context.Context, c client.Client, log logr.Logger, app *appv1alpha1.App, host string) {
	// Touch last-active so the operator's shouldAutoHibernate returns false on
	// the next reconcile, allowing the Ingress to switch back to the app.
	base := app.DeepCopy()
	if app.Annotations == nil {
		app.Annotations = map[string]string{}
	}
	app.Annotations[annotLastActive] = time.Now().UTC().Format(time.RFC3339)
	if err := c.Patch(ctx, app, client.MergeFrom(base)); err != nil {
		log.Error(err, "patching last-active annotation", "app", app.Name)
	}

	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, dep); err == nil {
		one := int32(1)
		depBase := dep.DeepCopy()
		dep.Spec.Replicas = &one
		if err := c.Patch(ctx, dep, client.MergeFrom(depBase)); err != nil {
			log.Error(err, "patching deployment replicas", "app", app.Name)
		}
	}
	log.Info("waking app", "app", app.Name, "host", host)
}

func writeWakeResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "5")
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, loadingPage)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprint(w, `{"error":"service hibernated","retryAfter":5}`)
}

func serveMaintenance(w http.ResponseWriter, r *http.Request, app *appv1alpha1.App) {
	serveMaintenanceWithFetcher(w, r, app, fetchCustomPage)
}

func serveMaintenanceWithFetcher(
	w http.ResponseWriter,
	r *http.Request,
	app *appv1alpha1.App,
	fetch func(context.Context, *appv1alpha1.App, string) (*http.Response, error),
) {
	w.Header().Set("Cache-Control", "no-store")
	if app.Spec.MaintenanceMode.URI == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		if r.Method != http.MethodHead {
			_, _ = io.WriteString(w, maintenancePage)
		}
		return
	}

	resp, err := fetch(r.Context(), app, app.Spec.MaintenanceMode.URI)
	if err != nil {
		http.Error(w, "custom maintenance page unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, customPageMaxSize+1))
	if err != nil || len(body) > customPageMaxSize {
		http.Error(w, "custom maintenance page unavailable", http.StatusBadGateway)
		return
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	status := resp.StatusCode
	if status >= 200 && status < 400 {
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func fetchCustomPage(ctx context.Context, app *appv1alpha1.App, rawURI string) (*http.Response, error) {
	u, err := validateCustomPageURL(rawURI, app)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: customPageTimeout,
		DialContext:           netutil.SafeDialContext(customPageTimeout),
	}
	defer transport.CloseIdleConnections()
	hc := &http.Client{
		Transport:     transport,
		Timeout:       customPageTimeout,
		CheckRedirect: customPageRedirectPolicy(app),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "bex-maintenance-responder/1.0")
	return hc.Do(req)
}

func customPageRedirectPolicy(app *appv1alpha1.App) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errors.New("too many redirects")
		}
		_, err := validateCustomPageURL(req.URL.String(), app)
		return err
	}
}

func validateCustomPageURL(rawURI string, app *appv1alpha1.App) (*url.URL, error) {
	u, err := url.Parse(rawURI)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("maintenance uri must be an absolute HTTP(S) URL")
	}
	if u.User != nil {
		return nil, errors.New("maintenance uri must not contain credentials")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	for owned := range appHosts(app) {
		if host == owned {
			return nil, errors.New("maintenance uri must not point to this service")
		}
	}
	return u, nil
}

func appHosts(app *appv1alpha1.App) map[string]struct{} {
	hosts := make(map[string]struct{})
	add := func(value string) {
		if u, err := url.Parse(value); err == nil && u.Hostname() != "" {
			hosts[strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))] = struct{}{}
			return
		}
		value = requestHost(value)
		if value != "" {
			hosts[value] = struct{}{}
		}
	}
	add(app.Spec.Host)
	for _, host := range app.Spec.Hosts {
		add(host)
	}
	add(app.Status.URL)
	for _, value := range app.Status.URLs {
		add(value)
	}
	return hosts
}

func requestHost(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return strings.ToLower(strings.TrimSuffix(value, "."))
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
		if _, ok := appHosts(app)[requestHost(host)]; ok {
			return app, nil
		}
	}
	return nil, nil
}

const maintenancePage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Under maintenance</title>
  <style>
    body{font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0;background:#111827;color:#f9fafb}
    main{max-width:36rem;padding:2rem;text-align:center}h1{font-size:1.75rem}p{color:#d1d5db;line-height:1.6}
  </style>
</head>
<body><main><h1>This site is currently under maintenance.</h1><p>The owner will restore service as soon as possible. Please try again later.</p></main></body>
</html>`

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
